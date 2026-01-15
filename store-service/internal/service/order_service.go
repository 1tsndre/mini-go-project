package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	Checkout(ctx context.Context, userID uuid.UUID, shippingAddress string) (*model.OrderResponse, error)
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

func (s *orderService) Checkout(ctx context.Context, userID uuid.UUID, shippingAddress string) (*model.OrderResponse, error) {
	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, errors.New("cart not found")
	}
	if len(cart.Items) == 0 {
		return nil, errors.New("cart is empty")
	}

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

	// Phase 1: validate all items and capture snapshots (no DB writes yet)
	type itemSnapshot struct {
		product   *model.Product
		orderItem model.OrderItem
		newStock  int
	}
	snapshots := make([]itemSnapshot, 0, len(cart.Items))
	totalAmount := decimal.NewFromInt(0)

	for _, item := range cart.Items {
		product, err := s.productRepo.FindByID(ctx, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("product %s not found", item.ProductID)
		}

		if product.Stock < item.Quantity {
			return nil, fmt.Errorf("insufficient stock for product %s", product.Name)
		}

		subtotal := product.Price.Mul(decimal.NewFromInt(int64(item.Quantity)))
		totalAmount = totalAmount.Add(subtotal)

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

	// Phase 2: apply stock updates; rollback already-applied on partial failure
	var orderItems []model.OrderItem
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
		orderItems = append(orderItems, snap.orderItem)
	}

	order := &model.Order{
		UserID:          userID,
		Status:          constant.OrderStatusPending,
		TotalAmount:     totalAmount,
		ShippingAddress: shippingAddress,
		OrderItems:      orderItems,
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		// Rollback all stock updates
		for _, snap := range snapshots {
			if rbErr := s.productRepo.UpdateStock(ctx, snap.product.ID, snap.product.Stock); rbErr != nil {
				logger.Error(ctx, "failed to rollback stock after order creation failure", rbErr, map[string]interface{}{
					"product_id": snap.product.ID.String(),
				})
			}
		}
		logger.Error(ctx, "failed to create order", err)
		return nil, errors.New("failed to create order")
	}

	if err := s.cartRepo.DeleteCart(ctx, userID); err != nil {
		logger.Error(ctx, "failed to clear cart after checkout", err, map[string]interface{}{
			"user_id": userID.String(),
		})
	}

	if s.nsqProducer != nil {
		msg, err := json.Marshal(map[string]interface{}{
			"order_id":     order.ID.String(),
			"user_id":      userID.String(),
			"total_amount": totalAmount.String(),
		})
		if err != nil {
			logger.Error(ctx, "failed to marshal order.created payload", err)
		} else if err := s.nsqProducer.Publish(constant.TopicOrderCreated, msg); err != nil {
			logger.Error(ctx, "failed to publish order.created", err)
		}
	}

	logger.Info(ctx, "order created", map[string]interface{}{
		"order_id":     order.ID.String(),
		"total_amount": totalAmount.String(),
	})

	resp := order.ToResponse()
	return &resp, nil
}

func (s *orderService) GetOrders(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.OrderResponse, int64, error) {
	page, perPage = pagination.Normalize(page, perPage)

	orders, total, err := s.orderRepo.FindByUserID(ctx, userID, page, perPage)
	if err != nil {
		return nil, 0, err
	}

	var responses []model.OrderResponse
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

	for _, item := range order.OrderItems {
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
			logger.Error(ctx, "failed to restore stock for cancelled order", err, map[string]interface{}{
				"product_id": item.ProductID.String(),
			})
		}
		mutex.Unlock()
	}

	if err := s.orderRepo.UpdateStatus(ctx, id, constant.OrderStatusCancelled); err != nil {
		return errors.New("failed to cancel order")
	}

	logger.Info(ctx, "order cancelled", map[string]interface{}{
		"order_id": id.String(),
	})

	return nil
}

func (s *orderService) UpdateOrderStatus(ctx context.Context, sellerID uuid.UUID, id uuid.UUID, status string) error {
	validTransitions := map[string][]string{
		constant.OrderStatusPaid:       {constant.OrderStatusProcessing},
		constant.OrderStatusProcessing: {constant.OrderStatusShipping},
		constant.OrderStatusShipping:   {constant.OrderStatusShipped},
		constant.OrderStatusShipped:    {constant.OrderStatusCompleted},
	}

	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("order not found")
	}

	allowed, ok := validTransitions[order.Status]
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

	hasItem := false
	for _, item := range order.OrderItems {
		product, err := s.productRepo.FindByID(ctx, item.ProductID)
		if err != nil {
			continue
		}
		if product.StoreID == store.ID {
			hasItem = true
			break
		}
	}
	if !hasItem {
		return errors.New("forbidden: no items from your store in this order")
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
		return nil, 0, err
	}

	var responses []model.OrderResponse
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

	payment, _ := s.orderRepo.FindPaymentByOrderID(ctx, orderID)

	if success {
		now := time.Now()
		if payment != nil {
			payment.Status = model.PaymentStatusSuccess
			payment.PaidAt = &now
			if err := s.orderRepo.UpdatePayment(ctx, payment); err != nil {
				logger.Error(ctx, "failed to update payment status to success", err, map[string]interface{}{
					"order_id": order.ID.String(),
				})
				return err
			}
		}
		if err := s.orderRepo.UpdateStatus(ctx, orderID, constant.OrderStatusPaid); err != nil {
			logger.Error(ctx, "failed to update order status to paid", err, map[string]interface{}{
				"order_id": order.ID.String(),
			})
			return err
		}

		logger.Info(ctx, "payment success", map[string]interface{}{
			"order_id": order.ID.String(),
		})
	} else {
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
