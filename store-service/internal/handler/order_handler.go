package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/pagination"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
)

type OrderHandler struct {
	service service.OrderService
}

func NewOrderHandler(service service.OrderService) *OrderHandler {
	return &OrderHandler{service: service}
}

func (h *OrderHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	var req model.CheckoutRequest

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	if !decodeBody(w, r, &req) {
		return
	}

	addressChars := utf8.RuneCountInString(req.ShippingAddress)
	switch {
	case req.ShippingAddress == "":
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "shipping_address", Message: constant.MsgIsRequired})
		return
	case addressChars > constant.MaxShippingAddressLength:
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "shipping_address", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxShippingAddressLength)})
		return
	}

	orders, err := h.service.Checkout(r.Context(), userID, req.ShippingAddress)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, orders, middleware.BuildMeta(r))
}

func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	page, perPage = pagination.Normalize(page, perPage)

	orders, total, err := h.service.GetOrders(r.Context(), userID, page, perPage)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}

	meta := middleware.BuildMeta(r)
	meta.Pagination = &response.Pagination{
		CurrentPage: page,
		PerPage:     perPage,
		TotalItems:  total,
		TotalPages:  pagination.TotalPages(total, perPage),
	}
	response.Success(w, http.StatusOK, orders, meta)
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "order")})
		return
	}

	order, err := h.service.GetOrderByID(r.Context(), userID, id)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, order, middleware.BuildMeta(r))
}

func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "order")})
		return
	}

	if err := h.service.CancelOrder(r.Context(), userID, id); err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": constant.MsgOrderCancelled}, middleware.BuildMeta(r))
}

func (h *OrderHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateOrderStatusRequest

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "order")})
		return
	}

	if !decodeBody(w, r, &req) {
		return
	}

	if req.Status == "" {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "status", Message: constant.MsgIsRequired})
		return
	}

	if err := h.service.UpdateOrderStatus(r.Context(), userID, id, req.Status); err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": constant.MsgOrderStatusUpdated}, middleware.BuildMeta(r))
}

func (h *OrderHandler) GetSellerOrders(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	page, perPage = pagination.Normalize(page, perPage)

	orders, total, err := h.service.GetSellerOrders(r.Context(), userID, page, perPage)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}

	meta := middleware.BuildMeta(r)
	meta.Pagination = &response.Pagination{
		CurrentPage: page,
		PerPage:     perPage,
		TotalItems:  total,
		TotalPages:  pagination.TotalPages(total, perPage),
	}
	response.Success(w, http.StatusOK, orders, meta)
}
