package jwt

import (
	"time"

	"github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken    string `json:"access_token"`
	RefreshToken   string `json:"refresh_token"`
	RefreshTokenID string `json:"-"`
}

type JWTManager struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewJWTManager(secret string, accessExpiry, refreshExpiry time.Duration) *JWTManager {
	return &JWTManager{
		secret:        []byte(secret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

func (m *JWTManager) GenerateTokenPair(userID, email, role string) (*TokenPair, error) {
	accessToken, err := m.generateToken(userID, email, role, constant.TokenTypeAccess, m.accessExpiry, "")
	if err != nil {
		return nil, err
	}

	refreshTokenID := uuid.NewString()
	refreshToken, err := m.generateToken(userID, email, role, constant.TokenTypeRefresh, m.refreshExpiry, refreshTokenID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:    accessToken,
		RefreshToken:   refreshToken,
		RefreshTokenID: refreshTokenID,
	}, nil
}

func (m *JWTManager) RefreshExpiry() time.Duration {
	return m.refreshExpiry
}

func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func (m *JWTManager) generateToken(userID, email, role, tokenType string, expiry time.Duration, id string) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        id,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}
