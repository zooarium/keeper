package impersonation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"keeper/ent"
	entsession "keeper/ent/impersonationsession"
	entuser "keeper/ent/user"
)

type impersonationRepository struct {
	client *ent.Client
}

// NewImpersonationRepository creates a new impersonation repository.
func NewImpersonationRepository(client *ent.Client) *impersonationRepository {
	return &impersonationRepository{client: client}
}

// CreateSession persists a new impersonation audit record.
func (r *impersonationRepository) CreateSession(ctx context.Context, s ImpersonationSession) (*ImpersonationSession, error) {
	created, err := r.client.ImpersonationSession.
		Create().
		SetSessionID(s.SessionID).
		SetAppID(s.AppID).
		SetDivisionID(s.DivisionID).
		SetImpersonatorUserID(s.ImpersonatorUserID).
		SetTargetUserID(s.TargetUserID).
		SetAudience(s.Audience).
		SetReadOnly(s.ReadOnly).
		SetReason(s.Reason).
		SetStatus(s.Status).
		SetExpiresAt(s.ExpiresAt).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to create impersonation session", "target_user_id", s.TargetUserID, "impersonator", s.ImpersonatorUserID, "error", err)
		return nil, err
	}
	return r.mapToModel(created), nil
}

// GetBySessionID returns the session matching the opaque session id.
func (r *impersonationRepository) GetBySessionID(ctx context.Context, sessionID string) (*ImpersonationSession, error) {
	s, err := r.client.ImpersonationSession.Query().
		Where(entsession.SessionID(sessionID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("impersonation session not found: %w", err)
		}
		slog.Error("database error: failed to get impersonation session by session id", "error", err)
		return nil, err
	}
	return r.mapToModel(s), nil
}

// GetByID returns the session by primary key.
func (r *impersonationRepository) GetByID(ctx context.Context, id int) (*ImpersonationSession, error) {
	s, err := r.client.ImpersonationSession.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("impersonation session not found: %w", err)
		}
		slog.Error("database error: failed to get impersonation session by id", "id", id, "error", err)
		return nil, err
	}
	return r.mapToModel(s), nil
}

// ListActive returns active sessions; appID > 0 restricts to that app.
func (r *impersonationRepository) ListActive(ctx context.Context, appID, limit, offset int) ([]*ImpersonationSession, error) {
	q := r.client.ImpersonationSession.Query().
		Where(entsession.Status(1))
	if appID > 0 {
		q = q.Where(entsession.AppID(appID))
	}
	sessions, err := q.
		Order(ent.Desc(entsession.FieldID)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		slog.Error("database error: failed to list impersonation sessions", "app_id", appID, "error", err)
		return nil, err
	}
	result := make([]*ImpersonationSession, len(sessions))
	for i, s := range sessions {
		result[i] = r.mapToModel(s)
	}
	return result, nil
}

// RevokeBySessionID marks the session revoked. Idempotent: revoking an already
// revoked or missing session is not an error for the caller's purposes.
func (r *impersonationRepository) RevokeBySessionID(ctx context.Context, sessionID string) error {
	now := time.Now()
	n, err := r.client.ImpersonationSession.Update().
		Where(entsession.SessionID(sessionID), entsession.Status(1)).
		SetStatus(0).
		SetRevokedAt(now).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to revoke impersonation session", "session_id", sessionID, "error", err)
		return err
	}
	slog.Info("impersonation session revoked", "session_id", sessionID, "rows", n)
	return nil
}

// GetUser fetches the minimal target user snapshot, selecting only the columns
// needed to mint a token and populate the UI user object.
func (r *impersonationRepository) GetUser(ctx context.Context, id int) (*TargetUser, error) {
	u, err := r.client.User.Query().
		Where(entuser.ID(id)).
		Select(
			entuser.FieldID,
			entuser.FieldAppID,
			entuser.FieldDivisionID,
			entuser.FieldFirstname,
			entuser.FieldLastname,
			entuser.FieldEmail,
			entuser.FieldStatus,
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		slog.Error("database error: failed to get impersonation target user", "id", id, "error", err)
		return nil, err
	}
	return &TargetUser{
		ID:         u.ID,
		AppID:      u.AppID,
		DivisionID: u.DivisionID,
		Firstname:  u.Firstname,
		Lastname:   u.Lastname,
		Email:      u.Email,
		Status:     u.Status,
	}, nil
}

func (r *impersonationRepository) mapToModel(s *ent.ImpersonationSession) *ImpersonationSession {
	m := &ImpersonationSession{
		ID:                 s.ID,
		SessionID:          s.SessionID,
		AppID:              s.AppID,
		DivisionID:         s.DivisionID,
		ImpersonatorUserID: s.ImpersonatorUserID,
		TargetUserID:       s.TargetUserID,
		Audience:           s.Audience,
		ReadOnly:           s.ReadOnly,
		Reason:             s.Reason,
		Status:             s.Status,
		CreatedAt:          s.CreatedAt,
		ExpiresAt:          s.ExpiresAt,
		RevokedAt:          s.RevokedAt,
	}
	return m
}
