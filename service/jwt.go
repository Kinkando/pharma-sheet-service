package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/kinkando/pharma-sheet/pkg/generator"
	"github.com/kinkando/pharma-sheet/pkg/profile"
	"github.com/mitchellh/mapstructure"
)

type JWTService interface {
	EncodeJWT(ctx context.Context, userID string, role profile.Role) (accessTokenClaim profile.AccessToken, refreshTokenClaim profile.RefreshToken, err error)
	SignedJWT(ctx context.Context, jwtClaim jwt.Claims) (token string, err error)
	DecodeAccessToken(ctx context.Context, token string) (accessToken profile.AccessToken, err error)
	DecodeRefreshToken(ctx context.Context, token string) (refreshToken profile.RefreshToken, err error)
}

type jwtService struct {
	jwtSecretKey           string
	accessTokenExpireTime  time.Duration
	refreshTokenExpireTime time.Duration
}

func NewJWTService(jwtSecretKey string, accessTokenExpireTime, refreshTokenExpireTime time.Duration) JWTService {
	return &jwtService{
		jwtSecretKey:           jwtSecretKey,
		accessTokenExpireTime:  accessTokenExpireTime,
		refreshTokenExpireTime: refreshTokenExpireTime,
	}
}

func (s *jwtService) EncodeJWT(ctx context.Context, userID string, role profile.Role) (accessTokenClaim profile.AccessToken, refreshTokenClaim profile.RefreshToken, err error) {
	now := time.Now()
	sessionID := generator.UUID()

	accessTokenClaim = profile.AccessToken{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: now.Add(s.accessTokenExpireTime).Unix(),
			IssuedAt:  now.Unix(),
		},
		UserID:    userID,
		SessionID: sessionID,
		Role:      role,
		Type:      profile.Access,
	}

	refreshTokenClaim = profile.RefreshToken{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: now.Add(s.refreshTokenExpireTime).Unix(),
			IssuedAt:  now.Unix(),
		},
		UserID:    userID,
		SessionID: sessionID,
		Role:      role,
		Type:      profile.Refresh,
	}
	return
}

func (s *jwtService) SignedJWT(ctx context.Context, jwtClaim jwt.Claims) (signedToken string, err error) {
	unsignedRefreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaim)
	signedToken, err = unsignedRefreshToken.SignedString([]byte(s.jwtSecretKey))
	if err != nil {
		return
	}
	return
}

func (s *jwtService) DecodeAccessToken(ctx context.Context, jwtToken string) (accessToken profile.AccessToken, err error) {
	jwtClaims, err := s.decodeJWT(jwtToken)
	if err != nil {
		return
	}

	err = mapstructure.Decode(jwtClaims, &accessToken)
	if err != nil {
		return profile.AccessToken{}, fmt.Errorf(`invalid token structure: %v`, err)
	}
	return
}

func (s *jwtService) DecodeRefreshToken(ctx context.Context, jwtToken string) (refreshToken profile.RefreshToken, err error) {
	jwtClaims, err := s.decodeJWT(jwtToken)
	if err != nil {
		return
	}

	err = mapstructure.Decode(jwtClaims, &refreshToken)
	if err != nil {
		return profile.RefreshToken{}, fmt.Errorf(`invalid token structure: %v`, err)
	}
	return
}

func (s *jwtService) decodeJWT(jwtToken string) (jwt.MapClaims, error) {
	jwtClaims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(jwtToken, jwtClaims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("error parsing jwt token: %v", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid jwt token")
	}

	jwtClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf(`unable to map token to map claims: %v`, err)
	}

	return jwtClaims, nil
}
