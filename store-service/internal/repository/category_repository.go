package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const categoryColumns = "id, name, created_at, updated_at"

type CategoryRepository interface {
	Create(ctx context.Context, category *model.Category) error
	FindAll(ctx context.Context) ([]model.Category, error)
	Update(ctx context.Context, category *model.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type categoryRepository struct {
	db *sqlx.DB
}

func NewCategoryRepository(db *sqlx.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) Create(ctx context.Context, category *model.Category) error {
	err := r.db.QueryRowxContext(ctx,
		"INSERT INTO categories (name) VALUES ($1) RETURNING "+categoryColumns,
		category.Name,
	).StructScan(category)
	return translateDatabaseError(err)
}

func (r *categoryRepository) FindAll(ctx context.Context) ([]model.Category, error) {
	categories := []model.Category{}

	err := r.db.SelectContext(ctx, &categories, "SELECT "+categoryColumns+" FROM categories ORDER BY name ASC")
	return categories, translateDatabaseError(err)
}

func (r *categoryRepository) Update(ctx context.Context, category *model.Category) error {
	err := r.db.QueryRowxContext(ctx,
		"UPDATE categories SET name = $1, updated_at = NOW() WHERE id = $2 RETURNING "+categoryColumns,
		category.Name, category.ID,
	).StructScan(category)
	return translateDatabaseError(err)
}

func (r *categoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.db.GetContext(ctx, &id, "DELETE FROM categories WHERE id = $1 RETURNING id", id)
	return translateDatabaseError(err)
}
