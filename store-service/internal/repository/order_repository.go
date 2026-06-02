package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderRepository interface {
	CreateOrders(ctx context.Context, orders []*model.Order) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Order, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.Order, int64, error)
	FindByStoreID(ctx context.Context, storeID uuid.UUID, page, perPage int) ([]model.Order, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateStatusIfCurrent(ctx context.Context, id uuid.UUID, fromStatus, toStatus string) (bool, error)
	CreatePayment(ctx context.Context, payment *model.Payment) error
	UpdatePayment(ctx context.Context, payment *model.Payment) error
	FindPaymentByOrderID(ctx context.Context, orderID uuid.UUID) (*model.Payment, error)
}

type orderRepository struct {
	db databases.Database
}

func NewOrderRepository(db databases.Database) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) CreateOrders(ctx context.Context, orders []*model.Order) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, order := range orders {
			if err := tx.Create(order).Error; err != nil {
				return err
			}
		}
		return nil
	})
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

func (r *orderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.DB().WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ?", id).
		Update("status", status).Error
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

func (r *orderRepository) CreatePayment(ctx context.Context, payment *model.Payment) error {
	return r.db.DB().WithContext(ctx).Create(payment).Error
}

func (r *orderRepository) UpdatePayment(ctx context.Context, payment *model.Payment) error {
	return r.db.DB().WithContext(ctx).Save(payment).Error
}

func (r *orderRepository) FindPaymentByOrderID(ctx context.Context, orderID uuid.UUID) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.DB().WithContext(ctx).First(&payment, "order_id = ?", orderID).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}
