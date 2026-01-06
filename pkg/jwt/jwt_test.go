package jwt

import (
	"testing"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager_GenerateTokenPair(t *testing.T) {
	manager := NewJWTManager("secret", time.Minute, time.Hour)

	pair, err := manager.GenerateTokenPair("user-1", "a@example.com", "buyer")
	require.NoError(t, err)

	access, err := manager.ValidateToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, constant.TokenTypeAccess, access.Type)
	assert.Equal(t, "user-1", access.UserID)
	assert.Equal(t, "a@example.com", access.Email)
	assert.Equal(t, "buyer", access.Role)
	assert.Empty(t, access.ID)

	refresh, err := manager.ValidateToken(pair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, constant.TokenTypeRefresh, refresh.Type)
	assert.Equal(t, pair.RefreshTokenID, refresh.ID)
	assert.NotEmpty(t, refresh.ID)
}

func TestJWTManager_ValidateToken_Rejects(t *testing.T) {
	var (
		manager = NewJWTManager("secret", time.Minute, time.Hour)
		claims  = &Claims{UserID: "user-1", Type: constant.TokenTypeAccess}
	)

	expired, err := NewJWTManager("secret", -time.Minute, time.Hour).GenerateTokenPair("user-1", "", "buyer")
	require.NoError(t, err)
	foreign, err := NewJWTManager("other-secret", time.Minute, time.Hour).GenerateTokenPair("user-1", "", "buyer")
	require.NoError(t, err)
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	otherMethod, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte("secret"))
	require.NoError(t, err)

	tests := []struct {
		name  string
		token string
	}{
		{name: "expired", token: expired.AccessToken},
		{name: "signed with another secret", token: foreign.AccessToken},
		{name: "unsigned", token: unsigned},
		{name: "another signing method", token: otherMethod},
		{name: "malformed", token: "not-a-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.ValidateToken(tt.token)

			assert.Error(t, err)
		})
	}
}
