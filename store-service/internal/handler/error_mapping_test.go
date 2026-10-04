package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func requestAs(userID uuid.UUID, method, target, body string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), middleware.ContextUserID, userID.String()))
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) response.Error {
	t.Helper()
	var body response.Response
	if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body)) && assert.Len(t, body.Errors, 1) {
		return body.Errors[0]
	}
	return response.Error{}
}

// A seller-supplied status is echoed in the error message; words like "failed"
// in it used to turn a plain invalid request into a 500.
func TestUpdateOrderStatus_InvalidStatusIs400WhateverItSays(t *testing.T) {
	for _, status := range []string{"failed", "not found", "forbidden", "unknown"} {
		t.Run(status, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			orderID := uuid.New()
			orderRepo := mocks.NewMockOrderRepository(ctrl)
			orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{ID: orderID, Status: constant.OrderStatusPaid}, nil)

			svc := service.NewOrderService(orderRepo, mocks.NewMockCartRepository(ctrl), mocks.NewMockStoreRepository(ctrl), nil, nil)
			h := NewOrderHandler(svc, nil)

			req := requestAs(uuid.New(), http.MethodPut, "/api/v1/orders/"+orderID.String()+"/status", `{"status":"`+status+`"}`)
			req.SetPathValue("id", orderID.String())
			rec := httptest.NewRecorder()
			h.UpdateOrderStatus(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, constant.ErrCodeInvalidStatus, decodeError(t, rec).Code)
		})
	}
}

// A product name is part of the insufficient-stock message, so a name containing
// "not found" or "failed" used to be reported as a 404 or 500.
func TestCheckout_InsufficientStockIs400WhateverTheProductName(t *testing.T) {
	for _, name := range []string{"Lost and not found mug", "failed prototype tee", "Plain mug"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			userID := uuid.New()
			productID := uuid.New()
			cartRepo := mocks.NewMockCartRepository(ctrl)
			orderRepo := mocks.NewMockOrderRepository(ctrl)
			cartRepo.EXPECT().GetCart(gomock.Any(), userID).Return(&model.Cart{
				UserID: userID,
				Items:  []model.CartItem{{ProductID: productID, Quantity: 5}},
			}, nil)
			orderRepo.EXPECT().CreateOrdersWithStock(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, &repository.ErrInsufficientStock{ProductID: productID, ProductName: name})

			svc := service.NewOrderService(orderRepo, cartRepo, mocks.NewMockStoreRepository(ctrl), nil, nil)
			h := NewOrderHandler(svc, nil)

			rec := httptest.NewRecorder()
			h.Checkout(rec, requestAs(userID, http.MethodPost, "/api/v1/orders", `{"shipping_address":"Jl. Test 1"}`))

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			errBody := decodeError(t, rec)
			assert.Equal(t, constant.ErrCodeInsufficientStock, errBody.Code)
			assert.Contains(t, errBody.Message, name)
		})
	}
}

func TestAddCartItem_InsufficientStockCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.New()
	productID := uuid.New()
	productRepo := mocks.NewMockProductRepository(ctrl)
	productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, Stock: 1}, nil)

	h := NewCartHandler(service.NewCartService(mocks.NewMockCartRepository(ctrl), productRepo, nil))

	rec := httptest.NewRecorder()
	h.AddItem(rec, requestAs(userID, http.MethodPost, "/api/v1/cart/items", `{"product_id":"`+productID.String()+`","quantity":2}`))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, constant.ErrCodeInsufficientStock, decodeError(t, rec).Code)
}
