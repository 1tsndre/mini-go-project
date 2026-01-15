package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client *redis.Client
}

func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

func (rl *RateLimiter) Limit(limit int, window time.Duration, keyType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identifier, _, _ := net.SplitHostPort(r.RemoteAddr)
			if identifier == "" {
				identifier = r.RemoteAddr
			}
			if userID := GetUserID(r.Context()); userID != "" {
				identifier = userID
			}

			key := fmt.Sprintf(constant.KeyRateLimit, keyType, identifier)
			allowed, remaining, resetAt, err := rl.allow(r.Context(), key, limit, window)
			if err != nil {
				// If Redis fails, allow the request
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

			if !allowed {
				meta := BuildMeta(r)
				response.ErrorResponse(w, http.StatusTooManyRequests, meta,
					response.NewError(constant.ErrCodeRateLimited, "rate limit exceeded"),
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
	now := time.Now().UnixMilli()
	windowStart := now - window.Milliseconds()
	resetAt := time.Now().Add(window)

	pipe := rl.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
	countCmd := pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, limit, resetAt, err
	}

	count := int(countCmd.Val())
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	return count <= limit, remaining, resetAt, nil
}
