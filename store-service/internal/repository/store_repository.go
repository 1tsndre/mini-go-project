package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
)

type StoreRepository interface {
	Create(ctx context.Context, store *model.Store) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Store, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Store, error)
	Update(ctx context.Context, store *model.Store) error
}

type storeRepository struct {
	db databases.Database
}

func NewStoreRepository(db databases.Database) StoreRepository {
	return &storeRepository{db: db}
}

func (r *storeRepository) Create(ctx context.Context, store *model.Store) error {
	return r.db.DB().WithContext(ctx).Create(store).Error
}

func (r *storeRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Store, error) {
	var store model.Store
	err := r.db.DB().WithContext(ctx).First(&store, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &store, nil
}

func (r *storeRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Store, error) {
	var store model.Store
	err := r.db.DB().WithContext(ctx).First(&store, "user_id = ?", userID).Error
	if err != nil {
		return nil, err
	}
	return &store, nil
}

func (r *storeRepository) Update(ctx context.Context, store *model.Store) error {
	return r.db.DB().WithContext(ctx).Save(store).Error
}
