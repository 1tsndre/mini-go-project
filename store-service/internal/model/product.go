package model

import (
	"time"

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

type ProductResponse struct {
	ID          uuid.UUID       `json:"id"`
	StoreID     uuid.UUID       `json:"store_id"`
	CategoryID  uuid.UUID       `json:"category_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Price       decimal.Decimal `json:"price"`
	Stock       int             `json:"stock"`
	ImageURL    string          `json:"image_url"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (p *Product) ToResponse() ProductResponse {
	return ProductResponse{
		ID:          p.ID,
		StoreID:     p.StoreID,
		CategoryID:  p.CategoryID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Stock:       p.Stock,
		ImageURL:    p.ImageURL,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}
