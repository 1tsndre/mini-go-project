package service

import (
	"context"
	"errors"
	"math"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ProductService interface {
	CreateProduct(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, req model.CreateProductRequest) (*model.Product, error)
	GetProducts(ctx context.Context, filter model.ProductFilter) ([]model.Product, int64, error)
	GetProductByID(ctx context.Context, id uuid.UUID) (*model.Product, error)
	UpdateProduct(ctx context.Context, userID uuid.UUID, id uuid.UUID, categoryID uuid.UUID, req model.UpdateProductRequest) (*model.Product, error)
	DeleteProduct(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	UpdateImage(ctx context.Context, userID uuid.UUID, id uuid.UUID, imageURL string) (*model.Product, error)
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

var maxPrice = decimal.New(1, 13) // DECIMAL(15,2)

func (s *productService) getStoreByOwner(ctx context.Context, userID uuid.UUID) (*model.Store, error) {
	store, err := s.storeRepo.FindByUserID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgStoreNotFoundForUser}
	}
	if err != nil {
		logger.Error(ctx, constant.LogStoreLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	return store, nil
}

func (s *productService) findOwned(ctx context.Context, userID, id uuid.UUID) (*model.Product, error) {
	store, err := s.getStoreByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}

	product, err := s.GetProductByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if product.StoreID != store.ID {
		return nil, &Error{Code: constant.ErrCodeForbidden, Message: constant.MsgNotProductOwner}
	}

	return product, nil
}

func (s *productService) CreateProduct(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, req model.CreateProductRequest) (*model.Product, error) {
	store, err := s.getStoreByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}

	price, err := model.ParsePrice(req.Price)
	switch {
	case err != nil:
		return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgInvalidPrice}
	case !price.IsPositive():
		return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgPriceNotPositive}
	case price.GreaterThanOrEqual(maxPrice):
		return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgPriceTooLarge}
	case !price.Equal(price.Round(constant.PriceDecimals)):
		return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgPriceDecimals}
	}

	switch {
	case req.Stock < 0:
		return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgStockNegative}
	case req.Stock > math.MaxInt32:
		return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgStockTooLarge}
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
		if errors.Is(err, repository.ErrForeignKeyViolation) {
			return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgCategoryNotFound}
		}
		logger.Error(ctx, constant.LogProductCreateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgProductCreateFailed}
	}

	logger.Info(ctx, constant.LogProductCreated, logger.Fields{
		"product_id": product.ID.String(),
		"store_id":   store.ID.String(),
	})

	return product, nil
}

func (s *productService) GetProducts(ctx context.Context, filter model.ProductFilter) ([]model.Product, int64, error) {
	products, total, err := s.productRepo.FindAll(ctx, filter)
	if err != nil {
		logger.Error(ctx, constant.LogProductFetchFailed, err)
		return nil, 0, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgProductFetchFailed}
	}

	return products, total, nil
}

func (s *productService) GetProductByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	product, err := s.productRepo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgProductNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogProductLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	return product, nil
}

func (s *productService) UpdateProduct(ctx context.Context, userID uuid.UUID, id uuid.UUID, categoryID uuid.UUID, req model.UpdateProductRequest) (*model.Product, error) {
	product, err := s.findOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		product.Name = req.Name
	}
	if req.Description != "" {
		product.Description = req.Description
	}

	if req.Price != "" {
		price, err := model.ParsePrice(req.Price)
		switch {
		case err != nil:
			return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgInvalidPrice}
		case !price.IsPositive():
			return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgPriceNotPositive}
		case price.GreaterThanOrEqual(maxPrice):
			return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgPriceTooLarge}
		case !price.Equal(price.Round(constant.PriceDecimals)):
			return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgPriceDecimals}
		}
		product.Price = price
	}

	if categoryID != uuid.Nil {
		product.CategoryID = categoryID
	}

	if req.Stock != nil {
		switch {
		case *req.Stock < 0:
			return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgStockNegative}
		case *req.Stock > math.MaxInt32:
			return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgStockTooLarge}
		}
	}

	if err := s.productRepo.Update(ctx, product); err != nil {
		if errors.Is(err, repository.ErrForeignKeyViolation) {
			return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgCategoryNotFound}
		}
		logger.Error(ctx, constant.LogProductUpdateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgProductUpdateFailed}
	}

	if req.Stock != nil {
		if err := s.productRepo.UpdateStock(ctx, id, *req.Stock); err != nil {
			logger.Error(ctx, constant.LogProductStockUpdateFailed, err)
			return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgProductStockUpdateFailed}
		}
		product.Stock = *req.Stock
	}

	return product, nil
}

func (s *productService) DeleteProduct(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	if _, err := s.findOwned(ctx, userID, id); err != nil {
		return err
	}

	if err := s.productRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrForeignKeyViolation) {
			return &Error{Code: constant.ErrCodeConflict, Message: constant.MsgProductInUse}
		}
		logger.Error(ctx, constant.LogProductDeleteFailed, err)
		return &Error{Code: constant.ErrCodeInternal, Message: constant.MsgProductDeleteFailed}
	}
	return nil
}

func (s *productService) UpdateImage(ctx context.Context, userID uuid.UUID, id uuid.UUID, imageURL string) (*model.Product, error) {
	product, err := s.findOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	product.ImageURL = imageURL
	if err := s.productRepo.UpdateImage(ctx, product); err != nil {
		logger.Error(ctx, constant.LogProductImageUpdateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgProductImageUpdateFailed}
	}

	return product, nil
}
