package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var allowedProductSortFields = map[string]bool{
	"price":      true,
	"name":       true,
	"created_at": true,
}

type ProductRepository interface {
	Create(ctx context.Context, product *model.Product) error
	FindAll(ctx context.Context, filter model.ProductFilter) ([]model.Product, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error)
	Update(ctx context.Context, product *model.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateStock(ctx context.Context, id uuid.UUID, quantity int) error
}

type productRepository struct {
	db    databases.Database
	cache caches.Cache
}

func NewProductRepository(db databases.Database, cache caches.Cache) ProductRepository {
	return &productRepository{db: db, cache: cache}
}

func (r *productRepository) Create(ctx context.Context, product *model.Product) error {
	return r.db.DB().WithContext(ctx).Create(product).Error
}

func (r *productRepository) FindAll(ctx context.Context, filter model.ProductFilter) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	query := r.db.DB().WithContext(ctx).Model(&model.Product{})

	if filter.CategoryID != "" {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if filter.StoreID != "" {
		query = query.Where("store_id = ?", filter.StoreID)
	}
	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", search, search)
	}
	if filter.MinPrice != "" {
		if minPrice, err := decimal.NewFromString(filter.MinPrice); err == nil {
			query = query.Where("price >= ?", minPrice)
		}
	}
	if filter.MaxPrice != "" {
		if maxPrice, err := decimal.NewFromString(filter.MaxPrice); err == nil {
			query = query.Where("price <= ?", maxPrice)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortBy := "created_at"
	if filter.SortBy != "" {
		if allowedProductSortFields[filter.SortBy] {
			sortBy = filter.SortBy
		}
	}
	sortOrder := "DESC"
	if filter.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	offset := (filter.Page - 1) * filter.PerPage
	err := query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder)).
		Offset(offset).
		Limit(filter.PerPage).
		Find(&products).Error

	return products, total, err
}

func (r *productRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	cacheKey := fmt.Sprintf(constant.KeyProduct, id.String())

	cached, err := r.cache.Get(ctx, cacheKey)
	if err == nil {
		var product model.Product
		if json.Unmarshal(cached, &product) == nil {
			return &product, nil
		}
	}

	var product model.Product
	err = r.db.DB().WithContext(ctx).First(&product, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	r.cache.Set(ctx, cacheKey, product, 15*time.Minute)

	return &product, nil
}

func (r *productRepository) Update(ctx context.Context, product *model.Product) error {
	if err := r.db.DB().WithContext(ctx).Save(product).Error; err != nil {
		return err
	}
	cacheKey := fmt.Sprintf(constant.KeyProduct, product.ID.String())
	r.cache.Delete(ctx, cacheKey)
	return nil
}

func (r *productRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.DB().WithContext(ctx).Delete(&model.Product{}, "id = ?", id).Error; err != nil {
		return err
	}
	cacheKey := fmt.Sprintf(constant.KeyProduct, id.String())
	r.cache.Delete(ctx, cacheKey)
	return nil
}

func (r *productRepository) UpdateStock(ctx context.Context, id uuid.UUID, quantity int) error {
	if err := r.db.DB().WithContext(ctx).
		Model(&model.Product{}).
		Where("id = ?", id).
		Update("stock", quantity).Error; err != nil {
		return err
	}
	cacheKey := fmt.Sprintf(constant.KeyProduct, id.String())
	r.cache.Delete(ctx, cacheKey)
	return nil
}
