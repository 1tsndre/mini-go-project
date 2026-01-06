package service

import (
	"context"
	"errors"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// newTestOrderService creates an OrderService with nil redsync and nsq producer,
// suitable for testing methods that do not exercise distributed locking or messaging.
func newTestOrderService(
	orderRepo *mocks.MockOrderRepository,
	cartRepo *mocks.MockCartRepository,
	productRepo *mocks.MockProductRepository,
	storeRepo *mocks.MockStoreRepository,
) OrderService {
	return NewOrderService(orderRepo, cartRepo, productRepo, storeRepo, nil, nil)
}

func TestOrderService_Checkout(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		mockSetup   func(orderRepo *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository)
		errContains string
	}{
		{
			name: "cart not found",
			mockSetup: func(_ *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(nil, errors.New("not found"))
			},
			errContains: "cart not found",
		},
		{
			name: "cart is empty",
			mockSetup: func(_ *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
					UserID: userID,
					Items:  []model.CartItem{},
				}, nil)
			},
			errContains: "cart is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, cartRepo, productRepo, storeRepo)

			svc := newTestOrderService(orderRepo, cartRepo, productRepo, storeRepo)
			resp, err := svc.Checkout(context.Background(), userID, "Jl. Test No. 1, Jakarta")

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.Nil(t, resp)
		})
	}
}

