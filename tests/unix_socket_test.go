//go:build linux || darwin || freebsd

package rpc

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tests/helpers"

	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/logger/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/stretchr/testify/require"
)

func TestUnixSocketInitErrors(t *testing.T) {
	cases := []struct {
		name    string
		listen  string
		options string
		wantErr string
	}{
		{name: "TCP options", listen: "tcp://127.0.0.1:0", options: `{mode: "0600"}`, wantErr: "filesystem unix:// address"},
		{name: "invalid mode", listen: "unix://rpc.sock", options: `{mode: "0780"}`, wantErr: "invalid unix socket mode"},
		{name: "unquoted mode", listen: "unix://rpc.sock", options: "{mode: 0600}", wantErr: "invalid unix socket mode"},
		{name: "empty socket address", listen: "unix://", options: `{mode: "0600"}`, wantErr: "filesystem unix:// address"},
		{name: "negative UID", listen: "unix://rpc.sock", options: "{uid: -1}", wantErr: "invalid unix socket uid"},
		{name: "negative GID", listen: "unix://rpc.sock", options: "{gid: -1}", wantErr: "invalid unix socket gid"},
		{name: "reserved UID", listen: "unix://rpc.sock", options: "{uid: 4294967295}", wantErr: "invalid unix socket uid"},
		{name: "reserved GID", listen: "unix://rpc.sock", options: "{gid: 4294967295}", wantErr: "invalid unix socket gid"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Plugin{Path: unixSocketConfig(t, tc.listen, tc.options)}
			require.NoError(t, cfg.Init())
			log := logger.NewLogger(logger.ChannelConfig{}, slog.New(slog.DiscardHandler))
			p := &rpcPlugin.Plugin{}

			require.ErrorContains(t, p.Init(cfg, log), tc.wantErr)
		})
	}
}

func TestUnixSocketMode(t *testing.T) {
	cases := []struct {
		name  string
		flags []string
		mode  os.FileMode
	}{
		{name: "configured mode", mode: 0o600},
		{name: "mode override", flags: []string{"rpc.unix_socket.mode=0640"}, mode: 0o640},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			path := unixSocketConfig(t, "unix://rpc.sock", `{mode: "0600"}`)
			helpers.Start(t, path, rpcPlugins(), helpers.WithConfigFlags(tc.flags...))

			info, err := os.Stat("rpc.sock")
			require.NoError(t, err)
			require.Equal(t, tc.mode, info.Mode().Perm())
		})
	}
}

func TestServesUnixSocket(t *testing.T) {
	t.Chdir(t.TempDir())
	path := unixSocketConfig(t, "unix://rpc.sock", `{mode: "0600"}`)
	helpers.Start(t, path, rpcPlugins())
	client := helpers.NewRPCClient(t, "unix", "rpc.sock")

	var got string
	require.NoError(t, client.Call("rpc_test.plugin1.Hello", "Valery", &got))
	require.Equal(t, "Hello, username: Valery", got)
}

func TestUnixSocketStopRemovesListener(t *testing.T) {
	t.Chdir(t.TempDir())
	path := unixSocketConfig(t, "unix://rpc.sock", `{mode: "0600"}`)
	stop := helpers.Start(t, path, rpcPlugins())
	require.FileExists(t, "rpc.sock")

	stop()

	require.NoFileExists(t, "rpc.sock")
}

func TestUnixSocketOwnershipErrorRemovesListener(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Requires an unprivileged process.")
	}
	t.Chdir(t.TempDir())
	cfg := &config.Plugin{Path: unixSocketConfig(t, "unix://rpc.sock", "{uid: 0}")}
	require.NoError(t, cfg.Init())
	log := logger.NewLogger(logger.ChannelConfig{}, slog.New(slog.DiscardHandler))
	p := &rpcPlugin.Plugin{}
	require.NoError(t, p.Init(cfg, log))
	t.Cleanup(func() { require.NoError(t, p.Stop(context.Background())) })

	select {
	case err := <-p.Serve():
		require.ErrorContains(t, err, "chown unix socket")
	case <-time.After(5 * time.Second):
		t.Fatal("expected an ownership error")
	}
	require.NoFileExists(t, "rpc.sock")
}

func unixSocketConfig(t *testing.T, listen, options string) string {
	t.Helper()

	contents := fmt.Sprintf(`version: "3"
rpc:
  listen: %q
  unix_socket: %s
`, listen, options)
	path := filepath.Join(t.TempDir(), ".rr.yaml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}
