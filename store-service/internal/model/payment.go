package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Payment struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrderID   uuid.UUID       `gorm:"type:uuid;uniqueIndex;not null" json:"order_id"`
	Method    string          `gorm:"not null;default:mock" json:"method"`
	Status    string          `gorm:"not null;default:pending" json:"status"`
	Amount    decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"amount"`
	PaidAt    *time.Time      `json:"paid_at"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

const (
	PaymentStatusPending = "pending"
	PaymentStatusSuccess = "success"
	PaymentStatusFailed  = "failed"

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
