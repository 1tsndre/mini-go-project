package service

import (
	"context"
	"errors"
	"strings"

	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest) (*model.UserResponse, error)
	Login(ctx context.Context, req model.LoginRequest) (*jwt.TokenPair, error)
	RefreshToken(ctx context.Context, req model.RefreshRequest) (*jwt.TokenPair, error)
	Logout(ctx context.Context, req model.LogoutRequest) error
}

type authService struct {
	userRepo   repository.UserRepository
	tokenRepo  repository.RefreshTokenRepository
	jwtManager *jwt.JWTManager
}

func NewAuthService(userRepo repository.UserRepository, tokenRepo repository.RefreshTokenRepository, jwtManager *jwt.JWTManager) AuthService {
	return &authService{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		jwtManager: jwtManager,
	}
}

// normalizeEmail lower-cases an email address. Emails are effectively
// case-insensitive (mobile keyboards capitalise the first letter), so an account
// registered as "Andreas@x.com" must be reachable as "andreas@x.com" and must not
// be registrable twice.
func normalizeEmail(email string) string {
	return strings.ToLower(email)
}

func (s *authService) Register(ctx context.Context, req model.RegisterRequest) (*model.UserResponse, error) {
	email := normalizeEmail(req.Email)

	existing, _ := s.userRepo.FindByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("email already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, "failed to hash password", err)
		return nil, errors.New("internal server error")
	}

	user := &model.User{
		Email:    email,
		Password: string(hashedPassword),
		Name:     req.Name,
		Role:     constant.RoleBuyer,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		// A concurrent registration with the same email passed the check above.
		if errors.Is(err, repository.ErrDuplicateKey) {
			return nil, errors.New("email already registered")
		}
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
	user, err := s.userRepo.FindByEmail(ctx, normalizeEmail(req.Email))
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	tokenPair, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
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

	if claims.Type != jwt.TokenTypeRefresh {
		return nil, errors.New("invalid refresh token")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Each refresh token works once: taking it out of the store before the next
	// pair is issued means a token replayed after its owner refreshed, or after
	// logout, is refused.
	ownerID, err := s.tokenRepo.Consume(ctx, claims.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errors.New("invalid refresh token")
	}
	if err != nil {
		logger.Error(ctx, "failed to consume refresh token", err)
		return nil, errors.New("internal server error")
	}
	if ownerID != userID {
		return nil, errors.New("invalid refresh token")
	}

	// Reload the user so the new tokens carry the current role (e.g. buyer -> seller
	// after creating a store) instead of the one baked into the old refresh token.
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	return s.issueTokens(ctx, user)
}

// Logout revokes the refresh token, so its session cannot be refreshed again.
// The access tokens already issued stay valid until they expire.
func (s *authService) Logout(ctx context.Context, req model.LogoutRequest) error {
	claims, err := s.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil || claims.Type != jwt.TokenTypeRefresh {
		return errors.New("invalid refresh token")
	}

	if err := s.tokenRepo.Revoke(ctx, claims.ID); err != nil {
		logger.Error(ctx, "failed to revoke refresh token", err)
		return errors.New("internal server error")
	}

	logger.Info(ctx, "user logged out", map[string]interface{}{
		"user_id": claims.UserID,
	})
	return nil
}

// issueTokens generates a token pair for the user and stores its refresh token.
func (s *authService) issueTokens(ctx context.Context, user *model.User) (*jwt.TokenPair, error) {
	tokenPair, err := s.jwtManager.GenerateTokenPair(user.ID.String(), user.Email, user.Role)
	if err != nil {
		logger.Error(ctx, "failed to generate token pair", err)
		return nil, errors.New("internal server error")
	}

	if err := s.tokenRepo.Save(ctx, tokenPair.RefreshTokenID, user.ID, s.jwtManager.RefreshExpiry()); err != nil {
		logger.Error(ctx, "failed to store refresh token", err)
		return nil, errors.New("internal server error")
	}

	return tokenPair, nil
}
