// Package runner is the YAML-driven WAMP spec-conformance harness.
//
// Each test plan is a YAML file under spec/plans/ describing a sequence of
// `send` / `expect` steps against a freshly-started nexus router. Messages
// are written in raw WAMP list form (e.g. [1, "realm1", {...}] for HELLO);
// the runner serializes them through a real serializer and exchanges them
// over a real (rawsocket TCP loopback) transport, so deviations in the
// wire format are caught.
//
// Placeholders in `expect` lists allow matching against router-assigned
// values (session IDs, request IDs, etc.). See matcher.go for syntax.
//
// This package is a skeleton: the 5 seed plans in spec/plans/basic/ are
// representative cases. Phase 1 expands coverage to ~150-200 plans across
// basic profile, advanced features, auth flows, and standard error URIs.
package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gammazero/nexus/v3/transport/serialize"
	"github.com/gammazero/nexus/v3/wamp"
)

// Plan is the on-disk format of a YAML test plan.
type Plan struct {
	Name        string `yaml:"name"`
	SpecSection string `yaml:"spec_section"`
	Realm       string `yaml:"realm"`
	Serializer  string `yaml:"serializer"`
	Skip        string `yaml:"skip"` // non-empty → t.Skip with this reason
	Steps       []Step `yaml:"steps"`
}

// Step is one send-or-expect operation in a plan.
type Step struct {
	Send   []any `yaml:"send,omitempty"`
	Expect []any `yaml:"expect,omitempty"`

	// TimeoutMS overrides the default per-step timeout.
	TimeoutMS int `yaml:"timeout_ms,omitempty"`

	// Note is a free-form annotation, ignored by the runner.
	Note string `yaml:"note,omitempty"`
}

// LoadPlan reads a YAML file into a Plan.
func LoadPlan(path string) (*Plan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if p.Serializer == "" {
		p.Serializer = "json"
	}
	if p.Realm == "" {
		p.Realm = "realm1"
	}
	return &p, nil
}

// RunPlan executes a single plan against a freshly-started router. It is
// the main entry point used by tests under spec/runner/.
func RunPlan(t *testing.T, p *Plan) {
	t.Helper()
	if p.Skip != "" {
		t.Skip(p.Skip)
	}

	ser, serID, err := pickSerializer(p.Serializer)
	require.NoErrorf(t, err, "plan %s", p.Name)

	r := startRouter(t, p.Realm)
	addr := serveRawSocket(t, r)
	peer := dialPeer(t, addr, serID)
	t.Cleanup(func() { peer.Close() })

	captures := map[string]any{}

	for i, step := range p.Steps {
		stepName := fmt.Sprintf("step %d", i+1)
		switch {
		case step.Send != nil:
			require.NoErrorf(t, doSend(peer, step.Send, ser),
				"%s: send failed", stepName)

		case step.Expect != nil:
			timeout := time.Duration(step.TimeoutMS) * time.Millisecond
			if timeout == 0 {
				timeout = 2 * time.Second
			}
			actual, err := doExpect(peer, ser, timeout)
			require.NoErrorf(t, err, "%s: receive failed", stepName)
			require.NoErrorf(t, matchList(step.Expect, actual, captures),
				"%s: expected %v, got %v", stepName, step.Expect, actual)

		default:
			t.Fatalf("%s: must specify either `send` or `expect`", stepName)
		}
	}
}

// RunDir walks a directory of YAML plans and runs each as a subtest. The
// subtest name is derived from the file path so test selection works
// naturally with `go test -run`.
func RunDir(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	require.NoError(t, err)
	if len(matches) == 0 {
		t.Skipf("no plans found in %s", dir)
	}
	for _, path := range matches {
		path := path
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			plan, err := LoadPlan(path)
			require.NoError(t, err)
			RunPlan(t, plan)
		})
	}
}

func pickSerializer(name string) (serialize.Serializer, serialize.Serialization, error) {
	switch name {
	case "json", "":
		return &serialize.JSONSerializer{}, serialize.JSON, nil
	case "msgpack":
		return &serialize.MessagePackSerializer{}, serialize.MSGPACK, nil
	case "cbor":
		return &serialize.CBORSerializer{}, serialize.CBOR, nil
	default:
		return nil, 0, fmt.Errorf("unknown serializer %q", name)
	}
}

// doSend converts the YAML list form to a wamp.Message and writes it to
// the peer. The first element is interpreted as the message-type integer.
func doSend(peer wamp.Peer, list []any, ser serialize.Serializer) error {
	if len(list) == 0 {
		return fmt.Errorf("empty send list")
	}
	mt, err := asInt(list[0])
	if err != nil {
		return fmt.Errorf("first element must be message-type integer: %w", err)
	}

	// Round-trip through serializer so the test exercises the full
	// encode/decode path and the message struct is correctly populated.
	msg := wamp.NewMessage(wamp.MessageType(mt))
	if msg == nil {
		return fmt.Errorf("unsupported message type %d", mt)
	}

	jsonBytes, err := serializeListAsBytes(list)
	if err != nil {
		return err
	}
	deserialized, err := ser.Deserialize(jsonBytes)
	if err != nil {
		return fmt.Errorf("deserialize send list: %w", err)
	}

	select {
	case peer.Send() <- deserialized:
	case <-time.After(time.Second):
		return fmt.Errorf("send timed out")
	}
	return nil
}

func doExpect(peer wamp.Peer, ser serialize.Serializer, timeout time.Duration) ([]any, error) {
	select {
	case msg, ok := <-peer.Recv():
		if !ok {
			return nil, fmt.Errorf("peer closed")
		}
		// Re-serialize to bytes then to []any so the matcher operates on
		// the same shape as the YAML expected list.
		bytes, err := ser.Serialize(msg)
		if err != nil {
			return nil, fmt.Errorf("re-serialize received: %w", err)
		}
		return jsonBytesToList(bytes)
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for message")
	}
}
