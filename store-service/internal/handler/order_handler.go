package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	var req model.CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	if req.ShippingAddress == "" {
		response.ValidationError(w, meta, []response.Error{
			response.NewFieldError(constant.ErrCodeValidation, "shipping_address", "is required"),
		})
		return
	}

	resp, err := h.service.Checkout(r.Context(), userID, req.ShippingAddress)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		case strings.Contains(msg, "failed"):
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		default:
			response.ErrorResponse(w, http.StatusBadRequest, meta,
				response.NewError(constant.ErrCodeValidation, msg))
		}
		return
	}

	response.Success(w, http.StatusCreated, resp, meta)
}

func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))

	orders, total, err := h.service.GetOrders(r.Context(), userID, page, perPage)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, meta,
			response.NewError(constant.ErrCodeInternal, err.Error()),
		)
		return
	}

	page, perPage = pagination.Normalize(page, perPage)
	response.SuccessWithPagination(w, http.StatusOK, orders, meta, &response.Pagination{
		CurrentPage: page,
		PerPage:     perPage,
		TotalItems:  total,
		TotalPages:  pagination.TotalPages(total, perPage),
	})
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid order id"),
		)
		return
	}

	resp, err := h.service.GetOrderByID(r.Context(), userID, id)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not found") {
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		} else {
			response.ErrorResponse(w, http.StatusForbidden, meta,
				response.NewError(constant.ErrCodeForbidden, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid order id"),
		)
		return
	}

	if err := h.service.CancelOrder(r.Context(), userID, id); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		case strings.Contains(msg, "forbidden"):
			response.ErrorResponse(w, http.StatusForbidden, meta,
				response.NewError(constant.ErrCodeForbidden, msg))
		case strings.Contains(msg, "failed"):
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		default:
			response.ErrorResponse(w, http.StatusBadRequest, meta,
				response.NewError(constant.ErrCodeInvalidStatus, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "order cancelled"}, meta)
}

func (h *OrderHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid order id"),
		)
		return
	}

	var req model.UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	if req.Status == "" {
		response.ValidationError(w, meta, []response.Error{
			response.NewFieldError(constant.ErrCodeValidation, "status", "is required"),
		})
		return
	}

	if err := h.service.UpdateOrderStatus(r.Context(), userID, id, req.Status); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		case strings.Contains(msg, "forbidden"):
			response.ErrorResponse(w, http.StatusForbidden, meta,
				response.NewError(constant.ErrCodeForbidden, msg))
		case strings.Contains(msg, "failed"):
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		default:
			response.ErrorResponse(w, http.StatusBadRequest, meta,
				response.NewError(constant.ErrCodeInvalidStatus, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "order status updated"}, meta)
}

func (h *OrderHandler) GetSellerOrders(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))

	orders, total, err := h.service.GetSellerOrders(r.Context(), userID, page, perPage)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not found") {
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		} else {
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		}
		return
	}

	page, perPage = pagination.Normalize(page, perPage)
	response.SuccessWithPagination(w, http.StatusOK, orders, meta, &response.Pagination{
		CurrentPage: page,
		PerPage:     perPage,
		TotalItems:  total,
		TotalPages:  pagination.TotalPages(total, perPage),
	})
}
