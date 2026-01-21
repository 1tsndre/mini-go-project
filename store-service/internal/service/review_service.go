package service

import (
	"context"
	"errors"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
)

type ReviewService interface {
	CreateReview(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.CreateReviewRequest) (*model.ReviewResponse, error)
	GetProductReviews(ctx context.Context, productID uuid.UUID, page, perPage int) ([]model.ReviewResponse, int64, error)
}

type reviewService struct {
	reviewRepo repository.ReviewRepository
}

func NewReviewService(reviewRepo repository.ReviewRepository) ReviewService {
	return &reviewService{reviewRepo: reviewRepo}
}

func (s *reviewService) CreateReview(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.CreateReviewRequest) (*model.ReviewResponse, error) {
	purchased, err := s.reviewRepo.HasUserPurchased(ctx, userID, productID)
	if err != nil {
		logger.Error(ctx, constant.LogReviewPurchaseCheckFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgReviewPurchaseCheckFailed}
	}
	if !purchased {
		return nil, &Error{Code: constant.ErrCodeForbidden, Message: constant.MsgReviewNotPurchased}
	}

	review := &model.Review{
		UserID:    userID,
		ProductID: productID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	}

	if err := s.reviewRepo.Create(ctx, review); err != nil {
		if errors.Is(err, repository.ErrDuplicateKey) {
			return nil, &Error{Code: constant.ErrCodeConflict, Message: constant.MsgReviewExists}
		}
		logger.Error(ctx, constant.LogReviewCreateFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgReviewCreateFailed}
	}

	logger.Info(ctx, constant.LogReviewCreated, logger.Fields{
		"review_id":  review.ID.String(),
		"product_id": productID.String(),
	})

	resp := review.ToResponse()
	return &resp, nil
}

func (s *reviewService) GetProductReviews(ctx context.Context, productID uuid.UUID, page, perPage int) ([]model.ReviewResponse, int64, error) {
	reviews, total, err := s.reviewRepo.FindByProductID(ctx, productID, page, perPage)
	if err != nil {
		logger.Error(ctx, constant.LogReviewFetchFailed, err)
		return nil, 0, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgReviewFetchFailed}
	}

	responses := make([]model.ReviewResponse, 0, len(reviews))
	for _, review := range reviews {
		responses = append(responses, review.ToResponse())
	}

	return responses, total, nil
}
