package service

import (
	"context"
	"errors"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCartService_GetCart(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()

	tests := []struct {
		name      string
		mockSetup func(cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository)
		wantErr   bool
		checkResp func(t *testing.T, resp *model.CartResponse)
	}{
		{
			name: "success",
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
					UserID: userID,
					Items: []model.CartItem{
						{
							ProductID: productID,
							Name:      "Test Product",
							Price:     decimal.NewFromFloat(10000),
							Quantity:  2,
						},
					},
				}, nil)
			},
			checkResp: func(t *testing.T, resp *model.CartResponse) {
				assert.Len(t, resp.Items, 1)
				assert.True(t, decimal.NewFromFloat(20000).Equal(resp.Total))
			},
		},
		{
			name: "repo error",
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(nil, errors.New("redis error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			tt.mockSetup(cartRepo, productRepo)

			svc := NewCartService(cartRepo, productRepo, nil)
			resp, err := svc.GetCart(context.Background(), userID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			if tt.checkResp != nil {
				tt.checkResp(t, resp)
			}
		})
	}
}

func TestCartService_AddItem(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()

	tests := []struct {
		name        string
		req         model.AddCartItemRequest
		mockSetup   func(cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success - new item",
			req:  model.AddCartItemRequest{ProductID: productID.String(), Quantity: 2},
			mockSetup: func(cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository) {
				productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{
					ID:    productID,
					Name:  "Test Product",
					Price: decimal.NewFromFloat(10000),
					Stock: 10,
				}, nil)
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(nil, errors.New("not found"))
				cartRepo.EXPECT().SaveCart(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "success - existing item incremented",
			req:  model.AddCartItemRequest{ProductID: productID.String(), Quantity: 1},
			mockSetup: func(cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository) {
				productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{
					ID:    productID,
					Name:  "Test Product",
					Price: decimal.NewFromFloat(10000),
					Stock: 10,
				}, nil)
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
					UserID: userID,
					Items: []model.CartItem{
						{ProductID: productID, Name: "Test Product", Price: decimal.NewFromFloat(10000), Quantity: 2},
					},
				}, nil)
				cartRepo.EXPECT().SaveCart(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:        "invalid product_id",
			req:         model.AddCartItemRequest{ProductID: "not-a-uuid", Quantity: 1},
			mockSetup:   func(_ *mocks.MockCartRepository, _ *mocks.MockProductRepository) {},
			wantErr:     true,
			errContains: "invalid product_id",
		},
		{
			name:        "quantity zero",
			req:         model.AddCartItemRequest{ProductID: productID.String(), Quantity: 0},
			mockSetup:   func(_ *mocks.MockCartRepository, _ *mocks.MockProductRepository) {},
			wantErr:     true,
			errContains: "quantity must be greater than 0",
		},
		{
			name: "product not found",
			req:  model.AddCartItemRequest{ProductID: productID.String(), Quantity: 1},
			mockSetup: func(_ *mocks.MockCartRepository, productRepo *mocks.MockProductRepository) {
				productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "product not found",
		},
		{
			name: "insufficient stock",
			req:  model.AddCartItemRequest{ProductID: productID.String(), Quantity: 5},
			mockSetup: func(_ *mocks.MockCartRepository, productRepo *mocks.MockProductRepository) {
				productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{
					ID:    productID,
					Stock: 3,
				}, nil)
			},
			wantErr:     true,
			errContains: "insufficient stock",
		},
		{
			name: "save cart fails",
			req:  model.AddCartItemRequest{ProductID: productID.String(), Quantity: 1},
			mockSetup: func(cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository) {
				productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{
					ID:    productID,
					Stock: 10,
				}, nil)
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(nil, errors.New("not found"))
				cartRepo.EXPECT().SaveCart(gomock.Any(), gomock.Any()).Return(errors.New("redis error"))
			},
			wantErr:     true,
			errContains: "failed to save cart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			tt.mockSetup(cartRepo, productRepo)

			svc := NewCartService(cartRepo, productRepo, nil)
			resp, err := svc.AddItem(context.Background(), userID, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

func TestCartService_UpdateItem(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()
	otherProductID := uuid.New()

	tests := []struct {
		name        string
		productID   uuid.UUID
		req         model.UpdateCartItemRequest
		mockSetup   func(cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:      "success",
			productID: productID,
			req:       model.UpdateCartItemRequest{Quantity: 3},
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
					UserID: userID,
					Items:  []model.CartItem{{ProductID: productID, Quantity: 1}},
				}, nil)
				cartRepo.EXPECT().SaveCart(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:        "quantity zero",
			productID:   productID,
			req:         model.UpdateCartItemRequest{Quantity: 0},
			mockSetup:   func(_ *mocks.MockCartRepository, _ *mocks.MockProductRepository) {},
			wantErr:     true,
			errContains: "quantity must be greater than 0",
		},
		{
			name:      "cart not found",
			productID: productID,
			req:       model.UpdateCartItemRequest{Quantity: 1},
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(nil, errors.New("db error"))
			},
			wantErr:     true,
			errContains: "failed to load cart",
		},
		{
			name:      "item not in cart",
			productID: otherProductID,
			req:       model.UpdateCartItemRequest{Quantity: 1},
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
					UserID: userID,
					Items:  []model.CartItem{{ProductID: productID, Quantity: 1}},
				}, nil)
			},
			wantErr:     true,
			errContains: "item not found in cart",
		},
		{
			name:      "save fails",
			productID: productID,
			req:       model.UpdateCartItemRequest{Quantity: 2},
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
					UserID: userID,
					Items:  []model.CartItem{{ProductID: productID, Quantity: 1}},
				}, nil)
				cartRepo.EXPECT().SaveCart(gomock.Any(), gomock.Any()).Return(errors.New("redis error"))
			},
			wantErr:     true,
			errContains: "failed to save cart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			tt.mockSetup(cartRepo, productRepo)

			svc := NewCartService(cartRepo, productRepo, nil)
			resp, err := svc.UpdateItem(context.Background(), userID, tt.productID, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

func TestCartService_RemoveItem(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()
	otherProductID := uuid.New()

	tests := []struct {
		name        string
		productID   uuid.UUID
		mockSetup   func(cartRepo *mocks.MockCartRepository, productRepo *mocks.MockProductRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:      "success",
			productID: productID,
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
					UserID: userID,
					Items:  []model.CartItem{{ProductID: productID, Quantity: 1}},
				}, nil)
				cartRepo.EXPECT().SaveCart(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:      "cart not found",
			productID: productID,
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(nil, errors.New("db error"))
			},
			wantErr:     true,
			errContains: "failed to load cart",
		},
		{
			name:      "item not in cart",
			productID: otherProductID,
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
					UserID: userID,
					Items:  []model.CartItem{{ProductID: productID, Quantity: 1}},
				}, nil)
			},
			wantErr:     true,
			errContains: "item not found in cart",
		},
		{
			name:      "save fails",
			productID: productID,
			mockSetup: func(cartRepo *mocks.MockCartRepository, _ *mocks.MockProductRepository) {
				cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
					UserID: userID,
					Items:  []model.CartItem{{ProductID: productID, Quantity: 1}},
				}, nil)
				cartRepo.EXPECT().SaveCart(gomock.Any(), gomock.Any()).Return(errors.New("redis error"))
			},
			wantErr:     true,
			errContains: "failed to save cart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cartRepo := mocks.NewMockCartRepository(ctrl)
			productRepo := mocks.NewMockProductRepository(ctrl)
			tt.mockSetup(cartRepo, productRepo)

			svc := NewCartService(cartRepo, productRepo, nil)
			resp, err := svc.RemoveItem(context.Background(), userID, tt.productID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Empty(t, resp.Items)
		})
	}
}