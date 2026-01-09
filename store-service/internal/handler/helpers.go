package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
)

var statusByCode = map[string]int{
	constant.ErrCodeValidation:        http.StatusBadRequest,
	constant.ErrCodeInsufficientStock: http.StatusBadRequest,
	constant.ErrCodeInvalidStatus:     http.StatusBadRequest,
	constant.ErrCodeUnauthorized:      http.StatusUnauthorized,
	constant.ErrCodeForbidden:         http.StatusForbidden,
	constant.ErrCodeNotFound:          http.StatusNotFound,
	constant.ErrCodeConflict:          http.StatusConflict,
	constant.ErrCodeInternal:          http.StatusInternalServerError,
}

func serviceFailure(w http.ResponseWriter, r *http.Request, err error) {
	var svcErr *service.Error

	// Past the deadline every error is really a timeout.
	if errors.Is(r.Context().Err(), context.DeadlineExceeded) {
		response.Failure(w, http.StatusGatewayTimeout, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeTimeout, Message: constant.MsgRequestTimedOut})
		return
	}

	if !errors.As(err, &svcErr) {
		svcErr = &service.Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}
	response.Failure(w, statusByCode[svcErr.Code], middleware.BuildMeta(r), response.Error{Code: svcErr.Code, Message: svcErr.Message})
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, err := io.ReadAll(r.Body)
	if err != nil || bytes.Contains(body, []byte(constant.EscapedNUL)) || json.Unmarshal(body, dst) != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: constant.MsgInvalidRequestBody})
		return false
	}
	return true
}
