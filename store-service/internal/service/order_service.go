package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/pagination"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/go-redsync/redsync/v4"
	"github.com/google/uuid"
	"github.com/nsqio/go-nsq"
	"github.com/shopspring/decimal"
)

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
	orderRepo   repository.OrderRepository
	cartRepo    repository.CartRepository
	productRepo repository.ProductRepository
	storeRepo   repository.StoreRepository
	redsync     *redsync.Redsync
	nsqProducer *nsq.Producer
}

func NewOrderService(
	orderRepo repository.OrderRepository,
	cartRepo repository.CartRepository,
	productRepo repository.ProductRepository,
	storeRepo repository.StoreRepository,
	rs *redsync.Redsync,
	producer *nsq.Producer,
) OrderService {
	return &orderService{
		orderRepo:   orderRepo,
		cartRepo:    cartRepo,
		productRepo: productRepo,
		storeRepo:   storeRepo,
		redsync:     rs,
		nsqProducer: producer,
	}
}

func (s *orderService) Checkout(ctx context.Context, userID uuid.UUID, shippingAddress string) ([]model.OrderResponse, error) {
	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, errors.New("cart not found")
	}
	if len(cart.Items) == 0 {
		return nil, errors.New("cart is empty")
	}

	sort.Slice(cart.Items, func(i, j int) bool {
		return cart.Items[i].ProductID.String() < cart.Items[j].ProductID.String()
	})

	var mutexes []*redsync.Mutex
	for _, item := range cart.Items {
		lockKey := fmt.Sprintf(constant.KeyStockLock, item.ProductID.String())
		mutex := s.redsync.NewMutex(lockKey, redsync.WithExpiry(10*time.Second))
		if err := mutex.Lock(); err != nil {
			for _, m := range mutexes {
				m.Unlock()
			}
			logger.Error(ctx, "failed to acquire stock lock", err, map[string]interface{}{
				"product_id": item.ProductID.String(),
			})
			return nil, errors.New("failed to process checkout, please try again")
		}
		mutexes = append(mutexes, mutex)
	}
	defer func() {
		for _, m := range mutexes {
			m.Unlock()
		}
	}()

	type itemSnapshot struct {
		product   *model.Product
		orderItem model.OrderItem
		newStock  int
	}
	snapshots := make([]itemSnapshot, 0, len(cart.Items))

	for _, item := range cart.Items {
		product, err := s.productRepo.FindByID(ctx, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("product %s not found", item.ProductID)
		}

		if product.Stock < item.Quantity {
			return nil, fmt.Errorf("insufficient stock for product %s", product.Name)
		}

		snapshots = append(snapshots, itemSnapshot{
			product: product,
			orderItem: model.OrderItem{
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
				Price:     product.Price,
			},
			newStock: product.Stock - item.Quantity,
		})
	}

	for i, snap := range snapshots {
		if err := s.productRepo.UpdateStock(ctx, snap.product.ID, snap.newStock); err != nil {
			for j := 0; j < i; j++ {
				if rbErr := s.productRepo.UpdateStock(ctx, snapshots[j].product.ID, snapshots[j].product.Stock); rbErr != nil {
					logger.Error(ctx, "failed to rollback stock update", rbErr, map[string]interface{}{
						"product_id": snapshots[j].product.ID.String(),
					})
				}
			}
			logger.Error(ctx, "failed to update stock", err)
			return nil, errors.New("failed to process checkout")
		}
	}

	ordersByStore := make(map[uuid.UUID]*model.Order)
	storeOrder := make([]uuid.UUID, 0)
	for _, snap := range snapshots {
		storeID := snap.product.StoreID
		order, ok := ordersByStore[storeID]
		if !ok {
			order = &model.Order{
				UserID:          userID,
				StoreID:         storeID,
				Status:          constant.OrderStatusPending,
				TotalAmount:     decimal.NewFromInt(0),
				ShippingAddress: shippingAddress,
			}
			ordersByStore[storeID] = order
			storeOrder = append(storeOrder, storeID)
		}
		subtotal := snap.orderItem.Price.Mul(decimal.NewFromInt(int64(snap.orderItem.Quantity)))
		order.TotalAmount = order.TotalAmount.Add(subtotal)
		order.OrderItems = append(order.OrderItems, snap.orderItem)
	}

	orders := make([]*model.Order, 0, len(storeOrder))
	for _, storeID := range storeOrder {
		orders = append(orders, ordersByStore[storeID])
	}

	if err := s.orderRepo.CreateOrders(ctx, orders); err != nil {
		for _, snap := range snapshots {
			if rbErr := s.productRepo.UpdateStock(ctx, snap.product.ID, snap.product.Stock); rbErr != nil {
				logger.Error(ctx, "failed to rollback stock after order creation failure", rbErr, map[string]interface{}{
					"product_id": snap.product.ID.String(),
				})
			}
		}
		logger.Error(ctx, "failed to create orders", err)
		return nil, errors.New("failed to create order")
	}

	if err := s.cartRepo.DeleteCart(ctx, userID); err != nil {
		logger.Error(ctx, "failed to clear cart after checkout", err, map[string]interface{}{
			"user_id": userID.String(),
		})
	}

	if s.nsqProducer != nil {
		for _, order := range orders {
			msg, err := json.Marshal(map[string]interface{}{
				"order_id":     order.ID.String(),
				"user_id":      userID.String(),
				"total_amount": order.TotalAmount.String(),
			})
			if err != nil {
				logger.Error(ctx, "failed to marshal order.created payload", err)
				continue
			}
			if err := s.nsqProducer.Publish(constant.TopicOrderCreated, msg); err != nil {
				logger.Error(ctx, "failed to publish order.created", err, map[string]interface{}{
					"order_id": order.ID.String(),
				})
			}
		}
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

	claimed, err := s.orderRepo.UpdateStatusIfCurrent(ctx, id, order.Status, constant.OrderStatusCancelled)
	if err != nil {
		return errors.New("failed to cancel order")
	}
	if !claimed {
		return errors.New("cannot cancel order, status changed")
	}

	s.restoreStock(ctx, order.OrderItems)

	logger.Info(ctx, "order cancelled", map[string]interface{}{
		"order_id": id.String(),
	})

	return nil
}

