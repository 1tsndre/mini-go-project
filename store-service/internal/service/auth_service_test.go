package service

import (
	"context"
	"errors"
	"testing"

	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
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
