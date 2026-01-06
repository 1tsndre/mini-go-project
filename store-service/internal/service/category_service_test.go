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

func TestCategoryService_CreateCategory(t *testing.T) {
	tests := []struct {
		name        string
		req         model.CreateCategoryRequest
		mockSetup   func(repo *mocks.MockCategoryRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			req:  model.CreateCategoryRequest{Name: "Electronics"},
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "create fails",
			req:  model.CreateCategoryRequest{Name: "Electronics"},
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("duplicate"))
			},
			wantErr:     true,
			errContains: "category already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockCategoryRepository(ctrl)
			tt.mockSetup(repo)

			svc := NewCategoryService(repo)
			resp, err := svc.CreateCategory(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, "Electronics", resp.Name)
		})
	}
}

func TestCategoryService_GetAllCategories(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(repo *mocks.MockCategoryRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name: "success with categories",
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().FindAll(gomock.Any()).Return([]model.Category{
					{ID: uuid.New(), Name: "Electronics"},
					{ID: uuid.New(), Name: "Clothing"},
				}, nil)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "success empty",
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().FindAll(gomock.Any()).Return([]model.Category{}, nil)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "db error",
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().FindAll(gomock.Any()).Return(nil, errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockCategoryRepository(ctrl)
			tt.mockSetup(repo)

			svc := NewCategoryService(repo)
			resp, err := svc.GetAllCategories(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, resp, tt.wantCount)
		})
	}
}

func TestCategoryService_DeleteCategory(t *testing.T) {
	catID := uuid.New()

	tests := []struct {
		name        string
		id          uuid.UUID
		mockSetup   func(repo *mocks.MockCategoryRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			id:   catID,
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().FindByID(gomock.Any(), catID).Return(&model.Category{ID: catID, Name: "Electronics"}, nil)
				repo.EXPECT().Delete(gomock.Any(), catID).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "not found",
			id:   catID,
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().FindByID(gomock.Any(), catID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "category not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := mocks.NewMockCategoryRepository(ctrl)
			tt.mockSetup(repo)

			svc := NewCategoryService(repo)
			err := svc.DeleteCategory(context.Background(), tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
		})
	}
}
