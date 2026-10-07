package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	orderColumns     = "id, user_id, store_id, status, total_amount, shipping_address, created_at, updated_at"
	orderItemColumns = "id, order_id, product_id, quantity, price, created_at"
	paymentColumns   = "id, order_id, method, status, amount, paid_at, created_at, updated_at"
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

	err := withTx(ctx, r.db.DB(), func(tx *sqlx.Tx) error {
		reserved := make([]model.Product, 0, len(reservations))
		for _, res := range reservations {
			var product model.Product
			// The conditional decrement takes a row lock, so concurrent checkouts of the
			// same product serialize here and can never drive stock below zero.
			err := tx.GetContext(ctx, &product, `
				UPDATE products
				SET stock = stock - $1, updated_at = NOW()
				WHERE id = $2 AND stock >= $1
				RETURNING `+productColumns,
				res.Quantity, res.ProductID,
			)
			if errors.Is(err, sql.ErrNoRows) {
				return reservationError(ctx, tx, res.ProductID)
			}
			if err != nil {
				return err
			}
			reserved = append(reserved, product)
		}

		built, err := build(reserved)
		if err != nil {
			return err
		}
		for _, order := range built {
			if err := insertOrder(ctx, tx, order); err != nil {
				return err
			}
		}
		orders = built
		return nil
	})
	if err != nil {
		return nil, translateError(err)
	}

	for _, res := range reservations {
		r.invalidateProduct(ctx, res.ProductID)
	}
	return orders, nil
}

// reservationError explains why a conditional stock decrement matched no row.
func reservationError(ctx context.Context, tx *sqlx.Tx, productID uuid.UUID) error {
	var name string
	err := tx.GetContext(ctx, &name, "SELECT name FROM products WHERE id = $1", productID)
	if errors.Is(err, sql.ErrNoRows) {
		return &ErrProductNotFound{ProductID: productID}
	}
	if err != nil {
		return err
	}
	return &ErrInsufficientStock{ProductID: productID, ProductName: name}
}

