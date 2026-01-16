package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Product struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	StoreID     uuid.UUID       `gorm:"type:uuid;not null;index" json:"store_id"`
	CategoryID  uuid.UUID       `gorm:"type:uuid;not null;index" json:"category_id"`
	Name        string          `gorm:"not null" json:"name"`
	Description string          `json:"description"`
	Price       decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"price"`
	Stock       int             `gorm:"not null;default:0" json:"stock"`
	ImageURL    string          `json:"image_url"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`

	Store    Store    `gorm:"foreignKey:StoreID" json:"-"`
	Category Category `gorm:"foreignKey:CategoryID" json:"-"`
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
