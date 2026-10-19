package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const streamName = "HEKATE_EVENTS"

// EventDomains are the first token of every event subject ("ticket.created" belongs to "ticket").
// The stream lists them explicitly: a catch-all "*.>" would also capture NATS's own $JS API subjects,
// which JetStream refuses. A test checks that every Emit call in the code uses a listed domain.
var EventDomains = []string{
	"ticket", "client", "lead", "quote", "invoice", "payment", "credit_note", "intervention",
	"stock", "rfq", "purchase_order", "supplier", "supplier_invoice", "leave", "expense", "payroll", "accounting", "document", "contract", "form",
}

func eventSubjects() []string {
	out := make([]string, len(EventDomains))
	for i, d := range EventDomains {
		out[i] = d + ".>"
	}
	return out
}

// NATSPublisher publishes outbox events to a JetStream stream that captures every domain event.
type NATSPublisher struct {
	nc *nats.Conn
	js jetstream.JetStream
}

// NewNATSPublisher connects and makes sure the events stream exists.
func NewNATSPublisher(ctx context.Context, url string) (*NATSPublisher, error) {
	nc, err := nats.Connect(url, nats.Name("hekate-api"), nats.MaxReconnects(-1), nats.ReconnectWait(2*time.Second))
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       streamName,
		Subjects:   eventSubjects(), // every event is "<domain>.<verb>[.<detail>]"
		Retention:  jetstream.LimitsPolicy,
		MaxAge:     30 * 24 * time.Hour,
		Duplicates: 24 * time.Hour, // window in which a repeated message id is dropped
		Storage:    jetstream.FileStorage,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create stream: %w", err)
	}
	return &NATSPublisher{nc: nc, js: js}, nil
}

// Publish sends an event; the outbox id doubles as the message id so a redelivery is ignored.
func (p *NATSPublisher) Publish(ctx context.Context, subject string, payload []byte, id string) error {
	_, err := p.js.Publish(ctx, subject, payload, jetstream.WithMsgID(id))
	return err
}

// Close drains the connection.
func (p *NATSPublisher) Close() { _ = p.nc.Drain() }
