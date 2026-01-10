package service

import (
	"context"
	"errors"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCategoryService_CreateCategory(t *testing.T) {
	tests := []struct {
		name        string
		req         model.CategoryRequest
		mockSetup   func(repo *mocks.MockCategoryRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			req:  model.CategoryRequest{Name: "Electronics"},
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "duplicate name",
			req:  model.CategoryRequest{Name: "Electronics"},
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrDuplicateKey)
			},
			wantErr:     true,
			errContains: constant.MsgCategoryExists,
		},
		{
			name: "create fails",
			req:  model.CategoryRequest{Name: "Electronics"},
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("connection reset"))
			},
			wantErr:     true,
			errContains: constant.MsgCategoryCreateFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctrl = gomock.NewController(t)
				repo = mocks.NewMockCategoryRepository(ctrl)
				svc  = NewCategoryService(repo)
			)

			defer ctrl.Finish()

			tt.mockSetup(repo)

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

func TestCategoryService_UpdateCategory(t *testing.T) {
	id := uuid.New()

	tests := []struct {
		name        string
		mockSetup   func(repo *mocks.MockCategoryRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Update(gomock.Any(), &model.Category{ID: id, Name: "Gadgets"}).Return(nil)
			},
		},
		{
			name: "category not found",
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgCategoryNotFound,
		},
		{
			name: "name taken",
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(repository.ErrDuplicateKey)
			},
			wantErr:     true,
			errContains: constant.MsgCategoryExists,
		},
		{
			name: "update fails",
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("connection reset"))
			},
			wantErr:     true,
			errContains: constant.MsgCategoryUpdateFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctrl = gomock.NewController(t)
				repo = mocks.NewMockCategoryRepository(ctrl)
				svc  = NewCategoryService(repo)
			)

			defer ctrl.Finish()

			tt.mockSetup(repo)

			resp, err := svc.UpdateCategory(context.Background(), id, model.CategoryRequest{Name: "Gadgets"})

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, "Gadgets", resp.Name)
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
			var (
				ctrl = gomock.NewController(t)
				repo = mocks.NewMockCategoryRepository(ctrl)
				svc  = NewCategoryService(repo)
			)

			defer ctrl.Finish()

			tt.mockSetup(repo)

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
				repo.EXPECT().Delete(gomock.Any(), catID).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "not found",
			id:   catID,
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Delete(gomock.Any(), catID).Return(repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgCategoryNotFound,
		},
		{
			name: "still referenced by products",
			id:   catID,
			mockSetup: func(repo *mocks.MockCategoryRepository) {
				repo.EXPECT().Delete(gomock.Any(), catID).Return(repository.ErrForeignKeyViolation)
			},
			wantErr:     true,
			errContains: constant.MsgCategoryInUse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctrl = gomock.NewController(t)
				repo = mocks.NewMockCategoryRepository(ctrl)
				svc  = NewCategoryService(repo)
			)

			defer ctrl.Finish()

			tt.mockSetup(repo)

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
