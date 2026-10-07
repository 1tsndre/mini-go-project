package service

import (
	"context"
	"errors"
	"testing"

	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
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

			svc := NewAuthService(repo, newTestJWTManager())
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

	tests := []struct {
		name        string
		req         model.LoginRequest
		mockSetup   func(repo *mocks.MockUserRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			req: model.LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockSetup: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(&model.User{
					ID:       uuid.New(),
					Email:    "test@example.com",
					Password: string(hashedPassword),
					Role:     "buyer",
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "invalid email",
			req: model.LoginRequest{
				Email:    "wrong@example.com",
				Password: "password123",
			},
			mockSetup: func(repo *mocks.MockUserRepository) {
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
			mockSetup: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().FindByEmail(gomock.Any(), "test@example.com").Return(&model.User{
					ID:       uuid.New(),
					Email:    "test@example.com",
					Password: string(hashedPassword),
					Role:     "buyer",
				}, nil)
			},
			wantErr:     true,
			errContains: "invalid email or password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockUserRepository(ctrl)
			tt.mockSetup(repo)

			svc := NewAuthService(repo, newTestJWTManager())
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
		mockSetup   func(repo *mocks.MockUserRepository)
		wantErr     bool
		errContains string
		wantRole    string
	}{
		{
			name:  "success - role reloaded from database",
			token: pair.RefreshToken,
			mockSetup: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().FindByID(gomock.Any(), userID).Return(&model.User{
					ID:    userID,
					Email: "test@example.com",
					Role:  "seller",
				}, nil)
			},
			wantRole: "seller",
		},
		{
			name:        "access token rejected",
			token:       pair.AccessToken,
			mockSetup:   func(_ *mocks.MockUserRepository) {},
			wantErr:     true,
			errContains: "invalid refresh token",
		},
		{
			name:        "malformed token",
			token:       "not-a-token",
			mockSetup:   func(_ *mocks.MockUserRepository) {},
			wantErr:     true,
			errContains: "invalid refresh token",
		},
		{
			name:  "user no longer exists",
			token: pair.RefreshToken,
			mockSetup: func(repo *mocks.MockUserRepository) {
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
			tt.mockSetup(repo)

			svc := NewAuthService(repo, jwtManager)
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

		resp, err := NewAuthService(repo, newTestJWTManager()).Register(context.Background(), model.RegisterRequest{
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

		_, err := NewAuthService(repo, newTestJWTManager()).Register(context.Background(), model.RegisterRequest{
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

		pair, err := NewAuthService(repo, newTestJWTManager()).Login(context.Background(), model.LoginRequest{
			Email:    "Andreas@Example.com",
			Password: "password123",
		})

		assert.NoError(t, err)
		assert.NotNil(t, pair)
	})
}
