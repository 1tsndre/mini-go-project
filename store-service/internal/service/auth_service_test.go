package service

import (
	"context"
	"errors"
	"testing"

	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches/memory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
	"time"
)

func newTestJWTManager() *jwt.JWTManager {
	return jwt.NewJWTManager("test-secret", 15*time.Minute, 168*time.Hour)
}

func TestAuthService_Register(t *testing.T) {
	tests := []struct {
		name        string
		req         model.RegisterRequest
		mockSetup   func(repo *mocks.MockUserRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			req: model.RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			mockSetup: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(nil, errors.New("not found"))
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "email already registered",
			req: model.RegisterRequest{
				Email:    "existing@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			mockSetup: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "existing@example.com").Return(&model.User{
					ID:    uuid.New(),
					Email: "existing@example.com",
				}, nil)
			},
			wantErr:     true,
			errContains: "email already registered",
		},
		{
			name: "create user fails",
			req: model.RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			mockSetup: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(nil, errors.New("not found"))
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: "failed to create user",
		},
		{
			name: "concurrent registration with same email",
			req: model.RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			mockSetup: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(nil, errors.New("not found"))
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrDuplicateKey)
			},
			wantErr:     true,
			errContains: "email already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockUserRepository(ctrl)
			tt.mockSetup(repo)

			svc := NewAuthService(repo, mocks.NewMockRefreshTokenRepository(ctrl), newTestJWTManager())
			resp, err := svc.Register(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.req.Email, resp.Email)
			assert.Equal(t, tt.req.Name, resp.Name)
			assert.Equal(t, "buyer", resp.Role)
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	userID := uuid.New()
	user := &model.User{ID: userID, Email: "test@example.com", Password: string(hashedPassword), Role: "buyer"}

	tests := []struct {
		name        string
		req         model.LoginRequest
		mockSetup   func(repo *mocks.MockUserRepository, tokens *mocks.MockRefreshTokenRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			req: model.LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockSetup: func(repo *mocks.MockUserRepository, tokens *mocks.MockRefreshTokenRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(user, nil)
				tokens.EXPECT().Save(gomock.Any(), gomock.Any(), userID, 168*time.Hour).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "invalid email",
			req: model.LoginRequest{
				Email:    "wrong@example.com",
				Password: "password123",
			},
			mockSetup: func(repo *mocks.MockUserRepository, _ *mocks.MockRefreshTokenRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "wrong@example.com").Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "invalid email or password",
		},
		{
			name: "wrong password",
			req: model.LoginRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
			},
			mockSetup: func(repo *mocks.MockUserRepository, _ *mocks.MockRefreshTokenRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(user, nil)
			},
			wantErr:     true,
			errContains: "invalid email or password",
		},
		{
			name: "refresh token cannot be stored",
			req: model.LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockSetup: func(repo *mocks.MockUserRepository, tokens *mocks.MockRefreshTokenRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(user, nil)
				tokens.EXPECT().Save(gomock.Any(), gomock.Any(), userID, 168*time.Hour).Return(errors.New("redis down"))
			},
			wantErr:     true,
			errContains: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockUserRepository(ctrl)
			tokens := mocks.NewMockRefreshTokenRepository(ctrl)
			tt.mockSetup(repo, tokens)

			svc := NewAuthService(repo, tokens, newTestJWTManager())
			tokenPair, err := svc.Login(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, tokenPair)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, tokenPair)
			assert.NotEmpty(t, tokenPair.AccessToken)
			assert.NotEmpty(t, tokenPair.RefreshToken)
		})
	}
}

