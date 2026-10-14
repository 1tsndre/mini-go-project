package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
)

// idempotentResponse is what the cache holds for an Idempotency-Key.
type idempotentResponse struct {
	// Fingerprint is the SHA-256 of the request body, so the key cannot be reused
	// for a different request.
	Fingerprint string `json:"fingerprint"`
	// Status is 0 while the first request is still being handled.
	Status int    `json:"status"`
	Body   string `json:"body,omitempty"`
}

// Idempotency makes a request with an Idempotency-Key header safe to retry, as in
// the IETF Idempotency-Key draft: the first request with a key is handled and its
// response stored for the user; a retry with the same key and body gets that
// response again, marked with Idempotent-Replayed, instead of being handled twice.
// The same key with another body is refused with 422, and a retry while the first
// request is still running with 409. A 5xx response is not stored, so the request
// can be retried. Requests without the header are handled as usual.
//
// It runs after authentication, as the stored responses belong to the user.
func Idempotency(cache caches.Cache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(constant.HeaderIdempotencyKey)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			meta := BuildMeta(r)
			if len(key) > constant.MaxIdempotencyKeyLength {
				response.ErrorResponse(w, http.StatusBadRequest, meta, response.NewError(constant.ErrCodeValidation,
					fmt.Sprintf("Idempotency-Key must be at most %d characters", constant.MaxIdempotencyKeyLength)))
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				var maxBytesErr *http.MaxBytesError
				if errors.As(err, &maxBytesErr) {
					response.ErrorResponse(w, http.StatusRequestEntityTooLarge, meta,
						response.NewError(constant.ErrCodeValidation, "request body too large"))
				} else {
					response.ErrorResponse(w, http.StatusBadRequest, meta,
						response.NewError(constant.ErrCodeValidation, "invalid request body"))
				}
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			sum := sha256.Sum256(body)
			fingerprint := hex.EncodeToString(sum[:])
			cacheKey := fmt.Sprintf(constant.KeyIdempotency, GetUserID(r.Context()), key)

			// Storing the response must not depend on the request: a client that gave up
			// (or got the 504) is exactly the one that retries.
			ctx := context.WithoutCancel(r.Context())

			reserved, err := cache.SetNX(ctx, cacheKey, idempotentResponse{Fingerprint: fingerprint}, constant.TTLIdempotencyPending)
			if err != nil {
				logger.Error(ctx, "failed to reserve idempotency key", err)
				response.ErrorResponse(w, http.StatusInternalServerError, meta,
					response.NewError(constant.ErrCodeInternal, "internal server error"))
				return
			}
			if !reserved {
				replay(w, r, cache, cacheKey, fingerprint)
				return
			}

			recorder := &responseRecorder{ResponseWriter: w}
			defer func() {
				// A 5xx, or a panic, leaves the key free for the retry.
				if recorder.status == 0 || recorder.status >= http.StatusInternalServerError {
					if err := cache.Delete(ctx, cacheKey); err != nil {
						logger.Error(ctx, "failed to release idempotency key", err)
					}
				}
			}()
			next.ServeHTTP(recorder, r)

			if recorder.status < http.StatusInternalServerError {
				stored := idempotentResponse{Fingerprint: fingerprint, Status: recorder.status, Body: recorder.body.String()}
				if err := cache.Set(ctx, cacheKey, stored, constant.TTLIdempotency); err != nil {
					logger.Error(ctx, "failed to store idempotent response", err)
				}
			}
		})
	}
}

// replay answers a request whose key is already in the cache.
func replay(w http.ResponseWriter, r *http.Request, cache caches.Cache, cacheKey, fingerprint string) {
	meta := BuildMeta(r)

	var stored idempotentResponse
	data, err := cache.Get(r.Context(), cacheKey)
	if err == nil {
		err = json.Unmarshal(data, &stored)
	}
	if err != nil {
		// The key expired between the two calls, or the cache is unreachable.
		logger.Error(r.Context(), "failed to read idempotent response", err)
		response.ErrorResponse(w, http.StatusInternalServerError, meta,
			response.NewError(constant.ErrCodeInternal, "internal server error"))
		return
	}

	switch {
	case stored.Fingerprint != fingerprint:
		response.ErrorResponse(w, http.StatusUnprocessableEntity, meta,
			response.NewError(constant.ErrCodeValidation, "Idempotency-Key was already used with a different request"))
	case stored.Status == 0:
		response.ErrorResponse(w, http.StatusConflict, meta,
			response.NewError(constant.ErrCodeConflict, "a request with this Idempotency-Key is still being processed"))
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(constant.HeaderIdempotentReplayed, "true")
		w.WriteHeader(stored.Status)
		io.WriteString(w, stored.Body)
	}
}

// responseRecorder passes the response through and keeps a copy of its status and body.
type responseRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.body.Write(p)
	return r.ResponseWriter.Write(p)
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
