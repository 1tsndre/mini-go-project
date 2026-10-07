package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
)

const categoryColumns = "id, name, created_at, updated_at"

type CategoryRepository interface {
	Create(ctx context.Context, category *model.Category) error
	FindAll(ctx context.Context) ([]model.Category, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Category, error)
	Update(ctx context.Context, category *model.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type categoryRepository struct {
	db databases.Database
}

func NewCategoryRepository(db databases.Database) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) Create(ctx context.Context, category *model.Category) error {
	err := r.db.DB().QueryRowxContext(ctx,
		"INSERT INTO categories (name) VALUES ($1) RETURNING "+categoryColumns,
		category.Name,
	).StructScan(category)
	return translateError(err)
}

func (r *categoryRepository) FindAll(ctx context.Context) ([]model.Category, error) {
	var categories []model.Category
	err := r.db.DB().SelectContext(ctx, &categories, "SELECT "+categoryColumns+" FROM categories ORDER BY name ASC")
	return categories, translateError(err)
}

func (r *categoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	var category model.Category
	err := r.db.DB().GetContext(ctx, &category, "SELECT "+categoryColumns+" FROM categories WHERE id = $1", id)
	if err != nil {
		return nil, translateError(err)
	}
	return &category, nil
}

func (r *categoryRepository) Update(ctx context.Context, category *model.Category) error {
	err := r.db.DB().QueryRowxContext(ctx,
		"UPDATE categories SET name = $1, updated_at = NOW() WHERE id = $2 RETURNING "+categoryColumns,
		category.Name, category.ID,
	).StructScan(category)
	return translateError(err)
}

func (r *categoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.DB().ExecContext(ctx, "DELETE FROM categories WHERE id = $1", id)
	return translateError(err)
}
