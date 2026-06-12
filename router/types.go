package router

import (
	"github.com/gammazero/nexus/v3/stdlog"
	"github.com/gammazero/nexus/v3/wamp"
)

// Broker handles publish/subscribe and topic-side meta procedures
// for a single realm. The default implementation in [broker] is the
// in-process actor-loop broker; alternative implementations (e.g. a
// clustered Raft-backed broker, a forwarding proxy) can be supplied
// per-realm via RealmConfig once the factory hook lands.
//
// Methods are called concurrently from multiple router goroutines:
// the per-session message handler goroutines (one per attached
// session, plus the realm's meta session), the realm's session
// lifecycle paths (RemoveSession on disconnect), and the realm actor
// during construction and shutdown (PreInitEventHistoryTopics, Role,
// Close). Implementations must be safe for concurrent entry. The
// default implementation serializes its state by running all
// mutations on a single internal actor goroutine.
type Broker interface {
	// Publish dispatches a PUBLISH message from a session to all
	// matching subscribers. The pub session must not be nil; trusted
	// internal publications (e.g. meta events) are published from the
	// realm's meta session, and callers integrating external sources
	// must construct a session carrying the publisher's identity.
	Publish(pub *wamp.Session, msg *wamp.Publish)

	// Subscribe registers the session for the topic in msg, with
	// optional match policy. Replies to the session with SUBSCRIBED
	// or ERROR.
	Subscribe(sub *wamp.Session, msg *wamp.Subscribe)

	// Unsubscribe removes the subscription identified in msg. Replies
	// with UNSUBSCRIBED or ERROR(no_such_subscription).
	Unsubscribe(sub *wamp.Session, msg *wamp.Unsubscribe)

	// RemoveSession removes all subscriptions held by the session,
	// emitting on_unsubscribe / on_delete meta events as appropriate.
	// Called by the realm when a client disconnects.
	RemoveSession(sess *wamp.Session)

	// Close shuts down the broker, draining any pending action queue
	// and stopping the actor goroutine. Called by the realm during
	// orderly shutdown after all client sessions have ended.
	Close()

	// Role returns the broker role + advertised features for the
	// WELCOME message.
	Role() wamp.Dict

	// PreInitEventHistoryTopics initializes event-history retention
	// for the given configs. Called once during realm construction
	// before any sessions are attached.
	PreInitEventHistoryTopics(cfgs []*TopicEventHistoryConfig) error

	// Topic-side session meta-procedure handlers. Each takes an
	// INVOCATION and returns the response message (RESULT or ERROR).
	SubList(*wamp.Invocation) wamp.Message
	SubLookup(*wamp.Invocation) wamp.Message
	SubMatch(*wamp.Invocation) wamp.Message
	SubGet(*wamp.Invocation) wamp.Message
	SubListSubscribers(*wamp.Invocation) wamp.Message
	SubCountSubscribers(*wamp.Invocation) wamp.Message
	SubEventHistory(*wamp.Invocation) wamp.Message
}

// Dealer handles register/call/yield and procedure-side meta
// procedures for a single realm. Same concurrency contract as
// [Broker]: concurrent entry from per-session handler goroutines.
type Dealer interface {
	// Register registers the session as a callee for the procedure
	// in msg. Replies with REGISTERED or ERROR.
	Register(callee *wamp.Session, msg *wamp.Register)

	// Unregister removes the registration identified in msg.
	Unregister(callee *wamp.Session, msg *wamp.Unregister)

	// Call dispatches a CALL message from a caller to a registered
	// callee, applying shared-registration policy if set.
	Call(caller *wamp.Session, msg *wamp.Call)

	// Cancel cancels a pending call. Behavior depends on
	// msg.Options["mode"]: skip / kill / killnowait. See dealer.cancel
	// docs for the per-mode contract.
	Cancel(caller *wamp.Session, msg *wamp.Cancel)

	// Yield delivers a callee's YIELD back to the original caller as
	// RESULT (or progressive RESULT if options.progress=true).
	Yield(callee *wamp.Session, msg *wamp.Yield)

	// Error delivers a callee's ERROR for an INVOCATION back to the
	// original caller as a CALL ERROR.
	Error(callee *wamp.Session, msg *wamp.Error)

	// RemoveSession removes all registrations and pending invocations
	// held by the session. Called by the realm on session disconnect.
	RemoveSession(sess *wamp.Session)

	// Close shuts down the dealer, draining any pending action queue
	// and stopping the actor goroutine.
	Close()

	// Role returns the dealer role + advertised features for the
	// WELCOME message.
	Role() wamp.Dict

	// SetMetaPeer installs the meta-event peer used to publish
	// dealer-originated meta events (e.g. wamp.registration.on_create).
	SetMetaPeer(metaPeer wamp.Peer)

	// Procedure-side meta-procedure handlers.
	RegList(*wamp.Invocation) wamp.Message
	RegLookup(*wamp.Invocation) wamp.Message
	RegMatch(*wamp.Invocation) wamp.Message
	RegGet(*wamp.Invocation) wamp.Message
	RegListCallees(*wamp.Invocation) wamp.Message
	RegCountCallees(*wamp.Invocation) wamp.Message
}

// Compile-time assertions that the default in-process implementations
// satisfy the public interfaces. If a method is added to either
// interface but not the concrete struct (or vice versa), the package
// fails to compile here rather than at any consumer's call site.
var (
	_ Broker = (*broker)(nil)
	_ Dealer = (*dealer)(nil)
)

// BrokerFactory constructs the Broker for a given realm. Set it on
// RealmConfig.BrokerFactory to swap in a custom implementation
// (e.g. clustered, metrics-wrapped, forwarding proxy). When unset
// the router uses the default in-process Broker.
type BrokerFactory func(cfg *RealmConfig, logger stdlog.StdLog, debug bool) (Broker, error)

// DealerFactory constructs the Dealer for a given realm. Same shape
// as BrokerFactory.
type DealerFactory func(cfg *RealmConfig, logger stdlog.StdLog, debug bool) (Dealer, error)

// defaultBrokerFactory builds the in-process actor-loop broker used
// when RealmConfig.BrokerFactory is unset.
func defaultBrokerFactory(cfg *RealmConfig, logger stdlog.StdLog, debug bool) (Broker, error) {
	return newBroker(logger, cfg.StrictURI, cfg.AllowDisclose, debug,
		cfg.PublishFilterFactory, cfg.TopicEventHistoryConfigs)
}

// defaultDealerFactory builds the in-process actor-loop dealer used
// when RealmConfig.DealerFactory is unset.
func defaultDealerFactory(cfg *RealmConfig, logger stdlog.StdLog, debug bool) (Dealer, error) {
	return newDealer(logger, cfg.StrictURI, cfg.AllowDisclose, debug), nil
}

// NewDefaultBroker constructs the same in-process broker the router uses
// when RealmConfig.BrokerFactory is unset. Custom factories that decorate
// the default implementation (rather than replace it) wrap the value
// returned here.
func NewDefaultBroker(cfg *RealmConfig, logger stdlog.StdLog, debug bool) (Broker, error) {
	return defaultBrokerFactory(cfg, logger, debug)
}

// NewDefaultDealer is the dealer counterpart of [NewDefaultBroker].
func NewDefaultDealer(cfg *RealmConfig, logger stdlog.StdLog, debug bool) (Dealer, error) {
	return defaultDealerFactory(cfg, logger, debug)
}
