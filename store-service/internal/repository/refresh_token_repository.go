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

type RefreshTokenRepository interface {
	Save(ctx context.Context, tokenID string, userID uuid.UUID, ttl time.Duration) error
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
	return r.cache.Set(ctx, fmt.Sprintf(constant.KeyRefreshToken, tokenID), userID.String(), ttl)
}

func (r *refreshTokenRepository) Consume(ctx context.Context, tokenID string) (uuid.UUID, error) {
	var userID string

	data, err := r.cache.Take(ctx, fmt.Sprintf(constant.KeyRefreshToken, tokenID))
	if errors.Is(err, caches.ErrMiss) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}

	if err := json.Unmarshal(data, &userID); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(userID)
}

func (r *refreshTokenRepository) Revoke(ctx context.Context, tokenID string) error {
	return r.cache.Delete(ctx, fmt.Sprintf(constant.KeyRefreshToken, tokenID))
}
