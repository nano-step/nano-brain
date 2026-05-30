package eventbus

func (b *Bus) testSubscriberDropped(ch <-chan Event) uint64 {
	b.subsMu.RLock()
	defer b.subsMu.RUnlock()

	for s := range b.subs {
		if s.ch == ch {
			return s.dropped.Load()
		}
	}
	return 0
}
