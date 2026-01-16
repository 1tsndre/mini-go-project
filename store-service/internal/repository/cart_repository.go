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
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
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

	type cartRow struct {
		ProductID uuid.UUID       `gorm:"column:product_id"`
		Quantity  int             `gorm:"column:quantity"`
		Name      string          `gorm:"column:name"`
		Price     decimal.Decimal `gorm:"column:price"`
		ImageURL  string          `gorm:"column:image_url"`
	}

	var rows []cartRow
	err = r.db.DB().WithContext(ctx).
		Table("cart_items").
		Select("cart_items.product_id, cart_items.quantity, products.name, products.price, products.image_url").
		Joins("JOIN products ON products.id = cart_items.product_id").
		Where("cart_items.user_id = ?", userID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	cart := &model.Cart{
		UserID: userID,
		Items:  make([]model.CartItem, 0, len(rows)),
	}

	for _, row := range rows {
		cart.Items = append(cart.Items, model.CartItem{
			ProductID: row.ProductID,
			Name:      row.Name,
			Price:     row.Price,
			Quantity:  row.Quantity,
			ImageURL:  row.ImageURL,
		})
	}

	r.cache.Set(ctx, cacheKey, cart, 0)

	return cart, nil
}

func (r *cartRepository) SaveCart(ctx context.Context, cart *model.Cart) error {
	cacheKey := fmt.Sprintf(constant.KeyCart, cart.UserID.String())

	if err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", cart.UserID).Delete(&model.CartItemDB{}).Error; err != nil {
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

		return tx.Create(&dbItems).Error
	}); err != nil {
		return err
	}

	r.cache.Set(ctx, cacheKey, cart, 0)
	return nil
}

func (r *cartRepository) DeleteCart(ctx context.Context, userID uuid.UUID) error {
	cacheKey := fmt.Sprintf(constant.KeyCart, userID.String())

	if err := r.db.DB().WithContext(ctx).Where("user_id = ?", userID).Delete(&model.CartItemDB{}).Error; err != nil {
		return err
	}
	r.cache.Delete(ctx, cacheKey)
	return nil
}
