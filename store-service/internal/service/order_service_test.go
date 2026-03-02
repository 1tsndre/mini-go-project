package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type fakePublisher struct {
	messages []string
	err      error
}

func (f *fakePublisher) MultiPublish(topic string, bodies [][]byte) error {
	if f.err != nil {
		return f.err
	}
	for _, body := range bodies {
		f.messages = append(f.messages, topic+" "+string(body))
	}
	return nil
}

func TestOrderService_Checkout(t *testing.T) {
	var (
		userID    = uuid.New()
		storeA    = uuid.New()
		storeB    = uuid.New()
		productA1 = uuid.New()
		productA2 = uuid.New()
		productB1 = uuid.New()
	)

	cart := &model.Cart{
		UserID: userID,
		Items: []model.CartItem{
			{ProductID: productA1, Quantity: 2},
			{ProductID: productB1, Quantity: 1},
			{ProductID: productA2, Quantity: 3},
		},
	}
	catalog := map[uuid.UUID]model.Product{
		productA1: {ID: productA1, StoreID: storeA, Price: decimal.NewFromInt(100)},
		productA2: {ID: productA2, StoreID: storeA, Price: decimal.NewFromInt(10)},
		productB1: {ID: productB1, StoreID: storeB, Price: decimal.NewFromInt(50)},
	}

	// Runs build like the real repository.
	reserveAll := func(_ context.Context, _ uuid.UUID, build func([]model.CartItem, []model.Product) []*model.Order) ([]*model.Order, error) {
		reserved := make([]model.Product, 0, len(cart.Items))
		for _, item := range cart.Items {
			reserved = append(reserved, catalog[item.ProductID])
		}
		orders := build(cart.Items, reserved)
		for _, o := range orders {
			o.ID = uuid.New()
		}
		return orders, nil
	}

	tests := []struct {
		name        string
		mockSetup   func(orderRepo *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository)
		publishErr  error
		errContains string
		wantOrders  int
	}{
		{
			name: "cart is empty",
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository) {
				orderRepo.EXPECT().CreateOrdersFromCart(gomock.Any(), userID, gomock.Any()).Return(nil, repository.ErrCartEmpty)
			},
			errContains: constant.MsgCartEmpty,
		},
		{
			name: "success - split into one order per store",
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository) {
				orderRepo.EXPECT().CreateOrdersFromCart(gomock.Any(), userID, gomock.Any()).DoAndReturn(reserveAll)
			},
			wantOrders: 2,
		},
		{
			name: "publish fails - orders are cancelled and the cart is given back",
			mockSetup: func(orderRepo *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository) {
				orderRepo.EXPECT().CreateOrdersFromCart(gomock.Any(), userID, gomock.Any()).DoAndReturn(reserveAll)
				orderRepo.EXPECT().CancelAndRestock(gomock.Any(), gomock.Any(), constant.OrderStatusPending).Return(true, nil).Times(2)
				cartRepo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, saved *model.Cart) error {
					assert.Equal(t, userID, saved.UserID)
					assert.Len(t, saved.Items, len(cart.Items))
					return nil
				})
			},
			publishErr:  errors.New("nsqd down"),
			errContains: constant.MsgCheckoutFailed,
		},
		{
			name: "insufficient stock",
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository) {
				orderRepo.EXPECT().CreateOrdersFromCart(gomock.Any(), userID, gomock.Any()).
					Return(nil, fmt.Errorf("%w for product Laptop", repository.ErrInsufficientStock))
			},
			errContains: "insufficient stock for product Laptop",
		},
		{
			name: "database failure",
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository) {
				orderRepo.EXPECT().CreateOrdersFromCart(gomock.Any(), userID, gomock.Any()).
					Return(nil, errors.New("connection reset"))
			},
			errContains: constant.MsgCheckoutFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			cartRepo := mocks.NewMockCartRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, cartRepo)

			publisher := &fakePublisher{err: tt.publishErr}
			svc := NewOrderService(orderRepo, cartRepo, storeRepo, nil, publisher)
			resp, err := svc.Checkout(context.Background(), userID, "Jl. Test No. 1, Jakarta")

			if tt.errContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, resp, tt.wantOrders)
			assert.Len(t, publisher.messages, tt.wantOrders, "one order.created per order")
			for i, message := range publisher.messages {
				assert.Contains(t, message, constant.TopicOrderCreated)
				assert.Contains(t, message, resp[i].ID.String())
			}
		})
	}
}

