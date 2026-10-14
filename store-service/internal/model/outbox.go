package model

import "time"

// OutboxMessage is a message waiting in the outbox for the relay to publish it.
type OutboxMessage struct {
	ID        int64     `db:"id"`
	Topic     string    `db:"topic"`
	Payload   string    `db:"payload"`
	CreatedAt time.Time `db:"created_at"`
}
