package service

import (
	"context"
	"errors"

	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest) (*model.UserResponse, error)
	Login(ctx context.Context, req model.LoginRequest) (*jwt.TokenPair, error)
	RefreshToken(ctx context.Context, req model.RefreshRequest) (*jwt.TokenPair, error)
}

type authService struct {
	userRepo   repository.UserRepository
	jwtManager *jwt.JWTManager
}

func NewAuthService(userRepo repository.UserRepository, jwtManager *jwt.JWTManager) AuthService {
	return &authService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
	}
}

func (s *authService) Register(ctx context.Context, req model.RegisterRequest) (*model.UserResponse, error) {
	existing, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if existing != nil {
		return nil, errors.New("email already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, "failed to hash password", err)
		return nil, errors.New("internal server error")
	}

	user := &model.User{
		Email:    req.Email,
		Password: string(hashedPassword),
		Name:     req.Name,
		Role:     constant.RoleBuyer,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		logger.Error(ctx, "failed to create user", err)
		return nil, errors.New("failed to create user")
	}

	logger.Info(ctx, "user registered", map[string]interface{}{
		"user_id": user.ID.String(),
		"email":   user.Email,
	})

	resp := user.ToResponse()
	return &resp, nil
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest) (*jwt.TokenPair, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	tokenPair, err := s.jwtManager.GenerateTokenPair(user.ID.String(), user.Email, user.Role)
	if err != nil {
		logger.Error(ctx, "failed to generate token pair", err)
		return nil, errors.New("internal server error")
	}

	logger.Info(ctx, "user logged in", map[string]interface{}{
		"user_id": user.ID.String(),
	})

	return tokenPair, nil
}

func (s *authService) RefreshToken(ctx context.Context, req model.RefreshRequest) (*jwt.TokenPair, error) {
	claims, err := s.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	tokenPair, err := s.jwtManager.GenerateTokenPair(claims.UserID, claims.Email, claims.Role)
	if err != nil {
		logger.Error(ctx, "failed to generate token pair", err)
		return nil, errors.New("internal server error")
	}

	return tokenPair, nil
}