func TestAuthService_RefreshToken(t *testing.T) {
	jwtManager := newTestJWTManager()
	userID := uuid.New()

	pair, err := jwtManager.GenerateTokenPair(userID.String(), "test@example.com", "buyer")
	assert.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		mockSetup   func(repo *mocks.MockUserRepository, tokens *mocks.MockRefreshTokenRepository)
		wantErr     bool
		errContains string
		wantRole    string
	}{
		{
			name:  "success - role reloaded from database",
			token: pair.RefreshToken,
			mockSetup: func(repo *mocks.MockUserRepository, tokens *mocks.MockRefreshTokenRepository) {
				tokens.EXPECT().Consume(gomock.Any(), pair.RefreshTokenID).Return(userID, nil)
				repo.EXPECT().FindByID(gomock.Any(), userID).Return(&model.User{
					ID:    userID,
					Email: "test@example.com",
					Role:  "seller",
				}, nil)
				tokens.EXPECT().Save(gomock.Any(), gomock.Not(pair.RefreshTokenID), userID, 168*time.Hour).Return(nil)
			},
			wantRole: "seller",
		},
		{
			name:        "access token rejected",
			token:       pair.AccessToken,
			mockSetup:   func(_ *mocks.MockUserRepository, _ *mocks.MockRefreshTokenRepository) {},
			wantErr:     true,
			errContains: "invalid refresh token",
		},
		{
			name:        "malformed token",
			token:       "not-a-token",
			mockSetup:   func(_ *mocks.MockUserRepository, _ *mocks.MockRefreshTokenRepository) {},
			wantErr:     true,
			errContains: "invalid refresh token",
		},
		{
			name:  "already used or revoked",
			token: pair.RefreshToken,
			mockSetup: func(_ *mocks.MockUserRepository, tokens *mocks.MockRefreshTokenRepository) {
				tokens.EXPECT().Consume(gomock.Any(), pair.RefreshTokenID).Return(uuid.Nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: "invalid refresh token",
		},
		{
			name:  "stored for another user",
			token: pair.RefreshToken,
			mockSetup: func(_ *mocks.MockUserRepository, tokens *mocks.MockRefreshTokenRepository) {
				tokens.EXPECT().Consume(gomock.Any(), pair.RefreshTokenID).Return(uuid.New(), nil)
			},
			wantErr:     true,
			errContains: "invalid refresh token",
		},
		{
			name:  "token store unavailable",
			token: pair.RefreshToken,
			mockSetup: func(_ *mocks.MockUserRepository, tokens *mocks.MockRefreshTokenRepository) {
				tokens.EXPECT().Consume(gomock.Any(), pair.RefreshTokenID).Return(uuid.Nil, errors.New("redis down"))
			},
			wantErr:     true,
			errContains: "internal server error",
		},
		{
			name:  "user no longer exists",
			token: pair.RefreshToken,
			mockSetup: func(repo *mocks.MockUserRepository, tokens *mocks.MockRefreshTokenRepository) {
				tokens.EXPECT().Consume(gomock.Any(), pair.RefreshTokenID).Return(userID, nil)
				repo.EXPECT().FindByID(gomock.Any(), userID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "invalid refresh token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockUserRepository(ctrl)
			tokens := mocks.NewMockRefreshTokenRepository(ctrl)
			tt.mockSetup(repo, tokens)

			svc := NewAuthService(repo, tokens, jwtManager)
			tokenPair, err := svc.RefreshToken(context.Background(), model.RefreshRequest{RefreshToken: tt.token})

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, tokenPair)
				return
			}
			assert.NoError(t, err)
			claims, err := jwtManager.ValidateToken(tokenPair.AccessToken)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantRole, claims.Role)
		})
	}
}

