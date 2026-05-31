package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	subscriberBufferSize = 64
	incomingBufferSize   = 1024
)

// LagTickerInterval controls how often the lag ticker runs.
// Tests override this via t.Cleanup to speed up lag-event verification.
var LagTickerInterval = 5 * time.Second

// Now returns the current time. Tests override this to inject a fake clock.
var Now = time.Now

// Event is a typed message published through the bus.
type Event struct {
	Type      string          `json:"type"`
	Workspace string          `json:"workspace,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	TS        time.Time       `json:"ts"`
}

// Publisher is the produce-side interface. Producers depend on this;
// the concrete *Bus is wired at server startup.
type Publisher interface {
	Publish(Event)
}

type subscriber struct {
	ch        chan Event
	workspace string
	dropped   atomic.Uint64
	lastLagTS atomic.Value // stores time.Time
	once      sync.Once
}

// Bus is a typed in-process pub/sub event bus.
type Bus struct {
	ctx             context.Context
	cancel          context.CancelFunc
	incoming        chan Event
	subs            map[*subscriber]struct{}
	subsMu          sync.RWMutex
	closeOnce       sync.Once
	lagTickInterval time.Duration
}

// New creates a Bus and starts its fan-out and lag-ticker goroutines.
// The goroutines exit when ctx is cancelled or Close is called.
func New(ctx context.Context) *Bus {
	ctx, cancel := context.WithCancel(ctx)
	b := &Bus{
		ctx:             ctx,
		cancel:          cancel,
		incoming:        make(chan Event, incomingBufferSize),
		subs:            make(map[*subscriber]struct{}),
		lagTickInterval: LagTickerInterval,
	}
	go b.runFanout()
	go b.runLagTicker()
	return b
}

// Publish sends an event to the bus. It never blocks the caller.
// If the internal buffer is full, the event is silently dropped.
func (b *Bus) Publish(e Event) {
	if e.TS.IsZero() {
		e.TS = Now()
	}
	select {
	case b.incoming <- e:
	default:
	}
}

// Subscribe returns a channel that receives events matching the given
// workspace filter and a closer function to unsubscribe. An empty
// workspace subscribes to all events. The closer is idempotent.
func (b *Bus) Subscribe(workspace string) (<-chan Event, func()) {
	s := &subscriber{
		ch:        make(chan Event, subscriberBufferSize),
		workspace: workspace,
	}
	s.lastLagTS.Store(Now())

	b.subsMu.Lock()
	b.subs[s] = struct{}{}
	b.subsMu.Unlock()

	closer := func() {
		s.once.Do(func() {
			b.subsMu.Lock()
			_, exists := b.subs[s]
			delete(b.subs, s)
			b.subsMu.Unlock()
			if exists {
				close(s.ch)
			}
			// If !exists, Close() already closed s.ch — skip to avoid panic.
		})
	}
	return s.ch, closer
}

// Close shuts down the bus: cancels the context (stopping fan-out and
// lag ticker), drains remaining incoming events, and closes every
// subscriber channel. Idempotent.
func (b *Bus) Close() {
	b.closeOnce.Do(func() {
		b.cancel()

		// Drain remaining incoming events (best effort).
		for i := 0; i < incomingBufferSize; i++ {
			select {
			case e := <-b.incoming:
				b.fanoutEvent(e)
			default:
				goto done
			}
		}
	done:

		b.subsMu.Lock()
		for s := range b.subs {
			close(s.ch)
			delete(b.subs, s)
		}
		b.subsMu.Unlock()
	})
}

func (b *Bus) runFanout() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case e := <-b.incoming:
			b.fanoutEvent(e)
		}
	}
}

func (b *Bus) fanoutEvent(e Event) {
	b.subsMu.RLock()
	defer b.subsMu.RUnlock()

	for s := range b.subs {
		if !matchesWorkspace(s.workspace, e.Workspace) {
			continue
		}
		select {
		case s.ch <- e:
		default:
			s.dropped.Add(1)
		}
	}
}

func matchesWorkspace(subFilter, eventWS string) bool {
	if eventWS == "" {
		return true // global events go to all subscribers
	}
	if subFilter == "" {
		return true // subscriber with empty filter receives all
	}
	return subFilter == eventWS
}

func (b *Bus) runLagTicker() {
	ticker := time.NewTicker(b.lagTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.emitLagEvents()
		}
	}
}

func (b *Bus) emitLagEvents() {
	b.subsMu.RLock()
	defer b.subsMu.RUnlock()

	for s := range b.subs {
		dropped := s.dropped.Swap(0)
		if dropped == 0 {
			continue
		}

		sinceTS, _ := s.lastLagTS.Load().(time.Time)
		s.lastLagTS.Store(Now())

		payload := fmt.Sprintf(`{"dropped":%d,"since_ts":"%s"}`, dropped, sinceTS.Format(time.RFC3339))
		lagEvent := Event{
			Type:      "lag",
			Workspace: s.workspace,
			Payload:   json.RawMessage(payload),
			TS:        Now(),
		}

		select {
		case s.ch <- lagEvent:
		default:
			// Perpetually full subscriber — don't deadlock the ticker.
			// Re-add the dropped count so it's reported next tick.
			s.dropped.Add(dropped)
		}
	}
}
