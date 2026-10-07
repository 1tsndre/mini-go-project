package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
)

const storeColumns = "id, user_id, name, description, logo_url, created_at, updated_at"

type StoreRepository interface {
	Create(ctx context.Context, store *model.Store) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Store, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Store, error)
	Update(ctx context.Context, store *model.Store) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type storeRepository struct {
	db databases.Database
}

func NewStoreRepository(db databases.Database) StoreRepository {
	return &storeRepository{db: db}
}

func (r *storeRepository) Create(ctx context.Context, store *model.Store) error {
	err := r.db.DB().QueryRowxContext(ctx, `
		INSERT INTO stores (user_id, name, description, logo_url)
		VALUES ($1, $2, $3, $4)
		RETURNING `+storeColumns,
		store.UserID, store.Name, store.Description, store.LogoURL,
	).StructScan(store)
	return translateError(err)
}

func (r *storeRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Store, error) {
	var store model.Store
	err := r.db.DB().GetContext(ctx, &store, "SELECT "+storeColumns+" FROM stores WHERE id = $1", id)
	if err != nil {
		return nil, translateError(err)
	}
	return &store, nil
}

func (r *storeRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Store, error) {
	var store model.Store
	err := r.db.DB().GetContext(ctx, &store, "SELECT "+storeColumns+" FROM stores WHERE user_id = $1", userID)
	if err != nil {
		return nil, translateError(err)
	}
	return &store, nil
}

func (r *storeRepository) Update(ctx context.Context, store *model.Store) error {
	err := r.db.DB().QueryRowxContext(ctx, `
		UPDATE stores
		SET name = $1, description = $2, logo_url = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING `+storeColumns,
		store.Name, store.Description, store.LogoURL, store.ID,
	).StructScan(store)
	return translateError(err)
}

func (r *storeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.DB().ExecContext(ctx, "DELETE FROM stores WHERE id = $1", id)
	return translateError(err)
}
