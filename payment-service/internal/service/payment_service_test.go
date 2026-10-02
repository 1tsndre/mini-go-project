package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaymentService_ProcessPayment_IsIdempotent(t *testing.T) {
	svc := NewPaymentService()
	ctx := context.Background()

	first := svc.ProcessPayment(ctx, "order-1", "100.00", "mock")
	second := svc.ProcessPayment(ctx, "order-1", "100.00", "mock")

	assert.Equal(t, first.Success, second.Success)
	assert.Equal(t, first.PaymentID, second.PaymentID)
	assert.Equal(t, first.Message, second.Message)

	rec, ok := svc.GetStatus("order-1")
	assert.True(t, ok)
	assert.Equal(t, first.PaymentID, rec.PaymentID)
}

func TestPaymentService_ProcessPayment_ConcurrentDuplicatesAgree(t *testing.T) {
	svc := NewPaymentService()
	ctx := context.Background()

	const n = 5
	results := make([]*PaymentResult, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = svc.ProcessPayment(ctx, "order-2", "50.00", "mock")
		}(i)
	}
	wg.Wait()

	rec, ok := svc.GetStatus("order-2")
	assert.True(t, ok)
	for _, r := range results {
		assert.Equal(t, rec.Status == "success", r.Success, "every duplicate must report the recorded outcome")
	}
}

func TestPaymentService_GetStatus_Unknown(t *testing.T) {
	_, ok := NewPaymentService().GetStatus("missing")
	assert.False(t, ok)
}
