package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

var _ Publisher = (*Bus)(nil)

func TestPublishWithoutSubscribersIsNonBlocking(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := New(ctx)
	defer b.Close()

	start := time.Now()
	for i := 0; i < 1000; i++ {
		b.Publish(Event{Type: "test", Workspace: "ws"})
	}
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Fatalf("publishing 1000 events without subscribers took %v (>100ms)", elapsed)
	}
}

func TestSubscribeReceivesFilteredEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := New(ctx)
	defer b.Close()

	chA, closeA := b.Subscribe("ws-a")
	defer closeA()
	chAll, closeAll := b.Subscribe("")
	defer closeAll()
	chB, closeBx := b.Subscribe("ws-b")
	defer closeBx()

	events := []Event{
		{Type: "t1", Workspace: "ws-a"},
		{Type: "t2", Workspace: "ws-b"},
		{Type: "t3", Workspace: ""},
	}
	for _, e := range events {
		b.Publish(e)
	}

	drain := func(ch <-chan Event, n int) []Event {
		var out []Event
		for i := 0; i < n; i++ {
			select {
			case e := <-ch:
				out = append(out, e)
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for event %d", i)
			}
		}
		return out
	}

	// ws-a subscriber: receives ws-a event + global event = 2
	gotA := drain(chA, 2)
	if gotA[0].Type != "t1" || gotA[1].Type != "t3" {
		t.Fatalf("ws-a subscriber got %v, want [t1, t3]", gotA)
	}

	// all subscriber: receives all 3
	gotAll := drain(chAll, 3)
	types := make([]string, len(gotAll))
	for i, e := range gotAll {
		types[i] = e.Type
	}
	if types[0] != "t1" || types[1] != "t2" || types[2] != "t3" {
		t.Fatalf("all subscriber got types %v, want [t1, t2, t3]", types)
	}

	// ws-b subscriber: receives ws-b event + global event = 2
	gotB := drain(chB, 2)
	if gotB[0].Type != "t2" || gotB[1].Type != "t3" {
		t.Fatalf("ws-b subscriber got %v, want [t2, t3]", gotB)
	}

	// ws-a should NOT have received ws-b event
	select {
	case e := <-chA:
		t.Fatalf("ws-a subscriber unexpectedly received %v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestUnsubscribeStopsEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := New(ctx)
	defer b.Close()

	ch, closer := b.Subscribe("ws")

	b.Publish(Event{Type: "before"})
	select {
	case e := <-ch:
		if e.Type != "before" {
			t.Fatalf("got type %q, want before", e.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first event")
	}

	closer()

	b.Publish(Event{Type: "after"})
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("received event after unsubscribe; channel should be closed")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("channel was not closed after unsubscribe")
	}

	closer() // idempotent — must not panic
}

func TestDropNewestBackpressure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := New(ctx)
	defer b.Close()

	ch, closer := b.Subscribe("ws")
	defer closer()

	// Fill the buffer (64 events) + 5 more that should be dropped.
	for i := 0; i < 69; i++ {
		b.Publish(Event{Type: "fill", Workspace: "ws", Payload: json.RawMessage(`{"i":` + itoa(i) + `}`)})
	}

	// Give fan-out time to process.
	time.Sleep(100 * time.Millisecond)

	dropped := b.testSubscriberDropped(ch)
	if dropped < 1 {
		t.Fatalf("expected dropped >= 1, got %d", dropped)
	}

	// Verify the FIRST 64 events are still readable (drop-newest, not drop-oldest).
	for i := 0; i < 64; i++ {
		select {
		case e := <-ch:
			if e.Type != "fill" {
				t.Fatalf("event %d: got type %q, want fill", i, e.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out reading event %d", i)
		}
	}
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

func TestLagEventEmittedWithin5s(t *testing.T) {
	orig := LagTickerInterval
	LagTickerInterval = 50 * time.Millisecond
	t.Cleanup(func() { LagTickerInterval = orig })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := New(ctx)
	defer b.Close()

	ch, closer := b.Subscribe("ws")
	defer closer()

	// Fill buffer (64) + 3 extra dropped events = 67 total.
	for i := 0; i < 67; i++ {
		b.Publish(Event{Type: "fill", Workspace: "ws"})
	}
	time.Sleep(50 * time.Millisecond) // let fan-out process

	// Drain the 64 buffered events to make space for the lag event.
	for i := 0; i < 64; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("timed out draining event %d", i)
		}
	}

	// Wait for lag event (ticker at 50ms, give up to 500ms).
	var lagEvent Event
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case e := <-ch:
			if e.Type == "lag" {
				lagEvent = e
				goto found
			}
		case <-deadline:
			t.Fatal("timed out waiting for lag event")
		}
	}
found:

	var payload struct {
		Dropped int    `json:"dropped"`
		SinceTS string `json:"since_ts"`
	}
	if err := json.Unmarshal(lagEvent.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal lag payload: %v", err)
	}
	if payload.Dropped != 3 {
		t.Fatalf("lag event dropped=%d, want 3", payload.Dropped)
	}
	if payload.SinceTS == "" {
		t.Fatal("lag event since_ts is empty")
	}

	// Counter must be 0 after lag event.
	d := b.testSubscriberDropped(ch)
	if d != 0 {
		t.Fatalf("dropped counter after lag event = %d, want 0", d)
	}
}

func TestCloseGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := New(ctx)

	const numSubs = 5
	channels := make([]<-chan Event, numSubs)
	closers := make([]func(), numSubs)
	for i := 0; i < numSubs; i++ {
		channels[i], closers[i] = b.Subscribe("ws")
	}

	for i := 0; i < 10; i++ {
		b.Publish(Event{Type: "pre-close", Workspace: "ws"})
	}
	time.Sleep(50 * time.Millisecond) // let fan-out deliver

	b.Close()

	// All channels must be closed within 500ms.
	var wg sync.WaitGroup
	for i, ch := range channels {
		wg.Add(1)
		go func(idx int, c <-chan Event) {
			defer wg.Done()
			deadline := time.After(500 * time.Millisecond)
			for {
				select {
				case _, ok := <-c:
					if !ok {
						return
					}
				case <-deadline:
					t.Errorf("subscriber %d channel not closed within 500ms", idx)
					return
				}
			}
		}(i, ch)
	}
	wg.Wait()

	// Subsequent Publish must not panic.
	b.Publish(Event{Type: "after-close"})

	// Subsequent Close must not panic (idempotent).
	b.Close()

	// Closers must not panic after Close.
	for _, c := range closers {
		c()
	}
}

func TestConcurrentPublishersRaceFree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := New(ctx)
	defer b.Close()

	const (
		numPublishers = 100
		eventsEach    = 100
		numSubs       = 10
	)

	var subWg sync.WaitGroup
	for i := 0; i < numSubs; i++ {
		ch, closer := b.Subscribe(fmt.Sprintf("ws-%d", i%3))
		subWg.Add(1)
		go func(c <-chan Event, cl func()) {
			defer subWg.Done()
			defer cl()
			for range c {
			}
		}(ch, closer)
	}

	var pubWg sync.WaitGroup
	for i := 0; i < numPublishers; i++ {
		pubWg.Add(1)
		go func(id int) {
			defer pubWg.Done()
			ws := fmt.Sprintf("ws-%d", id%3)
			for j := 0; j < eventsEach; j++ {
				b.Publish(Event{Type: "stress", Workspace: ws})
			}
		}(i)
	}
	pubWg.Wait()

	b.Close()
	subWg.Wait()
}
