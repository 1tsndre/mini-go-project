package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type CartRepository interface {
	GetCart(ctx context.Context, userID uuid.UUID) (*model.Cart, error)
	SaveCart(ctx context.Context, cart *model.Cart) error
	DeleteCart(ctx context.Context, userID uuid.UUID) error
}

type cartRepository struct {
	db    databases.Database
	cache caches.Cache
}

func NewCartRepository(db databases.Database, cache caches.Cache) CartRepository {
	return &cartRepository{db: db, cache: cache}
}

func (r *cartRepository) GetCart(ctx context.Context, userID uuid.UUID) (*model.Cart, error) {
	cacheKey := fmt.Sprintf(constant.KeyCart, userID.String())

	cached, err := r.cache.Get(ctx, cacheKey)
	if err == nil {
		var cart model.Cart
		if json.Unmarshal(cached, &cart) == nil {
			return &cart, nil
		}
	}

	cart := &model.Cart{
		UserID: userID,
		Items:  []model.CartItem{},
	}
	err = r.db.DB().SelectContext(ctx, &cart.Items, `
		SELECT ci.product_id, ci.quantity, p.name, p.price, p.image_url
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, translateError(err)
	}

	r.cache.Set(ctx, cacheKey, cart, constant.TTLCart)

	return cart, nil
}

func (r *cartRepository) SaveCart(ctx context.Context, cart *model.Cart) error {
	cacheKey := fmt.Sprintf(constant.KeyCart, cart.UserID.String())

	err := withTx(ctx, r.db.DB(), func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM cart_items WHERE user_id = $1", cart.UserID); err != nil {
			return err
		}

		if len(cart.Items) == 0 {
			return nil
		}

		dbItems := make([]model.CartItemDB, 0, len(cart.Items))
		for _, item := range cart.Items {
			dbItems = append(dbItems, model.CartItemDB{
				UserID:    cart.UserID,
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
			})
		}

		_, err := tx.NamedExecContext(ctx,
			"INSERT INTO cart_items (user_id, product_id, quantity) VALUES (:user_id, :product_id, :quantity)",
			dbItems,
		)
		return err
	})
	if err != nil {
		return translateError(err)
	}

	r.cache.Set(ctx, cacheKey, cart, constant.TTLCart)
	return nil
}

func (r *cartRepository) DeleteCart(ctx context.Context, userID uuid.UUID) error {
	cacheKey := fmt.Sprintf(constant.KeyCart, userID.String())

	if _, err := r.db.DB().ExecContext(ctx, "DELETE FROM cart_items WHERE user_id = $1", userID); err != nil {
		return translateError(err)
	}
	r.cache.Delete(ctx, cacheKey)
	return nil
}
