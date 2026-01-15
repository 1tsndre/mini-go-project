package service

import (
	"log"
	"math/rand"
	"time"
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

func (s *PaymentService) ProcessPayment(orderID, amount, method string) *PaymentResult {
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
		log.Printf("payment success for order %s, amount: %s", orderID, amount)
	} else {
		result.Message = "payment declined"
		log.Printf("payment failed for order %s, amount: %s", orderID, amount)
	}

	return result
}
