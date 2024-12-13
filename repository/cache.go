package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
	goredis "github.com/redis/go-redis/v9"
)

type Cache interface {
	CreateAccessToken(ctx context.Context, accessToken profile.AccessToken) error
	CreateRefreshToken(ctx context.Context, refreshToken profile.RefreshToken) error
	DeleteToken(ctx context.Context, userID, sessionID, role string, tokenType profile.TokenType) error
}

type cache struct {
	db                     *goredis.Client
	accessTokenExpireTime  time.Duration
	refreshTokenExpireTime time.Duration
}

func NewCacheRepository(client *goredis.Client, accessTokenExpireTime, refreshTokenExpireTime time.Duration) Cache {
	return &cache{
		db:                     client,
		accessTokenExpireTime:  accessTokenExpireTime,
		refreshTokenExpireTime: refreshTokenExpireTime,
	}
}

func (tr *cache) CreateAccessToken(ctx context.Context, accessToken profile.AccessToken) error {
	key := fmt.Sprintf("%s:%s:%s:%s:%s", profile.ApplicationPrefix, profile.AccessTokenPrefix, accessToken.Role, accessToken.UserID, accessToken.SessionID)
	value := fmt.Sprintf("%d:%d", accessToken.IssuedAt, accessToken.ExpiresAt)
	err := tr.db.Set(ctx, key, value, tr.accessTokenExpireTime).Err()
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	return nil
}

func (tr *cache) CreateRefreshToken(ctx context.Context, refreshToken profile.RefreshToken) error {
	key := fmt.Sprintf("%s:%s:%s:%s:%s", profile.ApplicationPrefix, profile.RefreshTokenPrefix, refreshToken.Role, refreshToken.UserID, refreshToken.SessionID)
	value := fmt.Sprintf("%d:%d", refreshToken.IssuedAt, refreshToken.ExpiresAt)
	err := tr.db.Set(ctx, key, value, tr.refreshTokenExpireTime).Err()
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	return nil
}

func (tr *cache) DeleteToken(ctx context.Context, userID, sessionID, role string, tokenType profile.TokenType) error {
	prefix := profile.AccessTokenPrefix
	if tokenType == profile.Refresh {
		prefix = profile.RefreshTokenPrefix
	}
	key := fmt.Sprintf("%s:%s:%s:%s:%s", profile.ApplicationPrefix, prefix, role, userID, sessionID)
	_, err := tr.db.Del(ctx, key).Result()
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	return nil
}