func TestAuthService_Logout(t *testing.T) {
	jwtManager := newTestJWTManager()
	userID := uuid.New()

	pair, err := jwtManager.GenerateTokenPair(userID.String(), "test@example.com", "buyer")
	assert.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		mockSetup   func(tokens *mocks.MockRefreshTokenRepository)
		errContains string
	}{
		{
			name:  "success",
			token: pair.RefreshToken,
			mockSetup: func(tokens *mocks.MockRefreshTokenRepository) {
				tokens.EXPECT().Revoke(gomock.Any(), pair.RefreshTokenID).Return(nil)
			},
		},
		{
			name:        "access token rejected",
			token:       pair.AccessToken,
			mockSetup:   func(_ *mocks.MockRefreshTokenRepository) {},
			errContains: "invalid refresh token",
		},
		{
			name:        "malformed token",
			token:       "not-a-token",
			mockSetup:   func(_ *mocks.MockRefreshTokenRepository) {},
			errContains: "invalid refresh token",
		},
		{
			name:  "token store unavailable",
			token: pair.RefreshToken,
			mockSetup: func(tokens *mocks.MockRefreshTokenRepository) {
				tokens.EXPECT().Revoke(gomock.Any(), pair.RefreshTokenID).Return(errors.New("redis down"))
			},
			errContains: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tokens := mocks.NewMockRefreshTokenRepository(ctrl)
			tt.mockSetup(tokens)

			err := NewAuthService(mocks.NewMockUserRepository(ctrl), tokens, jwtManager).
				Logout(context.Background(), model.LogoutRequest{RefreshToken: tt.token})

			if tt.errContains != "" {
				assert.ErrorContains(t, err, tt.errContains)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// With the real token store: a refresh token works once, and not after logout.
func TestAuthService_RefreshTokenRotation(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	user := &model.User{ID: uuid.New(), Email: "test@example.com", Password: string(hashedPassword), Role: "buyer"}

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockUserRepository(ctrl)
	repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(user, nil)
	repo.EXPECT().FindByID(gomock.Any(), user.ID).Return(user, nil)
	svc := NewAuthService(repo, repository.NewRefreshTokenRepository(memory.NewCache()), newTestJWTManager())
	ctx := context.Background()

	first, err := svc.Login(ctx, model.LoginRequest{Email: "test@example.com", Password: "password123"})
	assert.NoError(t, err)

	second, err := svc.RefreshToken(ctx, model.RefreshRequest{RefreshToken: first.RefreshToken})
	assert.NoError(t, err)

	_, err = svc.RefreshToken(ctx, model.RefreshRequest{RefreshToken: first.RefreshToken})
	assert.ErrorContains(t, err, "invalid refresh token", "a refresh token works only once")

	assert.NoError(t, svc.Logout(ctx, model.LogoutRequest{RefreshToken: second.RefreshToken}))
	_, err = svc.RefreshToken(ctx, model.RefreshRequest{RefreshToken: second.RefreshToken})
	assert.ErrorContains(t, err, "invalid refresh token", "logout revokes the refresh token")
}

func TestAuthService_EmailIsCaseInsensitive(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)

	t.Run("register stores the email in lower case", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockUserRepository(ctrl)
		var created *model.User
		repo.EXPECT().FindByEmail(gomock.Any(), "andreas@example.com").Return(nil, errors.New("not found"))
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u *model.User) error {
			created = u
			return nil
		})

		resp, err := NewAuthService(repo, mocks.NewMockRefreshTokenRepository(ctrl), newTestJWTManager()).Register(context.Background(), model.RegisterRequest{
			Email:    "Andreas@Example.COM",
			Password: "password123",
			Name:     "Andreas",
		})

		assert.NoError(t, err)
		assert.Equal(t, "andreas@example.com", resp.Email)
		if assert.NotNil(t, created) {
			assert.Equal(t, "andreas@example.com", created.Email)
		}
	})

	t.Run("register rejects a case variant of an existing email", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockUserRepository(ctrl)
		repo.EXPECT().FindByEmail(gomock.Any(), "andreas@example.com").Return(&model.User{ID: uuid.New(), Email: "andreas@example.com"}, nil)

		_, err := NewAuthService(repo, mocks.NewMockRefreshTokenRepository(ctrl), newTestJWTManager()).Register(context.Background(), model.RegisterRequest{
			Email:    "ANDREAS@example.com",
			Password: "password123",
			Name:     "Andreas",
		})

		assert.ErrorContains(t, err, "email already registered")
	})

	t.Run("login matches regardless of case", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockUserRepository(ctrl)
		repo.EXPECT().FindByEmail(gomock.Any(), "andreas@example.com").Return(&model.User{
			ID:       uuid.New(),
			Email:    "andreas@example.com",
			Password: string(hashedPassword),
			Role:     "buyer",
		}, nil)

		tokens := mocks.NewMockRefreshTokenRepository(ctrl)
		tokens.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		pair, err := NewAuthService(repo, tokens, newTestJWTManager()).Login(context.Background(), model.LoginRequest{
			Email:    "Andreas@Example.com",
			Password: "password123",
		})

		assert.NoError(t, err)
		assert.NotNil(t, pair)
	})
}
