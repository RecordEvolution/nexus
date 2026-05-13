package test_test

import "testing"

// These tests are pinning markers for WAMP advanced-profile features the
// router does not yet implement. Each one corresponds to a feature listed
// "No" in README.md or absent from the dealer/broker advertised feature set.
// Discoverable via `go test -v -run SpecUnimplemented`. When a feature lands,
// flip the t.Skip to a real assertion.

func TestSpecUnimplementedRegistrationRevocation(t *testing.T) {
	t.Skip("pending: registration_revocation — wamp.registration.revoke meta procedure not implemented")
}

func TestSpecUnimplementedShardedRegistration(t *testing.T) {
	t.Skip("pending: sharded_registration — REGISTER does not honor shard option")
}

func TestSpecUnimplementedShardedSubscription(t *testing.T) {
	t.Skip("pending: sharded_subscription — SUBSCRIBE does not honor shard option")
}

func TestSpecUnimplementedCallTrustLevels(t *testing.T) {
	t.Skip("pending: call_trustlevels — INVOCATION details do not carry trustlevel")
}

func TestSpecUnimplementedPublicationTrustLevels(t *testing.T) {
	t.Skip("pending: publication_trustlevels — EVENT details do not carry trustlevel")
}

func TestSpecUnimplementedProcedureReflection(t *testing.T) {
	t.Skip("pending: procedure_reflection — wamp.reflection.procedure.* not implemented")
}

func TestSpecUnimplementedTopicReflection(t *testing.T) {
	t.Skip("pending: topic_reflection — wamp.reflection.topic.* not implemented")
}

func TestSpecUnimplementedBatchedWSTransport(t *testing.T) {
	t.Skip("pending: batched WebSocket transport (wamp.2.json.batched / msgpack.batched)")
}

func TestSpecUnimplementedLongPollTransport(t *testing.T) {
	t.Skip("pending: WAMP long-polling HTTP transport")
}
