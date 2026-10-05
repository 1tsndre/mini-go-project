package service

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestProductService_CreateProduct(t *testing.T) {
	userID := uuid.New()
	storeID := uuid.New()
	categoryID := uuid.New()

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
				CategoryID:  categoryID.String(),
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
				CategoryID: categoryID.String(),
				Name:       "Laptop",
				Price:      "15000000",
			},
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "store not found",
		},
		{
			name:   "invalid price",
			userID: userID,
			req: model.CreateProductRequest{
				CategoryID: categoryID.String(),
				Name:       "Laptop",
				Price:      "not-a-number",
			},
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
			},
			wantErr:     true,
			errContains: "invalid price",
		},
		{
			name:   "invalid category_id",
			userID: userID,
			req: model.CreateProductRequest{
				CategoryID: "not-a-uuid",
				Name:       "Laptop",
				Price:      "15000000",
			},
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
			},
			wantErr:     true,
			errContains: "invalid category_id",
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
			resp, err := svc.CreateProduct(context.Background(), tt.userID, tt.req)

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
			name:   "default pagination when zero",
			filter: model.ProductFilter{Page: 0, PerPage: 0},
			mockSetup: func(prodRepo *mocks.MockProductRepository) {
				prodRepo.EXPECT().FindAll(gomock.Any(), gomock.Any()).Return([]model.Product{}, int64(0), nil)
			},
			wantCount: 0,
			wantTotal: 0,
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
	userID := uuid.New()
	storeID := uuid.New()
	productID := uuid.New()
	otherStoreID := uuid.New()

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
			errContains: "forbidden",
		},
		{
			name:      "product not found",
			userID:    userID,
			productID: productID,
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
				prodRepo.EXPECT().FindByID(gomock.Any(), productID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "product not found",
		},
		{
			name:      "still referenced by orders or carts",
			userID:    userID,
			productID: productID,
			mockSetup: func(prodRepo *mocks.MockProductRepository, storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: storeID, UserID: userID}, nil)
				prodRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, StoreID: storeID}, nil)
				prodRepo.EXPECT().Delete(gomock.Any(), productID).Return(gorm.ErrForeignKeyViolated)
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

// Prices and stock must fit the DECIMAL(15,2) and INTEGER columns exactly;
// otherwise PostgreSQL rejects them (a 500) or silently rounds the price.
func TestValidatePriceAndStock(t *testing.T) {
	priceTests := []struct {
		price   string
		wantErr string
	}{
		{price: "0", wantErr: "greater than 0"},
		{price: "-1", wantErr: "greater than 0"},
		{price: "0.01"},
		{price: "100.000"},
		{price: "9999999999999.99"},
		{price: "10000000000000", wantErr: "too large"},
		{price: "10.005", wantErr: "at most 2 decimal places"},
	}
	for _, tt := range priceTests {
		err := validatePrice(decimal.RequireFromString(tt.price))
		if tt.wantErr == "" {
			assert.NoError(t, err, tt.price)
			continue
		}
		assert.ErrorContains(t, err, tt.wantErr, tt.price)
	}

	assert.ErrorContains(t, validateStock(-1), "must not be negative")
	assert.NoError(t, validateStock(0))
	assert.NoError(t, validateStock(math.MaxInt32))
	assert.ErrorContains(t, validateStock(math.MaxInt32+1), "too large")
}

func TestProductService_CreateProduct_RejectsPriceTheColumnCannotHold(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.New()
	storeRepo := mocks.NewMockStoreRepository(ctrl)
	storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{ID: uuid.New(), UserID: userID}, nil)

	svc := NewProductService(mocks.NewMockProductRepository(ctrl), storeRepo)
	_, err := svc.CreateProduct(context.Background(), userID, model.CreateProductRequest{
		CategoryID: uuid.New().String(),
		Name:       "Laptop",
		Price:      "99999999999999",
	})

	assert.ErrorContains(t, err, "price is too large")
}
