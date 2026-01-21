package service

import (
	"context"
	"errors"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestStoreService_CreateStore(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		req         model.CreateStoreRequest
		mockSetup   func(storeRepo *mocks.MockStoreRepository, userRepo *mocks.MockUserRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			req:  model.CreateStoreRequest{Name: "My Store", Description: "A test store"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository, userRepo *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(nil, errors.New("not found"))
				storeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				userRepo.EXPECT().UpdateRole(gomock.Any(), userID, constant.RoleSeller).Return(nil)
			},
		},
		{
			name: "user already has a store",
			req:  model.CreateStoreRequest{Name: "My Store"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{
					ID:     uuid.New(),
					UserID: userID,
				}, nil)
			},
			wantErr:     true,
			errContains: "user already has a store",
		},
		{
			name: "create fails",
			req:  model.CreateStoreRequest{Name: "My Store"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(nil, errors.New("not found"))
				storeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: "failed to create store",
		},
		{
			name: "update role fails",
			req:  model.CreateStoreRequest{Name: "My Store"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository, userRepo *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(nil, errors.New("not found"))
				storeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				userRepo.EXPECT().UpdateRole(gomock.Any(), userID, constant.RoleSeller).Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: "failed to create store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storeRepo := mocks.NewMockStoreRepository(ctrl)
			userRepo := mocks.NewMockUserRepository(ctrl)
			tt.mockSetup(storeRepo, userRepo)

			svc := NewStoreService(storeRepo, userRepo)
			resp, err := svc.CreateStore(context.Background(), userID, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.req.Name, resp.Name)
			assert.Equal(t, userID, resp.UserID)
		})
	}
}

func TestStoreService_GetStoreByID(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name        string
		mockSetup   func(storeRepo *mocks.MockStoreRepository, userRepo *mocks.MockUserRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: userID,
					Name:   "My Store",
				}, nil)
			},
		},
		{
			name: "store not found",
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "store not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storeRepo := mocks.NewMockStoreRepository(ctrl)
			userRepo := mocks.NewMockUserRepository(ctrl)
			tt.mockSetup(storeRepo, userRepo)

			svc := NewStoreService(storeRepo, userRepo)
			resp, err := svc.GetStoreByID(context.Background(), storeID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, storeID, resp.ID)
		})
	}
}

func TestStoreService_UpdateStore(t *testing.T) {
	storeID := uuid.New()
	ownerID := uuid.New()
	otherUserID := uuid.New()

	tests := []struct {
		name        string
		callerID    uuid.UUID
		req         model.UpdateStoreRequest
		mockSetup   func(storeRepo *mocks.MockStoreRepository, userRepo *mocks.MockUserRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:     "success",
			callerID: ownerID,
			req:      model.UpdateStoreRequest{Name: "Updated Store", Description: "Updated desc"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: ownerID,
					Name:   "My Store",
				}, nil)
				storeRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:     "store not found",
			callerID: ownerID,
			req:      model.UpdateStoreRequest{Name: "Updated"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "store not found",
		},
		{
			name:     "not store owner",
			callerID: otherUserID,
			req:      model.UpdateStoreRequest{Name: "Updated"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: ownerID,
				}, nil)
			},
			wantErr:     true,
			errContains: "forbidden",
		},
		{
			name:     "update fails",
			callerID: ownerID,
			req:      model.UpdateStoreRequest{Name: "Updated"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: ownerID,
				}, nil)
				storeRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: "failed to update store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storeRepo := mocks.NewMockStoreRepository(ctrl)
			userRepo := mocks.NewMockUserRepository(ctrl)
			tt.mockSetup(storeRepo, userRepo)

			svc := NewStoreService(storeRepo, userRepo)
			resp, err := svc.UpdateStore(context.Background(), tt.callerID, storeID, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.req.Name, resp.Name)
		})
	}
}

func TestStoreService_UpdateLogo(t *testing.T) {
	storeID := uuid.New()
	ownerID := uuid.New()
	otherUserID := uuid.New()
	logoURL := "https://example.com/logo.png"

	tests := []struct {
		name        string
		callerID    uuid.UUID
		mockSetup   func(storeRepo *mocks.MockStoreRepository, userRepo *mocks.MockUserRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:     "success",
			callerID: ownerID,
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: ownerID,
				}, nil)
				storeRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:     "store not found",
			callerID: ownerID,
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(nil, errors.New("not found"))
			},
			wantErr:     true,
			errContains: "store not found",
		},
		{
			name:     "not store owner",
			callerID: otherUserID,
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: ownerID,
				}, nil)
			},
			wantErr:     true,
			errContains: "forbidden",
		},
		{
			name:     "update fails",
			callerID: ownerID,
			mockSetup: func(storeRepo *mocks.MockStoreRepository, _ *mocks.MockUserRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: ownerID,
				}, nil)
				storeRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: "failed to update store logo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storeRepo := mocks.NewMockStoreRepository(ctrl)
			userRepo := mocks.NewMockUserRepository(ctrl)
			tt.mockSetup(storeRepo, userRepo)

			svc := NewStoreService(storeRepo, userRepo)
			resp, err := svc.UpdateLogo(context.Background(), tt.callerID, storeID, logoURL)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, logoURL, resp.LogoURL)
		})
	}
}