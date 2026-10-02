package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/pagination"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/go-redsync/redsync/v4"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// MessagePublisher is the subset of *nsq.Producer the order service needs.
type MessagePublisher interface {
	Publish(topic string, body []byte) error
}

type OrderService interface {
	Checkout(ctx context.Context, userID uuid.UUID, shippingAddress string) ([]model.OrderResponse, error)
	GetOrders(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.OrderResponse, int64, error)
	GetOrderByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*model.OrderResponse, error)
	CancelOrder(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	UpdateOrderStatus(ctx context.Context, sellerID uuid.UUID, id uuid.UUID, status string) error
	GetSellerOrders(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.OrderResponse, int64, error)
	ProcessPaymentResult(ctx context.Context, orderID uuid.UUID, success bool) error
}

type orderService struct {
	orderRepo repository.OrderRepository
	cartRepo  repository.CartRepository
	storeRepo repository.StoreRepository
	redsync   *redsync.Redsync
	publisher MessagePublisher
}

func NewOrderService(
	orderRepo repository.OrderRepository,
	cartRepo repository.CartRepository,
	storeRepo repository.StoreRepository,
	rs *redsync.Redsync,
	publisher MessagePublisher,
) OrderService {
	return &orderService{
		orderRepo: orderRepo,
		cartRepo:  cartRepo,
		storeRepo: storeRepo,
		redsync:   rs,
		publisher: publisher,
	}
}

func (s *orderService) Checkout(ctx context.Context, userID uuid.UUID, shippingAddress string) ([]model.OrderResponse, error) {
	// Hold the cart lock from reading the cart until it is cleared: a double-submitted
	// checkout then finds the cart already empty instead of ordering it twice, and an
	// item added meanwhile is not wiped by DeleteCart.
	unlock, err := lockUserCart(s.redsync, userID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to load cart")
	}
	if len(cart.Items) == 0 {
		return nil, errors.New("cart is empty")
	}

	// A stable product order makes concurrent checkouts lock product rows in the
	// same sequence, which avoids deadlocks between them.
	items := make([]model.CartItem, len(cart.Items))
	copy(items, cart.Items)
	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductID.String() < items[j].ProductID.String()
	})

	reservations := make([]repository.StockReservation, 0, len(items))
	for _, item := range items {
		reservations = append(reservations, repository.StockReservation{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	orders, err := s.orderRepo.CreateOrdersWithStock(ctx, reservations, func(reserved []model.Product) ([]*model.Order, error) {
		return buildOrdersByStore(userID, shippingAddress, items, reserved), nil
	})
	if err != nil {
		var notFound *repository.ErrProductNotFound
		var insufficient *repository.ErrInsufficientStock
		switch {
		case errors.As(err, &insufficient):
			return nil, fmt.Errorf("insufficient stock for product %s", insufficient.ProductName)
		case errors.As(err, &notFound):
			return nil, err
		default:
			logger.Error(ctx, "failed to create orders", err)
			return nil, errors.New("failed to process checkout")
		}
	}

	if err := s.cartRepo.DeleteCart(ctx, userID); err != nil {
		logger.Error(ctx, "failed to clear cart after checkout", err, map[string]interface{}{
			"user_id": userID.String(),
		})
	}

	// A failed publish is logged but does not fail the checkout: the orders are
	// already committed.
	for _, order := range orders {
		s.publishOrderCreated(ctx, order)
	}

	responses := make([]model.OrderResponse, 0, len(orders))
	for _, order := range orders {
		responses = append(responses, order.ToResponse())
	}

	logger.Info(ctx, "orders created", map[string]interface{}{
		"user_id": userID.String(),
		"count":   len(orders),
	})

	return responses, nil
}

// buildOrdersByStore splits the checked-out items into one pending order per
// store, priced at the reserved products' current price. items and reserved
// must be in the same order.
func buildOrdersByStore(userID uuid.UUID, shippingAddress string, items []model.CartItem, reserved []model.Product) []*model.Order {
	ordersByStore := make(map[uuid.UUID]*model.Order)
	storeOrder := make([]uuid.UUID, 0)

	for i, product := range reserved {
		order, ok := ordersByStore[product.StoreID]
		if !ok {
			order = &model.Order{
				UserID:          userID,
				StoreID:         product.StoreID,
				Status:          constant.OrderStatusPending,
				TotalAmount:     decimal.NewFromInt(0),
				ShippingAddress: shippingAddress,
			}
			ordersByStore[product.StoreID] = order
			storeOrder = append(storeOrder, product.StoreID)
		}

		quantity := items[i].Quantity
		order.OrderItems = append(order.OrderItems, model.OrderItem{
			ProductID: product.ID,
			Quantity:  quantity,
			Price:     product.Price,
		})
		order.TotalAmount = order.TotalAmount.Add(product.Price.Mul(decimal.NewFromInt(int64(quantity))))
	}

	orders := make([]*model.Order, 0, len(storeOrder))
	for _, storeID := range storeOrder {
		order := ordersByStore[storeID]
		order.Payment = &model.Payment{
			Method: model.PaymentMethodMock,
			Status: model.PaymentStatusPending,
			Amount: order.TotalAmount,
		}
		orders = append(orders, order)
	}
	return orders
}

func (s *orderService) publishOrderCreated(ctx context.Context, order *model.Order) {
	if s.publisher == nil {
		return
	}

	msg, err := json.Marshal(map[string]interface{}{
		"order_id":     order.ID.String(),
		"user_id":      order.UserID.String(),
		"total_amount": order.TotalAmount.String(),
	})
	if err != nil {
		logger.Error(ctx, "failed to marshal order.created payload", err)
		return
	}
	if err := s.publisher.Publish(constant.TopicOrderCreated, msg); err != nil {
		logger.Error(ctx, "failed to publish order.created", err, map[string]interface{}{
			"order_id": order.ID.String(),
		})
	}
}

func (s *orderService) GetOrders(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.OrderResponse, int64, error) {
	page, perPage = pagination.Normalize(page, perPage)

	orders, total, err := s.orderRepo.FindByUserID(ctx, userID, page, perPage)
	if err != nil {
		return nil, 0, errors.New("failed to fetch orders")
	}

	responses := make([]model.OrderResponse, 0, len(orders))
	for _, o := range orders {
		responses = append(responses, o.ToResponse())
	}

	return responses, total, nil
}

func (s *orderService) GetOrderByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*model.OrderResponse, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("order not found")
	}

	if order.UserID != userID {
		return nil, errors.New("forbidden")
	}

	resp := order.ToResponse()
	return &resp, nil
}