// insertOrder inserts the order with its items and payment, and reads back the
// IDs and timestamps the database assigns.
func insertOrder(ctx context.Context, tx *sqlx.Tx, order *model.Order) error {
	err := tx.QueryRowxContext(ctx, `
		INSERT INTO orders (user_id, store_id, status, total_amount, shipping_address)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+orderColumns,
		order.UserID, order.StoreID, order.Status, order.TotalAmount, order.ShippingAddress,
	).StructScan(order)
	if err != nil {
		return err
	}

	for i := range order.OrderItems {
		item := &order.OrderItems[i]
		err := tx.QueryRowxContext(ctx, `
			INSERT INTO order_items (order_id, product_id, quantity, price)
			VALUES ($1, $2, $3, $4)
			RETURNING `+orderItemColumns,
			order.ID, item.ProductID, item.Quantity, item.Price,
		).StructScan(item)
		if err != nil {
			return err
		}
	}

	if order.Payment == nil {
		return nil
	}
	return tx.QueryRowxContext(ctx, `
		INSERT INTO payments (order_id, method, status, amount)
		VALUES ($1, $2, $3, $4)
		RETURNING `+paymentColumns,
		order.ID, order.Payment.Method, order.Payment.Status, order.Payment.Amount,
	).StructScan(order.Payment)
}

func (r *orderRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Order, error) {
	orders := make([]model.Order, 1)
	if err := r.db.DB().GetContext(ctx, &orders[0], "SELECT "+orderColumns+" FROM orders WHERE id = $1", id); err != nil {
		return nil, translateError(err)
	}
	if err := r.loadDetails(ctx, orders); err != nil {
		return nil, err
	}
	return &orders[0], nil
}

func (r *orderRepository) FindByUserID(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.Order, int64, error) {
	return r.findPage(ctx, "user_id", userID, page, perPage)
}

func (r *orderRepository) FindByStoreID(ctx context.Context, storeID uuid.UUID, page, perPage int) ([]model.Order, int64, error) {
	return r.findPage(ctx, "store_id", storeID, page, perPage)
}

// findPage returns a page of the orders whose column equals value, newest first,
// with their items and payments. column is a literal from the callers above,
// never user input.
func (r *orderRepository) findPage(ctx context.Context, column string, value uuid.UUID, page, perPage int) ([]model.Order, int64, error) {
	var total int64
	if err := r.db.DB().GetContext(ctx, &total, "SELECT COUNT(*) FROM orders WHERE "+column+" = $1", value); err != nil {
		return nil, 0, translateError(err)
	}

	var orders []model.Order
	err := r.db.DB().SelectContext(ctx, &orders, `
		SELECT `+orderColumns+`
		FROM orders
		WHERE `+column+` = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`,
		value, perPage, (page-1)*perPage,
	)
	if err != nil {
		return nil, 0, translateError(err)
	}
	if err := r.loadDetails(ctx, orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// loadDetails fills in the items and payment of each order, with one query for
// all items and one for all payments.
func (r *orderRepository) loadDetails(ctx context.Context, orders []model.Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(orders))
	byID := make(map[uuid.UUID]*model.Order, len(orders))
	for i := range orders {
		ids[i] = orders[i].ID
		byID[orders[i].ID] = &orders[i]
	}

	var items []model.OrderItem
	if err := r.selectIn(ctx, &items,
		"SELECT "+orderItemColumns+" FROM order_items WHERE order_id IN (?) ORDER BY product_id", ids); err != nil {
		return err
	}
	for _, item := range items {
		order := byID[item.OrderID]
		order.OrderItems = append(order.OrderItems, item)
	}

	var payments []model.Payment
	if err := r.selectIn(ctx, &payments, "SELECT "+paymentColumns+" FROM payments WHERE order_id IN (?)", ids); err != nil {
		return err
	}
	for i := range payments {
		byID[payments[i].OrderID].Payment = &payments[i]
	}
	return nil
}

// selectIn runs query after expanding its "IN (?)" into one placeholder per id.
func (r *orderRepository) selectIn(ctx context.Context, dest any, query string, ids []uuid.UUID) error {
	query, args, err := sqlx.In(query, ids)
	if err != nil {
		return err
	}
	return translateError(r.db.DB().SelectContext(ctx, dest, r.db.DB().Rebind(query), args...))
}

func (r *orderRepository) FindStalePending(ctx context.Context, createdBefore time.Time, limit int) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.DB().SelectContext(ctx, &orders, `
		SELECT `+orderColumns+`
		FROM orders
		WHERE status = $1 AND created_at < $2
		ORDER BY created_at ASC
		LIMIT $3`,
		constant.OrderStatusPending, createdBefore, limit,
	)
	return orders, translateError(err)
}

func (r *orderRepository) UpdateStatusIfCurrent(ctx context.Context, id uuid.UUID, fromStatus, toStatus string) (bool, error) {
	res, err := r.db.DB().ExecContext(ctx,
		"UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3",
		toStatus, id, fromStatus,
	)
	if err != nil {
		return false, translateError(err)
	}
	return changedRows(res)
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

	err := withTx(ctx, r.db.DB(), func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx,
			"UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3",
			constant.OrderStatusCancelled, id, fromStatus,
		)
		if err != nil {
			return err
		}
		cancelled, err = changedRows(res)
		if err != nil || !cancelled {
			return err
		}

		// Restock in product_id order, the same order checkout reserves in, so a
		// concurrent checkout and cancel cannot deadlock on product row locks.
		err = tx.SelectContext(ctx, &items,
			"SELECT "+orderItemColumns+" FROM order_items WHERE order_id = $1 ORDER BY product_id", id)
		if err != nil {
			return err
		}
		for _, item := range items {
			_, err := tx.ExecContext(ctx,
				"UPDATE products SET stock = stock + $1, updated_at = NOW() WHERE id = $2",
				item.Quantity, item.ProductID,
			)
			if err != nil {
				return err
			}
		}

		_, err = tx.ExecContext(ctx,
			"UPDATE payments SET status = $1, updated_at = NOW() WHERE order_id = $2 AND status = $3",
			pendingPaymentStatus, id, model.PaymentStatusPending,
		)
		return err
	})
	if err != nil {
		return false, translateError(err)
	}

	for _, item := range items {
		r.invalidateProduct(ctx, item.ProductID)
	}
	return cancelled, nil
}

func (r *orderRepository) MarkPaymentSucceeded(ctx context.Context, orderID uuid.UUID) (bool, error) {
	paid := false

	err := withTx(ctx, r.db.DB(), func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx,
			"UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3",
			constant.OrderStatusPaid, orderID, constant.OrderStatusPending,
		)
		if err != nil {
			return err
		}
		paid, err = changedRows(res)
		if err != nil || !paid {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE payments
			SET status = $1, paid_at = NOW(), updated_at = NOW()
			WHERE order_id = $2 AND status = $3`,
			model.PaymentStatusSuccess, orderID, model.PaymentStatusPending,
		)
		return err
	})
	if err != nil {
		return false, translateError(err)
	}
	return paid, nil
}

func changedRows(res sql.Result) (bool, error) {
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *orderRepository) invalidateProduct(ctx context.Context, productID uuid.UUID) {
	r.cache.Delete(ctx, fmt.Sprintf(constant.KeyProduct, productID.String()))
}
