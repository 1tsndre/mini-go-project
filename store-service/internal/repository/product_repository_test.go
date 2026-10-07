package repository

import (
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestProductFilterQuery(t *testing.T) {
	tests := []struct {
		name      string
		filter    model.ProductFilter
		wantWhere string
		wantArgs  []any
	}{
		{
			name:   "no filters",
			filter: model.ProductFilter{Page: 1, PerPage: 10},
		},
		{
			name: "all filters, with the search placeholder used twice",
			filter: model.ProductFilter{
				CategoryID: "cat", StoreID: "store", Search: "mug", MinPrice: "10", MaxPrice: "20.5",
			},
			wantWhere: " WHERE category_id = $1 AND store_id = $2 AND (name ILIKE $3 OR description ILIKE $3)" +
				" AND price >= $4 AND price <= $5",
			wantArgs: []any{"cat", "store", "%mug%", decimal.RequireFromString("10"), decimal.RequireFromString("20.5")},
		},
		{
			name:      "unparsable prices are ignored",
			filter:    model.ProductFilter{MinPrice: "cheap", MaxPrice: "20"},
			wantWhere: " WHERE price <= $1",
			wantArgs:  []any{decimal.RequireFromString("20")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			where, args := productFilterQuery(tt.filter)
			assert.Equal(t, tt.wantWhere, where)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestProductOrderBy(t *testing.T) {
	tests := []struct {
		sortBy, sortOrder string
		want              string
	}{
		{sortBy: "", sortOrder: "", want: " ORDER BY created_at DESC, id DESC"},
		{sortBy: "price", sortOrder: "asc", want: " ORDER BY price ASC, id ASC"},
		{sortBy: "name", sortOrder: "desc", want: " ORDER BY name DESC, id DESC"},
		// Anything outside the whitelist falls back to the default column.
		{sortBy: "price; DROP TABLE products", sortOrder: "asc", want: " ORDER BY created_at ASC, id ASC"},
	}

	for _, tt := range tests {
		t.Run(tt.sortBy+"/"+tt.sortOrder, func(t *testing.T) {
			assert.Equal(t, tt.want, productOrderBy(model.ProductFilter{SortBy: tt.sortBy, SortOrder: tt.sortOrder}))
		})
	}
}
