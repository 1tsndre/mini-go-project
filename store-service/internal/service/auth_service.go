package service

import (
	"context"
	"errors"
	"strings"

	pkgconstant "github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Login compares against this when the email is unknown, so it takes as long as a wrong password.
var dummyPasswordHash, _ = bcrypt.GenerateFromPassword(nil, bcrypt.DefaultCost)

type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest) (*model.User, error)
	Login(ctx context.Context, req model.LoginRequest) (*jwt.TokenPair, error)
	Refresh(ctx context.Context, req model.RefreshTokenRequest) (*jwt.TokenPair, error)
	Logout(ctx context.Context, req model.RefreshTokenRequest) error
}

type authService struct {
	userRepo         repository.UserRepository
	refreshTokenRepo repository.RefreshTokenRepository
	jwtManager       *jwt.JWTManager
}

func NewAuthService(userRepo repository.UserRepository, refreshTokenRepo repository.RefreshTokenRepository, jwtManager *jwt.JWTManager) AuthService {
	return &authService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		jwtManager:       jwtManager,
	}
}

func (s *authService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, constant.LogPasswordHashFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	user := &model.User{
		Email:    strings.ToLower(req.Email),
		Password: string(hashedPassword),
		Name:     req.Name,
		Role:     constant.RoleBuyer,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrDuplicateKey) {
			return nil, &Error{Code: constant.ErrCodeConflict, Message: constant.MsgEmailRegistered}
		}
		logger.Error(ctx, constant.LogUserCreateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgUserCreateFailed}
	}

	logger.Info(ctx, constant.LogUserRegistered, logger.Fields{
		"user_id": user.ID.String(),
		"email":   user.Email,
	})

	return user, nil
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest) (*jwt.TokenPair, error) {
	user, err := s.userRepo.FindByEmail(ctx, strings.ToLower(req.Email))
	if errors.Is(err, repository.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
		return nil, &Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidCredentials}
	}
	if err != nil {
		logger.Error(ctx, constant.LogUserLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, &Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidCredentials}
	}

	tokenPair, err := s.jwtManager.GenerateTokenPair(user.ID.String(), user.Email, user.Role)
	if err != nil {
		logger.Error(ctx, constant.LogTokenPairFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	if err := s.refreshTokenRepo.Save(ctx, tokenPair.RefreshTokenID, user.ID, s.jwtManager.RefreshExpiry()); err != nil {
		logger.Error(ctx, constant.LogRefreshTokenStoreFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	logger.Info(ctx, constant.LogUserLoggedIn, logger.Fields{
		"user_id": user.ID.String(),
	})

	return tokenPair, nil
}

func (s *authService) Refresh(ctx context.Context, req model.RefreshTokenRequest) (*jwt.TokenPair, error) {
	claims, err := s.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil || claims.Type != pkgconstant.TokenTypeRefresh {
		return nil, &Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidRefreshToken}
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, &Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidRefreshToken}
	}

	// Load the user first, so a failing database does not use up the token.
	user, err := s.userRepo.FindByID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidRefreshToken}
	}
	if err != nil {
		logger.Error(ctx, constant.LogUserLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	// Each refresh token works once.
	ownerID, err := s.refreshTokenRepo.Consume(ctx, claims.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidRefreshToken}
	}
	if err != nil {
		logger.Error(ctx, constant.LogRefreshTokenConsumeFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}
	if ownerID != userID {
		return nil, &Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidRefreshToken}
	}

	tokenPair, err := s.jwtManager.GenerateTokenPair(user.ID.String(), user.Email, user.Role)
	if err != nil {
		logger.Error(ctx, constant.LogTokenPairFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	if err := s.refreshTokenRepo.Save(ctx, tokenPair.RefreshTokenID, user.ID, s.jwtManager.RefreshExpiry()); err != nil {
		logger.Error(ctx, constant.LogRefreshTokenStoreFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	return tokenPair, nil
}

func (s *authService) Logout(ctx context.Context, req model.RefreshTokenRequest) error {
	claims, err := s.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil || claims.Type != pkgconstant.TokenTypeRefresh {
		return &Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidRefreshToken}
	}

	if err := s.refreshTokenRepo.Revoke(ctx, claims.ID); err != nil {
		logger.Error(ctx, constant.LogRefreshTokenRevokeFailed, err)
		return &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	logger.Info(ctx, constant.LogUserLoggedOut, logger.Fields{
		"user_id": claims.UserID,
	})
	return nil
}
