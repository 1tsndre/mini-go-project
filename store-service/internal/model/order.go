package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Order struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID          uuid.UUID       `gorm:"type:uuid;not null;index" json:"user_id"`
	Status          string          `gorm:"not null;default:pending" json:"status"`
	TotalAmount     decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"total_amount"`
	ShippingAddress string          `gorm:"not null;default:''" json:"shipping_address"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`

	User       User        `gorm:"foreignKey:UserID" json:"-"`
	OrderItems []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
	Payment    *Payment    `gorm:"foreignKey:OrderID" json:"payment,omitempty"`
}

type OrderItem struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrderID   uuid.UUID       `gorm:"type:uuid;not null;index" json:"order_id"`
	ProductID uuid.UUID       `gorm:"type:uuid;not null" json:"product_id"`
	Quantity  int             `gorm:"not null" json:"quantity"`
	Price     decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"price"`
	CreatedAt time.Time       `json:"created_at"`

	Product Product `gorm:"foreignKey:ProductID" json:"-"`
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