func TestOrderService_GetOrders(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name      string
		page      int
		perPage   int
		mockSetup func(orderRepo *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository)
		wantErr   bool
		wantCount int
		wantTotal int64
	}{
		{
			name:    "success",
			page:    1,
			perPage: 10,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByUserID(gomock.Any(), userID, 1, 10).Return([]model.Order{
					{ID: uuid.New(), UserID: userID, Status: constant.OrderStatusPending, TotalAmount: decimal.NewFromFloat(50000)},
					{ID: uuid.New(), UserID: userID, Status: constant.OrderStatusPaid, TotalAmount: decimal.NewFromFloat(100000)},
				}, int64(2), nil)
			},
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name:    "default pagination on zero values",
			page:    0,
			perPage: 0,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByUserID(gomock.Any(), userID, 1, 10).Return([]model.Order{}, int64(0), nil)
			},
			wantCount: 0,
			wantTotal: 0,
		},
		{
			name:    "db error",
			page:    1,
			perPage: 10,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
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
			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, cartRepo, productRepo, storeRepo)

			svc := newTestOrderService(orderRepo, cartRepo, productRepo, storeRepo)
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
	ownerID := uuid.New()
	otherUserID := uuid.New()
	orderID := uuid.New()

	tests := []struct {
		name        string
		callerID    uuid.UUID
		mockSetup   func(orderRepo *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:     "success",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
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
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "order not found",
		},
		{
			name:     "forbidden - different user",
			callerID: otherUserID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					UserID: ownerID,
				}, nil)
			},
			wantErr:     true,
			errContains: "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, cartRepo, productRepo, storeRepo)

			svc := newTestOrderService(orderRepo, cartRepo, productRepo, storeRepo)
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
	ownerID := uuid.New()
	otherUserID := uuid.New()
	orderID := uuid.New()

	tests := []struct {
		name        string
		callerID    uuid.UUID
		mockSetup   func(orderRepo *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			// Success path with no order items avoids the redsync distributed lock entirely.
			name:     "success - order with no items",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:         orderID,
					UserID:     ownerID,
					Status:     constant.OrderStatusPending,
					OrderItems: []model.OrderItem{},
				}, nil)
				orderRepo.EXPECT().UpdateStatus(gomock.Any(), orderID, constant.OrderStatusCancelled).Return(nil)
			},
		},
		{
			name:     "order not found",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "order not found",
		},
		{
			name:     "forbidden - different user",
			callerID: otherUserID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					UserID: ownerID,
					Status: constant.OrderStatusPending,
				}, nil)
			},
			wantErr:     true,
			errContains: "forbidden",
		},
		{
			name:     "cannot cancel shipped order",
			callerID: ownerID,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
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
			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, cartRepo, productRepo, storeRepo)

			svc := newTestOrderService(orderRepo, cartRepo, productRepo, storeRepo)
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
	orderID := uuid.New()
	sellerID := uuid.New()
	storeID := uuid.New()
	productID := uuid.New()

	tests := []struct {
		name        string
		sellerID    uuid.UUID
		newStatus   string
		mockSetup   func(orderRepo *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:      "success - paid to processing",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:         orderID,
					Status:     constant.OrderStatusPaid,
					OrderItems: []model.OrderItem{{ProductID: productID, Quantity: 1}},
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(&model.Store{ID: storeID}, nil)
				productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, StoreID: storeID}, nil)
				orderRepo.EXPECT().UpdateStatus(gomock.Any(), orderID, constant.OrderStatusProcessing).Return(nil)
			},
		},
		{
			name:      "success - processing to shipping",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusShipping,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:         orderID,
					Status:     constant.OrderStatusProcessing,
					OrderItems: []model.OrderItem{{ProductID: productID, Quantity: 1}},
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(&model.Store{ID: storeID}, nil)
				productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, StoreID: storeID}, nil)
				orderRepo.EXPECT().UpdateStatus(gomock.Any(), orderID, constant.OrderStatusShipping).Return(nil)
			},
		},
		{
			name:      "order not found",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "order not found",
		},
		{
			name:      "invalid from status - pending has no valid transition",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					Status: constant.OrderStatusPending,
				}, nil)
			},
			wantErr:     true,
			errContains: "cannot transition from status",
		},
		{
			name:      "invalid to status - paid cannot skip to shipped",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusShipped,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:     orderID,
					Status: constant.OrderStatusPaid,
				}, nil)
			},
			wantErr:     true,
			errContains: "invalid status transition",
		},
		{
			name:      "store not found",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:         orderID,
					Status:     constant.OrderStatusPaid,
					OrderItems: []model.OrderItem{{ProductID: productID, Quantity: 1}},
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "store not found",
		},
		{
			name:      "forbidden - order not from seller's store",
			sellerID:  sellerID,
			newStatus: constant.OrderStatusProcessing,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				otherStoreID := uuid.New()
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{
					ID:         orderID,
					Status:     constant.OrderStatusPaid,
					OrderItems: []model.OrderItem{{ProductID: productID, Quantity: 1}},
				}, nil)
				storeRepo.EXPECT().FindByUserID(gomock.Any(), sellerID).Return(&model.Store{ID: storeID}, nil)
				productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, StoreID: otherStoreID}, nil)
			},
			wantErr:     true,
			errContains: "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, cartRepo, productRepo, storeRepo)

			svc := newTestOrderService(orderRepo, cartRepo, productRepo, storeRepo)
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
	userID := uuid.New()
	storeID := uuid.New()

	tests := []struct {
		name        string
		page        int
		perPage     int
		mockSetup   func(orderRepo *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
		wantCount   int
	}{
		{
			name:    "success",
			page:    1,
			perPage: 10,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
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
			mockSetup: func(_ *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "store not found",
		},
		{
			name:    "db error on orders",
			page:    1,
			perPage: 10,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
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
			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, cartRepo, productRepo, storeRepo)

			svc := newTestOrderService(orderRepo, cartRepo, productRepo, storeRepo)
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
		name        string
		success     bool
		mockSetup   func(orderRepo *mocks.MockOrderRepository, cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:    "order not found",
			success: true,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "order not found",
		},
		{
			name:    "payment success - with payment record",
			success: true,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				payment := &model.Payment{ID: uuid.New(), OrderID: orderID}
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{ID: orderID}, nil)
				orderRepo.EXPECT().FindPaymentByOrderID(gomock.Any(), orderID).Return(payment, nil)
				orderRepo.EXPECT().UpdatePayment(gomock.Any(), gomock.Any()).Return(nil)
				orderRepo.EXPECT().UpdateStatus(gomock.Any(), orderID, constant.OrderStatusPaid).Return(nil)
			},
		},
		{
			name:    "payment success - no payment record",
			success: true,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{ID: orderID}, nil)
				orderRepo.EXPECT().FindPaymentByOrderID(gomock.Any(), orderID).Return(nil, errors.New("not found"))
				orderRepo.EXPECT().UpdateStatus(gomock.Any(), orderID, constant.OrderStatusPaid).Return(nil)
			},
		},
		{
			name:    "payment failed - with payment record",
			success: false,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				payment := &model.Payment{ID: uuid.New(), OrderID: orderID}
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{ID: orderID}, nil)
				orderRepo.EXPECT().FindPaymentByOrderID(gomock.Any(), orderID).Return(payment, nil)
				orderRepo.EXPECT().UpdatePayment(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:    "payment failed - no payment record",
			success: false,
			mockSetup: func(orderRepo *mocks.MockOrderRepository, _ *mocks.MockCartRepository, _ *mocks.MockProductRepository, _ *mocks.MockStoreRepository) {
				orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{ID: orderID}, nil)
				orderRepo.EXPECT().FindPaymentByOrderID(gomock.Any(), orderID).Return(nil, errors.New("not found"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orderRepo := mocks.NewMockOrderRepository(ctrl)
			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(orderRepo, cartRepo, productRepo, storeRepo)

			svc := newTestOrderService(orderRepo, cartRepo, productRepo, storeRepo)
			err := svc.ProcessPaymentResult(context.Background(), orderID, tt.success)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
		})
	}
}