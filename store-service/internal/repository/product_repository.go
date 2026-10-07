package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const productColumns = "id, store_id, category_id, name, description, price, stock, image_url, created_at, updated_at"

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
	err := r.db.DB().QueryRowxContext(ctx, `
		INSERT INTO products (store_id, category_id, name, description, price, stock, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+productColumns,
		product.StoreID, product.CategoryID, product.Name, product.Description,
		product.Price, product.Stock, product.ImageURL,
	).StructScan(product)
	return translateError(err)
}

func (r *productRepository) FindAll(ctx context.Context, filter model.ProductFilter) ([]model.Product, int64, error) {
	where, args := productFilterQuery(filter)

	var total int64
	if err := r.db.DB().GetContext(ctx, &total, "SELECT COUNT(*) FROM products"+where, args...); err != nil {
		return nil, 0, translateError(err)
	}

	query := fmt.Sprintf("SELECT %s FROM products%s%s LIMIT $%d OFFSET $%d",
		productColumns, where, productOrderBy(filter), len(args)+1, len(args)+2)
	args = append(args, filter.PerPage, (filter.Page-1)*filter.PerPage)

	var products []model.Product
	if err := r.db.DB().SelectContext(ctx, &products, query, args...); err != nil {
		return nil, 0, translateError(err)
	}
	return products, total, nil
}

// productFilterQuery builds the WHERE clause for filter, with placeholders
// numbered from $1, and returns it with the matching arguments.
func productFilterQuery(filter model.ProductFilter) (string, []any) {
	var conds []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.CategoryID != "" {
		conds = append(conds, "category_id = "+arg(filter.CategoryID))
	}
	if filter.StoreID != "" {
		conds = append(conds, "store_id = "+arg(filter.StoreID))
	}
	if filter.Search != "" {
		search := arg("%" + filter.Search + "%")
		conds = append(conds, "(name ILIKE "+search+" OR description ILIKE "+search+")")
	}
	if filter.MinPrice != "" {
		if minPrice, err := decimal.NewFromString(filter.MinPrice); err == nil {
			conds = append(conds, "price >= "+arg(minPrice))
		}
	}
	if filter.MaxPrice != "" {
		if maxPrice, err := decimal.NewFromString(filter.MaxPrice); err == nil {
			conds = append(conds, "price <= "+arg(maxPrice))
		}
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// productOrderBy returns the ORDER BY clause for filter. The column comes from
// allowedProductSortFields, so the clause is safe to put into the query.
func productOrderBy(filter model.ProductFilter) string {
	sortBy := "created_at"
	if allowedProductSortFields[filter.SortBy] {
		sortBy = filter.SortBy
	}
	sortOrder := "DESC"
	if filter.SortOrder == "asc" {
		sortOrder = "ASC"
	}
	// id breaks ties, so products with equal values are neither repeated nor
	// skipped across pages.
	return fmt.Sprintf(" ORDER BY %s %s, id %s", sortBy, sortOrder, sortOrder)
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
	if err := r.db.DB().GetContext(ctx, &product, "SELECT "+productColumns+" FROM products WHERE id = $1", id); err != nil {
		return nil, translateError(err)
	}

	r.cache.Set(ctx, cacheKey, product, constant.TTLProduct)

	return &product, nil
}

// Update never writes stock: stock changes go through UpdateStock or the atomic
// checkout/cancel paths, otherwise a stale read here would overwrite a
// concurrent checkout's decrement. The stored row, including its current stock,
// is read back into product.
func (r *productRepository) Update(ctx context.Context, product *model.Product) error {
	err := r.db.DB().QueryRowxContext(ctx, `
		UPDATE products
		SET category_id = $1, name = $2, description = $3, price = $4, image_url = $5, updated_at = NOW()
		WHERE id = $6
		RETURNING `+productColumns,
		product.CategoryID, product.Name, product.Description, product.Price, product.ImageURL, product.ID,
	).StructScan(product)
	if err != nil {
		return translateError(err)
	}
	cacheKey := fmt.Sprintf(constant.KeyProduct, product.ID.String())
	r.cache.Delete(ctx, cacheKey)
	return nil
}

func (r *productRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.DB().ExecContext(ctx, "DELETE FROM products WHERE id = $1", id); err != nil {
		return translateError(err)
	}
	cacheKey := fmt.Sprintf(constant.KeyProduct, id.String())
	r.cache.Delete(ctx, cacheKey)
	return nil
}

func (r *productRepository) UpdateStock(ctx context.Context, id uuid.UUID, quantity int) error {
	if _, err := r.db.DB().ExecContext(ctx,
		"UPDATE products SET stock = $1, updated_at = NOW() WHERE id = $2", quantity, id); err != nil {
		return translateError(err)
	}
	cacheKey := fmt.Sprintf(constant.KeyProduct, id.String())
	r.cache.Delete(ctx, cacheKey)
	return nil
}
