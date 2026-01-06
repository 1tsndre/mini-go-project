package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/pkg/upload"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
)

type StoreHandler struct {
	service  service.StoreService
	uploader *upload.Uploader
}

func NewStoreHandler(service service.StoreService, uploader *upload.Uploader) *StoreHandler {
	return &StoreHandler{service: service, uploader: uploader}
}

func (h *StoreHandler) CreateStore(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	var req model.CreateStoreRequest
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

	resp, err := h.service.CreateStore(r.Context(), userID, req)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already") {
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

func (h *StoreHandler) GetStore(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid store id"),
		)
		return
	}

	resp, err := h.service.GetStoreByID(r.Context(), id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, meta,
			response.NewError(constant.ErrCodeNotFound, err.Error()),
		)
		return
	}

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *StoreHandler) UpdateStore(w http.ResponseWriter, r *http.Request) {
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
			response.NewError(constant.ErrCodeValidation, "invalid store id"),
		)
		return
	}

	var req model.UpdateStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	resp, err := h.service.UpdateStore(r.Context(), userID, id, req)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		case strings.Contains(msg, "forbidden"):
			response.ErrorResponse(w, http.StatusForbidden, meta,
				response.NewError(constant.ErrCodeForbidden, msg))
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *StoreHandler) UploadLogo(w http.ResponseWriter, r *http.Request) {
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
			response.NewError(constant.ErrCodeValidation, "invalid store id"),
		)
		return
	}

	file, header, err := r.FormFile("logo")
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "logo file is required"),
		)
		return
	}
	defer file.Close()

	path, err := h.uploader.Upload(file, header, "stores")
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, err.Error()),
		)
		return
	}

	resp, err := h.service.UpdateLogo(r.Context(), userID, id, path)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		case strings.Contains(msg, "forbidden"):
			response.ErrorResponse(w, http.StatusForbidden, meta,
				response.NewError(constant.ErrCodeForbidden, msg))
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, resp, meta)
}