func TestBuildOrdersByStore(t *testing.T) {
	var (
		userID = uuid.New()
		storeA = uuid.New()
		storeB = uuid.New()
		items  = []model.CartItem{
			{ProductID: uuid.New(), Quantity: 2},
			{ProductID: uuid.New(), Quantity: 1},
			{ProductID: uuid.New(), Quantity: 3},
		}
		reserved = []model.Product{
			{ID: items[0].ProductID, StoreID: storeA, Price: decimal.NewFromInt(100)},
			{ID: items[1].ProductID, StoreID: storeB, Price: decimal.NewFromInt(50)},
			{ID: items[2].ProductID, StoreID: storeA, Price: decimal.NewFromInt(10)},
		}
		orders = buildOrdersByStore(userID, "addr", items, reserved)
	)

	assert.Len(t, orders, 2)
	assert.Equal(t, storeA, orders[0].StoreID)
	assert.Len(t, orders[0].OrderItems, 2)
	assert.True(t, decimal.NewFromInt(230).Equal(orders[0].TotalAmount))
	assert.Equal(t, storeB, orders[1].StoreID)
	assert.True(t, decimal.NewFromInt(50).Equal(orders[1].TotalAmount))
	for _, o := range orders {
		assert.Equal(t, constant.OrderStatusPending, o.Status)
		assert.Equal(t, userID, o.UserID)
		if assert.NotNil(t, o.Payment) {
			assert.Equal(t, constant.PaymentStatusPending, o.Payment.Status)
			assert.True(t, o.TotalAmount.Equal(o.Payment.Amount))
		}
	}
}

func TestOrderService_Checkout_CancelsUnpublishedOrdersAfterTimeout(t *testing.T) {
	var (
		ctrl      = gomock.NewController(t)
		userID    = uuid.New()
		order     = &model.Order{ID: uuid.New()}
		orderRepo = mocks.NewMockOrderRepository(ctrl)
		cartRepo  = mocks.NewMockCartRepository(ctrl)
	)

	ctx, cancel := context.WithCancel(context.Background())

	orderRepo.EXPECT().CreateOrdersFromCart(gomock.Any(), userID, gomock.Any()).
		DoAndReturn(func(context.Context, uuid.UUID, func([]model.CartItem, []model.Product) []*model.Order) ([]*model.Order, error) {
			cancel()
			return []*model.Order{order}, nil
		})
	orderRepo.EXPECT().CancelAndRestock(gomock.Any(), order.ID, constant.OrderStatusPending).
		DoAndReturn(func(ctx context.Context, _ uuid.UUID, _ string) (bool, error) {
			assert.NoError(t, ctx.Err())
			return true, nil
		})
	cartRepo.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ *model.Cart) error {
			assert.NoError(t, ctx.Err())
			return nil
		})

	svc := NewOrderService(orderRepo, cartRepo, mocks.NewMockStoreRepository(ctrl), nil, &fakePublisher{err: errors.New("nsqd down")})
	_, err := svc.Checkout(ctx, userID, "Jl. Test 1")

	assert.ErrorContains(t, err, constant.MsgCheckoutFailed)
}

func TestOrderService_GetOrders(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name      string
		page      int
		perPage   int
		mockSetup func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository)
		wantErr   bool
		wantCount int
		wantTotal int64
	}{
		{
			name:    "success",
			page:    1,
			perPage: 10,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByUserID(gomock.Any(), userID, 1, 10).Return([]model.Order{
					{ID: uuid.New(), UserID: userID, Status: constant.OrderStatusPending, TotalAmount: decimal.NewFromFloat(50000)},
					{ID: uuid.New(), UserID: userID, Status: constant.OrderStatusPaid, TotalAmount: decimal.NewFromFloat(100000)},
				}, int64(2), nil)
			},
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name:    "db error",
			page:    1,
			perPage: 10,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByUserID(gomock.Any(), userID, 1, 10).Return(nil, int64(0), errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, storeRepo)

			svc := NewOrderService(orderRepo, nil, storeRepo, nil, nil)
			orders, total, err := svc.GetOrders(context.Background(), userID, tt.page, tt.perPage)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, orders, tt.wantCount)
			assert.Equal(t, tt.wantTotal, total)
		})
	}
}

