package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const storeColumns = "id, user_id, name, description, logo_url, created_at, updated_at"

type StoreRepository interface {
	// Create also makes the owner a seller.
	Create(ctx context.Context, store *model.Store) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Store, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Store, error)
	// Update leaves logo_url alone, so a concurrent logo upload is not undone.
	Update(ctx context.Context, store *model.Store) error
	UpdateLogo(ctx context.Context, store *model.Store) error
}

type storeRepository struct {
	db *sqlx.DB
}

func NewStoreRepository(db *sqlx.DB) StoreRepository {
	return &storeRepository{db: db}
}

func (r *storeRepository) Create(ctx context.Context, store *model.Store) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return translateDatabaseError(err)
	}
	defer tx.Rollback() // no-op after Commit

	err = tx.QueryRowxContext(ctx, `
		INSERT INTO stores (user_id, name, description, logo_url)
		VALUES ($1, $2, $3, $4)
		RETURNING `+storeColumns,
		store.UserID, store.Name, store.Description, store.LogoURL,
	).StructScan(store)
	if err != nil {
		return translateDatabaseError(err)
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2", constant.RoleSeller, store.UserID)
	if err != nil {
		return translateDatabaseError(err)
	}

	if err := tx.Commit(); err != nil {
		return translateDatabaseError(err)
	}
	return nil
}

func (r *storeRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Store, error) {
	var store model.Store

	err := r.db.GetContext(ctx, &store, "SELECT "+storeColumns+" FROM stores WHERE id = $1", id)
	if err != nil {
		return nil, translateDatabaseError(err)
	}
	return &store, nil
}

func (r *storeRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Store, error) {
	var store model.Store

	err := r.db.GetContext(ctx, &store, "SELECT "+storeColumns+" FROM stores WHERE user_id = $1", userID)
	if err != nil {
		return nil, translateDatabaseError(err)
	}
	return &store, nil
}

func (r *storeRepository) Update(ctx context.Context, store *model.Store) error {
	err := r.db.QueryRowxContext(ctx, `
		UPDATE stores
		SET name = $1, description = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING `+storeColumns,
		store.Name, store.Description, store.ID,
	).StructScan(store)
	return translateDatabaseError(err)
}

func (r *storeRepository) UpdateLogo(ctx context.Context, store *model.Store) error {
	err := r.db.QueryRowxContext(ctx, `
		UPDATE stores
		SET logo_url = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING `+storeColumns,
		store.LogoURL, store.ID,
	).StructScan(store)
	return translateDatabaseError(err)
}
