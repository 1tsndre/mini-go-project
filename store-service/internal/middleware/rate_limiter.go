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
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client *redis.Client
}

func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

func (rl *RateLimiter) Limit(limit int, keyType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count, err := rl.count(r.Context(), fmt.Sprintf(constant.KeyRateLimit, keyType, clientID(r)))
			if err != nil {
				// Login and register share this key; logins need Redis anyway, so keep password guessing out.
				if keyType == constant.RateLimitKeyLogin {
					response.Failure(w, http.StatusServiceUnavailable, BuildMeta(r), response.Error{Code: constant.ErrCodeInternal, Message: constant.MsgAuthUnavailable})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set(constant.HeaderRateLimitLimit, strconv.Itoa(limit))
			w.Header().Set(constant.HeaderRateLimitRemaining, strconv.Itoa(max(limit-count, 0)))
			w.Header().Set(constant.HeaderRateLimitReset, strconv.FormatInt(time.Now().Add(constant.RateLimitWindow).Unix(), 10))

			if count > limit {
				response.Failure(w, http.StatusTooManyRequests, BuildMeta(r), response.Error{Code: constant.ErrCodeRateLimited, Message: constant.MsgRateLimitExceeded})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// count adds this request to the sliding window and returns how many it holds.
func (rl *RateLimiter) count(ctx context.Context, key string) (int, error) {
	var (
		now         = time.Now().UnixMilli()
		windowStart = now - constant.RateLimitWindow.Milliseconds()
		pipe        = rl.client.Pipeline()
	)

	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: uuid.NewString()})
	count := pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, constant.RateLimitWindow)

	_, err := pipe.Exec(ctx)
	return int(count.Val()), err
}

func clientID(r *http.Request) string {
	if userID := GetUserID(r.Context()); userID != "" {
		return userID
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
