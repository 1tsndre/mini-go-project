package handler

import (
	"fmt"
	"net/http"

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
	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	cart, err := h.service.GetCart(r.Context(), userID)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, cart, middleware.BuildMeta(r))
}

func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	var (
		req  model.AddCartItemRequest
		errs []response.Error
	)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	if !decodeBody(w, r, &req) {
		return
	}

	productID, err := uuid.Parse(req.ProductID)
	switch {
	case req.ProductID == "":
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "product_id", Message: constant.MsgIsRequired})
	case err != nil:
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "product_id", Message: constant.MsgInvalidUUID})
	}

	if req.Quantity <= 0 {
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "quantity", Message: constant.MsgGreaterThanZero})
	}

	if len(errs) > 0 {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), errs...)
		return
	}

	cart, err := h.service.AddItem(r.Context(), userID, productID, req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, cart, middleware.BuildMeta(r))
}

func (h *CartHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateCartItemRequest

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	productID, err := uuid.Parse(r.PathValue("product_id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "product")})
		return
	}

	if !decodeBody(w, r, &req) {
		return
	}

	if req.Quantity <= 0 {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "quantity", Message: constant.MsgGreaterThanZero})
		return
	}

	cart, err := h.service.UpdateItem(r.Context(), userID, productID, req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, cart, middleware.BuildMeta(r))
}

func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	productID, err := uuid.Parse(r.PathValue("product_id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "product")})
		return
	}

	cart, err := h.service.RemoveItem(r.Context(), userID, productID)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, cart, middleware.BuildMeta(r))
}
