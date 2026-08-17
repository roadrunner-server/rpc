package rpc

import (
	"testing"

	"tests/helpers"

	"github.com/roadrunner-server/rpc/v6"
	"github.com/stretchr/testify/require"
)

const rpcAddr = "127.0.0.1:6001"

func rpcPlugins() []any {
	return []any{&rpc.Plugin{}, &Plugin1{}}
}

// TestServesRegisteredPlugin dials the listener and calls the method Plugin1
// exposes through RPC(), so the assertion covers the whole path: the plugin was
// collected, its receiver was registered under its Name(), and goridge encoded
// the round trip.
func TestServesRegisteredPlugin(t *testing.T) {
	helpers.Start(t, "configs/.rr.yaml", rpcPlugins(), helpers.WithTCPProbe(rpcAddr))

	client := helpers.NewRPCClient(t, rpcAddr)

	var got string
	require.NoError(t, client.Call("rpc_test.plugin1.Hello", "Valery", &got))

	require.Equal(t, "Hello, username: Valery", got)
}

// TestUnknownMethodIsRejected proves the dispatcher does not silently accept a
// method nobody registered.
func TestUnknownMethodIsRejected(t *testing.T) {
	helpers.Start(t, "configs/.rr.yaml", rpcPlugins(), helpers.WithTCPProbe(rpcAddr))

	client := helpers.NewRPCClient(t, rpcAddr)

	var got string
	err := client.Call("rpc_test.plugin1.NoSuchMethod", "Valery", &got)

	require.Error(t, err)
}

// TestDisabledWithoutRPCSection covers the config with no rpc block: the plugin
// reports Disabled, so the container still starts but nothing binds the port.
func TestDisabledWithoutRPCSection(t *testing.T) {
	helpers.StartExpectNoListener(t, "configs/.rr-rpc-disabled.yaml", rpcPlugins(), rpcAddr)
}
