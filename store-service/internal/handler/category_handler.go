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

type CategoryHandler struct {
	service service.CategoryService
}

func NewCategoryHandler(service service.CategoryService) *CategoryHandler {
	return &CategoryHandler{service: service}
}

func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req model.CategoryRequest

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

	category, err := h.service.CreateCategory(r.Context(), req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, category, middleware.BuildMeta(r))
}

func (h *CategoryHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.service.GetAllCategories(r.Context())
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, categories, middleware.BuildMeta(r))
}

func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req model.CategoryRequest

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "category")})
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

	category, err := h.service.UpdateCategory(r.Context(), id, req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, category, middleware.BuildMeta(r))
}

func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "category")})
		return
	}

	if err := h.service.DeleteCategory(r.Context(), id); err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": constant.MsgCategoryDeleted}, middleware.BuildMeta(r))
}
