package service

import (
	"context"
	"errors"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/pagination"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
)

type ReviewService interface {
	CreateReview(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.CreateReviewRequest) (*model.ReviewResponse, error)
	GetProductReviews(ctx context.Context, productID uuid.UUID, page, perPage int) ([]model.ReviewResponse, int64, error)
}

type reviewService struct {
	repo repository.ReviewRepository
}

func NewReviewService(repo repository.ReviewRepository) ReviewService {
	return &reviewService{repo: repo}
}

func (s *reviewService) CreateReview(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.CreateReviewRequest) (*model.ReviewResponse, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}

	purchased, err := s.repo.HasUserPurchased(ctx, userID, productID)
	if err != nil {
		return nil, errors.New("failed to verify purchase")
	}
	if !purchased {
		return nil, errors.New("you must purchase this product before reviewing")
	}

	reviewed, err := s.repo.HasUserReviewed(ctx, userID, productID)
	if err != nil {
		return nil, errors.New("failed to check existing review")
	}
	if reviewed {
		return nil, errors.New("you have already reviewed this product")
	}

	review := &model.Review{
		UserID:    userID,
		ProductID: productID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	}

	if err := s.repo.Create(ctx, review); err != nil {
		logger.Error(ctx, "failed to create review", err)
		return nil, errors.New("failed to create review")
	}

	resp := review.ToResponse()
	return &resp, nil
}

func (s *reviewService) GetProductReviews(ctx context.Context, productID uuid.UUID, page, perPage int) ([]model.ReviewResponse, int64, error) {
	page, perPage = pagination.Normalize(page, perPage)

	reviews, total, err := s.repo.FindByProductID(ctx, productID, page, perPage)
	if err != nil {
		return nil, 0, err
	}

	var responses []model.ReviewResponse
	for _, r := range reviews {
		resp := r.ToResponse()
		resp.UserName = r.User.Name
		responses = append(responses, resp)
	}

	return responses, total, nil
}
