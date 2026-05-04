package runner

import (
	"context"
	"log"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gammazero/nexus/v3/router"
	"github.com/gammazero/nexus/v3/transport"
	"github.com/gammazero/nexus/v3/transport/serialize"
	"github.com/gammazero/nexus/v3/wamp"
)

var (
	rtrLogger = log.New(os.Stderr, "SPEC-ROUTER> ", log.LstdFlags|log.Lmicroseconds)
	cliLogger = log.New(os.Stderr, "SPEC-CLIENT> ", log.LstdFlags|log.Lmicroseconds)

	// portMu guards the loopback-port pool used by parallel spec subtests.
	portMu sync.Mutex
)

// startRouter creates a router with one anonymous-auth realm matching the
// plan's `realm` field. The router is closed via t.Cleanup.
func startRouter(t *testing.T, realm string) router.Router {
	t.Helper()
	cfg := &router.Config{
		RealmConfigs: []*router.RealmConfig{
			{
				URI:           wamp.URI(realm),
				StrictURI:     false,
				AnonymousAuth: true,
				AllowDisclose: true,
			},
		},
	}
	r, err := router.NewRouter(cfg, rtrLogger)
	require.NoError(t, err)
	t.Cleanup(func() { r.Close() })
	return r
}

// serveRawSocket starts a TCP rawsocket server on a free loopback port and
// returns its address. The listener is closed via t.Cleanup.
func serveRawSocket(t *testing.T, r router.Router) string {
	t.Helper()

	portMu.Lock()
	defer portMu.Unlock()

	srv := router.NewRawSocketServer(r)
	srv.KeepAlive = 0
	// :0 means the OS picks a free port. The returned closer is the
	// underlying net.Listener; we type-assert to read the chosen address.
	closer, err := srv.ListenAndServe("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { closer.Close() })

	ln, ok := closer.(net.Listener)
	require.True(t, ok, "rawsocket ListenAndServe must return a net.Listener")
	return ln.Addr().String()
}

// dialPeer connects a client-side rawsocket peer to the given address.
func dialPeer(t *testing.T, addr string, serID serialize.Serialization) wamp.Peer {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	peer, err := transport.ConnectRawSocketPeer(ctx, "tcp", addr, serID, nil, cliLogger, 0)
	require.NoError(t, err)
	return peer
}