func (s *orderService) restoreStock(ctx context.Context, items []model.OrderItem) {
	for _, item := range items {
		lockKey := fmt.Sprintf(constant.KeyStockLock, item.ProductID.String())
		mutex := s.redsync.NewMutex(lockKey, redsync.WithExpiry(10*time.Second))
		if err := mutex.Lock(); err != nil {
			logger.Error(ctx, "failed to acquire lock for stock restore", err)
			continue
		}
		product, err := s.productRepo.FindByID(ctx, item.ProductID)
		if err != nil {
			mutex.Unlock()
			continue
		}
		if err := s.productRepo.UpdateStock(ctx, item.ProductID, product.Stock+item.Quantity); err != nil {
			logger.Error(ctx, "failed to restore stock", err, map[string]interface{}{
				"product_id": item.ProductID.String(),
			})
		}
		mutex.Unlock()
	}
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

	if err := s.orderRepo.UpdateStatus(ctx, id, status); err != nil {
		logger.Error(ctx, "failed to update order status", err)
		return errors.New("failed to update order status")
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
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return errors.New("order not found")
	}

	if success {
		updated, err := s.orderRepo.UpdateStatusIfCurrent(ctx, orderID, constant.OrderStatusPending, constant.OrderStatusPaid)
		if err != nil {
			logger.Error(ctx, "failed to update order status to paid", err, map[string]interface{}{
				"order_id": order.ID.String(),
			})
			return err
		}
		if !updated {
			logger.Warn(ctx, "ignoring payment success for non-pending order", map[string]interface{}{
				"order_id": order.ID.String(),
				"status":   order.Status,
			})
			return nil
		}

		payment, _ := s.orderRepo.FindPaymentByOrderID(ctx, orderID)
		if payment != nil {
			now := time.Now()
			payment.Status = model.PaymentStatusSuccess
			payment.PaidAt = &now
			if err := s.orderRepo.UpdatePayment(ctx, payment); err != nil {
				logger.Error(ctx, "failed to update payment status to success", err, map[string]interface{}{
					"order_id": order.ID.String(),
				})
				return err
			}
		}

		logger.Info(ctx, "payment success", map[string]interface{}{
			"order_id": order.ID.String(),
		})
	} else {
		cancelled, err := s.orderRepo.UpdateStatusIfCurrent(ctx, orderID, constant.OrderStatusPending, constant.OrderStatusCancelled)
		if err != nil {
			logger.Error(ctx, "failed to cancel order after payment failure", err, map[string]interface{}{
				"order_id": order.ID.String(),
			})
			return err
		}
		if cancelled {
			s.restoreStock(ctx, order.OrderItems)
		}

		payment, _ := s.orderRepo.FindPaymentByOrderID(ctx, orderID)
		if payment != nil {
			payment.Status = model.PaymentStatusFailed
			if err := s.orderRepo.UpdatePayment(ctx, payment); err != nil {
				logger.Error(ctx, "failed to update payment status to failed", err, map[string]interface{}{
					"order_id": order.ID.String(),
				})
				return err
			}
		}

		logger.Info(ctx, "payment failed", map[string]interface{}{
			"order_id": order.ID.String(),
		})
	}

	return nil
}
