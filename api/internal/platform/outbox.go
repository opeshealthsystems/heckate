package platform

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Publisher delivers outbox events to a bus (NATS JetStream in production).
type Publisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

// LogPublisher is the default publisher until NATS is wired in.
type LogPublisher struct{}

func (LogPublisher) Publish(_ context.Context, subject string, payload []byte) error {
	slog.Info("event", "subject", subject, "payload", string(payload))
	return nil
}

// RelayOutbox publishes unpublished events in order until ctx is cancelled.
func RelayOutbox(ctx context.Context, pool *pgxpool.Pool, pub Publisher, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := relayOnce(ctx, pool, pub); err != nil {
				slog.Error("outbox relay", "err", err)
			}
		}
	}
}

func relayOnce(ctx context.Context, pool *pgxpool.Pool, pub Publisher) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id, subject, payload FROM platform.outbox WHERE published_at IS NULL ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return err
	}
	type ev struct {
		id      int64
		subject string
		payload []byte
	}
	var evs []ev
	for rows.Next() {
		var e ev
		if err := rows.Scan(&e.id, &e.subject, &e.payload); err != nil {
			return err
		}
		evs = append(evs, e)
	}
	rows.Close()
	for _, e := range evs {
		if err := pub.Publish(ctx, e.subject, e.payload); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE platform.outbox SET published_at = now() WHERE id=$1`, e.id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
