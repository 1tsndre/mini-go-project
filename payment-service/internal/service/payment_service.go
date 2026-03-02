package service

import (
	"context"
	"math/rand"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
)

type PaymentResult struct {
	OrderID   string
	Success   bool
	PaymentID string
	Message   string
}

type PaymentService struct{}

func NewPaymentService() *PaymentService {
	return &PaymentService{}
}

func (s *PaymentService) ProcessPayment(ctx context.Context, orderID, amount, method string) *PaymentResult {
	time.Sleep(time.Duration(500+rand.Intn(1500)) * time.Millisecond)

	// Mock: 90% success rate
	success := rand.Float32() < 0.9

	suffix := orderID
	if len(orderID) > 8 {
		suffix = orderID[:8]
	}
	result := &PaymentResult{
		OrderID:   orderID,
		Success:   success,
		PaymentID: "pay_" + suffix,
	}

	if success {
		result.Message = "payment processed successfully"
		logger.Info(ctx, "payment processed successfully", map[string]any{"order_id": orderID, "amount": amount})
	} else {
		result.Message = "payment declined"
		logger.Warn(ctx, "payment declined", map[string]any{"order_id": orderID, "amount": amount})
	}

	return result
}
