package handler

import (
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
)

type StoreHandler struct {
	service service.StoreService
}

func NewStoreHandler(service service.StoreService) *StoreHandler {
	return &StoreHandler{service: service}
}

func (h *StoreHandler) CreateStore(w http.ResponseWriter, r *http.Request) {
	var req model.StoreRequest

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	if !decodeBody(w, r, &req) {
		return
	}

	nameChars := utf8.RuneCountInString(req.Name)
	switch {
	case req.Name == "":
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "name", Message: constant.MsgIsRequired})
		return
	case nameChars > constant.MaxVarcharLength:
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "name", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxVarcharLength)})
		return
	}

	descriptionChars := utf8.RuneCountInString(req.Description)
	if descriptionChars > constant.MaxDescriptionLength {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "description", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxDescriptionLength)})
		return
	}

	store, err := h.service.CreateStore(r.Context(), userID, req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, store, middleware.BuildMeta(r))
}

func (h *StoreHandler) GetStore(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "store")})
		return
	}

	store, err := h.service.GetStoreByID(r.Context(), id)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, store, middleware.BuildMeta(r))
}

func (h *StoreHandler) GetMyStore(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	store, err := h.service.GetStoreByUserID(r.Context(), userID)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, store, middleware.BuildMeta(r))
}

func (h *StoreHandler) UpdateStore(w http.ResponseWriter, r *http.Request) {
	var req model.StoreRequest

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "store")})
		return
	}

	if !decodeBody(w, r, &req) {
		return
	}

	nameChars := utf8.RuneCountInString(req.Name)
	if nameChars > constant.MaxVarcharLength {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "name", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxVarcharLength)})
		return
	}

	descriptionChars := utf8.RuneCountInString(req.Description)
	if descriptionChars > constant.MaxDescriptionLength {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "description", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxDescriptionLength)})
		return
	}

	store, err := h.service.UpdateStore(r.Context(), userID, id, req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, store, middleware.BuildMeta(r))
}
