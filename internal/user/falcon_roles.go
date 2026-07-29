package user

import (
	"context"
	"fmt"
	"net/http"

	"keeper/pkg/auth"
	"keeper/pkg/s2s"
)

// RoleResolver resolves a user's falcon-assigned role assignments at login
// time. Login fails closed on any error — see Service.Authenticate.
type RoleResolver interface {
	ResolveRoles(ctx context.Context, appID, userID, divisionID, role int) ([]auth.RoleAssignment, error)
}

// falconRoleResolver calls falcon's internal-s2s /user-roles endpoint. It has
// no incoming JWT to forward at login time, so it self-signs a short-lived
// token with jwt (keeper's own JWTManager, same AUTH.JWT_SECRET falcon's
// internal-s2s listener verifies against).
type falconRoleResolver struct {
	client *s2s.Client
	jwt    *auth.JWTManager
}

// NewFalconRoleResolver builds a RoleResolver bound to falcon's internal-s2s
// base URL. httpClient must carry a non-zero timeout (keeper/pkg/httpclient).
func NewFalconRoleResolver(httpClient *http.Client, falconBaseURL string, jwt *auth.JWTManager) RoleResolver {
	return &falconRoleResolver{client: s2s.New(httpClient, falconBaseURL), jwt: jwt}
}

type falconUserRole struct {
	RoleName  string `json:"role_name"`
	ServiceID int    `json:"service_id"`
	AppID     *int   `json:"app_id"`
}

func (f *falconRoleResolver) ResolveRoles(ctx context.Context, appID, userID, divisionID, role int) ([]auth.RoleAssignment, error) {
	token, err := f.jwt.Generate(appID, userID, divisionID, role)
	if err != nil {
		return nil, fmt.Errorf("sign s2s token: %w", err)
	}

	var assignments []falconUserRole
	if err := f.client.GetAuth(ctx, fmt.Sprintf("/user-roles?user_id=%d&limit=500", userID), token, &assignments); err != nil {
		return nil, fmt.Errorf("falcon user-roles: %w", err)
	}

	roles := make([]auth.RoleAssignment, len(assignments))
	for i, a := range assignments {
		roles[i] = auth.RoleAssignment{Name: a.RoleName, ServiceID: a.ServiceID, AppID: a.AppID}
	}
	return roles, nil
}
