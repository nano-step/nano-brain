// Package eventbus provides a typed in-process publish/subscribe bus for
// streaming runtime events to SSE clients.
//
// The bus has zero dependencies on other internal/ packages. Producers
// depend on the [Publisher] interface (Publish-only); the concrete [*Bus]
// is wired at server startup via constructor injection — the same pattern
// used by embed.Queue + QueueQuerier.
//
// Subscriber channels are bounded (64 events). When a subscriber's buffer
// is full, the newest event is dropped and a periodic ticker emits a
// synthetic "lag" event so clients know to re-query authoritative state
// via REST.
package eventbus
