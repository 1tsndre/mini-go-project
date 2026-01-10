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

type CategoryService interface {
	CreateCategory(ctx context.Context, req model.CategoryRequest) (*model.Category, error)
	GetAllCategories(ctx context.Context) ([]model.Category, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, req model.CategoryRequest) (*model.Category, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
}

type categoryService struct {
	categoryRepo repository.CategoryRepository
}

func NewCategoryService(categoryRepo repository.CategoryRepository) CategoryService {
	return &categoryService{categoryRepo: categoryRepo}
}

func (s *categoryService) CreateCategory(ctx context.Context, req model.CategoryRequest) (*model.Category, error) {
	category := &model.Category{Name: req.Name}

	if err := s.categoryRepo.Create(ctx, category); err != nil {
		if errors.Is(err, repository.ErrDuplicateKey) {
			return nil, &Error{Code: constant.ErrCodeConflict, Message: constant.MsgCategoryExists}
		}
		logger.Error(ctx, constant.LogCategoryCreateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgCategoryCreateFailed}
	}

	logger.Info(ctx, constant.LogCategoryCreated, logger.Fields{
		"category_id": category.ID.String(),
	})

	return category, nil
}

func (s *categoryService) GetAllCategories(ctx context.Context) ([]model.Category, error) {
	categories, err := s.categoryRepo.FindAll(ctx)
	if err != nil {
		logger.Error(ctx, constant.LogCategoryFetchFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgCategoryFetchFailed}
	}

	return categories, nil
}

func (s *categoryService) UpdateCategory(ctx context.Context, id uuid.UUID, req model.CategoryRequest) (*model.Category, error) {
	category := &model.Category{ID: id, Name: req.Name}

	err := s.categoryRepo.Update(ctx, category)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgCategoryNotFound}
	}
	if errors.Is(err, repository.ErrDuplicateKey) {
		return nil, &Error{Code: constant.ErrCodeConflict, Message: constant.MsgCategoryExists}
	}
	if err != nil {
		logger.Error(ctx, constant.LogCategoryUpdateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgCategoryUpdateFailed}
	}

	return category, nil
}

func (s *categoryService) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	err := s.categoryRepo.Delete(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgCategoryNotFound}
	}
	if errors.Is(err, repository.ErrForeignKeyViolation) {
		return &Error{Code: constant.ErrCodeConflict, Message: constant.MsgCategoryInUse}
	}
	if err != nil {
		logger.Error(ctx, constant.LogCategoryDeleteFailed, err)
		return &Error{Code: constant.ErrCodeInternal, Message: constant.MsgCategoryDeleteFailed}
	}

	return nil
}
