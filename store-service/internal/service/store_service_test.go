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

func TestStoreService_CreateStore(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		req         model.StoreRequest
		mockSetup   func(storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			req:  model.StoreRequest{Name: "My Store", Description: "A test store"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "create fails",
			req:  model.StoreRequest{Name: "My Store"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: constant.MsgStoreCreateFailed,
		},
		{
			name: "user already has a store",
			req:  model.StoreRequest{Name: "My Store"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrDuplicateKey)
			},
			wantErr:     true,
			errContains: constant.MsgStoreExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(storeRepo)

			svc := NewStoreService(storeRepo)
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
	var (
		storeID = uuid.New()
		userID  = uuid.New()
	)

	tests := []struct {
		name        string
		mockSetup   func(storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "success",
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: userID,
					Name:   "My Store",
				}, nil)
			},
		},
		{
			name: "store not found",
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgStoreNotFound,
		},
		{
			name: "database fails",
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(nil, errors.New("connection refused"))
			},
			wantErr:     true,
			errContains: constant.MsgInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(storeRepo)

			svc := NewStoreService(storeRepo)
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

func TestStoreService_GetStoreByUserID(t *testing.T) {
	var (
		storeID = uuid.New()
		userID  = uuid.New()
	)

	tests := []struct {
		name        string
		mockSetup   func(storeRepo *mocks.MockStoreRepository)
		errContains string
	}{
		{
			name: "success",
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(&model.Store{
					ID:     storeID,
					UserID: userID,
					Name:   "My Store",
				}, nil)
			},
		},
		{
			name: "the user has no store",
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByUserID(gomock.Any(), userID).Return(nil, repository.ErrNotFound)
			},
			errContains: constant.MsgStoreNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(storeRepo)

			resp, err := NewStoreService(storeRepo).GetStoreByUserID(context.Background(), userID)

			if tt.errContains != "" {
				assert.ErrorContains(t, err, tt.errContains)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, storeID, resp.ID)
		})
	}
}

func TestStoreService_UpdateStore(t *testing.T) {
	var (
		storeID     = uuid.New()
		ownerID     = uuid.New()
		otherUserID = uuid.New()
	)

	tests := []struct {
		name        string
		callerID    uuid.UUID
		req         model.StoreRequest
		mockSetup   func(storeRepo *mocks.MockStoreRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:     "success",
			callerID: ownerID,
			req:      model.StoreRequest{Name: "Updated Store", Description: "Updated desc"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
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
			req:      model.StoreRequest{Name: "Updated"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(nil, repository.ErrNotFound)
			},
			wantErr:     true,
			errContains: constant.MsgStoreNotFound,
		},
		{
			name:     "not store owner",
			callerID: otherUserID,
			req:      model.StoreRequest{Name: "Updated"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: ownerID,
				}, nil)
			},
			wantErr:     true,
			errContains: constant.MsgNotStoreOwner,
		},
		{
			name:     "update fails",
			callerID: ownerID,
			req:      model.StoreRequest{Name: "Updated"},
			mockSetup: func(storeRepo *mocks.MockStoreRepository) {
				storeRepo.EXPECT().FindByID(gomock.Any(), storeID).Return(&model.Store{
					ID:     storeID,
					UserID: ownerID,
				}, nil)
				storeRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: constant.MsgStoreUpdateFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storeRepo := mocks.NewMockStoreRepository(ctrl)
			tt.mockSetup(storeRepo)

			svc := NewStoreService(storeRepo)
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
