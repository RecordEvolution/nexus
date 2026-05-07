package router //nolint:testpackage

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gammazero/nexus/v3/stdlog"
	"github.com/gammazero/nexus/v3/wamp"
)

// brokerFactoryProbe wraps the default broker but records that the
// factory was invoked, so the test can assert RealmConfig.BrokerFactory
// is honored when set.
type brokerFactoryProbe struct {
	Broker
	called bool
}

// dealerFactoryProbe is the analogous wrapper for Dealer.
type dealerFactoryProbe struct {
	Dealer
	called bool
}

// TestRealmConfigBrokerAndDealerFactoriesHonored verifies that custom
// BrokerFactory / DealerFactory hooks on RealmConfig are invoked
// instead of the default in-process implementations. Critical for
// downstream consumers (e.g. clustered nexus, ironflock-router) that
// need to substitute their own broker/dealer.
func TestRealmConfigBrokerAndDealerFactoriesHonored(t *testing.T) {
	const realmURI = wamp.URI("nexus.test.factory")

	var brokerProbe *brokerFactoryProbe
	var dealerProbe *dealerFactoryProbe

	r, err := NewRouter(&Config{
		RealmConfigs: []*RealmConfig{
			{
				URI:       realmURI,
				StrictURI: false,
				BrokerFactory: func(cfg *RealmConfig, logger stdlog.StdLog, debug bool) (Broker, error) {
					inner, err := defaultBrokerFactory(cfg, logger, debug)
					if err != nil {
						return nil, err
					}
					brokerProbe = &brokerFactoryProbe{Broker: inner, called: true}
					return brokerProbe, nil
				},
				DealerFactory: func(cfg *RealmConfig, logger stdlog.StdLog, debug bool) (Dealer, error) {
					inner, err := defaultDealerFactory(cfg, logger, debug)
					if err != nil {
						return nil, err
					}
					dealerProbe = &dealerFactoryProbe{Dealer: inner, called: true}
					return dealerProbe, nil
				},
			},
		},
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { r.Close() })

	require.NotNil(t, brokerProbe, "BrokerFactory was not called")
	require.True(t, brokerProbe.called)
	require.NotNil(t, dealerProbe, "DealerFactory was not called")
	require.True(t, dealerProbe.called)
}

// TestRealmConfigUnsetFactoriesUseDefaults exercises the fallback path
// — when neither factory is set, the realm uses the default in-process
// broker/dealer. Indirectly verified by every other test in this
// package, but pinned here so a future refactor that breaks the
// default selection fails loud at this single test.
func TestRealmConfigUnsetFactoriesUseDefaults(t *testing.T) {
	const realmURI = wamp.URI("nexus.test.factory.default")

	r, err := NewRouter(&Config{
		RealmConfigs: []*RealmConfig{
			{URI: realmURI, StrictURI: false, AnonymousAuth: true},
		},
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { r.Close() })

	// Reach into the realm to inspect the dynamic types of broker / dealer.
	rl := r.(*router).realms[realmURI]
	require.NotNil(t, rl)
	_, isDefaultBroker := rl.broker.(*broker)
	require.True(t, isDefaultBroker, "expected default *broker; got %T", rl.broker)
	_, isDefaultDealer := rl.dealer.(*dealer)
	require.True(t, isDefaultDealer, "expected default *dealer; got %T", rl.dealer)
}
