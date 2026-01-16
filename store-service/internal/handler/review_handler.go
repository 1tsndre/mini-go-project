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

type ReviewHandler struct {
	service service.ReviewService
}

func NewReviewHandler(service service.ReviewService) *ReviewHandler {
	return &ReviewHandler{service: service}
}

func (h *ReviewHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid product id"),
		)
		return
	}

	var req model.CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	var errors []response.Error
	if req.Rating < 1 || req.Rating > 5 {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "rating", "must be between 1 and 5"))
	}
	if len(errors) > 0 {
		response.ValidationError(w, meta, errors)
		return
	}

	resp, err := h.service.CreateReview(r.Context(), userID, productID, req)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "already reviewed"):
			response.ErrorResponse(w, http.StatusConflict, meta,
				response.NewError(constant.ErrCodeConflict, msg))
		case strings.Contains(msg, "must purchase"):
			response.ErrorResponse(w, http.StatusForbidden, meta,
				response.NewError(constant.ErrCodeForbidden, msg))
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

func (h *ReviewHandler) GetProductReviews(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid product id"),
		)
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))

	reviews, total, err := h.service.GetProductReviews(r.Context(), productID, page, perPage)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, meta,
			response.NewError(constant.ErrCodeInternal, err.Error()),
		)
		return
	}

	page, perPage = pagination.Normalize(page, perPage)
	response.SuccessWithPagination(w, http.StatusOK, reviews, meta, &response.Pagination{
		CurrentPage: page,
		PerPage:     perPage,
		TotalItems:  total,
		TotalPages:  pagination.TotalPages(total, perPage),
	})
}
