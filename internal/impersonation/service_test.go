package impersonation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"keeper/internal/policy"
	"keeper/pkg/auth"
)

// mockRoleResolver returns pre-seeded roles per user id; missing ids resolve
// to no roles (ordinary user).
type mockRoleResolver struct {
	roles map[int][]auth.RoleAssignment
}

func (m *mockRoleResolver) ResolveRoles(_ context.Context, _, userID, _ int) ([]auth.RoleAssignment, error) {
	return m.roles[userID], nil
}

// mockRepo is an in-memory ImpersonationRepository for tests.
type mockRepo struct {
	mu       sync.Mutex
	users    map[int]*TargetUser
	sessions map[string]*ImpersonationSession
	nextID   int
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:    map[int]*TargetUser{},
		sessions: map[string]*ImpersonationSession{},
	}
}

func (m *mockRepo) CreateSession(_ context.Context, s ImpersonationSession) (*ImpersonationSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	s.ID = m.nextID
	s.CreatedAt = time.Now()
	cp := s
	m.sessions[s.SessionID] = &cp
	return &cp, nil
}

func (m *mockRepo) GetBySessionID(_ context.Context, sessionID string) (*ImpersonationSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}
	return nil, errors.New("not found")
}

func (m *mockRepo) GetByID(_ context.Context, id int) (*ImpersonationSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockRepo) ListActive(_ context.Context, appID, limit, offset int) ([]*ImpersonationSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*ImpersonationSession
	for _, s := range m.sessions {
		if s.Status == 1 && (appID == 0 || s.AppID == appID) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *mockRepo) RevokeBySessionID(_ context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		s.Status = 0
		now := time.Now()
		s.RevokedAt = &now
	}
	return nil
}

func (m *mockRepo) GetUser(_ context.Context, id int) (*TargetUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("user not found")
}

// testPolicyStore compiles a "sysadmin" sudo role, mirroring falcon's export
// shape, for the privilege-escalation guard to check target roles against.
var testPolicyStore = policy.NewStoreFromPolicies(policy.Compile([]policy.Row{
	{Role: "sysadmin", IsSudo: true},
}))

func newService(repo *mockRepo) (ImpersonationService, *auth.JWTManager) {
	return newServiceWithRoles(repo, nil)
}

// newServiceWithRoles builds a service whose roleResolver resolves the given
// target-user-id -> roles map (empty/nil means every target is an ordinary,
// non-sudo user).
func newServiceWithRoles(repo *mockRepo, roles map[int][]auth.RoleAssignment) (ImpersonationService, *auth.JWTManager) {
	mgr := auth.NewJWTManager("imp-secret", 10*time.Minute)
	resolver := &mockRoleResolver{roles: roles}
	svc := NewImpersonationService(repo, resolver, testPolicyStore, mgr, 10*time.Minute, time.Minute, []ServiceInfo{{Key: "squirrel", Audience: "squirrel", UIExchangeURL: "http://localhost/x"}})
	return svc, mgr
}

func TestStartRejectsUnregisteredAudience(t *testing.T) {
	repo := newMockRepo()
	repo.users[42] = &TargetUser{ID: 42, AppID: 3, DivisionID: 7}
	svc, _ := newService(repo)

	_, err := svc.Start(context.Background(), 1, StartImpersonationRequest{TargetUserID: 42, Audience: "ant"})
	if !errors.Is(err, ErrServiceNotRegistered) {
		t.Fatalf("expected ErrServiceNotRegistered, got %v", err)
	}
}

func TestStartRejectsMissingTarget(t *testing.T) {
	repo := newMockRepo()
	svc, _ := newService(repo)

	_, err := svc.Start(context.Background(), 1, StartImpersonationRequest{TargetUserID: 7, Audience: "squirrel"})
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("expected ErrTargetNotFound, got %v", err)
	}
}

func TestStartRefusesSysadminTarget(t *testing.T) {
	repo := newMockRepo()
	repo.users[42] = &TargetUser{ID: 42, AppID: 3, DivisionID: 7}
	svc, _ := newServiceWithRoles(repo, map[int][]auth.RoleAssignment{42: {{Name: "sysadmin"}}})

	_, err := svc.Start(context.Background(), 1, StartImpersonationRequest{TargetUserID: 42, Audience: "squirrel"})
	if !errors.Is(err, ErrPrivilegeEscalation) {
		t.Fatalf("expected ErrPrivilegeEscalation, got %v", err)
	}
}

func TestStartExchangeMintsScopedToken(t *testing.T) {
	repo := newMockRepo()
	repo.users[42] = &TargetUser{ID: 42, AppID: 3, DivisionID: 7, Email: "u@x.com"}
	svc, mgr := newService(repo)

	start, err := svc.Start(context.Background(), 1, StartImpersonationRequest{TargetUserID: 42, Audience: "squirrel", Reason: "support"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	ex, err := svc.Exchange(context.Background(), ExchangeRequest{Code: start.Code})
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if ex.User.ID != 42 {
		t.Errorf("expected user 42, got %d", ex.User.ID)
	}

	claims, err := mgr.VerifyWithAudience(ex.Token, "squirrel")
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if !claims.IsImpersonating() || claims.UserID != 42 || claims.Impersonator != 1 {
		t.Errorf("claims wrong: %+v", claims)
	}
	if claims.SessionID != start.SessionID {
		t.Errorf("sid mismatch: %q vs %q", claims.SessionID, start.SessionID)
	}
	// Minted for squirrel must not verify against ant.
	if _, err := mgr.VerifyWithAudience(ex.Token, "ant"); err == nil {
		t.Error("expected audience mismatch against ant")
	}
}

func TestExchangeCodeIsSingleUse(t *testing.T) {
	repo := newMockRepo()
	repo.users[42] = &TargetUser{ID: 42, AppID: 3, DivisionID: 7}
	svc, _ := newService(repo)

	start, _ := svc.Start(context.Background(), 1, StartImpersonationRequest{TargetUserID: 42, Audience: "squirrel"})
	if _, err := svc.Exchange(context.Background(), ExchangeRequest{Code: start.Code}); err != nil {
		t.Fatalf("first exchange: %v", err)
	}
	if _, err := svc.Exchange(context.Background(), ExchangeRequest{Code: start.Code}); !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode on reuse, got %v", err)
	}
}

func TestRevokeBlocksExchangeAndStatus(t *testing.T) {
	repo := newMockRepo()
	repo.users[42] = &TargetUser{ID: 42, AppID: 3, DivisionID: 7}
	svc, _ := newService(repo)

	start, _ := svc.Start(context.Background(), 1, StartImpersonationRequest{TargetUserID: 42, Audience: "squirrel"})

	if active, _ := svc.IsSessionActive(context.Background(), start.SessionID); !active {
		t.Fatal("expected session active before revoke")
	}

	if err := svc.Revoke(context.Background(), start.SessionID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	if active, _ := svc.IsSessionActive(context.Background(), start.SessionID); active {
		t.Error("expected session inactive after revoke")
	}
	if _, err := svc.Exchange(context.Background(), ExchangeRequest{Code: start.Code}); !errors.Is(err, ErrSessionRevoked) {
		t.Errorf("expected ErrSessionRevoked, got %v", err)
	}
}
