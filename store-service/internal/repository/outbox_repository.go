package repository

import (
	"context"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/jmoiron/sqlx"
)

const outboxColumns = "id, topic, payload, created_at"

// OutboxRepository holds the messages written together with the changes they
// announce, until the relay publishes them.
type OutboxRepository interface {
	// PublishPending hands up to limit queued messages, oldest first, to publish and
	// deletes each one publish accepts. It stops at the first one publish rejects,
	// returning how many were published and that error. Messages another relay is
	// publishing are skipped, so several store-service instances can relay at once.
	PublishPending(ctx context.Context, limit int, publish func(model.OutboxMessage) error) (int, error)
}

type outboxRepository struct {
	db databases.Database
}

func NewOutboxRepository(db databases.Database) OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) PublishPending(ctx context.Context, limit int, publish func(model.OutboxMessage) error) (int, error) {
	published := 0
	var publishErr error

	err := withTx(ctx, r.db.DB(), func(tx *sqlx.Tx) error {
		var messages []model.OutboxMessage
		// The row locks keep a concurrent relay from publishing the same messages.
		if err := tx.SelectContext(ctx, &messages,
			"SELECT "+outboxColumns+" FROM outbox ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED", limit,
		); err != nil {
			return err
		}

		for _, message := range messages {
			if publishErr = publish(message); publishErr != nil {
				// Keep the rest queued, in order, for the next run.
				return nil
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM outbox WHERE id = $1", message.ID); err != nil {
				return err
			}
			published++
		}
		return nil
	})
	if err != nil {
		return 0, translateError(err)
	}
	return published, publishErr
}

// insertOutboxMessage queues a message in the transaction of the change it announces.
func insertOutboxMessage(ctx context.Context, tx *sqlx.Tx, topic string, payload []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO outbox (topic, payload) VALUES ($1, $2)", topic, string(payload))
	return err
}
