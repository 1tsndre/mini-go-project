package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Payment struct {
	ID        uuid.UUID       `db:"id" json:"id"`
	OrderID   uuid.UUID       `db:"order_id" json:"order_id"`
	Method    string          `db:"method" json:"method"`
	Status    string          `db:"status" json:"status"`
	Amount    decimal.Decimal `db:"amount" json:"amount"`
	PaidAt    *time.Time      `db:"paid_at" json:"paid_at"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

const (
	PaymentStatusPending = "pending"
	PaymentStatusSuccess = "success"
	PaymentStatusFailed  = "failed"
	// PaymentStatusCancelled marks a payment that was still pending when the buyer cancelled the order.
	PaymentStatusCancelled = "cancelled"

	PaymentMethodMock = "mock"
)

type PaymentResponse struct {
	ID        uuid.UUID       `json:"id"`
	OrderID   uuid.UUID       `json:"order_id"`
	Method    string          `json:"method"`
	Status    string          `json:"status"`
	Amount    decimal.Decimal `json:"amount"`
	PaidAt    *time.Time      `json:"paid_at"`
	CreatedAt time.Time       `json:"created_at"`
}

type PaymentStatusResponse struct {
	OrderID   string `json:"order_id"`
	PaymentID string `json:"payment_id"`
	Status    string `json:"status"`
	Amount    string `json:"amount"`
	Method    string `json:"method"`
}

func (p *Payment) ToResponse() PaymentResponse {
	return PaymentResponse{
		ID:        p.ID,
		OrderID:   p.OrderID,
		Method:    p.Method,
		Status:    p.Status,
		Amount:    p.Amount,
		PaidAt:    p.PaidAt,
		CreatedAt: p.CreatedAt,
	}
}
