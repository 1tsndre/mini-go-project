package service

import (
	"context"
	"hash/fnv"
	"math/rand"
	"sync"
	"time"

	"github.com/1tsndre/mini-go-project/payment-service/internal/constant"
	"github.com/1tsndre/mini-go-project/pkg/logger"
)

type Payment struct {
	OrderID   string
	PaymentID string
	Status    string
	Amount    string
	Method    string
}

func (p Payment) Success() bool {
	return p.Status == constant.StatusSuccess
}

type PaymentService struct {
	mu       sync.RWMutex
	payments map[string]Payment
}

func NewPaymentService() *PaymentService {
	return &PaymentService{payments: make(map[string]Payment)}
}

func (s *PaymentService) ProcessPayment(ctx context.Context, orderID, amount, method string) Payment {
	if payment, ok := s.GetPayment(orderID); ok {
		return payment
	}

	time.Sleep(time.Duration(constant.MinProcessingDelayMs+rand.Intn(constant.ExtraProcessingDelayMs)) * time.Millisecond)

	payment := Payment{
		OrderID:   orderID,
		PaymentID: constant.PaymentIDPrefix + orderID[:min(len(orderID), constant.PaymentIDSuffixLength)],
		Status:    constant.StatusFailed,
		Amount:    amount,
		Method:    method,
	}

	if isApproved(orderID) {
		payment.Status = constant.StatusSuccess
	}

	logger.Info(ctx, constant.LogPaymentProcessed, logger.Fields{"order_id": orderID, "amount": amount, "status": payment.Status})

	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[orderID] = payment
	return payment
}

func (s *PaymentService) GetPayment(orderID string) (Payment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	payment, ok := s.payments[orderID]
	return payment, ok
}

// Derived from the order ID, so a redelivered order gets the same result.
func isApproved(orderID string) bool {
	hash := fnv.New32a()
	hash.Write([]byte(orderID))
	return hash.Sum32()%constant.ApprovalOutOf < constant.ApprovedShare
}
