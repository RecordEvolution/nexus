package wamp

import (
	"errors"
	"time"
)

// Peer is the interface implemented by endpoints communicating via WAMP.
//
// # Lifecycle and concurrent shutdown
//
// Close is idempotent and safe to call concurrently with itself AND
// with Send. The architectural guarantee: the channel returned by
// Send is never closed by the implementation; only an internal
// forwarder/sendHandler reads from it, and that internal goroutine
// exits cleanly via Done. Bare `peer.Send() <- msg` therefore never
// panics with "send on closed channel" — but after Close, any send
// that the implementation cannot deliver blocks forever (rather
// than panicking, the previous behavior).
//
// Senders that may race with Close should use the cooperative
// pattern, which lets them abandon a blocked send when the peer
// shuts down:
//
//	select {
//	case peer.Send() <- msg:
//	case <-peer.Done():
//	    return // peer is closing; abandon
//	}
//
// Implementations close Done before stopping their internal forwarder,
// so a goroutine entering this select after Close has begun observes
// Done ready and exits cleanly. A goroutine already parked in this
// select when Close starts wakes via Done as well — the forwarder
// stops reading from Send, so the sender's parked send is never
// satisfied via the Send case.
type Peer interface {
	// Close closes the peer. Idempotent. Done is closed before the
	// internal forwarder/sendHandler exits.
	Close()

	// IsLocal returns true if the session is local.
	IsLocal() bool

	// Recv returns a channel of messages from the peer. The channel
	// is closed when the remote side closes its connection, or when
	// Close is called on this peer.
	Recv() <-chan Message

	// Send returns the peer's outgoing message channel. The channel
	// is never closed by the implementation; bare-channel Send is
	// always panic-free but blocks forever after Close. See the
	// package doc for the cooperative-shutdown pattern.
	Send() chan<- Message

	// Done returns a channel that is closed when Close is called.
	// Senders use it to detect peer closure cooperatively.
	Done() <-chan struct{}
}

// RecvTimeout receives a message from a peer within the specified time.
func RecvTimeout(p Peer, timeout time.Duration) (Message, error) {
	to := time.NewTimer(timeout)
	defer to.Stop()

	select {
	case msg, open := <-p.Recv():
		if !open {
			return nil, errors.New("receive channel closed")
		}
		return msg, nil
	case <-to.C:
		return nil, errors.New("timeout waiting for message")
	}
}
