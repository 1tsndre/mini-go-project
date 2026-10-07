package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
)

const reviewColumns = "id, user_id, product_id, rating, comment, created_at, updated_at"

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
	err := r.db.DB().QueryRowxContext(ctx, `
		INSERT INTO reviews (user_id, product_id, rating, comment)
		VALUES ($1, $2, $3, $4)
		RETURNING `+reviewColumns,
		review.UserID, review.ProductID, review.Rating, review.Comment,
	).StructScan(review)
	return translateError(err)
}

func (r *reviewRepository) FindByProductID(ctx context.Context, productID uuid.UUID, page, perPage int) ([]model.Review, int64, error) {
	var total int64
	if err := r.db.DB().GetContext(ctx, &total, "SELECT COUNT(*) FROM reviews WHERE product_id = $1", productID); err != nil {
		return nil, 0, translateError(err)
	}

	var reviews []model.Review
	err := r.db.DB().SelectContext(ctx, &reviews, `
		SELECT r.id, r.user_id, r.product_id, r.rating, r.comment, r.created_at, r.updated_at,
			u.name AS user_name
		FROM reviews r
		JOIN users u ON u.id = r.user_id
		WHERE r.product_id = $1
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2 OFFSET $3`,
		productID, perPage, (page-1)*perPage,
	)
	if err != nil {
		return nil, 0, translateError(err)
	}
	return reviews, total, nil
}

func (r *reviewRepository) HasUserReviewed(ctx context.Context, userID, productID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.DB().GetContext(ctx, &exists,
		"SELECT EXISTS (SELECT 1 FROM reviews WHERE user_id = $1 AND product_id = $2)",
		userID, productID,
	)
	return exists, translateError(err)
}

func (r *reviewRepository) HasUserPurchased(ctx context.Context, userID, productID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.DB().GetContext(ctx, &exists, `
		SELECT EXISTS (
			SELECT 1
			FROM order_items oi
			JOIN orders o ON o.id = oi.order_id
			WHERE o.user_id = $1 AND oi.product_id = $2 AND o.status IN ($3, $4)
		)`,
		userID, productID, constant.OrderStatusShipped, constant.OrderStatusCompleted,
	)
	return exists, translateError(err)
}
