package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body response.Response

	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Errors, 1)
	return body.Errors[0].Message
}

func TestAuth(t *testing.T) {
	manager := jwt.NewJWTManager("secret", time.Minute, time.Hour)

	pair, err := manager.GenerateTokenPair("user-1", "a@example.com", constant.RoleBuyer)
	require.NoError(t, err)

	tests := []struct {
		name        string
		header      string
		wantMessage string
	}{
		{name: "valid access token", header: "Bearer " + pair.AccessToken},
		{name: "no header", wantMessage: constant.MsgMissingAuthHeader},
		{name: "no scheme", header: pair.AccessToken, wantMessage: constant.MsgInvalidAuthFormat},
		{name: "other scheme", header: "Basic " + pair.AccessToken, wantMessage: constant.MsgInvalidAuthFormat},
		{name: "invalid token", header: "Bearer not-a-token", wantMessage: constant.MsgInvalidOrExpiredToken},
		{name: "refresh token", header: "Bearer " + pair.RefreshToken, wantMessage: constant.MsgInvalidTokenType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				userID string
				role   any
				req    = httptest.NewRequest(http.MethodGet, "/", nil)
				rec    = httptest.NewRecorder()
			)

			if tt.header != "" {
				req.Header.Set(constant.HeaderAuthorization, tt.header)
			}
			Auth(manager)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				userID, role = GetUserID(r.Context()), r.Context().Value(ContextRole)
			})).ServeHTTP(rec, req)

			if tt.wantMessage != "" {
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.Equal(t, tt.wantMessage, errorMessage(t, rec))
				assert.Empty(t, userID, "the handler must not run")
				return
			}
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "user-1", userID)
			assert.Equal(t, constant.RoleBuyer, role)
		})
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name     string
		role     any
		wantCode int
	}{
		{name: "required role", role: constant.RoleSeller, wantCode: http.StatusOK},
		{name: "other role", role: constant.RoleBuyer, wantCode: http.StatusForbidden},
		{name: "no role", wantCode: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				req = httptest.NewRequest(http.MethodGet, "/", nil)
				rec = httptest.NewRecorder()
			)

			req = req.WithContext(context.WithValue(req.Context(), ContextRole, tt.role))
			RequireRole(constant.RoleSeller)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}
