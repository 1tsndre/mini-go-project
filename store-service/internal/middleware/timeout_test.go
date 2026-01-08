package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeout_GivesTheRequestADeadline(t *testing.T) {
	h := Timeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			assert.ErrorIs(t, r.Context().Err(), context.DeadlineExceeded)
		case <-time.After(5 * time.Second):
			t.Error("the request context was not cancelled")
		}
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
