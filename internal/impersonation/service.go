package impersonation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"keeper/internal/policy"
	"keeper/pkg/auth"
)

// ErrServiceNotRegistered is returned when the requested audience is not a
// registered impersonation service.
var ErrServiceNotRegistered = errors.New("requested service is not registered for impersonation")

// ErrTargetNotFound is returned when the impersonation target user does not exist.
var ErrTargetNotFound = errors.New("target user not found")

// ErrInvalidCode is returned when a handoff code is unknown, already used, or expired.
var ErrInvalidCode = errors.New("invalid or expired code")

// ErrSessionRevoked is returned when exchanging a code for a session that has
// already been revoked.
var ErrSessionRevoked = errors.New("impersonation session revoked")

// ErrPrivilegeEscalation is returned when the target user holds a sudo
// (sysadmin-tier) role — impersonating a sysadmin is never allowed,
// regardless of the impersonator's own privileges.
var ErrPrivilegeEscalation = errors.New("cannot impersonate a user with sysadmin privileges")

// RoleResolver resolves the falcon-assigned role assignments of an arbitrary
// user by id. Used to check the impersonation target's privileges — not the
// caller's, which already arrive resolved on the request JWT.
type RoleResolver interface {
	ResolveRoles(ctx context.Context, appID, userID, divisionID int) ([]auth.RoleAssignment, error)
}

// ImpersonationRepository defines the data access contract.
type ImpersonationRepository interface {
	CreateSession(ctx context.Context, s ImpersonationSession) (*ImpersonationSession, error)
	GetBySessionID(ctx context.Context, sessionID string) (*ImpersonationSession, error)
	GetByID(ctx context.Context, id int) (*ImpersonationSession, error)
	ListActive(ctx context.Context, appID, limit, offset int) ([]*ImpersonationSession, error)
	RevokeBySessionID(ctx context.Context, sessionID string) error
	GetUser(ctx context.Context, id int) (*TargetUser, error)
}

// ImpersonationService defines the business logic for impersonation.
type ImpersonationService interface {
	Start(ctx context.Context, impersonatorUserID int, req StartImpersonationRequest) (*StartImpersonationResponse, error)
	Exchange(ctx context.Context, req ExchangeRequest) (*ExchangeResponse, error)
	Revoke(ctx context.Context, sessionID string) error
	RevokeByID(ctx context.Context, id int) (*ImpersonationSession, error)
	GetByID(ctx context.Context, id int) (*ImpersonationSession, error)
	List(ctx context.Context, appID, limit, offset int) ([]*ImpersonationSession, error)
	IsSessionActive(ctx context.Context, sessionID string) (bool, error)
	Services() []ServiceInfo
}

// pendingCode is the server-side state a one-time handoff code maps to. It
// holds everything needed to mint the token on exchange, so the token itself is
// never created until the target UI redeems the code.
type pendingCode struct {
	target       TargetUser
	sessionID    string
	audience     string
	impersonator int
	readOnly     bool
	expiresAt    time.Time
}

type impersonationService struct {
	repo         ImpersonationRepository
	roleResolver RoleResolver
	policyStore  *policy.Store
	impJWT       *auth.JWTManager
	impExpiry    time.Duration
	codeTTL      time.Duration
	services     []ServiceInfo
	audiences    map[string]bool

	// codes holds outstanding one-time handoff codes (in-memory, single-use).
	codesMu sync.Mutex
	codes   map[string]pendingCode

	// revoked is a fast in-memory denylist of revoked session ids so revocation
	// is enforced without a DB round-trip on the hot path.
	revokedMu sync.RWMutex
	revoked   map[string]struct{}
}

// NewImpersonationService creates the service. impJWT MUST be constructed with
// the dedicated impersonation secret. services is the registered service
// registry; its audiences form the set a session may target. roleResolver and
// policyStore resolve and check the impersonation *target's* privileges (the
// caller's own are already resolved on the request JWT) — see Start's
// privilege-escalation guard.
func NewImpersonationService(repo ImpersonationRepository, roleResolver RoleResolver, policyStore *policy.Store, impJWT *auth.JWTManager, impExpiry, codeTTL time.Duration, services []ServiceInfo) ImpersonationService {
	audiences := make(map[string]bool, len(services))
	for _, s := range services {
		audiences[s.Audience] = true
	}
	return &impersonationService{
		repo:         repo,
		roleResolver: roleResolver,
		policyStore:  policyStore,
		impJWT:       impJWT,
		impExpiry:    impExpiry,
		codeTTL:      codeTTL,
		services:     services,
		audiences:    audiences,
		codes:        make(map[string]pendingCode),
		revoked:      make(map[string]struct{}),
	}
}

// Services returns the registered impersonation target services.
func (s *impersonationService) Services() []ServiceInfo {
	return s.services
}

