package service

import (
	"context"
	"errors"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
)

type StoreService interface {
	CreateStore(ctx context.Context, userID uuid.UUID, req model.StoreRequest) (*model.Store, error)
	GetStoreByID(ctx context.Context, id uuid.UUID) (*model.Store, error)
	GetStoreByUserID(ctx context.Context, userID uuid.UUID) (*model.Store, error)
	UpdateStore(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.StoreRequest) (*model.Store, error)
	UpdateLogo(ctx context.Context, userID uuid.UUID, id uuid.UUID, logoURL string) (*model.Store, error)
}

type storeService struct {
	storeRepo repository.StoreRepository
}

func NewStoreService(storeRepo repository.StoreRepository) StoreService {
	return &storeService{storeRepo: storeRepo}
}

func (s *storeService) CreateStore(ctx context.Context, userID uuid.UUID, req model.StoreRequest) (*model.Store, error) {
	store := &model.Store{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.storeRepo.Create(ctx, store); err != nil {
		if errors.Is(err, repository.ErrDuplicateKey) {
			return nil, &Error{Code: constant.ErrCodeConflict, Message: constant.MsgStoreExists}
		}
		logger.Error(ctx, constant.LogStoreCreateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgStoreCreateFailed}
	}

	logger.Info(ctx, constant.LogStoreCreated, logger.Fields{
		"store_id": store.ID.String(),
		"user_id":  userID.String(),
	})

	return store, nil
}

func (s *storeService) GetStoreByID(ctx context.Context, id uuid.UUID) (*model.Store, error) {
	store, err := s.storeRepo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgStoreNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogStoreLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	return store, nil
}

func (s *storeService) GetStoreByUserID(ctx context.Context, userID uuid.UUID) (*model.Store, error) {
	store, err := s.storeRepo.FindByUserID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgStoreNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogStoreLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	return store, nil
}

func (s *storeService) UpdateStore(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.StoreRequest) (*model.Store, error) {
	store, err := s.storeRepo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgStoreNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogStoreLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}
	if store.UserID != userID {
		return nil, &Error{Code: constant.ErrCodeForbidden, Message: constant.MsgNotStoreOwner}
	}

	if req.Name != "" {
		store.Name = req.Name
	}
	if req.Description != "" {
		store.Description = req.Description
	}

	if err := s.storeRepo.Update(ctx, store); err != nil {
		logger.Error(ctx, constant.LogStoreUpdateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgStoreUpdateFailed}
	}

	return store, nil
}

func (s *storeService) UpdateLogo(ctx context.Context, userID uuid.UUID, id uuid.UUID, logoURL string) (*model.Store, error) {
	store, err := s.storeRepo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgStoreNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogStoreLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}
	if store.UserID != userID {
		return nil, &Error{Code: constant.ErrCodeForbidden, Message: constant.MsgNotStoreOwner}
	}

	store.LogoURL = logoURL
	if err := s.storeRepo.UpdateLogo(ctx, store); err != nil {
		logger.Error(ctx, constant.LogStoreLogoUpdateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgStoreLogoUpdateFailed}
	}

	return store, nil
}
