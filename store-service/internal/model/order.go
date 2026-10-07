package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Order struct {
	ID              uuid.UUID       `db:"id" json:"id"`
	UserID          uuid.UUID       `db:"user_id" json:"user_id"`
	StoreID         uuid.UUID       `db:"store_id" json:"store_id"`
	Status          string          `db:"status" json:"status"`
	TotalAmount     decimal.Decimal `db:"total_amount" json:"total_amount"`
	ShippingAddress string          `db:"shipping_address" json:"shipping_address"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at" json:"updated_at"`

	// Stored in their own tables; the repository loads and inserts them.
	OrderItems []OrderItem `db:"-" json:"items,omitempty"`
	Payment    *Payment    `db:"-" json:"payment,omitempty"`
}

type OrderItem struct {
	ID        uuid.UUID       `db:"id" json:"id"`
	OrderID   uuid.UUID       `db:"order_id" json:"order_id"`
	ProductID uuid.UUID       `db:"product_id" json:"product_id"`
	Quantity  int             `db:"quantity" json:"quantity"`
	Price     decimal.Decimal `db:"price" json:"price"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
}

type UpdateOrderStatusRequest struct {
	Status string `json:"status"`
}

type CheckoutRequest struct {
	ShippingAddress string `json:"shipping_address"`
}

type OrderResponse struct {
	ID              uuid.UUID           `json:"id"`
	UserID          uuid.UUID           `json:"user_id"`
	StoreID         uuid.UUID           `json:"store_id"`
	Status          string              `json:"status"`
	TotalAmount     decimal.Decimal     `json:"total_amount"`
	ShippingAddress string              `json:"shipping_address"`
	Items           []OrderItemResponse `json:"items"`
	Payment         *PaymentResponse    `json:"payment,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

type OrderItemResponse struct {
	ID        uuid.UUID       `json:"id"`
	ProductID uuid.UUID       `json:"product_id"`
	Quantity  int             `json:"quantity"`
	Price     decimal.Decimal `json:"price"`
	Subtotal  decimal.Decimal `json:"subtotal"`
}

func (o *Order) ToResponse() OrderResponse {
	resp := OrderResponse{
		ID:              o.ID,
		UserID:          o.UserID,
		StoreID:         o.StoreID,
		Status:          o.Status,
		TotalAmount:     o.TotalAmount,
		ShippingAddress: o.ShippingAddress,
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
	}

	for _, item := range o.OrderItems {
		resp.Items = append(resp.Items, OrderItemResponse{
			ID:        item.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     item.Price,
			Subtotal:  item.Price.Mul(decimal.NewFromInt(int64(item.Quantity))),
		})
	}

	if o.Payment != nil {
		pr := o.Payment.ToResponse()
		resp.Payment = &pr
	}

	return resp
}
