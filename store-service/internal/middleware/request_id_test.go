package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	clientID := uuid.NewString()

	tests := []struct {
		name    string
		header  string
		keepsIt bool
	}{
		{name: "missing", header: ""},
		{name: "a UUID from the client", header: clientID, keepsIt: true},
		{name: "anything else", header: strings.Repeat("x", 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				req = httptest.NewRequest(http.MethodGet, "/", nil)
				rec = httptest.NewRecorder()
			)

			req.Header.Set(constant.HeaderRequestID, tt.header)
			RequestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)

			got := rec.Header().Get(constant.HeaderRequestID)
			_, err := uuid.Parse(got)
			assert.NoError(t, err)
			assert.Equal(t, tt.keepsIt, got == tt.header)
		})
	}
}
