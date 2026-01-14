package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Cart struct {
	UserID    uuid.UUID  `json:"user_id"`
	Items     []CartItem `json:"items"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CartItem struct {
	ProductID uuid.UUID       `db:"product_id" json:"product_id"`
	Name      string          `db:"name" json:"name"`
	Price     decimal.Decimal `db:"price" json:"price"`
	Quantity  int             `db:"quantity" json:"quantity"`
	ImageURL  string          `db:"image_url" json:"image_url"`
}

type AddCartItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type UpdateCartItemRequest struct {
	Quantity int `json:"quantity"`
}

type CartResponse struct {
	Items     []CartItemResponse `json:"items"`
	Total     decimal.Decimal    `json:"total"`
	UpdatedAt time.Time          `json:"updated_at,omitzero"`
}

type CartItemResponse struct {
	ProductID uuid.UUID       `json:"product_id"`
	Name      string          `json:"name"`
	Price     decimal.Decimal `json:"price"`
	Quantity  int             `json:"quantity"`
	Subtotal  decimal.Decimal `json:"subtotal"`
	ImageURL  string          `json:"image_url"`
}
