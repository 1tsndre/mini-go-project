package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
)

type CartHandler struct {
	service service.CartService
}

func NewCartHandler(service service.CartService) *CartHandler {
	return &CartHandler{service: service}
}

func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	resp, err := h.service.GetCart(r.Context(), userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, meta,
			response.NewError(constant.ErrCodeInternal, err.Error()),
		)
		return
	}

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	var req model.AddCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	var errors []response.Error
	if req.ProductID == "" {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "product_id", "is required"))
	}
	if req.Quantity <= 0 {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "quantity", "must be greater than 0"))
	}
	if len(errors) > 0 {
		response.ValidationError(w, meta, errors)
		return
	}

	resp, err := h.service.AddItem(r.Context(), userID, req)
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

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *CartHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	productID, err := uuid.Parse(r.PathValue("product_id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid product id"),
		)
		return
	}

	var req model.UpdateCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	if req.Quantity <= 0 {
		response.ValidationError(w, meta, []response.Error{
			response.NewFieldError(constant.ErrCodeValidation, "quantity", "must be greater than 0"),
		})
		return
	}

	resp, err := h.service.UpdateItem(r.Context(), userID, productID, req)
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

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	productID, err := uuid.Parse(r.PathValue("product_id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid product id"),
		)
		return
	}

	resp, err := h.service.RemoveItem(r.Context(), userID, productID)
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

	response.Success(w, http.StatusOK, resp, meta)
}
