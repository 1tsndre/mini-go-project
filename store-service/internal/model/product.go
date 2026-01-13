package model

import (
	"errors"
	"strings"
	"time"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Product struct {
	ID          uuid.UUID       `db:"id" json:"id"`
	StoreID     uuid.UUID       `db:"store_id" json:"store_id"`
	CategoryID  uuid.UUID       `db:"category_id" json:"category_id"`
	Name        string          `db:"name" json:"name"`
	Description string          `db:"description" json:"description"`
	Price       decimal.Decimal `db:"price" json:"price"`
	Stock       int             `db:"stock" json:"stock"`
	ImageURL    string          `db:"image_url" json:"image_url"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updated_at"`
}

// An exponent such as "1e1000000000" would expand to gigabytes.
func ParsePrice(s string) (decimal.Decimal, error) {
	if len(s) > constant.MaxPriceLength || strings.ContainsAny(s, "eE") {
		return decimal.Decimal{}, errors.New(constant.MsgInvalidPrice)
	}
	return decimal.NewFromString(s)
}

type CreateProductRequest struct {
	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       string `json:"price"`
	Stock       int    `json:"stock"`
}

type UpdateProductRequest struct {
	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       string `json:"price"`
	Stock       *int   `json:"stock"`
}

type ProductFilter struct {
	CategoryID string
	StoreID    string
	Search     string
	MinPrice   string
	MaxPrice   string
	SortBy     string
	SortOrder  string
	Page       int
	PerPage    int
}
