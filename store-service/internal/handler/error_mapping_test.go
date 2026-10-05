package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestUpdateOrderStatus_InvalidStatusIs400WhateverItSays(t *testing.T) {
	for _, status := range []string{"failed", "not found", "forbidden", "unknown"} {
		t.Run(status, func(t *testing.T) {
			var (
				ctrl      = gomock.NewController(t)
				orderID   = uuid.New()
				storeID   = uuid.New()
				orderRepo = mocks.NewMockOrderRepository(ctrl)
				storeRepo = mocks.NewMockStoreRepository(ctrl)
			)

			orderRepo.EXPECT().FindByID(gomock.Any(), orderID).Return(&model.Order{ID: orderID, StoreID: storeID, Status: constant.OrderStatusPaid}, nil)
			storeRepo.EXPECT().FindByUserID(gomock.Any(), gomock.Any()).Return(&model.Store{ID: storeID}, nil)

			svc := service.NewOrderService(orderRepo, mocks.NewMockCartRepository(ctrl), storeRepo, nil, nil)
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

func TestCheckout_InsufficientStockIs400WhateverTheProductName(t *testing.T) {
	for _, name := range []string{"Lost and not found mug", "failed prototype tee", "Plain mug"} {
		t.Run(name, func(t *testing.T) {
			var (
				ctrl      = gomock.NewController(t)
				userID    = uuid.New()
				orderRepo = mocks.NewMockOrderRepository(ctrl)
			)

			orderRepo.EXPECT().CreateOrdersFromCart(gomock.Any(), userID, gomock.Any()).
				Return(nil, fmt.Errorf("%w for product %s", repository.ErrInsufficientStock, name))

			svc := service.NewOrderService(orderRepo, mocks.NewMockCartRepository(ctrl), mocks.NewMockStoreRepository(ctrl), nil, nil)
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

func TestServiceFailure(t *testing.T) {
	expired, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	tests := []struct {
		name        string
		ctx         context.Context
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{name: "validation", err: &service.Error{Code: constant.ErrCodeValidation, Message: "m"}, wantStatus: 400, wantCode: constant.ErrCodeValidation, wantMessage: "m"},
		{name: "insufficient stock", err: &service.Error{Code: constant.ErrCodeInsufficientStock, Message: "m"}, wantStatus: 400, wantCode: constant.ErrCodeInsufficientStock, wantMessage: "m"},
		{name: "invalid status", err: &service.Error{Code: constant.ErrCodeInvalidStatus, Message: "m"}, wantStatus: 400, wantCode: constant.ErrCodeInvalidStatus, wantMessage: "m"},
		{name: "unauthorized", err: &service.Error{Code: constant.ErrCodeUnauthorized, Message: "m"}, wantStatus: 401, wantCode: constant.ErrCodeUnauthorized, wantMessage: "m"},
		{name: "forbidden", err: &service.Error{Code: constant.ErrCodeForbidden, Message: "m"}, wantStatus: 403, wantCode: constant.ErrCodeForbidden, wantMessage: "m"},
		{name: "not found", err: &service.Error{Code: constant.ErrCodeNotFound, Message: "m"}, wantStatus: 404, wantCode: constant.ErrCodeNotFound, wantMessage: "m"},
		{name: "conflict", err: &service.Error{Code: constant.ErrCodeConflict, Message: "m"}, wantStatus: 409, wantCode: constant.ErrCodeConflict, wantMessage: "m"},
		{name: "internal", err: &service.Error{Code: constant.ErrCodeInternal, Message: "m"}, wantStatus: 500, wantCode: constant.ErrCodeInternal, wantMessage: "m"},
		{name: "any other error is not shown", err: errors.New("pq: secret detail"), wantStatus: 500, wantCode: constant.ErrCodeInternal, wantMessage: constant.MsgInternalServerError},
		{name: "past the deadline", ctx: expired, err: &service.Error{Code: constant.ErrCodeNotFound, Message: "m"}, wantStatus: 504, wantCode: constant.ErrCodeTimeout, wantMessage: constant.MsgRequestTimedOut},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.ctx != nil {
				r = r.WithContext(tt.ctx)
			}
			rec := httptest.NewRecorder()
			serviceFailure(rec, r, tt.err)

			assert.Equal(t, tt.wantStatus, rec.Code)
			errBody := decodeError(t, rec)
			assert.Equal(t, tt.wantCode, errBody.Code)
			assert.Equal(t, tt.wantMessage, errBody.Message)
		})
	}
}

func TestAddCartItem_InsufficientStockCode(t *testing.T) {
	var (
		ctrl        = gomock.NewController(t)
		userID      = uuid.New()
		productID   = uuid.New()
		productRepo = mocks.NewMockProductRepository(ctrl)
	)

	productRepo.EXPECT().FindByID(gomock.Any(), productID).Return(&model.Product{ID: productID, Stock: 1}, nil)

	h := NewCartHandler(service.NewCartService(mocks.NewMockCartRepository(ctrl), productRepo, nil))

	rec := httptest.NewRecorder()
	h.AddItem(rec, requestAs(userID, http.MethodPost, "/api/v1/cart/items", `{"product_id":"`+productID.String()+`","quantity":2}`))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, constant.ErrCodeInsufficientStock, decodeError(t, rec).Code)
}