func (s *impersonationService) Start(ctx context.Context, impersonatorUserID int, req StartImpersonationRequest) (*StartImpersonationResponse, error) {
	slog.Info("starting impersonation session", "impersonator", impersonatorUserID, "target_user_id", req.TargetUserID, "audience", req.Audience)

	if !s.audiences[req.Audience] {
		return nil, ErrServiceNotRegistered
	}

	target, err := s.repo.GetUser(ctx, req.TargetUserID)
	if err != nil {
		return nil, ErrTargetNotFound
	}

	// Privilege-escalation guard: never allow impersonating a sysadmin,
	// regardless of who is asking. The target's roles aren't on any JWT
	// (they belong to a different user than the caller), so resolve them
	// fresh from falcon and check for a sudo grant.
	targetRoles, err := s.roleResolver.ResolveRoles(ctx, target.AppID, target.ID, target.DivisionID)
	if err != nil {
		slog.Error("impersonation start failed: target role resolution unavailable", "target_user_id", target.ID, "error", err)
		return nil, fmt.Errorf("resolve target roles: %w", err)
	}
	policies := s.policyStore.Policies(ctx)
	for _, role := range targetRoles {
		if policies[role.Name].IsSudo {
			slog.Warn("impersonation refused: target holds a sudo role", "impersonator", impersonatorUserID, "target_user_id", target.ID, "role", role.Name)
			return nil, ErrPrivilegeEscalation
		}
	}

	sessionID, err := generateID("imps_")
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	code, err := generateID("impc_")
	if err != nil {
		return nil, fmt.Errorf("generate code: %w", err)
	}

	expiresAt := time.Now().Add(s.impExpiry)

	if _, err := s.repo.CreateSession(ctx, ImpersonationSession{
		SessionID:          sessionID,
		AppID:              target.AppID,
		DivisionID:         target.DivisionID,
		ImpersonatorUserID: impersonatorUserID,
		TargetUserID:       target.ID,
		Audience:           req.Audience,
		ReadOnly:           req.ReadOnly,
		Reason:             req.Reason,
		Status:             1,
		ExpiresAt:          expiresAt,
	}); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	s.codesMu.Lock()
	s.codes[code] = pendingCode{
		target:       *target,
		sessionID:    sessionID,
		audience:     req.Audience,
		impersonator: impersonatorUserID,
		readOnly:     req.ReadOnly,
		expiresAt:    time.Now().Add(s.codeTTL),
	}
	s.codesMu.Unlock()

	slog.Info("impersonation session created", "session_id", sessionID, "impersonator", impersonatorUserID, "target_user_id", target.ID, "audience", req.Audience, "read_only", req.ReadOnly)

	return &StartImpersonationResponse{
		Code:      code,
		SessionID: sessionID,
		Audience:  req.Audience,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *impersonationService) Exchange(ctx context.Context, req ExchangeRequest) (*ExchangeResponse, error) {
	// Burn the code: look up and delete atomically so it can only be redeemed once.
	s.codesMu.Lock()
	pc, ok := s.codes[req.Code]
	if ok {
		delete(s.codes, req.Code)
	}
	s.codesMu.Unlock()

	if !ok || time.Now().After(pc.expiresAt) {
		slog.Warn("impersonation exchange with invalid or expired code")
		return nil, ErrInvalidCode
	}

	// A session revoked between mint and exchange must not yield a token.
	if s.isRevoked(pc.sessionID) {
		return nil, ErrSessionRevoked
	}

	jti, err := generateID("impj_")
	if err != nil {
		return nil, fmt.Errorf("generate jti: %w", err)
	}

	token, err := s.impJWT.GenerateImpersonation(auth.ImpersonationParams{
		AppID:        pc.target.AppID,
		UserID:       pc.target.ID,
		DivisionID:   pc.target.DivisionID,
		Impersonator: pc.impersonator,
		Audience:     pc.audience,
		SessionID:    pc.sessionID,
		JTI:          jti,
		ReadOnly:     pc.readOnly,
	})
	if err != nil {
		slog.Error("failed to mint impersonation token", "session_id", pc.sessionID, "error", err)
		return nil, fmt.Errorf("mint impersonation token: %w", err)
	}

	slog.Info("impersonation token issued", "session_id", pc.sessionID, "target_user_id", pc.target.ID, "audience", pc.audience)

	return &ExchangeResponse{
		Token:     token,
		User:      pc.target,
		Audience:  pc.audience,
		SessionID: pc.sessionID,
		ExpiresAt: time.Now().Add(s.impExpiry),
	}, nil
}

func (s *impersonationService) Revoke(ctx context.Context, sessionID string) error {
	slog.Info("revoking impersonation session", "session_id", sessionID)
	if err := s.repo.RevokeBySessionID(ctx, sessionID); err != nil {
		return err
	}
	s.addRevoked(sessionID)
	return nil
}

func (s *impersonationService) RevokeByID(ctx context.Context, id int) (*ImpersonationSession, error) {
	sess, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.Revoke(ctx, sess.SessionID); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *impersonationService) GetByID(ctx context.Context, id int) (*ImpersonationSession, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *impersonationService) List(ctx context.Context, appID, limit, offset int) ([]*ImpersonationSession, error) {
	return s.repo.ListActive(ctx, appID, limit, offset)
}

// IsSessionActive reports whether a session is still valid: not in the
// in-memory denylist, present, active, and unexpired.
func (s *impersonationService) IsSessionActive(ctx context.Context, sessionID string) (bool, error) {
	if s.isRevoked(sessionID) {
		return false, nil
	}
	sess, err := s.repo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return false, nil
	}
	if sess.Status != 1 || time.Now().After(sess.ExpiresAt) {
		return false, nil
	}
	return true, nil
}

func (s *impersonationService) isRevoked(sessionID string) bool {
	s.revokedMu.RLock()
	defer s.revokedMu.RUnlock()
	_, ok := s.revoked[sessionID]
	return ok
}

func (s *impersonationService) addRevoked(sessionID string) {
	s.revokedMu.Lock()
	defer s.revokedMu.Unlock()
	s.revoked[sessionID] = struct{}{}
}

// generateID returns prefix + 48 hex chars of crypto/rand entropy.
func generateID(prefix string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
