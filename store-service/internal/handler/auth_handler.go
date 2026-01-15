package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type AuthHandler struct {
	service service.AuthService
}

func NewAuthHandler(service service.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	var errors []response.Error
	if req.Email == "" {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "email", "is required"))
	} else if !emailRegex.MatchString(req.Email) {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "email", "invalid email format"))
	}
	if req.Password == "" {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "password", "is required"))
	} else if len(req.Password) < 6 {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "password", "minimum 6 characters"))
	}
	if req.Name == "" {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "name", "is required"))
	}
	if len(errors) > 0 {
		response.ValidationError(w, meta, errors)
		return
	}

	resp, err := h.service.Register(r.Context(), req)
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

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	var errors []response.Error
	if req.Email == "" {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "email", "is required"))
	}
	if req.Password == "" {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "password", "is required"))
	}
	if len(errors) > 0 {
		response.ValidationError(w, meta, errors)
		return
	}

	tokenPair, err := h.service.Login(r.Context(), req)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "internal") {
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		} else {
			response.ErrorResponse(w, http.StatusUnauthorized, meta,
				response.NewError(constant.ErrCodeUnauthorized, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, tokenPair, meta)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	var req model.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	if req.RefreshToken == "" {
		response.ValidationError(w, meta, []response.Error{
			response.NewFieldError(constant.ErrCodeValidation, "refresh_token", "is required"),
		})
		return
	}

	tokenPair, err := h.service.RefreshToken(r.Context(), req)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "internal") {
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		} else {
			response.ErrorResponse(w, http.StatusUnauthorized, meta,
				response.NewError(constant.ErrCodeUnauthorized, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, tokenPair, meta)
}
