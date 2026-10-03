package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	orderColumns     = "id, user_id, store_id, status, total_amount, shipping_address, created_at, updated_at"
	orderItemColumns = "id, order_id, product_id, quantity, price, created_at"
	paymentColumns   = "id, order_id, method, status, amount, paid_at, created_at, updated_at"
)

type OrderRepository interface {
	// CreateOrdersFromCart reserves the stock for the user's cart, inserts the orders from build
	// and empties the cart, all or nothing.
	CreateOrdersFromCart(ctx context.Context, userID uuid.UUID, build func(items []model.CartItem, reserved []model.Product) []*model.Order) ([]*model.Order, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Order, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.Order, int64, error)
	FindByStoreID(ctx context.Context, storeID uuid.UUID, page, perPage int) ([]model.Order, int64, error)
	FindPending(ctx context.Context) ([]*model.Order, error)
	UpdateStatusIfCurrent(ctx context.Context, id uuid.UUID, fromStatus, toStatus string) (bool, error)
	// The methods below report false if the order was not in the expected status.
	CancelAndRestock(ctx context.Context, id uuid.UUID, fromStatus string) (bool, error)
	MarkPaymentSucceeded(ctx context.Context, orderID uuid.UUID) (bool, error)
	MarkPaymentFailed(ctx context.Context, orderID uuid.UUID) (bool, error)
}

type orderRepository struct {
	db    *sqlx.DB
	cache caches.Cache
}

func NewOrderRepository(db *sqlx.DB, cache caches.Cache) OrderRepository {
	return &orderRepository{db: db, cache: cache}
}

func (r *orderRepository) CreateOrdersFromCart(ctx context.Context, userID uuid.UUID, build func(items []model.CartItem, reserved []model.Product) []*model.Order) ([]*model.Order, error) {
	var items []model.CartItem

	// The cart is about to change, so Redis must not keep it.
	if err := r.cache.Delete(ctx, fmt.Sprintf(constant.KeyCart, userID)); err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, translateDatabaseError(err)
	}
	defer tx.Rollback() // no-op after Commit

	// Locking the cart makes a second checkout wait, then find it empty.
	// Product order is the lock order, so checkouts cannot deadlock.
	err = tx.SelectContext(ctx, &items, `
		SELECT ci.product_id, ci.quantity, p.name
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.user_id = $1
		ORDER BY ci.product_id
		FOR UPDATE OF ci`,
		userID,
	)
	if err != nil {
		return nil, translateDatabaseError(err)
	}
	if len(items) == 0 {
		return nil, ErrCartEmpty
	}

	reserved := make([]model.Product, 0, len(items))
	for _, item := range items {
		var product model.Product
		// The condition keeps concurrent checkouts from overselling.
		err := tx.GetContext(ctx, &product, `
			UPDATE products
			SET stock = stock - $1, updated_at = NOW()
			WHERE id = $2 AND stock >= $1
			RETURNING `+productColumns,
			item.Quantity, item.ProductID,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf(constant.MsgInsufficientStockFor, ErrInsufficientStock, item.Name)
		}
		if err != nil {
			return nil, translateDatabaseError(err)
		}
		reserved = append(reserved, product)
	}

	orders := build(items, reserved)
	for _, order := range orders {
		if err := insertOrder(ctx, tx, order); err != nil {
			return nil, translateDatabaseError(err)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM cart_items WHERE user_id = $1", userID); err != nil {
		return nil, translateDatabaseError(err)
	}

	if err := tx.Commit(); err != nil {
		return nil, translateDatabaseError(err)
	}

	for _, item := range items {
		r.cache.Delete(ctx, fmt.Sprintf(constant.KeyProduct, item.ProductID))
	}
	return orders, nil
}

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
	if err := r.db.GetContext(ctx, &orders[0], "SELECT "+orderColumns+" FROM orders WHERE id = $1", id); err != nil {
		return nil, translateDatabaseError(err)
	}

	if err := r.loadDetails(ctx, orders); err != nil {
		return nil, err
	}
	return &orders[0], nil
}

