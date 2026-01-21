package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"unicode/utf8"

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
	var req model.CreateReviewRequest

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "product")})
		return
	}

	if !decodeBody(w, r, &req) {
		return
	}

	if req.Rating < constant.MinRating || req.Rating > constant.MaxRating {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "rating", Message: constant.MsgRatingRange})
		return
	}

	commentChars := utf8.RuneCountInString(req.Comment)
	if commentChars > constant.MaxCommentLength {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "comment", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxCommentLength)})
		return
	}

	review, err := h.service.CreateReview(r.Context(), userID, productID, req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, review, middleware.BuildMeta(r))
}

func (h *ReviewHandler) GetProductReviews(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "product")})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	page, perPage = pagination.Normalize(page, perPage)

	reviews, total, err := h.service.GetProductReviews(r.Context(), productID, page, perPage)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}

	meta := middleware.BuildMeta(r)
	meta.Pagination = &response.Pagination{
		CurrentPage: page,
		PerPage:     perPage,
		TotalItems:  total,
		TotalPages:  pagination.TotalPages(total, perPage),
	}
	response.Success(w, http.StatusOK, reviews, meta)
}
