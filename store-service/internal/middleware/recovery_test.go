package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecovery(t *testing.T) {
	panicWith := func(value any) http.Handler {
		return Recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic(value)
		}))
	}

	t.Run("a panic becomes a 500", func(t *testing.T) {
		rec := httptest.NewRecorder()
		panicWith("boom").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("an aborted response keeps panicking", func(t *testing.T) {
		assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
			panicWith(http.ErrAbortHandler).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		})
	})
}
