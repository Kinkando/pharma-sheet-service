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
	ExistsToken(ctx context.Context, userID, sessionID string, tokenType profile.TokenType) (bool, error)
	CreateAccessToken(ctx context.Context, accessToken profile.AccessToken) error
	CreateRefreshToken(ctx context.Context, refreshToken profile.RefreshToken) error
	DeleteToken(ctx context.Context, userID, sessionID string, tokenType profile.TokenType) error
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

func (r *cache) ExistsToken(ctx context.Context, userID, sessionID string, tokenType profile.TokenType) (bool, error) {
	prefix := profile.AccessTokenPrefix
	if tokenType == profile.Refresh {
		prefix = profile.RefreshTokenPrefix
	}

	key := fmt.Sprintf("%s:%s:%s:%s", profile.ApplicationPrefix, prefix, userID, sessionID)
	result, err := r.db.Exists(ctx, key).Result()
	if err != nil {
		logger.Context(ctx).Error(err)
		return false, err
	}

	return result > 0, nil
}

func (r *cache) CreateAccessToken(ctx context.Context, accessToken profile.AccessToken) error {
	key := fmt.Sprintf("%s:%s:%s:%s", profile.ApplicationPrefix, profile.AccessTokenPrefix, accessToken.UserID, accessToken.SessionID)
	value := fmt.Sprintf("%d:%d", accessToken.IssuedAt, accessToken.ExpiresAt)
	err := r.db.Set(ctx, key, value, r.accessTokenExpireTime).Err()
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	return nil
}

func (r *cache) CreateRefreshToken(ctx context.Context, refreshToken profile.RefreshToken) error {
	key := fmt.Sprintf("%s:%s:%s:%s", profile.ApplicationPrefix, profile.RefreshTokenPrefix, refreshToken.UserID, refreshToken.SessionID)
	value := fmt.Sprintf("%d:%d", refreshToken.IssuedAt, refreshToken.ExpiresAt)
	err := r.db.Set(ctx, key, value, r.refreshTokenExpireTime).Err()
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	return nil
}

func (r *cache) DeleteToken(ctx context.Context, userID, sessionID string, tokenType profile.TokenType) error {
	prefix := profile.AccessTokenPrefix
	if tokenType == profile.Refresh {
		prefix = profile.RefreshTokenPrefix
	}
	key := fmt.Sprintf("%s:%s:%s:%s", profile.ApplicationPrefix, prefix, userID, sessionID)
	_, err := r.db.Del(ctx, key).Result()
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}
	return nil
}
