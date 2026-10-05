package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func jsonBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return string(b)
}

// The service is nil: reaching it would panic.
func TestHandlers_RejectInvalidInput(t *testing.T) {
	var (
		long            = strings.Repeat("a", constant.MaxVarcharLength+1)
		longDescription = strings.Repeat("a", constant.MaxDescriptionLength+1)
		longComment     = strings.Repeat("a", constant.MaxCommentLength+1)
		userID          = uuid.New()
		pathID          = uuid.NewString()
	)

	withID := func(r *http.Request) *http.Request {
		r.SetPathValue("id", pathID)
		return r
	}
	withProductID := func(r *http.Request) *http.Request {
		r.SetPathValue("product_id", pathID)
		return r
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		req     *http.Request
		field   string
		message string
	}{
		{
			name:    "register - email too long",
			handler: NewAuthHandler(nil).Register,
			req:     httptest.NewRequest(http.MethodPost, "/", strings.NewReader(jsonBody(t, map[string]string{"email": long + "@example.com", "password": "secret1", "name": "A"}))),
			field:   "email", message: "maximum 255 characters",
		},
		{
			name:    "register - name too long",
			handler: NewAuthHandler(nil).Register,
			req:     httptest.NewRequest(http.MethodPost, "/", strings.NewReader(jsonBody(t, map[string]string{"email": "a@example.com", "password": "secret1", "name": long}))),
			field:   "name", message: "maximum 255 characters",
		},
		{
			name:    "register - password longer than bcrypt accepts",
			handler: NewAuthHandler(nil).Register,
			req:     httptest.NewRequest(http.MethodPost, "/", strings.NewReader(jsonBody(t, map[string]string{"email": "a@example.com", "password": strings.Repeat("p", constant.MaxPasswordBytes+1), "name": "A"}))),
			field:   "password", message: "maximum 72 bytes",
		},
		{
			name:    "create store - name too long",
			handler: NewStoreHandler(nil, nil).CreateStore,
			req:     requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]string{"name": long})),
			field:   "name", message: "maximum 255 characters",
		},
		{
			name:    "update store - name too long",
			handler: NewStoreHandler(nil, nil).UpdateStore,
			req:     withID(requestAs(userID, http.MethodPut, "/", jsonBody(t, map[string]string{"name": long}))),
			field:   "name", message: "maximum 255 characters",
		},
		{
			name:    "create category - name too long",
			handler: NewCategoryHandler(nil).CreateCategory,
			req:     httptest.NewRequest(http.MethodPost, "/", strings.NewReader(jsonBody(t, map[string]string{"name": long}))),
			field:   "name", message: "maximum 255 characters",
		},
		{
			name:    "update category - name too long",
			handler: NewCategoryHandler(nil).UpdateCategory,
			req:     withID(httptest.NewRequest(http.MethodPut, "/", strings.NewReader(jsonBody(t, map[string]string{"name": long})))),
			field:   "name", message: "maximum 255 characters",
		},
		{
			name:    "create product - name too long",
			handler: NewProductHandler(nil, nil).CreateProduct,
			req:     requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]any{"name": long, "price": "10.00", "stock": 1, "category_id": uuid.NewString()})),
			field:   "name", message: "maximum 255 characters",
		},
		{
			name:    "update product - name too long",
			handler: NewProductHandler(nil, nil).UpdateProduct,
			req:     withID(requestAs(userID, http.MethodPut, "/", jsonBody(t, map[string]string{"name": long}))),
			field:   "name", message: "maximum 255 characters",
		},
		{
			name:    "create store - description too long",
			handler: NewStoreHandler(nil, nil).CreateStore,
			req:     requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]string{"name": "A", "description": longDescription})),
			field:   "description", message: "maximum 5000 characters",
		},
		{
			name:    "update store - description too long",
			handler: NewStoreHandler(nil, nil).UpdateStore,
			req:     withID(requestAs(userID, http.MethodPut, "/", jsonBody(t, map[string]string{"description": longDescription}))),
			field:   "description", message: "maximum 5000 characters",
		},
		{
			name:    "create product - description too long",
			handler: NewProductHandler(nil, nil).CreateProduct,
			req:     requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]any{"name": "A", "description": longDescription, "price": "10.00", "category_id": uuid.NewString()})),
			field:   "description", message: "maximum 5000 characters",
		},
		{
			name:    "update product - description too long",
			handler: NewProductHandler(nil, nil).UpdateProduct,
			req:     withID(requestAs(userID, http.MethodPut, "/", jsonBody(t, map[string]string{"description": longDescription}))),
			field:   "description", message: "maximum 5000 characters",
		},
		{
			name:    "create review - rating out of range",
			handler: NewReviewHandler(nil).CreateReview,
			req:     withID(requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]any{"rating": 6}))),
			field:   "rating", message: constant.MsgRatingRange,
		},
		{
			name:    "create review - comment too long",
			handler: NewReviewHandler(nil).CreateReview,
			req:     withID(requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]any{"rating": 5, "comment": longComment}))),
			field:   "comment", message: "maximum 2000 characters",
		},
		{
			name:    "add cart item - quantity not positive",
			handler: NewCartHandler(nil).AddItem,
			req:     requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]any{"product_id": uuid.NewString(), "quantity": 0})),
			field:   "quantity", message: constant.MsgGreaterThanZero,
		},
		{
			name:    "update cart item - quantity not positive",
			handler: NewCartHandler(nil).UpdateItem,
			req:     withProductID(requestAs(userID, http.MethodPut, "/", jsonBody(t, map[string]any{"quantity": 0}))),
			field:   "quantity", message: constant.MsgGreaterThanZero,
		},
		{
			name:    "create product - category_id is not a UUID",
			handler: NewProductHandler(nil, nil).CreateProduct,
			req:     requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]any{"name": "A", "price": "10.00", "category_id": "abc"})),
			field:   "category_id", message: constant.MsgInvalidUUID,
		},
		{
			name:    "update product - category_id is not a UUID",
			handler: NewProductHandler(nil, nil).UpdateProduct,
			req:     withID(requestAs(userID, http.MethodPut, "/", jsonBody(t, map[string]string{"category_id": "abc"}))),
			field:   "category_id", message: constant.MsgInvalidUUID,
		},
		{
			name:    "add cart item - product_id is not a UUID",
			handler: NewCartHandler(nil).AddItem,
			req:     requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]any{"product_id": "abc", "quantity": 1})),
			field:   "product_id", message: constant.MsgInvalidUUID,
		},
		{
			name:    "checkout - shipping address too long",
			handler: NewOrderHandler(nil, nil).Checkout,
			req:     requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]string{"shipping_address": strings.Repeat("a", constant.MaxShippingAddressLength+1)})),
			field:   "shipping_address", message: "maximum 500 characters",
		},
		{
			name:    "list products - category_id is not a UUID",
			handler: NewProductHandler(nil, nil).GetProducts,
			req:     httptest.NewRequest(http.MethodGet, "/api/v1/products?category_id=abc", nil),
			field:   "category_id", message: constant.MsgInvalidUUID,
		},
		{
			name:    "list products - store_id is not a UUID",
			handler: NewProductHandler(nil, nil).GetProducts,
			req:     httptest.NewRequest(http.MethodGet, "/api/v1/products?store_id=1%27%20or%201%3D1", nil),
			field:   "store_id", message: constant.MsgInvalidUUID,
		},
		{
			name:    "register - NUL character",
			handler: NewAuthHandler(nil).Register,
			req:     httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"a@example.com","password":"secret1","name":"A\u0000"}`)),
			field:   "", message: constant.MsgInvalidRequestBody,
		},
		{
			name:    "list products - search with a NUL character",
			handler: NewProductHandler(nil, nil).GetProducts,
			req:     httptest.NewRequest(http.MethodGet, "/api/v1/products?search=a%00", nil),
			field:   "search", message: constant.MsgInvalidChars,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler(rec, tt.req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			errBody := decodeError(t, rec)
			assert.Equal(t, constant.ErrCodeValidation, errBody.Code)
			assert.Equal(t, tt.field, errBody.Field)
			assert.Equal(t, tt.message, errBody.Message)
		})
	}
}

type stubCategoryService struct {
	service.CategoryService
}

func (stubCategoryService) CreateCategory(_ context.Context, req model.CategoryRequest) (*model.Category, error) {
	return &model.Category{Name: req.Name}, nil
}

func TestCreateCategory_CountsNameCharactersNotBytes(t *testing.T) {
	send := func(name string) int {
		var (
			rec = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodPost, "/api/v1/categories", strings.NewReader(jsonBody(t, model.CategoryRequest{Name: name})))
		)

		NewCategoryHandler(stubCategoryService{}).CreateCategory(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusCreated, send(strings.Repeat("é", constant.MaxVarcharLength)))
	assert.Equal(t, http.StatusBadRequest, send(strings.Repeat("é", constant.MaxVarcharLength+1)))
}

// The service is nil: reaching it would panic.
func TestRegister_CountsMinPasswordInCharactersNotBytes(t *testing.T) {
	var (
		password = strings.Repeat("é", constant.MinPasswordLength-1) // fewer characters, but enough bytes
		rec      = httptest.NewRecorder()
		req      = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(jsonBody(t, map[string]string{"email": "a@example.com", "password": password, "name": "A"})))
	)

	NewAuthHandler(nil).Register(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := decodeError(t, rec)
	assert.Equal(t, "password", errBody.Field)
	assert.Equal(t, "minimum 6 characters", errBody.Message)
}

type captureProductService struct {
	service.ProductService
	filter model.ProductFilter
}

func (c *captureProductService) GetProducts(_ context.Context, filter model.ProductFilter) ([]model.Product, int64, error) {
	c.filter = filter
	return nil, 0, nil
}

func TestGetProducts_PassesIDFiltersInCanonicalForm(t *testing.T) {
	var (
		categoryID = uuid.New()
		svc        = &captureProductService{}
		rec        = httptest.NewRecorder()
	)

	NewProductHandler(svc, nil).GetProducts(rec, httptest.NewRequest(http.MethodGet,
		"/api/v1/products?category_id=%7B"+strings.ToUpper(categoryID.String())+"%7D", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, categoryID.String(), svc.filter.CategoryID)
	assert.Empty(t, svc.filter.StoreID)
}
