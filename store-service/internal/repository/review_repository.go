package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const reviewColumns = "id, user_id, product_id, rating, comment, created_at, updated_at"

type ReviewRepository interface {
	Create(ctx context.Context, review *model.Review) error
	FindByProductID(ctx context.Context, productID uuid.UUID, page, perPage int) ([]model.Review, int64, error)
	HasUserPurchased(ctx context.Context, userID, productID uuid.UUID) (bool, error)
}

type reviewRepository struct {
	db *sqlx.DB
}

func NewReviewRepository(db *sqlx.DB) ReviewRepository {
	return &reviewRepository{db: db}
}

func (r *reviewRepository) Create(ctx context.Context, review *model.Review) error {
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO reviews (user_id, product_id, rating, comment)
		VALUES ($1, $2, $3, $4)
		RETURNING `+reviewColumns+`, (SELECT name FROM users WHERE id = $1) AS user_name`,
		review.UserID, review.ProductID, review.Rating, review.Comment,
	).StructScan(review)
	return translateDatabaseError(err)
}

func (r *reviewRepository) FindByProductID(ctx context.Context, productID uuid.UUID, page, perPage int) ([]model.Review, int64, error) {
	var (
		total   int64
		reviews []model.Review
	)

	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM reviews WHERE product_id = $1", productID); err != nil {
		return nil, 0, translateDatabaseError(err)
	}

	err := r.db.SelectContext(ctx, &reviews, `
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
		return nil, 0, translateDatabaseError(err)
	}
	return reviews, total, nil
}

func (r *reviewRepository) HasUserPurchased(ctx context.Context, userID, productID uuid.UUID) (bool, error) {
	var exists bool

	err := r.db.GetContext(ctx, &exists, `
		SELECT EXISTS (
			SELECT 1
			FROM order_items oi
			JOIN orders o ON o.id = oi.order_id
			WHERE o.user_id = $1 AND oi.product_id = $2 AND o.status IN ($3, $4)
		)`,
		userID, productID, constant.OrderStatusShipped, constant.OrderStatusCompleted,
	)
	return exists, translateDatabaseError(err)
}
