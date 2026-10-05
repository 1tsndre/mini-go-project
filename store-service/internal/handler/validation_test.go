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

// Inputs that PostgreSQL or bcrypt would reject must fail validation with a 400
// before reaching the service (nil here: calling it would panic the test).
func TestHandlers_RejectInputTheDatabaseCannotStore(t *testing.T) {
	long := strings.Repeat("a", constant.MaxVarcharLength+1)
	userID := uuid.New()
	pathID := uuid.New().String()

	withID := func(r *http.Request) *http.Request {
		r.SetPathValue("id", pathID)
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
			req:     requestAs(userID, http.MethodPost, "/", jsonBody(t, map[string]any{"name": long, "price": "10.00", "stock": 1, "category_id": uuid.New().String()})),
			field:   "name", message: "maximum 255 characters",
		},
		{
			name:    "update product - name too long",
			handler: NewProductHandler(nil, nil).UpdateProduct,
			req:     withID(requestAs(userID, http.MethodPut, "/", jsonBody(t, map[string]string{"name": long}))),
			field:   "name", message: "maximum 255 characters",
		},
		{
			name:    "list products - category_id is not a UUID",
			handler: NewProductHandler(nil, nil).GetProducts,
			req:     httptest.NewRequest(http.MethodGet, "/api/v1/products?category_id=abc", nil),
			field:   "category_id", message: "must be a valid UUID",
		},
		{
			name:    "list products - store_id is not a UUID",
			handler: NewProductHandler(nil, nil).GetProducts,
			req:     httptest.NewRequest(http.MethodGet, "/api/v1/products?store_id=1%27%20or%201%3D1", nil),
			field:   "store_id", message: "must be a valid UUID",
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

func TestExceedsVarchar_CountsCharactersNotBytes(t *testing.T) {
	// "é" is two bytes; PostgreSQL's VARCHAR(255) limit is in characters.
	assert.False(t, exceedsVarchar(strings.Repeat("é", constant.MaxVarcharLength)))
	assert.True(t, exceedsVarchar(strings.Repeat("é", constant.MaxVarcharLength+1)))
}

type captureProductService struct {
	service.ProductService
	filter model.ProductFilter
}

func (c *captureProductService) GetProducts(_ context.Context, filter model.ProductFilter) ([]model.ProductResponse, int64, error) {
	c.filter = filter
	return nil, 0, nil
}

func TestGetProducts_PassesIDFiltersInCanonicalForm(t *testing.T) {
	categoryID := uuid.New()
	svc := &captureProductService{}

	rec := httptest.NewRecorder()
	// Upper case and braces parse as a UUID; pass the canonical form to the query.
	NewProductHandler(svc, nil).GetProducts(rec, httptest.NewRequest(http.MethodGet,
		"/api/v1/products?category_id=%7B"+strings.ToUpper(categoryID.String())+"%7D", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, categoryID.String(), svc.filter.CategoryID)
	assert.Empty(t, svc.filter.StoreID)
}
