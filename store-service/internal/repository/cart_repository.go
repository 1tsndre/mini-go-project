package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type CartRepository interface {
	FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Cart, error)
	Save(ctx context.Context, cart *model.Cart) error
}

type cartRepository struct {
	db    *sqlx.DB
	cache caches.Cache
}

func NewCartRepository(db *sqlx.DB, cache caches.Cache) CartRepository {
	return &cartRepository{db: db, cache: cache}
}

func (r *cartRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Cart, error) {
	cart := &model.Cart{}

	cached, err := r.cache.Get(ctx, fmt.Sprintf(constant.KeyCart, userID))
	if err == nil && json.Unmarshal(cached, cart) == nil {
		return cart, nil
	}

	// Only Save writes Redis, so a read cannot put back an old cart.
	cart = &model.Cart{
		UserID: userID,
		Items:  []model.CartItem{},
	}

	err = r.db.SelectContext(ctx, &cart.Items, `
		SELECT ci.product_id, ci.quantity, p.name, p.price, p.image_url
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, translateDatabaseError(err)
	}
	return cart, nil
}

func (r *cartRepository) Save(ctx context.Context, cart *model.Cart) error {
	// Dropped first, so a failing Redis stops the save instead of keeping an old cart.
	if err := r.cache.Delete(ctx, fmt.Sprintf(constant.KeyCart, cart.UserID)); err != nil {
		return err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return translateDatabaseError(err)
	}
	defer tx.Rollback() // no-op after Commit

	if _, err := tx.ExecContext(ctx, "DELETE FROM cart_items WHERE user_id = $1", cart.UserID); err != nil {
		return translateDatabaseError(err)
	}

	for _, item := range cart.Items {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO cart_items (user_id, product_id, quantity) VALUES ($1, $2, $3)",
			cart.UserID, item.ProductID, item.Quantity,
		)
		if err != nil {
			return translateDatabaseError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return translateDatabaseError(err)
	}

	r.cache.Set(ctx, fmt.Sprintf(constant.KeyCart, cart.UserID), cart, constant.TTLCart)
	return nil
}
