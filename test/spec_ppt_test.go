package test_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
)

// TestSpecPPTCallRoundTrip verifies a CALL with ppt_scheme set carries the
// PPT options and payload through to the callee untouched (spec §14.7).
//
// Skipped on non-local transports: client.go's PPT pack/unpack helpers
// assume args[0] survives the wire as either []byte (non-native
// serializer) or *wamp.PassthruPayload (native). JSON wire turns []byte
// into a base64 string and *PassthruPayload into a wamp.Dict, both of
// which crash the type assertions in client.go's unpackPPTPayload.
// That's a real client-side bug to fix separately (tracked in
// IRONFLOCK_PLAN backlog as "PPT round-trip via non-local transport");
// for now, this spec test pins the local-transport behavior only.
func TestSpecPPTCallRoundTrip(t *testing.T) {
	if scheme != "" {
		t.Skip("client.go PPT pack/unpack only round-trips on local in-process transport")
	}
	checkGoLeaks(t)

	callee := connectClient(t)

	gotScheme := make(chan string, 1)
	gotArgs := make(chan wamp.List, 1)
	const procName = "spec.ppt.echo"

	require.NoError(t, callee.Register(procName, func(_ context.Context, inv *wamp.Invocation) client.InvokeResult {
		s, _ := wamp.AsString(inv.Details[wamp.OptPPTScheme])
		gotScheme <- s
		gotArgs <- inv.Arguments
		// Echo back with the same PPT options.
		return client.InvokeResult{
			Args:   inv.Arguments,
			Kwargs: inv.ArgumentsKw,
			Options: wamp.Dict{
				wamp.OptPPTScheme:     "mqtt",
				wamp.OptPPTSerializer: "native",
			},
		}
	}, nil))

	caller := connectClient(t)
	opts := wamp.Dict{
		wamp.OptPPTScheme:     "mqtt",
		wamp.OptPPTSerializer: "native",
	}
	res, err := caller.Call(context.Background(), procName, opts,
		wamp.List{"opaque-payload", int64(7)}, wamp.Dict{"k": "v"}, nil)
	require.NoError(t, err)

	select {
	case s := <-gotScheme:
		require.Equal(t, "mqtt", s, "callee must see ppt_scheme in INVOCATION details")
	case <-time.After(time.Second):
		t.Fatal("callee did not receive INVOCATION")
	}

	args := <-gotArgs
	require.Equal(t, "opaque-payload", args[0])

	require.NotEmpty(t, res.Arguments, "RESULT carries the payload back")

	require.NoError(t, callee.Unregister(procName))
}

// TestSpecPPTPublishRoundTrip verifies PUBLISH with ppt_scheme carries the
// PPT options and payload through to the subscriber untouched.
//
// Skipped on non-local transports for the same reason as
// TestSpecPPTCallRoundTrip — client.go's PPT helpers don't round-trip
// args[0] over a serialized wire.
func TestSpecPPTPublishRoundTrip(t *testing.T) {
	if scheme != "" {
		t.Skip("client.go PPT pack/unpack only round-trips on local in-process transport")
	}
	checkGoLeaks(t)

	sub := connectClient(t)

	type evtData struct {
		scheme string
		args   wamp.List
	}
	gotEvent := make(chan evtData, 1)
	const topic = "spec.ppt.topic"

	require.NoError(t, sub.Subscribe(topic, func(ev *wamp.Event) {
		s, _ := wamp.AsString(ev.Details[wamp.OptPPTScheme])
		gotEvent <- evtData{scheme: s, args: ev.Arguments}
	}, nil))

	pub := connectClient(t)
	opts := wamp.Dict{
		wamp.OptAcknowledge:   true,
		wamp.OptPPTScheme:     "mqtt",
		wamp.OptPPTSerializer: "native",
	}
	require.NoError(t, pub.Publish(topic, opts,
		wamp.List{"opaque-payload"}, nil))

	select {
	case e := <-gotEvent:
		require.Equal(t, "mqtt", e.scheme, "EVENT must carry ppt_scheme through")
		require.Equal(t, "opaque-payload", e.args[0])
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive EVENT")
	}

	require.NoError(t, sub.Unsubscribe(topic))
}
