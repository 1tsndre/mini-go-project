package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingHandler answers with status and counts how often it ran.
type countingHandler struct {
	status int
	calls  int
}

func (h *countingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.calls++
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.status)
	w.Write([]byte(`{"data":{"call":` + string(rune('0'+h.calls)) + `}}`))
}

func idempotentRequest(userID, key, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	if key != "" {
		r.Header.Set(constant.HeaderIdempotencyKey, key)
	}
	return r.WithContext(context.WithValue(r.Context(), ContextUserID, userID))
}

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var body struct {
		Errors []struct{ Code, Message string } `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Errors, 1)
	return body.Errors[0].Code, body.Errors[0].Message
}

func TestIdempotency(t *testing.T) {
	const body = `{"shipping_address":"Jl. Test 1"}`

	t.Run("a retry replays the first response instead of handling the request again", func(t *testing.T) {
		handler := &countingHandler{status: http.StatusCreated}
		h := Idempotency(memory.NewCache())(handler)

		first := serve(h, idempotentRequest("u1", "k1", body))
		retry := serve(h, idempotentRequest("u1", "k1", body))

		assert.Equal(t, 1, handler.calls)
		assert.Equal(t, http.StatusCreated, first.Code)
		assert.Empty(t, first.Header().Get(constant.HeaderIdempotentReplayed))
		assert.Equal(t, http.StatusCreated, retry.Code)
		assert.Equal(t, first.Body.String(), retry.Body.String())
		assert.Equal(t, "true", retry.Header().Get(constant.HeaderIdempotentReplayed))
		assert.Equal(t, "application/json", retry.Header().Get("Content-Type"))
	})

	t.Run("a 4xx response is replayed too", func(t *testing.T) {
		handler := &countingHandler{status: http.StatusBadRequest}
		h := Idempotency(memory.NewCache())(handler)

		serve(h, idempotentRequest("u1", "k1", body))
		retry := serve(h, idempotentRequest("u1", "k1", body))

		assert.Equal(t, 1, handler.calls)
		assert.Equal(t, http.StatusBadRequest, retry.Code)
	})

	t.Run("the same key with another body is refused", func(t *testing.T) {
		handler := &countingHandler{status: http.StatusCreated}
		h := Idempotency(memory.NewCache())(handler)

		serve(h, idempotentRequest("u1", "k1", body))
		other := serve(h, idempotentRequest("u1", "k1", `{"shipping_address":"Elsewhere"}`))

		assert.Equal(t, 1, handler.calls)
		assert.Equal(t, http.StatusUnprocessableEntity, other.Code)
		code, msg := errorOf(t, other)
		assert.Equal(t, constant.ErrCodeValidation, code)
		assert.Equal(t, "Idempotency-Key was already used with a different request", msg)
	})

	t.Run("a retry while the first request is running is refused", func(t *testing.T) {
		cache := memory.NewCache()
		handler := &countingHandler{status: http.StatusCreated}
		h := Idempotency(cache)(handler)
		var inner *httptest.ResponseRecorder
		// The first request is still in the handler when the retry arrives.
		blocking := Idempotency(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inner = serve(h, idempotentRequest("u1", "k1", body))
			w.WriteHeader(http.StatusCreated)
		}))

		serve(blocking, idempotentRequest("u1", "k1", body))

		assert.Zero(t, handler.calls)
		assert.Equal(t, http.StatusConflict, inner.Code)
		code, msg := errorOf(t, inner)
		assert.Equal(t, constant.ErrCodeConflict, code)
		assert.Equal(t, "a request with this Idempotency-Key is still being processed", msg)
	})

	t.Run("a 5xx response is not stored, so the request can be retried", func(t *testing.T) {
		handler := &countingHandler{status: http.StatusInternalServerError}
		h := Idempotency(memory.NewCache())(handler)

		serve(h, idempotentRequest("u1", "k1", body))
		retry := serve(h, idempotentRequest("u1", "k1", body))

		assert.Equal(t, 2, handler.calls)
		assert.Empty(t, retry.Header().Get(constant.HeaderIdempotentReplayed))
	})

	t.Run("a panic releases the key", func(t *testing.T) {
		cache := memory.NewCache()
		h := Idempotency(cache)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))

		assert.Panics(t, func() { serve(h, idempotentRequest("u1", "k1", body)) })

		exists, err := cache.Exists(context.Background(), "idempotency:u1:k1")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("keys are per user", func(t *testing.T) {
		handler := &countingHandler{status: http.StatusCreated}
		h := Idempotency(memory.NewCache())(handler)

		serve(h, idempotentRequest("u1", "k1", body))
		serve(h, idempotentRequest("u2", "k1", body))

		assert.Equal(t, 2, handler.calls)
	})

	t.Run("without the header every request is handled", func(t *testing.T) {
		handler := &countingHandler{status: http.StatusCreated}
		h := Idempotency(memory.NewCache())(handler)

		serve(h, idempotentRequest("u1", "", body))
		serve(h, idempotentRequest("u1", "", body))

		assert.Equal(t, 2, handler.calls)
	})

	t.Run("an overlong key is refused", func(t *testing.T) {
		handler := &countingHandler{status: http.StatusCreated}
		h := Idempotency(memory.NewCache())(handler)

		rec := serve(h, idempotentRequest("u1", strings.Repeat("k", 256), body))

		assert.Zero(t, handler.calls)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		_, msg := errorOf(t, rec)
		assert.Equal(t, "Idempotency-Key must be at most 255 characters", msg)
	})

	t.Run("a body over the limit is refused", func(t *testing.T) {
		handler := &countingHandler{status: http.StatusCreated}
		h := MaxBodyBytes(8)(Idempotency(memory.NewCache())(handler))

		rec := serve(h, idempotentRequest("u1", "k1", body))

		assert.Zero(t, handler.calls)
		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})

	t.Run("the handler still reads the whole body", func(t *testing.T) {
		var got string
		h := Idempotency(memory.NewCache())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				ShippingAddress string `json:"shipping_address"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			got = req.ShippingAddress
			w.WriteHeader(http.StatusCreated)
		}))

		serve(h, idempotentRequest("u1", "k1", body))

		assert.Equal(t, "Jl. Test 1", got)
	})
}
