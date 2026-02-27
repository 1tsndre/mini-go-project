package service

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPaymentService_ProcessPayment_IsIdempotent(t *testing.T) {
	var (
		svc    = NewPaymentService()
		ctx    = context.Background()
		first  = svc.ProcessPayment(ctx, "order-1", "100.00", "mock")
		second = svc.ProcessPayment(ctx, "order-1", "100.00", "mock")
	)

	assert.Equal(t, first, second)

	rec, ok := svc.GetPayment("order-1")
	assert.True(t, ok)
	assert.Equal(t, first.PaymentID, rec.PaymentID)
}

func TestPaymentService_ProcessPayment_ConcurrentDuplicatesAgree(t *testing.T) {
	var (
		svc = NewPaymentService()
		ctx = context.Background()
		wg  sync.WaitGroup
	)

	const n = 5
	results := make([]Payment, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = svc.ProcessPayment(ctx, "order-2", "50.00", "mock")
		}(i)
	}
	wg.Wait()

	rec, ok := svc.GetPayment("order-2")
	assert.True(t, ok)
	for _, r := range results {
		assert.Equal(t, rec, r, "every duplicate must report the recorded outcome")
	}
}

func TestIsApproved_DependsOnlyOnTheOrder(t *testing.T) {
	const n = 10000
	approved := 0
	for i := 0; i < n; i++ {
		id := uuid.NewString()
		if isApproved(id) {
			approved++
		}
		assert.Equal(t, isApproved(id), isApproved(id))
	}
	assert.InDelta(t, 0.9, float64(approved)/n, 0.02)
}

func TestPaymentService_GetPayment_Unknown(t *testing.T) {
	_, ok := NewPaymentService().GetPayment("missing")
	assert.False(t, ok)
}
