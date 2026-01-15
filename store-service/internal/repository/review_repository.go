package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *model.Review) error
	FindByProductID(ctx context.Context, productID uuid.UUID, page, perPage int) ([]model.Review, int64, error)
	HasUserReviewed(ctx context.Context, userID, productID uuid.UUID) (bool, error)
	HasUserPurchased(ctx context.Context, userID, productID uuid.UUID) (bool, error)
}

type reviewRepository struct {
	db databases.Database
}

func NewReviewRepository(db databases.Database) ReviewRepository {
	return &reviewRepository{db: db}
}

func (r *reviewRepository) Create(ctx context.Context, review *model.Review) error {
	return r.db.DB().WithContext(ctx).Create(review).Error
}

func (r *reviewRepository) FindByProductID(ctx context.Context, productID uuid.UUID, page, perPage int) ([]model.Review, int64, error) {
	var reviews []model.Review
	var total int64

	query := r.db.DB().WithContext(ctx).Model(&model.Review{}).Where("product_id = ?", productID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	err := query.
		Preload("User").
		Order("created_at DESC").
		Offset(offset).
		Limit(perPage).
		Find(&reviews).Error

	return reviews, total, err
}

func (r *reviewRepository) HasUserReviewed(ctx context.Context, userID, productID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.Review{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Count(&count).Error
	return count > 0, err
}

func (r *reviewRepository) HasUserPurchased(ctx context.Context, userID, productID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.OrderItem{}).
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.user_id = ? AND order_items.product_id = ? AND orders.status IN (?, ?)",
			userID, productID, constant.OrderStatusShipped, constant.OrderStatusCompleted).
		Count(&count).Error
	return count > 0, err
}
