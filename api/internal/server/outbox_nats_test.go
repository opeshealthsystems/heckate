package server

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func TestOutboxEventsReachJetStreamOnce(t *testing.T) {
	e := setup(t)
	srv, err := natsserver.NewServer(&natsserver.Options{Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS did not start")
	}
	defer srv.Shutdown()

	staff := token("nats-agent", "", "support")
	tid := e.newClient(staff, "NATS Co "+t.Name())
	_, tk, _ := e.do("POST", "/v1/tickets", staff, map[string]any{"tenant_id": tid, "subject": "page me", "severity": "P1", "channel": "phone"})
	ticketID := tk["id"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pub, err := platform.NewNATSPublisher(ctx, srv.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()
	pool, err := platform.NewPool(ctx, os.Getenv("HEKATE_TEST_DB"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for i := 0; i < 50; i++ { // drain whatever other tests left in the outbox, 100 rows per pass
		if err := platform.RelayOnce(ctx, pool, pub); err != nil {
			t.Fatal(err)
		}
		var left int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM platform.outbox WHERE published_at IS NULL`).Scan(&left)
		if left == 0 {
			break
		}
	}

	nc, _ := nats.Connect(srv.ClientURL())
	defer nc.Close()
	js, _ := jetstream.New(nc)
	stream, err := js.Stream(ctx, streamNameForTest)
	if err != nil {
		t.Fatal(err)
	}
	read := func(filter string) int {
		cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{FilterSubject: filter, AckPolicy: jetstream.AckExplicitPolicy})
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for {
			batch, _ := cons.Fetch(100, jetstream.FetchMaxWait(time.Second))
			got := 0
			for m := range batch.Messages() {
				got++
				var p struct {
					ID string `json:"id"`
				}
				_ = json.Unmarshal(m.Data(), &p)
				if p.ID == ticketID {
					n++
				}
				_ = m.Ack()
			}
			if got == 0 {
				return n
			}
		}
	}
	if n := read("ticket.created"); n != 1 {
		t.Errorf("ticket.created for our ticket should be on the stream once, got %d", n)
	}
	if n := read("ticket.p1.page"); n != 1 {
		t.Errorf("a P1 must publish ticket.p1.page once, got %d", n)
	}
	// Running the relay again must not publish anything twice.
	_ = platform.RelayOnce(ctx, pool, pub)
	info, _ := stream.Info(ctx)
	before := info.State.Msgs
	_ = platform.RelayOnce(ctx, pool, pub)
	info, _ = stream.Info(ctx)
	if info.State.Msgs != before {
		t.Errorf("a second relay pass must be a no-op, stream grew from %d to %d", before, info.State.Msgs)
	}
}

const streamNameForTest = "HEKATE_EVENTS"
