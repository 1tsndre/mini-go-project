package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/go-redsync/redsync/v4"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type OrderService interface {
	Checkout(ctx context.Context, userID uuid.UUID, shippingAddress string) ([]model.OrderResponse, error)
	GetOrders(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.OrderResponse, int64, error)
	GetOrderByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*model.OrderResponse, error)
	CancelOrder(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	UpdateOrderStatus(ctx context.Context, sellerID uuid.UUID, id uuid.UUID, status string) error
	GetSellerOrders(ctx context.Context, sellerID uuid.UUID, page, perPage int) ([]model.OrderResponse, int64, error)
}

type orderService struct {
	orderRepo repository.OrderRepository
	cartRepo  repository.CartRepository
	storeRepo repository.StoreRepository
	locker    *redsync.Redsync
}

func NewOrderService(
	orderRepo repository.OrderRepository,
	cartRepo repository.CartRepository,
	storeRepo repository.StoreRepository,
	locker *redsync.Redsync,
) OrderService {
	return &orderService{
		orderRepo: orderRepo,
		cartRepo:  cartRepo,
		storeRepo: storeRepo,
		locker:    locker,
	}
}

func (s *orderService) Checkout(ctx context.Context, userID uuid.UUID, shippingAddress string) ([]model.OrderResponse, error) {
	// Keeps cart changes out while the orders are made.
	unlock, err := lockUserCart(ctx, s.locker, userID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	orders, err := s.orderRepo.CreateOrdersFromCart(ctx, userID, func(items []model.CartItem, reserved []model.Product) []*model.Order {
		return buildOrdersByStore(userID, shippingAddress, items, reserved)
	})
	if errors.Is(err, repository.ErrCartEmpty) {
		return nil, &Error{Code: constant.ErrCodeValidation, Message: constant.MsgCartEmpty}
	}
	if errors.Is(err, repository.ErrInsufficientStock) {
		return nil, &Error{Code: constant.ErrCodeInsufficientStock, Message: err.Error()}
	}
	if err != nil {
		logger.Error(ctx, constant.LogOrdersCreateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgCheckoutFailed}
	}

	responses := make([]model.OrderResponse, 0, len(orders))
	for _, order := range orders {
		responses = append(responses, order.ToResponse())
	}

	logger.Info(ctx, constant.LogOrdersCreated, logger.Fields{
		"user_id": userID.String(),
		"count":   len(orders),
	})

	return responses, nil
}

// items and reserved must be in the same order.
func buildOrdersByStore(userID uuid.UUID, shippingAddress string, items []model.CartItem, reserved []model.Product) []*model.Order {
	var (
		orders  []*model.Order
		byStore = make(map[uuid.UUID]*model.Order)
	)

	for i, product := range reserved {
		order, ok := byStore[product.StoreID]
		if !ok {
			order = &model.Order{
				UserID:          userID,
				StoreID:         product.StoreID,
				Status:          constant.OrderStatusPending,
				ShippingAddress: shippingAddress,
				Payment:         &model.Payment{Method: constant.PaymentMethodMock, Status: constant.PaymentStatusPending},
			}
			byStore[product.StoreID] = order
			orders = append(orders, order)
		}

		order.OrderItems = append(order.OrderItems, model.OrderItem{
			ProductID: product.ID,
			Quantity:  items[i].Quantity,
			Price:     product.Price,
		})
		order.TotalAmount = order.TotalAmount.Add(product.Price.Mul(decimal.NewFromInt(int64(items[i].Quantity))))
		order.Payment.Amount = order.TotalAmount
	}
	return orders
}

func (s *orderService) GetOrders(ctx context.Context, userID uuid.UUID, page, perPage int) ([]model.OrderResponse, int64, error) {
	orders, total, err := s.orderRepo.FindByUserID(ctx, userID, page, perPage)
	if err != nil {
		logger.Error(ctx, constant.LogOrderFetchFailed, err)
		return nil, 0, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgOrderFetchFailed}
	}

	responses := make([]model.OrderResponse, 0, len(orders))
	for _, order := range orders {
		responses = append(responses, order.ToResponse())
	}

	return responses, total, nil
}

