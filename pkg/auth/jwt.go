package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTManager handles generation and validation of JWT tokens.
type JWTManager struct {
	secretKey     string
	tokenDuration time.Duration
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	return &JWTManager{secretKey, tokenDuration}
}

// UserClaims is a custom JWT claims that contains user's information.
type UserClaims struct {
	jwt.RegisteredClaims
	AppID      int `json:"app_id"`
	UserID     int `json:"user_id"`
	DivisionID int `json:"division_id"`
	Role       int `json:"role"`
}

// IsSysAdmin returns true when the claims carry sysadmin role.
func (c *UserClaims) IsSysAdmin() bool {
	return c.Role == 1
}

// Generate generates and signs a new token for a user.
func (manager *JWTManager) Generate(appID, userID, divisionID, role int) (string, error) {
	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.tokenDuration)),
		},
		AppID:      appID,
		UserID:     userID,
		DivisionID: divisionID,
		Role:       role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

// Verify verifies the access token string and return a user claims if the token is valid.
func (manager *JWTManager) Verify(accessToken string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(
		accessToken,
		&UserClaims{},
		func(token *jwt.Token) (interface{}, error) {
			_, ok := token.Method.(*jwt.SigningMethodHMAC)
			if !ok {
				return nil, fmt.Errorf("unexpected token signing method")
			}

			return []byte(manager.secretKey), nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