func (r *orderRepository) FindByUserID(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.Order, int64, error) {
	var (
		total  int64
		orders []model.Order
	)

	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM orders WHERE user_id = $1", userID); err != nil {
		return nil, 0, translateDatabaseError(err)
	}

	err := r.db.SelectContext(ctx, &orders, `
		SELECT `+orderColumns+`
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`,
		userID, perPage, (page-1)*perPage,
	)
	if err != nil {
		return nil, 0, translateDatabaseError(err)
	}

	if err := r.loadDetails(ctx, orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *orderRepository) FindByStoreID(ctx context.Context, storeID uuid.UUID, page, perPage int) ([]model.Order, int64, error) {
	var (
		total  int64
		orders []model.Order
	)

	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM orders WHERE store_id = $1", storeID); err != nil {
		return nil, 0, translateDatabaseError(err)
	}

	err := r.db.SelectContext(ctx, &orders, `
		SELECT `+orderColumns+`
		FROM orders
		WHERE store_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`,
		storeID, perPage, (page-1)*perPage,
	)
	if err != nil {
		return nil, 0, translateDatabaseError(err)
	}

	if err := r.loadDetails(ctx, orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *orderRepository) loadDetails(ctx context.Context, orders []model.Order) error {
	var (
		items    []model.OrderItem
		payments []model.Payment
	)

	if len(orders) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, len(orders))
	byID := make(map[uuid.UUID]*model.Order, len(orders))
	for i := range orders {
		ids[i] = orders[i].ID
		byID[orders[i].ID] = &orders[i]
	}

	if err := r.db.SelectContext(ctx, &items,
		"SELECT "+orderItemColumns+" FROM order_items WHERE order_id = ANY($1) ORDER BY product_id", ids); err != nil {
		return translateDatabaseError(err)
	}
	for _, item := range items {
		order := byID[item.OrderID]
		order.OrderItems = append(order.OrderItems, item)
	}

	if err := r.db.SelectContext(ctx, &payments, "SELECT "+paymentColumns+" FROM payments WHERE order_id = ANY($1)", ids); err != nil {
		return translateDatabaseError(err)
	}
	for i := range payments {
		byID[payments[i].OrderID].Payment = &payments[i]
	}
	return nil
}

func (r *orderRepository) FindPending(ctx context.Context) ([]*model.Order, error) {
	var orders []*model.Order

	err := r.db.SelectContext(ctx, &orders,
		"SELECT "+orderColumns+" FROM orders WHERE status = $1 ORDER BY created_at",
		constant.OrderStatusPending,
	)
	return orders, translateDatabaseError(err)
}

func (r *orderRepository) UpdateStatusIfCurrent(ctx context.Context, id uuid.UUID, fromStatus, toStatus string) (bool, error) {
	changed, err := moveStatus(ctx, r.db, id, fromStatus, toStatus)
	return changed, translateDatabaseError(err)
}

func (r *orderRepository) CancelAndRestock(ctx context.Context, id uuid.UUID, fromStatus string) (bool, error) {
	return r.cancelAndRestock(ctx, id, fromStatus, constant.PaymentStatusCancelled)
}

func (r *orderRepository) MarkPaymentFailed(ctx context.Context, orderID uuid.UUID) (bool, error) {
	return r.cancelAndRestock(ctx, orderID, constant.OrderStatusPending, constant.PaymentStatusFailed)
}

func (r *orderRepository) cancelAndRestock(ctx context.Context, id uuid.UUID, fromStatus, pendingPaymentStatus string) (bool, error) {
	var items []model.OrderItem

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, translateDatabaseError(err)
	}
	defer tx.Rollback() // no-op after Commit

	cancelled, err := moveStatus(ctx, tx, id, fromStatus, constant.OrderStatusCancelled)
	if err != nil {
		return false, translateDatabaseError(err)
	}
	if !cancelled {
		return false, nil
	}

	// Same product order as checkout, so the two cannot deadlock.
	err = tx.SelectContext(ctx, &items,
		"SELECT "+orderItemColumns+" FROM order_items WHERE order_id = $1 ORDER BY product_id", id)
	if err != nil {
		return false, translateDatabaseError(err)
	}

	for _, item := range items {
		_, err := tx.ExecContext(ctx,
			"UPDATE products SET stock = stock + $1, updated_at = NOW() WHERE id = $2",
			item.Quantity, item.ProductID,
		)
		if err != nil {
			return false, translateDatabaseError(err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE payments
		SET status = CASE status WHEN $1 THEN $2 ELSE $3 END, updated_at = NOW()
		WHERE order_id = $4`,
		constant.PaymentStatusSuccess, constant.PaymentStatusRefunded, pendingPaymentStatus, id,
	)
	if err != nil {
		return false, translateDatabaseError(err)
	}

	if err := tx.Commit(); err != nil {
		return false, translateDatabaseError(err)
	}

	for _, item := range items {
		r.cache.Delete(ctx, fmt.Sprintf(constant.KeyProduct, item.ProductID))
	}
	return true, nil
}

func (r *orderRepository) MarkPaymentSucceeded(ctx context.Context, orderID uuid.UUID) (bool, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, translateDatabaseError(err)
	}
	defer tx.Rollback() // no-op after Commit

	paid, err := moveStatus(ctx, tx, orderID, constant.OrderStatusPending, constant.OrderStatusPaid)
	if err != nil {
		return false, translateDatabaseError(err)
	}
	if !paid {
		return false, nil
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE payments
		SET status = $1, paid_at = NOW(), updated_at = NOW()
		WHERE order_id = $2 AND status = $3`,
		constant.PaymentStatusSuccess, orderID, constant.PaymentStatusPending,
	)
	if err != nil {
		return false, translateDatabaseError(err)
	}

	if err := tx.Commit(); err != nil {
		return false, translateDatabaseError(err)
	}
	return true, nil
}

func moveStatus(ctx context.Context, db sqlx.ExecerContext, id uuid.UUID, from, to string) (bool, error) {
	res, err := db.ExecContext(ctx,
		"UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3", to, id, from)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
