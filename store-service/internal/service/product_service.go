package service

import (
	"context"
	"errors"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ProductService interface {
	CreateProduct(ctx context.Context, userID uuid.UUID, req model.CreateProductRequest) (*model.ProductResponse, error)
	GetProducts(ctx context.Context, filter model.ProductFilter) ([]model.ProductResponse, int64, error)
	GetProductByID(ctx context.Context, id uuid.UUID) (*model.ProductResponse, error)
	UpdateProduct(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateProductRequest) (*model.ProductResponse, error)
	DeleteProduct(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	UpdateImage(ctx context.Context, userID uuid.UUID, id uuid.UUID, imageURL string) (*model.ProductResponse, error)
}

type productService struct {
	productRepo repository.ProductRepository
	storeRepo   repository.StoreRepository
}

func NewProductService(productRepo repository.ProductRepository, storeRepo repository.StoreRepository) ProductService {
	return &productService{
		productRepo: productRepo,
		storeRepo:   storeRepo,
	}
}

func (s *productService) getStoreByOwner(ctx context.Context, userID uuid.UUID) (*model.Store, error) {
	store, err := s.storeRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, errors.New("store not found for this user")
	}
	return store, nil
}

func (s *productService) CreateProduct(ctx context.Context, userID uuid.UUID, req model.CreateProductRequest) (*model.ProductResponse, error) {
	store, err := s.getStoreByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}

	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		return nil, errors.New("invalid category_id")
	}

	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		return nil, errors.New("invalid price")
	}

	product := &model.Product{
		StoreID:     store.ID,
		CategoryID:  categoryID,
		Name:        req.Name,
		Description: req.Description,
		Price:       price,
		Stock:       req.Stock,
	}

	if err := s.productRepo.Create(ctx, product); err != nil {
		logger.Error(ctx, "failed to create product", err)
		return nil, errors.New("failed to create product")
	}

	logger.Info(ctx, "product created", map[string]interface{}{
		"product_id": product.ID.String(),
		"store_id":   store.ID.String(),
	})

	resp := product.ToResponse()
	return &resp, nil
}

func (s *productService) GetProducts(ctx context.Context, filter model.ProductFilter) ([]model.ProductResponse, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 10
	}

	products, total, err := s.productRepo.FindAll(ctx, filter)
	if err != nil {
		logger.Error(ctx, "failed to fetch products", err)
		return nil, 0, errors.New("failed to fetch products")
	}

	var responses []model.ProductResponse
	for _, p := range products {
		responses = append(responses, p.ToResponse())
	}

	return responses, total, nil
}

func (s *productService) GetProductByID(ctx context.Context, id uuid.UUID) (*model.ProductResponse, error) {
	product, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("product not found")
	}

	resp := product.ToResponse()
	return &resp, nil
}

func (s *productService) UpdateProduct(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateProductRequest) (*model.ProductResponse, error) {
	store, err := s.getStoreByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}

	product, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("product not found")
	}

	if product.StoreID != store.ID {
		return nil, errors.New("forbidden: not product owner")
	}

	if req.Name != "" {
		product.Name = req.Name
	}
	if req.Description != "" {
		product.Description = req.Description
	}
	if req.Price != "" {
		price, err := decimal.NewFromString(req.Price)
		if err != nil {
			return nil, errors.New("invalid price")
		}
		product.Price = price
	}
	if req.CategoryID != "" {
		categoryID, err := uuid.Parse(req.CategoryID)
		if err != nil {
			return nil, errors.New("invalid category_id")
		}
		product.CategoryID = categoryID
	}
	if req.Stock != nil {
		product.Stock = *req.Stock
	}

	if err := s.productRepo.Update(ctx, product); err != nil {
		logger.Error(ctx, "failed to update product", err)
		return nil, errors.New("failed to update product")
	}

	resp := product.ToResponse()
	return &resp, nil
}

func (s *productService) DeleteProduct(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	store, err := s.getStoreByOwner(ctx, userID)
	if err != nil {
		return err
	}

	product, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("product not found")
	}

	if product.StoreID != store.ID {
		return errors.New("forbidden: not product owner")
	}

	return s.productRepo.Delete(ctx, id)
}

func (s *productService) UpdateImage(ctx context.Context, userID uuid.UUID, id uuid.UUID, imageURL string) (*model.ProductResponse, error) {
	store, err := s.getStoreByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}

	product, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("product not found")
	}

	if product.StoreID != store.ID {
		return nil, errors.New("forbidden: not product owner")
	}

	product.ImageURL = imageURL
	if err := s.productRepo.Update(ctx, product); err != nil {
		logger.Error(ctx, "failed to update product image", err)
		return nil, errors.New("failed to update product image")
	}

	resp := product.ToResponse()
	return &resp, nil
}
