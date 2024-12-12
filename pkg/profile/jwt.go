package profile

import (
	"github.com/golang-jwt/jwt"
)

type TokenType string
type PrefixToken string

const (
	Access  TokenType = "access"
	Refresh TokenType = "refresh"

	AccessTokenPrefix  = "ACCESS_TOKEN"
	RefreshTokenPrefix = "REFRESH_TOKEN"
)

type AccessToken struct {
	jwt.StandardClaims
	UserID    string    `json:"userID"`
	SessionID string    `json:"sessionID"`
	Role      Role      `json:"role"`
	Type      TokenType `json:"type"`
}

type RefreshToken struct {
	jwt.StandardClaims
	UserID    string    `json:"userID"`
	SessionID string    `json:"sessionID"`
	Role      Role      `json:"role"`
	Type      TokenType `json:"type"`
}
