package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"unicode/utf8"

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
	var (
		req  model.RegisterRequest
		errs []response.Error
	)

	if !decodeBody(w, r, &req) {
		return
	}

	emailChars := utf8.RuneCountInString(req.Email)
	switch {
	case req.Email == "":
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "email", Message: constant.MsgIsRequired})
	case emailChars > constant.MaxVarcharLength:
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "email", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxVarcharLength)})
	case !emailRegex.MatchString(req.Email):
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "email", Message: constant.MsgInvalidEmail})
	}

	passwordChars := utf8.RuneCountInString(req.Password)
	passwordBytes := len(req.Password)
	switch {
	case req.Password == "":
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "password", Message: constant.MsgIsRequired})
	case passwordChars < constant.MinPasswordLength:
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "password", Message: fmt.Sprintf(constant.MsgMinCharacters, constant.MinPasswordLength)})
	case passwordBytes > constant.MaxPasswordBytes:
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "password", Message: fmt.Sprintf(constant.MsgMaxBytes, constant.MaxPasswordBytes)})
	}

	nameChars := utf8.RuneCountInString(req.Name)
	switch {
	case req.Name == "":
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "name", Message: constant.MsgIsRequired})
	case nameChars > constant.MaxVarcharLength:
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "name", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxVarcharLength)})
	}

	if len(errs) > 0 {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), errs...)
		return
	}

	user, err := h.service.Register(r.Context(), req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, user, middleware.BuildMeta(r))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var (
		req  model.LoginRequest
		errs []response.Error
	)

	if !decodeBody(w, r, &req) {
		return
	}

	if req.Email == "" {
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "email", Message: constant.MsgIsRequired})
	}
	if req.Password == "" {
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "password", Message: constant.MsgIsRequired})
	}

	if len(errs) > 0 {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), errs...)
		return
	}

	tokens, err := h.service.Login(r.Context(), req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, tokens, middleware.BuildMeta(r))
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshTokenRequest

	if !decodeBody(w, r, &req) {
		return
	}

	if req.RefreshToken == "" {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "refresh_token", Message: constant.MsgIsRequired})
		return
	}

	tokens, err := h.service.Refresh(r.Context(), req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, tokens, middleware.BuildMeta(r))
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshTokenRequest

	if !decodeBody(w, r, &req) {
		return
	}

	if req.RefreshToken == "" {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "refresh_token", Message: constant.MsgIsRequired})
		return
	}

	if err := h.service.Logout(r.Context(), req); err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": constant.MsgLoggedOut}, middleware.BuildMeta(r))
}
