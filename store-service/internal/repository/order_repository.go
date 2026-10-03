package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StockReservation struct {
	ProductID uuid.UUID
	Quantity  int
}

type ErrProductNotFound struct {
	ProductID uuid.UUID
}

func (e *ErrProductNotFound) Error() string {
	return fmt.Sprintf("product %s not found", e.ProductID)
}

type ErrInsufficientStock struct {
	ProductID   uuid.UUID
	ProductName string
}

func (e *ErrInsufficientStock) Error() string {
	return fmt.Sprintf("insufficient stock for product %s", e.ProductName)
}

type OrderRepository interface {
	// CreateOrdersWithStock decrements stock for every reservation and inserts the
	// orders returned by build in a single transaction. build receives the reserved
	// products (post-decrement, in reservation order). Nothing is written unless
	// every reservation succeeds.
	CreateOrdersWithStock(ctx context.Context, reservations []StockReservation, build func(reserved []model.Product) ([]*model.Order, error)) ([]*model.Order, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Order, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.Order, int64, error)
	FindByStoreID(ctx context.Context, storeID uuid.UUID, page, perPage int) ([]model.Order, int64, error)
	FindStalePending(ctx context.Context, createdBefore time.Time, limit int) ([]model.Order, error)
	UpdateStatusIfCurrent(ctx context.Context, id uuid.UUID, fromStatus, toStatus string) (bool, error)
	// CancelAndRestock moves the order from fromStatus to cancelled, returns its items
	// to stock and closes a still-pending payment, all in one transaction. It reports
	// false if the order was no longer in fromStatus.
	CancelAndRestock(ctx context.Context, id uuid.UUID, fromStatus string) (bool, error)
	// MarkPaymentSucceeded moves a pending order to paid and records the payment as
	// successful. It reports false if the order was no longer pending.
	MarkPaymentSucceeded(ctx context.Context, orderID uuid.UUID) (bool, error)
	// MarkPaymentFailed cancels a pending order, restocks its items and records the
	// payment as failed. It reports false if the order was no longer pending.
	MarkPaymentFailed(ctx context.Context, orderID uuid.UUID) (bool, error)
}

type orderRepository struct {
	db    databases.Database
	cache caches.Cache
}

func NewOrderRepository(db databases.Database, cache caches.Cache) OrderRepository {
	return &orderRepository{db: db, cache: cache}
}

func (r *orderRepository) CreateOrdersWithStock(ctx context.Context, reservations []StockReservation, build func(reserved []model.Product) ([]*model.Order, error)) ([]*model.Order, error) {
	var orders []*model.Order

	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reserved := make([]model.Product, 0, len(reservations))
		for _, res := range reservations {
			product := model.Product{ID: res.ProductID}
			// The conditional decrement takes a row lock, so concurrent checkouts of the
			// same product serialize here and can never drive stock below zero.
			result := tx.Model(&product).
				Clauses(clause.Returning{}).
				Where("stock >= ?", res.Quantity).
				Update("stock", gorm.Expr("stock - ?", res.Quantity))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return r.reservationError(tx, res.ProductID)
			}
			reserved = append(reserved, product)
		}

		built, err := build(reserved)
		if err != nil {
			return err
		}
		for _, order := range built {
			if err := tx.Create(order).Error; err != nil {
				return err
			}
		}
		orders = built
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, res := range reservations {
		r.invalidateProduct(ctx, res.ProductID)
	}
	return orders, nil
}

// reservationError explains why a conditional stock decrement matched no row.
func (r *orderRepository) reservationError(tx *gorm.DB, productID uuid.UUID) error {
	var product model.Product
	err := tx.Select("id", "name").First(&product, "id = ?", productID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &ErrProductNotFound{ProductID: productID}
	}
	if err != nil {
		return err
	}
	return &ErrInsufficientStock{ProductID: productID, ProductName: product.Name}
}

func (r *orderRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Order, error) {
	var order model.Order
	err := r.db.DB().WithContext(ctx).
		Preload("OrderItems").
		Preload("Payment").
		First(&order, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) FindByUserID(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.Order, int64, error) {
	var orders []model.Order
	var total int64

	query := r.db.DB().WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	err := query.
		Preload("OrderItems").
		Preload("Payment").
		Order("created_at DESC").
		Offset(offset).
		Limit(perPage).
		Find(&orders).Error

	return orders, total, err
}

func (r *orderRepository) FindByStoreID(ctx context.Context, storeID uuid.UUID, page, perPage int) ([]model.Order, int64, error) {
	var orders []model.Order
	var total int64

	query := r.db.DB().WithContext(ctx).Model(&model.Order{}).Where("store_id = ?", storeID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	err := r.db.DB().WithContext(ctx).
		Preload("OrderItems").
		Preload("Payment").
		Where("store_id = ?", storeID).
		Order("created_at DESC").
		Offset(offset).
		Limit(perPage).
		Find(&orders).Error

	return orders, total, err
}

func (r *orderRepository) FindStalePending(ctx context.Context, createdBefore time.Time, limit int) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.DB().WithContext(ctx).
		Where("status = ? AND created_at < ?", constant.OrderStatusPending, createdBefore).
		Order("created_at ASC").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func (r *orderRepository) UpdateStatusIfCurrent(ctx context.Context, id uuid.UUID, fromStatus, toStatus string) (bool, error) {
	res := r.db.DB().WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Update("status", toStatus)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *orderRepository) CancelAndRestock(ctx context.Context, id uuid.UUID, fromStatus string) (bool, error) {
	return r.cancelAndRestock(ctx, id, fromStatus, model.PaymentStatusCancelled)
}

func (r *orderRepository) MarkPaymentFailed(ctx context.Context, orderID uuid.UUID) (bool, error) {
	return r.cancelAndRestock(ctx, orderID, constant.OrderStatusPending, model.PaymentStatusFailed)
}

func (r *orderRepository) cancelAndRestock(ctx context.Context, id uuid.UUID, fromStatus, pendingPaymentStatus string) (bool, error) {
	var items []model.OrderItem
	cancelled := false

	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", id, fromStatus).
			Update("status", constant.OrderStatusCancelled)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		cancelled = true

		// Restock in product_id order, the same order checkout reserves in, so a
		// concurrent checkout and cancel cannot deadlock on product row locks.
		if err := tx.Where("order_id = ?", id).Order("product_id").Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			if err := tx.Model(&model.Product{ID: item.ProductID}).
				Update("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
				return err
			}
		}

		return tx.Model(&model.Payment{}).
			Where("order_id = ? AND status = ?", id, model.PaymentStatusPending).
			Update("status", pendingPaymentStatus).Error
	})
	if err != nil {
		return false, err
	}

	for _, item := range items {
		r.invalidateProduct(ctx, item.ProductID)
	}
	return cancelled, nil
}

func (r *orderRepository) MarkPaymentSucceeded(ctx context.Context, orderID uuid.UUID) (bool, error) {
	paid := false

	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", orderID, constant.OrderStatusPending).
			Update("status", constant.OrderStatusPaid)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		paid = true

		return tx.Model(&model.Payment{}).
			Where("order_id = ? AND status = ?", orderID, model.PaymentStatusPending).
			Updates(map[string]any{"status": model.PaymentStatusSuccess, "paid_at": time.Now()}).Error
	})
	return paid, err
}

func (r *orderRepository) invalidateProduct(ctx context.Context, productID uuid.UUID) {
	r.cache.Delete(ctx, fmt.Sprintf(constant.KeyProduct, productID.String()))
}
