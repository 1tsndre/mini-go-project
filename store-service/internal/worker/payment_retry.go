package worker

import (
	"context"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
)

// PaymentRetrier periodically republishes order.created for orders that are
// still pending, so an order whose original publish failed (or whose message
// was lost) still reaches the payment service.
type PaymentRetrier struct {
	orderService service.OrderService
	interval     time.Duration
	olderThan    time.Duration
	batchSize    int

	cancel context.CancelFunc
	done   chan struct{}
}

func NewPaymentRetrier(orderService service.OrderService, interval, olderThan time.Duration, batchSize int) *PaymentRetrier {
	return &PaymentRetrier{
		orderService: orderService,
		interval:     interval,
		olderThan:    olderThan,
		batchSize:    batchSize,
	}
}

func (p *PaymentRetrier) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})

	go func() {
		defer close(p.done)
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.runOnce(ctx)
			}
		}
	}()

	logger.Info(context.Background(), "payment retry worker started", map[string]interface{}{
		"interval":   p.interval.String(),
		"older_than": p.olderThan.String(),
	})
}

func (p *PaymentRetrier) runOnce(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, p.interval)
	defer cancel()

	count, err := p.orderService.RetryPendingPayments(ctx, p.olderThan, p.batchSize)
	if err != nil {
		logger.Error(ctx, "failed to retry pending payments", err)
		return
	}
	if count > 0 {
		logger.Info(ctx, "republished pending orders", map[string]interface{}{"count": count})
	}
}

func (p *PaymentRetrier) Stop() {
	if p.cancel == nil {
		return
	}
	p.cancel()
	<-p.done
	logger.Info(context.Background(), "payment retry worker stopped")
}
