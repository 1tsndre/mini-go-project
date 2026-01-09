package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func send(h http.Handler, remoteAddr, userID string) *httptest.ResponseRecorder {
	var (
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		rec = httptest.NewRecorder()
	)

	req.RemoteAddr = remoteAddr
	if userID != "" {
		req = req.WithContext(context.WithValue(req.Context(), ContextUserID, userID))
	}
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimiter_Limit(t *testing.T) {
	var (
		server  = miniredis.RunT(t)
		limiter = NewRateLimiter(redis.NewClient(&redis.Options{Addr: server.Addr()}))
		next    = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		public  = limiter.Limit(2, constant.RateLimitKeyPublic)(next)
		login   = limiter.Limit(2, constant.RateLimitKeyLogin)(next)
	)

	first := send(public, "10.0.0.1:1000", "")
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, "2", first.Header().Get(constant.HeaderRateLimitLimit))
	assert.Equal(t, "1", first.Header().Get(constant.HeaderRateLimitRemaining))
	assert.NotEmpty(t, first.Header().Get(constant.HeaderRateLimitReset))

	assert.Equal(t, http.StatusOK, send(public, "10.0.0.1:2000", "").Code, "the port does not matter")

	third := send(public, "10.0.0.1:3000", "")
	assert.Equal(t, http.StatusTooManyRequests, third.Code)
	assert.Equal(t, "0", third.Header().Get(constant.HeaderRateLimitRemaining))
	assert.Equal(t, constant.MsgRateLimitExceeded, errorMessage(t, third))

	assert.Equal(t, http.StatusOK, send(public, "10.0.0.2:1000", "").Code, "another address has its own limit")
	assert.Equal(t, http.StatusOK, send(public, "10.0.0.1:1000", "user-1").Code, "so has a logged-in user")
	assert.Equal(t, http.StatusOK, send(login, "10.0.0.1:1000", "").Code, "and every kind of limit")
}

func TestRateLimiter_RedisDown(t *testing.T) {
	var (
		server  = miniredis.RunT(t)
		limiter = NewRateLimiter(redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1}))
		next    = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		public  = limiter.Limit(1, constant.RateLimitKeyPublic)(next)
		login   = limiter.Limit(1, constant.RateLimitKeyLogin)(next)
	)
	server.Close()

	assert.Equal(t, http.StatusOK, send(public, "10.0.0.1:1000", "").Code, "other routes stay open")
	assert.Equal(t, http.StatusOK, send(public, "10.0.0.1:1000", "").Code)

	rec := send(login, "10.0.0.1:1000", "")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "login and register are refused")
	assert.Equal(t, constant.MsgAuthUnavailable, errorMessage(t, rec))
}
