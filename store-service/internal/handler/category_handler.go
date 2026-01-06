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

type CategoryHandler struct {
	service service.CategoryService
}

func NewCategoryHandler(service service.CategoryService) *CategoryHandler {
	return &CategoryHandler{service: service}
}

func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	var req model.CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	if req.Name == "" {
		response.ValidationError(w, meta, []response.Error{
			response.NewFieldError(constant.ErrCodeValidation, "name", "is required"),
		})
		return
	}

	resp, err := h.service.CreateCategory(r.Context(), req)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already exists") {
			response.ErrorResponse(w, http.StatusConflict, meta,
				response.NewError(constant.ErrCodeConflict, msg))
		} else {
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		}
		return
	}

	response.Success(w, http.StatusCreated, resp, meta)
}

func (h *CategoryHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	resp, err := h.service.GetAllCategories(r.Context())
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, meta,
			response.NewError(constant.ErrCodeInternal, err.Error()),
		)
		return
	}

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid category id"),
		)
		return
	}

	var req model.UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	resp, err := h.service.UpdateCategory(r.Context(), id, req)
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

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid category id"),
		)
		return
	}

	if err := h.service.DeleteCategory(r.Context(), id); err != nil {
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

	response.Success(w, http.StatusOK, map[string]string{"message": "category deleted"}, meta)
}
