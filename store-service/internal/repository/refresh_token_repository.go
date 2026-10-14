package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
	"github.com/google/uuid"
)

// RefreshTokenRepository keeps the refresh tokens that may still be used, by jti,
// so a refresh token can be used only once and revoked on logout.
type RefreshTokenRepository interface {
	// Save stores the token for userID until ttl passes.
	Save(ctx context.Context, tokenID string, userID uuid.UUID, ttl time.Duration) error
	// Consume removes the token and returns its user. It returns ErrNotFound for a
	// token that was never stored, has expired, or was already used or revoked.
	Consume(ctx context.Context, tokenID string) (uuid.UUID, error)
	Revoke(ctx context.Context, tokenID string) error
}

type refreshTokenRepository struct {
	cache caches.Cache
}

func NewRefreshTokenRepository(cache caches.Cache) RefreshTokenRepository {
	return &refreshTokenRepository{cache: cache}
}

func (r *refreshTokenRepository) Save(ctx context.Context, tokenID string, userID uuid.UUID, ttl time.Duration) error {
	return r.cache.Set(ctx, refreshTokenKey(tokenID), userID.String(), ttl)
}

func (r *refreshTokenRepository) Consume(ctx context.Context, tokenID string) (uuid.UUID, error) {
	data, err := r.cache.Take(ctx, refreshTokenKey(tokenID))
	if errors.Is(err, caches.ErrMiss) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	var userID string
	if err := json.Unmarshal(data, &userID); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(userID)
}

func (r *refreshTokenRepository) Revoke(ctx context.Context, tokenID string) error {
	return r.cache.Delete(ctx, refreshTokenKey(tokenID))
}

func refreshTokenKey(tokenID string) string {
	return fmt.Sprintf(constant.KeyRefreshToken, tokenID)
}
