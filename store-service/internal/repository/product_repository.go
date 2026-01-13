package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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
	UpdateImage(ctx context.Context, product *model.Product) error
}

type productRepository struct {
	db    *sqlx.DB
	cache caches.Cache
}

func NewProductRepository(db *sqlx.DB, cache caches.Cache) ProductRepository {
	return &productRepository{db: db, cache: cache}
}

func (r *productRepository) Create(ctx context.Context, product *model.Product) error {
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO products (store_id, category_id, name, description, price, stock, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+productColumns,
		product.StoreID, product.CategoryID, product.Name, product.Description,
		product.Price, product.Stock, product.ImageURL,
	).StructScan(product)
	return translateDatabaseError(err)
}

func (r *productRepository) FindAll(ctx context.Context, filter model.ProductFilter) ([]model.Product, int64, error) {
	var (
		total    int64
		products = []model.Product{}
	)

	where, args := productFilterQuery(filter)

	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM products"+where, args...); err != nil {
		return nil, 0, translateDatabaseError(err)
	}

	query := fmt.Sprintf("SELECT %s FROM products%s%s LIMIT $%d OFFSET $%d",
		productColumns, where, productOrderBy(filter), len(args)+1, len(args)+2)
	args = append(args, filter.PerPage, (filter.Page-1)*filter.PerPage)

	if err := r.db.SelectContext(ctx, &products, query, args...); err != nil {
		return nil, 0, translateDatabaseError(err)
	}
	return products, total, nil
}

func productFilterQuery(filter model.ProductFilter) (string, []any) {
	var (
		conds []string
		args  []any
	)

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
		if minPrice, err := model.ParsePrice(filter.MinPrice); err == nil {
			conds = append(conds, "price >= "+arg(minPrice))
		}
	}
	if filter.MaxPrice != "" {
		if maxPrice, err := model.ParsePrice(filter.MaxPrice); err == nil {
			conds = append(conds, "price <= "+arg(maxPrice))
		}
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func productOrderBy(filter model.ProductFilter) string {
	const (
		defaultSort = "created_at"
		asc         = "ASC"
		desc        = "DESC"
	)

	sortBy := defaultSort
	if allowedProductSortFields[filter.SortBy] {
		sortBy = filter.SortBy
	}

	sortOrder := desc
	if filter.SortOrder == constant.SortOrderAsc {
		sortOrder = asc
	}

	// id breaks ties, so no product repeats or goes missing across pages.
	return fmt.Sprintf(" ORDER BY %s %s, id %s", sortBy, sortOrder, sortOrder)
}

func (r *productRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	var product model.Product

	cached, err := r.cache.Get(ctx, fmt.Sprintf(constant.KeyProduct, id))
	if err == nil && json.Unmarshal(cached, &product) == nil {
		return &product, nil
	}

	if err := r.db.GetContext(ctx, &product, "SELECT "+productColumns+" FROM products WHERE id = $1", id); err != nil {
		return nil, translateDatabaseError(err)
	}

	r.cache.Set(ctx, fmt.Sprintf(constant.KeyProduct, id), product, constant.TTLProduct)

	return &product, nil
}

// Update leaves stock and image_url alone: stale values would undo a concurrent checkout or image upload.
func (r *productRepository) Update(ctx context.Context, product *model.Product) error {
	err := r.db.QueryRowxContext(ctx, `
		UPDATE products
		SET category_id = $1, name = $2, description = $3, price = $4, updated_at = NOW()
		WHERE id = $5
		RETURNING `+productColumns,
		product.CategoryID, product.Name, product.Description, product.Price, product.ID,
	).StructScan(product)
	if err != nil {
		return translateDatabaseError(err)
	}
	r.cache.Delete(ctx, fmt.Sprintf(constant.KeyProduct, product.ID))
	return nil
}

func (r *productRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM products WHERE id = $1", id); err != nil {
		return translateDatabaseError(err)
	}
	r.cache.Delete(ctx, fmt.Sprintf(constant.KeyProduct, id))
	return nil
}

func (r *productRepository) UpdateStock(ctx context.Context, id uuid.UUID, quantity int) error {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE products SET stock = $1, updated_at = NOW() WHERE id = $2", quantity, id); err != nil {
		return translateDatabaseError(err)
	}
	r.cache.Delete(ctx, fmt.Sprintf(constant.KeyProduct, id))
	return nil
}

func (r *productRepository) UpdateImage(ctx context.Context, product *model.Product) error {
	err := r.db.QueryRowxContext(ctx, `
		UPDATE products
		SET image_url = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING `+productColumns,
		product.ImageURL, product.ID,
	).StructScan(product)
	if err != nil {
		return translateDatabaseError(err)
	}
	r.cache.Delete(ctx, fmt.Sprintf(constant.KeyProduct, product.ID))
	return nil
}
