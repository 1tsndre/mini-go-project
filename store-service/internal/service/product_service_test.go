package service

import (
	"context"
	"errors"
	"math"
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

func TestProductService_CreateProduct(t *testing.T) {
	var (
		userID     = uuid.New()
		storeID    = uuid.New()
		categoryID = uuid.New()
	)

	tests := []struct {
		name        string
		userID      uuid.UUID
		req         model.CreateProductRequest
		mockSetup   func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:   "success",
			userID: userID,
			req: model.CreateProductRequest{
				Name:        "Laptop",
				Description: "A nice laptop",
				Price:       "15000000",
				Stock:       10,
			},
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
				prodRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "no store found",
			userID: userID,
			req: model.CreateProductRequest{
				Name:  "Laptop",
				Price: "15000000",
			},
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgStoreNotFound,
		},
		{
			name:   "invalid price",
			userID: userID,
			req: model.CreateProductRequest{
				Name:  "Laptop",
				Price: "not-a-number",
			},
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
			},
			wantErr:     true,
			errContains: constant.MsgInvalidPrice,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			prodRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(prodRepo, storeRepo)

			svc := NewProductService(prodRepo, storeRepo)
			resp, err := svc.CreateProduct(context.Background(), tt.userID, categoryID, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, "Laptop", resp.Name)
		})
	}
}

func TestProductService_GetProducts(t *testing.T) {
	tests := []struct {
		name      string
		filter    model.ProductFilter
		mockSetup func(prodRepo *mocks.MockProductRepository)
		wantCount int
		wantTotal int64
		wantErr   bool
	}{
		{
			name:   "success with products",
			filter: model.ProductFilter{Page: 1, PerPage: 10},
			mockSetup: func(prodRepo *mocks.MockProductRepository) {
				prodRepo.EXPECT().FindAll(gomock.Any(), gomock.Any()).Return([]model.Product{
					{ID: uuid.New(), Name: "Laptop", Price: decimal.NewFromInt(15000000)},
					{ID: uuid.New(), Name: "Phone", Price: decimal.NewFromInt(5000000)},
				}, int64(2), nil)
			},
			wantCount: 2,
			wantTotal: 2,
			wantErr:   false,
		},
		{
			name:   "db error",
			filter: model.ProductFilter{Page: 1, PerPage: 10},
			mockSetup: func(prodRepo *mocks.MockProductRepository) {
				prodRepo.EXPECT().FindAll(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			prodRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(prodRepo)

			svc := NewProductService(prodRepo, storeRepo)
			resp, total, err := svc.GetProducts(context.Background(), tt.filter)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, resp, tt.wantCount)
			assert.Equal(t, tt.wantTotal, total)
		})
	}
}

func TestProductService_DeleteProduct(t *testing.T) {
	var (
		userID       = uuid.New()
		storeID      = uuid.New()
		productID    = uuid.New()
		otherStoreID = uuid.New()
	)

	tests := []struct {
		name        string
		userID      uuid.UUID
		productID   uuid.UUID
		mockSetup   func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:      "success",
			userID:    userID,
			productID: productID,
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
				prodRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, StoreID: storeID}, nil)
				prodRepo.EXPECT().Delete(gomock.Any(), productID).Return(nil)
			},
			wantErr: false,
		},
		{
			name:      "not product owner",
			userID:    userID,
			productID: productID,
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
				prodRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, StoreID: otherStoreID}, nil)
			},
			wantErr:     true,
			errContains: constant.MsgNotProductOwner,
		},
		{
			name:      "product not found",
			userID:    userID,
			productID: productID,
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
				prodRepo.EXPECT().FindByID(gomock.Any(), productID).Return(nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgProductNotFound,
		},
		{
			name:      "still referenced by orders or carts",
			userID:    userID,
			productID: productID,
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
				prodRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, StoreID: storeID}, nil)
				prodRepo.EXPECT().Delete(gomock.Any(), productID).Return(repository.ErrForeignKeyViolation)
			},
			wantErr:     true,
			errContains: "in use",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			prodRepo := mocks.NewMockProductRepository(ctrl)
			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(prodRepo, storeRepo)

			svc := NewProductService(prodRepo, storeRepo)
			err := svc.DeleteProduct(context.Background(), tt.userID, tt.productID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestProductService_CreateProduct_PriceAndStock(t *testing.T) {
	tests := []struct {
		name    string
		price   string
		stock   int
		wantErr string
	}{
		{name: "not a number", price: "abc", wantErr: constant.MsgInvalidPrice},
		{name: "zero price", price: "0", wantErr: constant.MsgPriceNotPositive},
		{name: "negative price", price: "-1", wantErr: constant.MsgPriceNotPositive},
		{name: "smallest price", price: "0.01"},
		{name: "trailing zeros", price: "100.000"},
		{name: "largest price", price: "9999999999999.99"},
		{name: "price the column cannot hold", price: "10000000000000", wantErr: constant.MsgPriceTooLarge},
		{name: "three decimals", price: "10.005", wantErr: constant.MsgPriceDecimals},
		{name: "negative stock", price: "1", stock: -1, wantErr: constant.MsgStockNegative},
		{name: "largest stock", price: "1", stock: math.MaxInt32},
		{name: "stock too large", price: "1", stock: math.MaxInt32 + 1, wantErr: constant.MsgStockTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctrl      = gomock.NewController(t)
				userID    = uuid.New()
				prodRepo  = mocks.NewMockProductRepository(ctrl)
				storeRepo = mocks.NewMockStoreRepository(ctrl)
			)

			storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: uuid.New(), UserID: userID}, nil)
			if tt.wantErr == "" {
				prodRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			}

			_, err := NewProductService(prodRepo, storeRepo).CreateProduct(context.Background(), userID, uuid.New(), model.CreateProductRequest{
				Name:  "Laptop",
				Price: tt.price,
				Stock: tt.stock,
			})

			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestProductService_UpdateProduct_RejectsBadPriceAndStock(t *testing.T) {
	var (
		negativeStock = -1
		hugeStock     = math.MaxInt32 + 1
	)

	tests := []struct {
		name    string
		req     model.UpdateProductRequest
		wantErr string
	}{
		{name: "not a number", req: model.UpdateProductRequest{Price: "abc"}, wantErr: constant.MsgInvalidPrice},
		{name: "zero price", req: model.UpdateProductRequest{Price: "0"}, wantErr: constant.MsgPriceNotPositive},
		{name: "price the column cannot hold", req: model.UpdateProductRequest{Price: "10000000000000"}, wantErr: constant.MsgPriceTooLarge},
		{name: "three decimals", req: model.UpdateProductRequest{Price: "10.005"}, wantErr: constant.MsgPriceDecimals},
		{name: "negative stock", req: model.UpdateProductRequest{Stock: &negativeStock}, wantErr: constant.MsgStockNegative},
		{name: "stock too large", req: model.UpdateProductRequest{Stock: &hugeStock}, wantErr: constant.MsgStockTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctrl      = gomock.NewController(t)
				userID    = uuid.New()
				storeID   = uuid.New()
				productID = uuid.New()
				prodRepo  = mocks.NewMockProductRepository(ctrl)
				storeRepo = mocks.NewMockStoreRepository(ctrl)
			)

			storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
			prodRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, StoreID: storeID}, nil)

			_, err := NewProductService(prodRepo, storeRepo).UpdateProduct(context.Background(), userID, productID, uuid.Nil, tt.req)

			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