func (s *orderService) CancelOrder(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("order not found")
	}

	if order.UserID != userID {
		return errors.New("forbidden")
	}

	if !constant.CancellableStatuses[order.Status] {
		return fmt.Errorf("cannot cancel order with status %s", order.Status)
	}

	cancelled, err := s.orderRepo.CancelAndRestock(ctx, id, order.Status)
	if err != nil {
		logger.Error(ctx, "failed to cancel order", err, map[string]interface{}{
			"order_id": id.String(),
		})
		return errors.New("failed to cancel order")
	}
	if !cancelled {
		return errors.New("cannot cancel order, status changed")
	}

	logger.Info(ctx, "order cancelled", map[string]interface{}{
		"order_id": id.String(),
	})

	return nil
}

func (s *orderService) UpdateOrderStatus(ctx context.Context, sellerID uuid.UUID, id uuid.UUID, status string) error {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("order not found")
	}

	allowed, ok := constant.OrderStatusTransitions[order.Status]
	if !ok {
		return fmt.Errorf("cannot transition from status %s", order.Status)
	}

	valid := false
	for _, allowedStatus := range allowed {
		if allowedStatus == status {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("invalid status transition from %s to %s", order.Status, status)
	}

	store, err := s.storeRepo.FindByUserID(ctx, sellerID)
	if err != nil {
		return errors.New("store not found")
	}

	if order.StoreID != store.ID {
		return errors.New("forbidden: order does not belong to your store")
	}

	updated, err := s.orderRepo.UpdateStatusIfCurrent(ctx, id, order.Status, status)
	if err != nil {
		logger.Error(ctx, "failed to update order status", err)
		return errors.New("failed to update order status")
	}
	if !updated {
		return errors.New("cannot update order, status changed")
	}
	return nil
}

func (s *orderService) GetSellerOrders(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.OrderResponse, int64, error) {
	page, perPage = pagination.Normalize(page, perPage)

	store, err := s.storeRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, 0, errors.New("store not found")
	}

	orders, total, err := s.orderRepo.FindByStoreID(ctx, store.ID, page, perPage)
	if err != nil {
		return nil, 0, errors.New("failed to fetch orders")
	}

	responses := make([]model.OrderResponse, 0, len(orders))
	for _, o := range orders {
		responses = append(responses, o.ToResponse())
	}

	return responses, total, nil
}

func (s *orderService) ProcessPaymentResult(ctx context.Context, orderID uuid.UUID, success bool) error {
	fields := map[string]interface{}{"order_id": orderID.String()}

	if success {
		paid, err := s.orderRepo.MarkPaymentSucceeded(ctx, orderID)
		if err != nil {
			logger.Error(ctx, "failed to mark order as paid", err, fields)
			return err
		}
		if !paid {
			logger.Warn(ctx, "ignoring payment success for non-pending order", fields)
			return nil
		}
		logger.Info(ctx, "payment success", fields)
		return nil
	}

	cancelled, err := s.orderRepo.MarkPaymentFailed(ctx, orderID)
	if err != nil {
		logger.Error(ctx, "failed to cancel order after payment failure", err, fields)
		return err
	}
	if !cancelled {
		logger.Warn(ctx, "ignoring payment failure for non-pending order", fields)
		return nil
	}
	logger.Info(ctx, "payment failed, order cancelled", fields)
	return nil
}
