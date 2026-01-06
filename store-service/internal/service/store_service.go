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
	CreateStore(ctx context.Context, userID uuid.UUID, req model.CreateStoreRequest) (*model.StoreResponse, error)
	GetStoreByID(ctx context.Context, id uuid.UUID) (*model.StoreResponse, error)
	UpdateStore(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateStoreRequest) (*model.StoreResponse, error)
	UpdateLogo(ctx context.Context, userID uuid.UUID, id uuid.UUID, logoURL string) (*model.StoreResponse, error)
}

type storeService struct {
	storeRepo repository.StoreRepository
	userRepo  repository.UserRepository
}

func NewStoreService(storeRepo repository.StoreRepository, userRepo repository.UserRepository) StoreService {
	return &storeService{
		storeRepo: storeRepo,
		userRepo:  userRepo,
	}
}

func (s *storeService) CreateStore(ctx context.Context, userID uuid.UUID, req model.CreateStoreRequest) (*model.StoreResponse, error) {
	existing, _ := s.storeRepo.FindByUserID(ctx, userID)
	if existing != nil {
		return nil, errors.New("user already has a store")
	}

	store := &model.Store{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.storeRepo.Create(ctx, store); err != nil {
		logger.Error(ctx, "failed to create store", err)
		return nil, errors.New("failed to create store")
	}

	if err := s.userRepo.UpdateRole(ctx, userID, constant.RoleSeller); err != nil {
		logger.Error(ctx, "failed to update user role", err)
		return nil, errors.New("failed to create store")
	}

	logger.Info(ctx, "store created", map[string]interface{}{
		"store_id": store.ID.String(),
		"user_id":  userID.String(),
	})

	resp := store.ToResponse()
	return &resp, nil
}

func (s *storeService) GetStoreByID(ctx context.Context, id uuid.UUID) (*model.StoreResponse, error) {
	store, err := s.storeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("store not found")
	}

	resp := store.ToResponse()
	return &resp, nil
}

func (s *storeService) UpdateStore(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateStoreRequest) (*model.StoreResponse, error) {
	store, err := s.storeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("store not found")
	}

	if store.UserID != userID {
		return nil, errors.New("forbidden: not store owner")
	}

	if req.Name != "" {
		store.Name = req.Name
	}
	if req.Description != "" {
		store.Description = req.Description
	}

	if err := s.storeRepo.Update(ctx, store); err != nil {
		logger.Error(ctx, "failed to update store", err)
		return nil, errors.New("failed to update store")
	}

	resp := store.ToResponse()
	return &resp, nil
}

func (s *storeService) UpdateLogo(ctx context.Context, userID uuid.UUID, id uuid.UUID, logoURL string) (*model.StoreResponse, error) {
	store, err := s.storeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("store not found")
	}

	if store.UserID != userID {
		return nil, errors.New("forbidden: not store owner")
	}

	store.LogoURL = logoURL
	if err := s.storeRepo.Update(ctx, store); err != nil {
		logger.Error(ctx, "failed to update store logo", err)
		return nil, errors.New("failed to update store logo")
	}

	resp := store.ToResponse()
	return &resp, nil
}
