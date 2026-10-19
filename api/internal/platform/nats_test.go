package platform

import (
	"context"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func startNATS(t *testing.T) string {
	t.Helper()
	srv, err := natsserver.NewServer(&natsserver.Options{Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("embedded NATS did not start")
	}
	t.Cleanup(srv.Shutdown)
	return srv.ClientURL()
}

func TestNATSPublisherDeliversAndDeduplicates(t *testing.T) {
	url := startNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pub, err := NewNATSPublisher(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	// The same outbox row published twice (a crash before it was marked done) lands once.
	for i := 0; i < 2; i++ {
		if err := pub.Publish(ctx, "ticket.created", []byte(`{"id":"1"}`), "42"); err != nil {
			t.Fatal(err)
		}
	}
	if err := pub.Publish(ctx, "invoice.paid", []byte(`{"id":"2"}`), "43"); err != nil {
		t.Fatal(err)
	}
	if err := pub.Publish(ctx, "ticket.p1.page", []byte(`{"id":"3"}`), "44"); err != nil {
		t.Fatal(err)
	}

	nc, _ := nats.Connect(url)
	defer nc.Close()
	js, _ := jetstream.New(nc)
	stream, err := js.Stream(ctx, streamName)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := stream.Info(ctx)
	if info.State.Msgs != 3 {
		t.Fatalf("want 3 messages (duplicate dropped), got %d", info.State.Msgs)
	}
	// A consumer filtered on a subject only sees that kind of event, including three-token subjects.
	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{FilterSubject: "ticket.>", AckPolicy: jetstream.AckExplicitPolicy})
	if err != nil {
		t.Fatal(err)
	}
	batch, _ := cons.Fetch(10, jetstream.FetchMaxWait(2*time.Second))
	n := 0
	for m := range batch.Messages() {
		n++
		_ = m.Ack()
	}
	if n != 2 {
		t.Errorf("ticket.> should match ticket.created and ticket.p1.page, got %d", n)
	}
}