func TestOrderService_GetOrderByID(t *testing.T) {
	var (
		ownerID     = uuid.New()
		otherUserID = uuid.New()
		orderID     = uuid.New()
	)

	tests := []struct {
		name        string
		callerID    uuid.UUID
		mockSetup   func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:     "success",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					UserID: ownerID,
					Status: constant.OrderStatusPending,
				}, nil)
			},
		},
		{
			name:     "order not found",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgOrderNotFound,
		},
		{
			name:     "different user gets not found",
			callerID: otherUserID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					UserID: ownerID,
				}, nil)
			},
			wantErr:     true,
			errContains: constant.MsgOrderNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, storeRepo)

			svc := NewOrderService(orderRepo, nil, storeRepo, nil, nil)
			resp, err := svc.GetOrderByID(context.Background(), tt.callerID, orderID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, orderID, resp.ID)
		})
	}
}

func TestOrderService_CancelOrder(t *testing.T) {
	var (
		ownerID     = uuid.New()
		otherUserID = uuid.New()
		orderID     = uuid.New()
	)

	tests := []struct {
		name        string
		callerID    uuid.UUID
		mockSetup   func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:     "success",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					UserID: ownerID,
					Status: constant.OrderStatusPaid,
				}, nil)
				orderRepo.EXPECT().CancelAndRestock(gomock.Any(), orderID, constant.OrderStatusPaid).Return(true, nil)
			},
		},
		{
			name:     "status changed concurrently",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					UserID: ownerID,
					Status: constant.OrderStatusProcessing,
				}, nil)
				orderRepo.EXPECT().CancelAndRestock(gomock.Any(), orderID, constant.OrderStatusProcessing).Return(false, nil)
			},
			wantErr:     true,
			errContains: "status changed",
		},
		{
			name:     "order not found",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgOrderNotFound,
		},
		{
			name:     "different user gets not found",
			callerID: otherUserID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					UserID: ownerID,
					Status: constant.OrderStatusPending,
				}, nil)
			},
			wantErr:     true,
			errContains: constant.MsgOrderNotFound,
		},
		{
			name:     "cannot cancel shipped order",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					UserID: ownerID,
					Status: constant.OrderStatusShipped,
				}, nil)
			},
			wantErr:     true,
			errContains: "cannot cancel order with status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, storeRepo)

			svc := NewOrderService(orderRepo, nil, storeRepo, nil, nil)
			err := svc.CancelOrder(context.Background(), tt.callerID, orderID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestOrderService_UpdateOrderStatus(t *testing.T) {
	var (
		orderID   = uuid.New()
		sellerID  = uuid.New()
		storeID   = uuid.New()
		productID = uuid.New()
	)

	tests := []struct {
		name        string
		sellerID    uuid.UUID
		newStatus   string
		mockSetup   func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:      "success - paid to processing",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:      orderID,
					StoreID: storeID,
					Status:  constant.OrderStatusPaid,
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(&model.Store{ID: storeID}, nil)
				orderRepo.EXPECT().UpdateStatusIfCurrent(gomock.Any(), orderID, constant.OrderStatusPaid, constant.OrderStatusProcessing).Return(true, nil)
			},
		},
		{
			name:      "success - processing to shipping",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusShipping,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:      orderID,
					StoreID: storeID,
					Status:  constant.OrderStatusProcessing,
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(&model.Store{ID: storeID}, nil)
				orderRepo.EXPECT().UpdateStatusIfCurrent(gomock.Any(), orderID, constant.OrderStatusProcessing, constant.OrderStatusShipping).Return(true, nil)
			},
		},
		{
			name:      "status changed concurrently - e.g. buyer cancelled",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusShipping,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:      orderID,
					StoreID: storeID,
					Status:  constant.OrderStatusProcessing,
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(&model.Store{ID: storeID}, nil)
				orderRepo.EXPECT().UpdateStatusIfCurrent(gomock.Any(), orderID, constant.OrderStatusProcessing, constant.OrderStatusShipping).Return(false, nil)
			},
			wantErr:     true,
			errContains: "status changed",
		},
		{
			name:      "order not found",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgOrderNotFound,
		},
		{
			name:      "invalid from status - pending has no valid transition",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:      orderID,
					StoreID: storeID,
					Status:  constant.OrderStatusPending,
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(&model.Store{ID: storeID}, nil)
			},
			wantErr:     true,
			errContains: "cannot transition from status",
		},
		{
			name:      "invalid to status - paid cannot skip to shipped",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusShipped,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:      orderID,
					StoreID: storeID,
					Status:  constant.OrderStatusPaid,
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(&model.Store{ID: storeID}, nil)
			},
			wantErr:     true,
			errContains: "invalid status transition",
		},
		{
			name:      "store not found",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:         orderID,
					Status:     constant.OrderStatusPaid,
					OrderItems: []model.OrderItem{{ProductID: productID, Quantity: 1}},
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgStoreNotFound,
		},
		{
			name:      "order from another store gets not found",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:      orderID,
					StoreID: uuid.New(),
					Status:  constant.OrderStatusPending,
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(&model.Store{ID: storeID}, nil)
			},
			wantErr:     true,
			errContains: constant.MsgOrderNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, storeRepo)

			svc := NewOrderService(orderRepo, nil, storeRepo, nil, nil)
			err := svc.UpdateOrderStatus(context.Background(), tt.sellerID, orderID, tt.newStatus)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestOrderService_GetSellerOrders(t *testing.T) {
	var (
		userID  = uuid.New()
		storeID = uuid.New()
	)

	tests := []struct {
		name        string
		page        int
		perPage     int
		mockSetup   func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
		wantCount   int
	}{
		{
			name:    "success",
			page:    1,
			perPage: 10,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{
					ID:     storeID,
					UserID: userID,
				}, nil)
				orderRepo.EXPECT().FindByStoreID(gomock.Any(), storeID, 1, 10).Return([]model.Order{
					{ID: uuid.New(), UserID: uuid.New(), Status: constant.OrderStatusPaid},
				}, int64(1), nil)
			},
			wantCount: 1,
		},
		{
			name:    "store not found",
			page:    1,
			perPage: 10,
			mockSetup: func(_ *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgStoreNotFound,
		},
		{
			name:    "db error on orders",
			page:    1,
			perPage: 10,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{
					ID:     storeID,
					UserID: userID,
				}, nil)
				orderRepo.EXPECT().FindByStoreID(gomock.Any(), storeID, 1, 10).Return(nil, int64(0), errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, storeRepo)

			svc := NewOrderService(orderRepo, nil, storeRepo, nil, nil)
			orders, _, err := svc.GetSellerOrders(context.Background(), userID, tt.page, tt.perPage)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Len(t, orders, tt.wantCount)
		})
	}
}

