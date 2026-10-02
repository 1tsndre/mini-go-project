package service

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
)

type PaymentResult struct {
	OrderID   string
	Success   bool
	PaymentID string
	Message   string
}

type PaymentRecord struct {
	OrderID   string
	PaymentID string
	Status    string
	Amount    string
	Method    string
}

type PaymentService struct {
	mu      sync.RWMutex
	records map[string]*PaymentRecord
}

func NewPaymentService() *PaymentService {
	return &PaymentService{records: make(map[string]*PaymentRecord)}
}

// ProcessPayment is idempotent per order: a redelivered or republished request
// for an order that was already processed returns the recorded outcome instead
// of charging again with a fresh random result.
func (s *PaymentService) ProcessPayment(ctx context.Context, orderID, amount, method string) *PaymentResult {
	if rec, ok := s.GetStatus(orderID); ok {
		logger.Info(ctx, "payment already processed, returning recorded result", map[string]any{"order_id": orderID})
		return rec.toResult()
	}

	time.Sleep(time.Duration(500+rand.Intn(1500)) * time.Millisecond)

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

	status := "failed"
	if success {
		status = "success"
		result.Message = "payment processed successfully"
		logger.Info(ctx, "payment processed successfully", map[string]any{"order_id": orderID, "amount": amount})
	} else {
		result.Message = "payment declined"
		logger.Warn(ctx, "payment declined", map[string]any{"order_id": orderID, "amount": amount})
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// A concurrent duplicate may have finished first; keep whichever was recorded first.
	if rec, ok := s.records[orderID]; ok {
		return rec.toResult()
	}
	s.records[orderID] = &PaymentRecord{
		OrderID:   orderID,
		PaymentID: result.PaymentID,
		Status:    status,
		Amount:    amount,
		Method:    method,
	}

	return result
}

func (r *PaymentRecord) toResult() *PaymentResult {
	result := &PaymentResult{
		OrderID:   r.OrderID,
		PaymentID: r.PaymentID,
		Success:   r.Status == "success",
		Message:   "payment declined",
	}
	if result.Success {
		result.Message = "payment processed successfully"
	}
	return result
}

func (s *PaymentService) GetStatus(orderID string) (*PaymentRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.records[orderID]
	return rec, ok
}
