package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/mocks"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type recordingPublisher struct {
	published []string
	err       error
}

func (p *recordingPublisher) Publish(topic string, body []byte) error {
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, topic+" "+string(body))
	return nil
}

func TestOutboxRelay_RunOnce(t *testing.T) {
	message := model.OutboxMessage{ID: 7, Topic: "order.created", Payload: `{"order_id":"o-1"}`}

	tests := []struct {
		name          string
		publishErr    error
		wantPublished []string
	}{
		{name: "publishes the queued message", wantPublished: []string{`order.created {"order_id":"o-1"}`}},
		{name: "reports a rejected publish to the outbox", publishErr: errors.New("nsqd down")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			outboxRepo := mocks.NewMockOutboxRepository(ctrl)
			publisher := &recordingPublisher{err: tt.publishErr}

			outboxRepo.EXPECT().PublishPending(gomock.Any(), 100, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ int, publish func(model.OutboxMessage) error) (int, error) {
					err := publish(message)
					assert.Equal(t, tt.publishErr, err)
					if err != nil {
						return 0, err
					}
					return 1, nil
				})

			NewOutboxRelay(outboxRepo, publisher, 0, 100).RunOnce(context.Background())

			assert.Equal(t, tt.wantPublished, publisher.published)
		})
	}
}
