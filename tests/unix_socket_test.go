//go:build linux || darwin || freebsd

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	netrpc "net/rpc"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/errors"
	goridgeRPC "github.com/roadrunner-server/goridge/v4/pkg/rpc"
	"github.com/roadrunner-server/logger/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/stretchr/testify/require"
)

func TestUnixSocketConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		listen  string
		options string
		wantErr string
	}{
		{name: "omitted", listen: "tcp://127.0.0.1:0"},
		{name: "null", listen: "tcp://127.0.0.1:0", options: "null"},
		{name: "empty TCP options", listen: "tcp://127.0.0.1:0", options: "{}"},
		{name: "empty UNIX options", listen: "unix://rpc.sock", options: "{}"},
		{name: "default address", options: "{}"},
		{name: "quoted mode", listen: "unix://rpc.sock", options: `{mode: "0000"}`},
		{name: "zero IDs", listen: "unix://rpc.sock", options: "{uid: 0, gid: 0}"},
		{name: "TCP options", listen: "tcp://127.0.0.1:0", options: `{mode: "0600"}`, wantErr: "filesystem unix:// address"},
		{name: "invalid mode", listen: "unix://rpc.sock", options: `{mode: "0780"}`, wantErr: "invalid unix socket mode"},
		{name: "unquoted mode", listen: "unix://rpc.sock", options: "{mode: 0600}", wantErr: "invalid unix socket mode"},
		{name: "empty socket address", listen: "unix://", options: `{mode: "0600"}`, wantErr: "filesystem unix:// address"},
		{name: "negative UID", listen: "unix://rpc.sock", options: "{uid: -1}", wantErr: "invalid unix socket uid"},
		{name: "negative GID", listen: "unix://rpc.sock", options: "{gid: -1}", wantErr: "invalid unix socket gid"},
		{name: "reserved UID", listen: "unix://rpc.sock", options: "{uid: 4294967295}", wantErr: "invalid unix socket uid"},
		{name: "reserved GID", listen: "unix://rpc.sock", options: "{gid: 4294967295}", wantErr: "invalid unix socket gid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := unixSocketConfig(t, tc.listen, tc.options, nil)
			log := logger.NewLogger(logger.ChannelConfig{}, slog.New(slog.DiscardHandler))
			p := &rpcPlugin.Plugin{}
			err := p.Init(cfg, log)
			if tc.wantErr != "" {
				require.False(t, errors.Is(errors.Disabled, err))
				require.ErrorContains(t, err, "rpc.unix_socket")
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestUnixSocketListener(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		mode  os.FileMode
	}{
		{name: "quoted mode", mode: 0o600},
		{name: "string override", flags: []string{"rpc.unix_socket.mode=0640"}, mode: 0o640},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("RR_TEST_SOCKET_UID", strconv.Itoa(os.Getuid()))
			t.Setenv("RR_TEST_SOCKET_GID", strconv.Itoa(os.Getgid()))
			const options = `{mode: "0600", uid: "${RR_TEST_SOCKET_UID}", gid: "${RR_TEST_SOCKET_GID}"}`
			cfg := unixSocketConfig(t, "unix://rpc.sock", options, tc.flags)
			p := &rpcPlugin.Plugin{}
			log := logger.NewLogger(logger.ChannelConfig{}, slog.New(slog.DiscardHandler))
			require.NoError(t, p.Init(cfg, log))
			stop := sync.OnceValue(func() error { return p.Stop(context.Background()) })
			t.Cleanup(func() { require.NoError(t, stop()) })
			errCh := p.Serve()
			select {
			case err := <-errCh:
				t.Fatalf("RPC serve: %v", err)
			default:
			}

			var d net.Dialer
			conn, err := d.DialContext(t.Context(), "unix", "rpc.sock")
			require.NoError(t, err)
			client := netrpc.NewClientWithCodec(goridgeRPC.NewClientCodec(conn))
			t.Cleanup(func() { _ = client.Close() })
			require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))

			var version string
			require.NoError(t, client.Call("rpc.Version", false, &version))
			require.Equal(t, cfg.RRVersion(), version)
			var raw []byte
			require.NoError(t, client.Call("rpc.Config", false, &raw))
			var whole struct {
				RPC struct{ Listen string }
			}
			require.NoError(t, json.Unmarshal(raw, &whole))
			require.Equal(t, "unix://rpc.sock", whole.RPC.Listen)

			info, err := os.Stat("rpc.sock")
			require.NoError(t, err)
			require.NotZero(t, info.Mode()&os.ModeSocket)
			require.Equal(t, tc.mode, info.Mode().Perm())
			stat := info.Sys().(*syscall.Stat_t)
			require.EqualValues(t, os.Getuid(), stat.Uid)
			require.EqualValues(t, os.Getgid(), stat.Gid)

			require.NoError(t, client.Close())
			require.NoError(t, stop())
			require.NoFileExists(t, "rpc.sock")
		})
	}
}

func TestUnixSocketOwnershipError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Requires an unprivileged process.")
	}
	groups, err := os.Getgroups()
	require.NoError(t, err)
	otherGID := 0
	for otherGID == os.Getegid() || slices.Contains(groups, otherGID) {
		otherGID++
	}

	for _, tc := range []struct {
		field string
		id    int
	}{
		{field: "uid", id: 0},
		{field: "gid", id: otherGID},
	} {
		t.Run(tc.field, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("RR_TEST_SOCKET_ID", strconv.Itoa(tc.id))
			options := fmt.Sprintf(`{%s: "${RR_TEST_SOCKET_ID}"}`, tc.field)
			cfg := unixSocketConfig(t, "unix://rpc.sock", options, nil)
			p := &rpcPlugin.Plugin{}
			log := logger.NewLogger(logger.ChannelConfig{}, slog.New(slog.DiscardHandler))
			require.NoError(t, p.Init(cfg, log))
			t.Cleanup(func() { require.NoError(t, p.Stop(context.Background())) })
			select {
			case err := <-p.Serve():
				require.ErrorContains(t, err, "chown unix socket")
			case <-time.After(5 * time.Second):
				t.Fatal("expected an ownership error")
			}
			require.NoFileExists(t, "rpc.sock")
		})
	}
}

func unixSocketConfig(t *testing.T, listen, options string, flags []string) *config.Plugin {
	t.Helper()
	contents := fmt.Sprintf(`version: "3"
rpc:
  listen: %q
`, listen)
	if options != "" {
		contents += "  unix_socket: " + options + "\n"
	}
	path := filepath.Join(t.TempDir(), ".rr.yaml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	cfg := &config.Plugin{Path: path, Flags: flags}
	require.NoError(t, cfg.Init())
	return cfg
}
