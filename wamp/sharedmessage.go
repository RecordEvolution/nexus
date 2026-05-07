package wamp

// SharedMessage wraps a Message that the broker is fanning out to
// multiple peers, carrying a per-serializer cache of pre-encoded
// bytes. The broker's actor goroutine pre-fills the cache (one entry
// per serializer in active use across the subscription group) before
// sending the wrapper down each subscriber's send channel.
//
// Per-session send goroutines see the wrapper, look up the bytes for
// their serializer, and write them straight to the wire — no encoding
// work in the hot fan-out path.
//
// # Why not lazy?
//
// An earlier attempt populated the cache lazily inside the
// serializer's Serialize() call. That defeats the optimization:
// 200 send goroutines all wake at the same time, all miss the cache,
// all encode, last one wins the cache write. Holding a mutex during
// encoding to force serial population then created a sequential
// bottleneck on one core.
//
// The correct pattern: `broker.run` is a single-threaded actor and
// is the *sole writer* to the cache. It populates the cache for
// every active serializer before any subscriber goroutine sees the
// wrapper. The wrapper's cache is therefore effectively immutable
// from the readers' perspective and needs no synchronization at all
// past that point.
//
// # Lifetime
//
// The wrapper is a per-event short-lived value. After all subscriber
// goroutines have drained their channels and serialized to the wire,
// the wrapper becomes garbage. Cached bytes are typically a few
// hundred bytes, so the GC pressure from the wrapper itself is small.
type SharedMessage struct {
	// Inner is the wrapped Message. Its MessageType is what
	// SharedMessage.MessageType returns, so a SharedMessage flows
	// transparently through wamp.Message-typed channels.
	Inner Message

	// cache is keyed by the integer value of a serialize.Serialization
	// constant (using int here to keep this package free of the
	// serialize-package import). Populated entirely before any reader
	// observes the wrapper; readers do unsynchronized reads.
	cache map[int][]byte
}

// NewSharedMessage wraps msg for fan-out caching. The cache starts
// empty and is populated by the producer before fan-out.
func NewSharedMessage(msg Message) *SharedMessage {
	return &SharedMessage{Inner: msg}
}

// MessageType returns the type of the wrapped message.
func (s *SharedMessage) MessageType() MessageType {
	return s.Inner.MessageType()
}

// Store records pre-encoded bytes for a serializer ID. Called only
// by the broker actor goroutine before fan-out; concurrent stores
// from multiple goroutines are NOT supported.
func (s *SharedMessage) Store(serID int, b []byte) {
	if s.cache == nil {
		s.cache = make(map[int][]byte, 3)
	}
	s.cache[serID] = b
}

// Cached returns pre-encoded bytes for serID, or (nil, false) if
// nothing was stored for that serializer.
func (s *SharedMessage) Cached(serID int) ([]byte, bool) {
	b, ok := s.cache[serID]
	return b, ok
}
