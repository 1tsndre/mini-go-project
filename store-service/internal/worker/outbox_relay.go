package worker

import (
	"context"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
)

// OutboxRelay publishes the messages queued in the outbox to NSQ. A message is
// only deleted once nsqd has accepted it, so a crash or an unreachable nsqd
// delays a message instead of losing it; one may be published twice, which the
// consumers tolerate.
type OutboxRelay struct {
	outboxRepo repository.OutboxRepository
	publisher  service.MessagePublisher
	interval   time.Duration
	batchSize  int

	cancel context.CancelFunc
	done   chan struct{}
}

func NewOutboxRelay(outboxRepo repository.OutboxRepository, publisher service.MessagePublisher, interval time.Duration, batchSize int) *OutboxRelay {
	return &OutboxRelay{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		interval:   interval,
		batchSize:  batchSize,
	}
}

func (o *OutboxRelay) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = cancel
	o.done = make(chan struct{})

	go func() {
		defer close(o.done)
		ticker := time.NewTicker(o.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				o.RunOnce(ctx)
			}
		}
	}()

	logger.Info(context.Background(), "outbox relay started", map[string]interface{}{
		"interval": o.interval.String(),
	})
}

// RunOnce publishes one batch of queued messages.
func (o *OutboxRelay) RunOnce(ctx context.Context) {
	_, err := o.outboxRepo.PublishPending(ctx, o.batchSize, func(message model.OutboxMessage) error {
		return o.publisher.Publish(message.Topic, []byte(message.Payload))
	})
	if err != nil {
		logger.Error(ctx, "failed to relay outbox messages", err)
	}
}

// Stop waits for a run in progress, so its messages are not published after NSQ closes.
func (o *OutboxRelay) Stop() {
	if o.cancel == nil {
		return
	}
	o.cancel()
	<-o.done
	logger.Info(context.Background(), "outbox relay stopped")
}
