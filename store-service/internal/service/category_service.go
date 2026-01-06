package service

import (
	"context"
	"errors"
	"strings"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
)

type CategoryService interface {
	CreateCategory(ctx context.Context, req model.CreateCategoryRequest) (*model.CategoryResponse, error)
	GetAllCategories(ctx context.Context) ([]model.CategoryResponse, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, req model.UpdateCategoryRequest) (*model.CategoryResponse, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
}

type categoryService struct {
	repo repository.CategoryRepository
}

func NewCategoryService(repo repository.CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

func (s *categoryService) CreateCategory(ctx context.Context, req model.CreateCategoryRequest) (*model.CategoryResponse, error) {
	category := &model.Category{
		Name: req.Name,
	}

	if err := s.repo.Create(ctx, category); err != nil {
		logger.Error(ctx, "failed to create category", err)
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, errors.New("category already exists")
		}
		return nil, errors.New("failed to create category")
	}

	resp := category.ToResponse()
	return &resp, nil
}

func (s *categoryService) GetAllCategories(ctx context.Context) ([]model.CategoryResponse, error) {
	categories, err := s.repo.FindAll(ctx)
	if err != nil {
		logger.Error(ctx, "failed to fetch categories", err)
		return nil, errors.New("failed to fetch categories")
	}

	var responses []model.CategoryResponse
	for _, c := range categories {
		responses = append(responses, c.ToResponse())
	}
	return responses, nil
}

func (s *categoryService) UpdateCategory(ctx context.Context, id uuid.UUID, req model.UpdateCategoryRequest) (*model.CategoryResponse, error) {
	category, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("category not found")
	}

	if req.Name != "" {
		category.Name = req.Name
	}

	if err := s.repo.Update(ctx, category); err != nil {
		logger.Error(ctx, "failed to update category", err)
		return nil, errors.New("failed to update category")
	}

	resp := category.ToResponse()
	return &resp, nil
}

func (s *categoryService) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.New("category not found")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		logger.Error(ctx, "failed to delete category", err)
		return errors.New("failed to delete category")
	}
	return nil
}