func TestOrderService_ProcessPaymentResult(t *testing.T) {
	orderID := uuid.New()

	tests := []struct {
		name      string
		success   bool
		mockSetup func(orderRepo *mocks.MockOrderRepository)
		wantErr   bool
	}{
		{
			name:    "payment success marks order paid",
			success: true,
			mockSetup: func(orderRepo *mocks.MockOrderRepository) {
				orderRepo.EXPECT().MarkPaymentSucceeded(gomock.Any(), orderID).Return(true, nil)
			},
		},
		{
			name:    "payment success for non-pending order is ignored",
			success: true,
			mockSetup: func(orderRepo *mocks.MockOrderRepository) {
				orderRepo.EXPECT().MarkPaymentSucceeded(gomock.Any(), orderID).Return(false, nil)
			},
		},
		{
			name:    "payment success - database error is returned for requeue",
			success: true,
			mockSetup: func(orderRepo *mocks.MockOrderRepository) {
				orderRepo.EXPECT().MarkPaymentSucceeded(gomock.Any(), orderID).Return(false, errors.New("db down"))
			},
			wantErr: true,
		},
		{
			name:    "payment failed cancels and restocks",
			success: false,
			mockSetup: func(orderRepo *mocks.MockOrderRepository) {
				orderRepo.EXPECT().MarkPaymentFailed(gomock.Any(), orderID).Return(true, nil)
			},
		},
		{
			name:    "payment failed for non-pending order is ignored",
			success: false,
			mockSetup: func(orderRepo *mocks.MockOrderRepository) {
				orderRepo.EXPECT().MarkPaymentFailed(gomock.Any(), orderID).Return(false, nil)
			},
		},
		{
			name:    "payment failed - database error is returned for requeue",
			success: false,
			mockSetup: func(orderRepo *mocks.MockOrderRepository) {
				orderRepo.EXPECT().MarkPaymentFailed(gomock.Any(), orderID).Return(false, errors.New("db down"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			tt.mockSetup(orderRepo)

			svc := NewOrderService(orderRepo, mocks.NewMockCartRepository(ctrl), mocks.NewMockStoreRepository(ctrl), nil, nil)
			err := svc.ProcessPaymentResult(context.Background(), orderID, tt.success)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