func (s *orderService) GetOrderByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*model.OrderResponse, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgOrderNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogOrderLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	// Someone else's order answers 404 too, so order IDs cannot be probed.
	if order.UserID != userID {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgOrderNotFound}
	}

	resp := order.ToResponse()
	return &resp, nil
}

func (s *orderService) CancelOrder(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	order, err := s.orderRepo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgOrderNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogOrderLoadFailed, err)
		return &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	// Someone else's order answers 404 too, so order IDs cannot be probed.
	if order.UserID != userID {
		return &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgOrderNotFound}
	}

	if !constant.CancellableStatuses[order.Status] {
		return &Error{Code: constant.ErrCodeInvalidStatus, Message: fmt.Sprintf(constant.MsgOrderNotCancellable, order.Status)}
	}

	cancelled, err := s.orderRepo.CancelAndRestock(ctx, id, order.Status)
	if err != nil {
		logger.Error(ctx, constant.LogOrderCancelFailed, err, logger.Fields{
			"order_id": id.String(),
		})
		return &Error{Code: constant.ErrCodeInternal, Message: constant.MsgOrderCancelFailed}
	}
	if !cancelled {
		return &Error{Code: constant.ErrCodeInvalidStatus, Message: constant.MsgOrderCancelConflict}
	}

	logger.Info(ctx, constant.LogOrderCancelled, logger.Fields{
		"order_id": id.String(),
	})

	return nil
}

func (s *orderService) UpdateOrderStatus(ctx context.Context, sellerID uuid.UUID, id uuid.UUID, status string) error {
	order, err := s.orderRepo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgOrderNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogOrderLoadFailed, err)
		return &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	store, err := s.storeRepo.FindByUserID(ctx, sellerID)
	if errors.Is(err, repository.ErrNotFound) {
		return &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgStoreNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogStoreLoadFailed, err)
		return &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	// Another store's order answers 404 too, so order IDs cannot be probed.
	if order.StoreID != store.ID {
		return &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgOrderNotFound}
	}

	allowed, ok := constant.OrderStatusTransitions[order.Status]
	if !ok {
		return &Error{Code: constant.ErrCodeInvalidStatus, Message: fmt.Sprintf(constant.MsgOrderNoTransition, order.Status)}
	}

	if !slices.Contains(allowed, status) {
		return &Error{Code: constant.ErrCodeInvalidStatus, Message: fmt.Sprintf(constant.MsgOrderInvalidTransition, order.Status, status)}
	}

	updated, err := s.orderRepo.UpdateStatusIfCurrent(ctx, id, order.Status, status)
	if err != nil {
		logger.Error(ctx, constant.LogOrderUpdateFailed, err)
		return &Error{Code: constant.ErrCodeInternal, Message: constant.MsgOrderUpdateFailed}
	}
	if !updated {
		return &Error{Code: constant.ErrCodeInvalidStatus, Message: constant.MsgOrderUpdateConflict}
	}
	return nil
}

func (s *orderService) GetSellerOrders(ctx context.Context, sellerID uuid.UUID, page, perPage int) ([]model.OrderResponse, int64, error) {
	store, err := s.storeRepo.FindByUserID(ctx, sellerID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, 0, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgStoreNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogStoreLoadFailed, err)
		return nil, 0, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}

	orders, total, err := s.orderRepo.FindByStoreID(ctx, store.ID, page, perPage)
	if err != nil {
		logger.Error(ctx, constant.LogOrderFetchFailed, err)
		return nil, 0, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgOrderFetchFailed}
	}

	responses := make([]model.OrderResponse, 0, len(orders))
	for _, order := range orders {
		responses = append(responses, order.ToResponse())
	}

	return responses, total, nil
}
