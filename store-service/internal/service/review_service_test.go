package service

import (
	"context"
	"errors"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestReviewService_CreateReview(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()

	tests := []struct {
		name        string
		userID      uuid.UUID
		productID   uuid.UUID
		req         model.CreateReviewRequest
		mockSetup   func(repo *mocks.MockReviewRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:      "success",
			userID:    userID,
			productID: productID,
			req:       model.CreateReviewRequest{Rating: 5, Comment: "Great product"},
			mockSetup: func(repo *mocks.MockReviewRepository) {
				repo.EXPECT().HasUserPurchased(gomock.Any(), userID, productID).Return(true, nil)
				repo.EXPECT().HasUserReviewed(gomock.Any(), userID, productID).Return(false, nil)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "rating out of range - too low",
			userID:      userID,
			productID:   productID,
			req:         model.CreateReviewRequest{Rating: 0, Comment: "Bad"},
			mockSetup:   func(repo *mocks.MockReviewRepository) {},
			wantErr:     true,
			errContains: "rating must be between 1 and 5",
		},
		{
			name:        "rating out of range - too high",
			userID:      userID,
			productID:   productID,
			req:         model.CreateReviewRequest{Rating: 6, Comment: "Amazing"},
			mockSetup:   func(repo *mocks.MockReviewRepository) {},
			wantErr:     true,
			errContains: "rating must be between 1 and 5",
		},
		{
			name:      "user has not purchased product",
			userID:    userID,
			productID: productID,
			req:       model.CreateReviewRequest{Rating: 4, Comment: "Good"},
			mockSetup: func(repo *mocks.MockReviewRepository) {
				repo.EXPECT().HasUserPurchased(gomock.Any(), userID, productID).Return(false, nil)
			},
			wantErr:     true,
			errContains: "you must purchase this product",
		},
		{
			name:      "user already reviewed",
			userID:    userID,
			productID: productID,
			req:       model.CreateReviewRequest{Rating: 3, Comment: "Okay"},
			mockSetup: func(repo *mocks.MockReviewRepository) {
				repo.EXPECT().HasUserPurchased(gomock.Any(), userID, productID).Return(true, nil)
				repo.EXPECT().HasUserReviewed(gomock.Any(), userID, productID).Return(true, nil)
			},
			wantErr:     true,
			errContains: "you have already reviewed",
		},
		{
			name:      "create fails",
			userID:    userID,
			productID: productID,
			req:       model.CreateReviewRequest{Rating: 5, Comment: "Great"},
			mockSetup: func(repo *mocks.MockReviewRepository) {
				repo.EXPECT().HasUserPurchased(gomock.Any(), userID, productID).Return(true, nil)
				repo.EXPECT().HasUserReviewed(gomock.Any(), userID, productID).Return(false, nil)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: "failed to create review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockReviewRepository(ctrl)
			tt.mockSetup(repo)

			svc := NewReviewService(repo)
			resp, err := svc.CreateReview(context.Background(), tt.userID, tt.productID, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.req.Rating, resp.Rating)
		})
	}
}
