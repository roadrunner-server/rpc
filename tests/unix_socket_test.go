//go:build linux || darwin || freebsd

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	netrpc "net/rpc"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/endure/v2"
	"github.com/roadrunner-server/errors"
	goridgeRPC "github.com/roadrunner-server/goridge/v4/pkg/rpc"
	"github.com/roadrunner-server/logger/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/stretchr/testify/require"
)

func TestUnixSocketConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	log := logger.NewLogger(logger.ChannelConfig{}, slog.New(slog.DiscardHandler))

	for _, tc := range []struct {
		name    string
		listen  string
		options string
		flags   []string
		mode    string
		zeroIDs bool
		wantErr string
	}{
		{name: "omitted", listen: "tcp://127.0.0.1:0"},
		{name: "null", listen: "tcp://127.0.0.1:0", options: "null"},
		{name: "quoted mode", listen: "unix://rpc.sock", options: `{mode: "0000"}`, mode: "0000"},
		{name: "zero IDs", listen: "unix://rpc.sock", options: "{uid: 0, gid: 0}", zeroIDs: true},
		{name: "string overrides", listen: "unix://rpc.sock", options: `{mode: "0600"}`, flags: []string{"rpc.unix_socket.mode=0640", "rpc.unix_socket.uid=0", "rpc.unix_socket.gid=0"}, mode: "0640", zeroIDs: true},
		{name: "TCP options", listen: "tcp://127.0.0.1:0", options: `{mode: "0600"}`, wantErr: "rpc.unix_socket"},
		{name: "empty TCP options", listen: "tcp://127.0.0.1:0", options: "{}", wantErr: "rpc.unix_socket"},
		{name: "invalid mode", listen: "unix://rpc.sock", options: `{mode: "0780"}`, wantErr: "rpc.unix_socket"},
		{name: "unquoted mode", listen: "unix://rpc.sock", options: "{mode: 0600}", wantErr: "rpc.unix_socket"},
		{name: "default address", options: "{}", wantErr: "rpc.unix_socket"},
		{name: "empty socket address", listen: "unix://", options: "{}", wantErr: "rpc.unix_socket"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := unixSocketConfig(t, tc.listen, tc.options, tc.flags)
			var decoded rpcPlugin.Config
			require.NoError(t, cfg.UnmarshalKey("rpc", &decoded))
			decoded.InitDefaults()

			p := &rpcPlugin.Plugin{}
			err := p.Init(cfg, log)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.NoFileExists(t, "rpc.sock")
				return
			}
			require.NoError(t, err)
			require.NoError(t, decoded.Valid())
			require.Equal(t, tc.listen, decoded.Listen)
			if tc.options == "" || tc.options == "null" {
				require.Nil(t, decoded.UnixSocket)
				ln, err := decoded.Listener()
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, ln.Close()) })
				require.Equal(t, "tcp", ln.Addr().Network())
				return
			}
			require.NotNil(t, decoded.UnixSocket)
			require.Equal(t, tc.mode, decoded.UnixSocket.Mode)
			if tc.zeroIDs {
				require.NotNil(t, decoded.UnixSocket.UID)
				require.NotNil(t, decoded.UnixSocket.GID)
				require.Zero(t, *decoded.UnixSocket.UID)
				require.Zero(t, *decoded.UnixSocket.GID)
			} else {
				require.Nil(t, decoded.UnixSocket.UID)
				require.Nil(t, decoded.UnixSocket.GID)
			}
		})
	}
}

func TestUnixSocketIDs(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("RR_TEST_UNIX_SOCKET_ID", "33")
	t.Setenv("RR_TEST_UNIX_SOCKET_UNSET_ID", "")
	require.NoError(t, os.Unsetenv("RR_TEST_UNIX_SOCKET_UNSET_ID"))
	log := logger.NewLogger(logger.ChannelConfig{}, slog.New(slog.DiscardHandler))

	for _, field := range []string{"uid", "gid"} {
		for _, tc := range []struct {
			name  string
			value string
			valid bool
			want  int
			json  bool
		}{
			{name: "false", value: "false"},
			{name: "true", value: "true"},
			{name: "fraction", value: "1.9"},
			{name: "negative fraction", value: "-0.5"},
			{name: "empty string", value: `""`},
			{name: "unset environment", value: `"${RR_TEST_UNIX_SOCKET_UNSET_ID}"`},
			{name: "negative ID", value: "-1"},
			{name: "oversized ID", value: "4294967295"},
			{name: "oversized float", value: "4294967295.0"},
			{name: "oversized unsigned ID", value: "18446744073709551615"},
			{name: "oversized string ID", value: `"4294967295"`},
			{name: "string overflow", value: `"18446744073709551616"`},
			{name: "NaN", value: ".nan"},
			{name: "infinity", value: ".inf"},
			{name: "map", value: "{id: 33}"},
			{name: "slice", value: "[33]"},
			{name: "JSON fraction", value: "1.9", json: true},
			{name: "zero", value: "0", valid: true},
			{name: "null", value: "null", valid: true},
			{name: "integer", value: "33", valid: true, want: 33},
			{name: "integer float", value: "33.0", valid: true, want: 33},
			{name: "JSON integer float", value: "33.0", valid: true, want: 33, json: true},
			{name: "environment", value: `"${RR_TEST_UNIX_SOCKET_ID}"`, valid: true, want: 33},
			{name: "octal string", value: `"041"`, valid: true, want: 33},
			{name: "hexadecimal string", value: `"0x21"`, valid: true, want: 33},
		} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				var cfg *config.Plugin
				if tc.json {
					path := filepath.Join(t.TempDir(), ".rr.json")
					contents := fmt.Sprintf(`{"version":"3","rpc":{"listen":"unix://rpc.sock","unix_socket":{"mode":"0600",%q:%s}}}`, field, tc.value)
					require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
					cfg = &config.Plugin{Path: path}
					require.NoError(t, cfg.Init())
				} else {
					options := fmt.Sprintf(`{mode: "0600", %s: %s}`, field, tc.value)
					cfg = unixSocketConfig(t, "unix://rpc.sock", options, nil)
				}
				p := &rpcPlugin.Plugin{}
				err := p.Init(cfg, log)
				require.NoFileExists(t, "rpc.sock")
				if !tc.valid {
					require.False(t, errors.Is(errors.Disabled, err), "invalid socket IDs must fail initialization")
					require.ErrorContains(t, err, "rpc.unix_socket."+field)

					logs := &logger.Plugin{}
					t.Cleanup(func() { require.NoError(t, logs.Stop(context.Background())) })
					cont := endure.New(slog.LevelError)
					require.NoError(t, cont.RegisterAll(&config.Plugin{Path: cfg.Path, Flags: cfg.Flags}, logs, &rpcPlugin.Plugin{}))
					err = cont.Init()
					require.ErrorContains(t, err, "rpc.unix_socket."+field)
					require.False(t, errors.Is(errors.Disabled, err))
					require.NoFileExists(t, "rpc.sock")
					return
				}
				require.NoError(t, err)
				var decoded rpcPlugin.Config
				require.NoError(t, cfg.UnmarshalKey("rpc", &decoded))
				require.NotNil(t, decoded.UnixSocket)
				id, other := decoded.UnixSocket.UID, decoded.UnixSocket.GID
				if field == "gid" {
					id, other = other, id
				}
				require.Nil(t, other)
				if tc.value == "null" {
					require.Nil(t, id)
				} else {
					require.NotNil(t, id)
					require.Equal(t, tc.want, *id)
				}
			})
		}
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
			options := fmt.Sprintf(`{mode: "0600", uid: %d, gid: %d}`, os.Getuid(), os.Getgid())
			cfg := unixSocketConfig(t, "unix://rpc.sock", options, tc.flags)
			p := &rpcPlugin.Plugin{}
			log := logger.NewLogger(logger.ChannelConfig{}, slog.New(slog.DiscardHandler))
			require.NoError(t, p.Init(cfg, log))
			t.Cleanup(func() { require.NoError(t, p.Stop(context.Background())) })
			errCh := p.Serve()
			select {
			case err := <-errCh:
				t.Fatalf("RPC serve: %v", err)
			default:
			}

			info, err := os.Stat("rpc.sock")
			require.NoError(t, err)
			require.Equal(t, tc.mode, info.Mode().Perm())
			stat, ok := info.Sys().(*syscall.Stat_t)
			require.True(t, ok)
			require.EqualValues(t, os.Getuid(), stat.Uid)
			require.EqualValues(t, os.Getgid(), stat.Gid)

			var decoded rpcPlugin.Config
			require.NoError(t, cfg.UnmarshalKey("rpc", &decoded))
			require.Equal(t, "unix://rpc.sock", decoded.Listen)
			conn, err := decoded.Dialer()
			require.NoError(t, err)
			client := netrpc.NewClientWithCodec(goridgeRPC.NewClientCodec(conn))
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
			require.Equal(t, "unix", conn.RemoteAddr().Network())
			require.Equal(t, "rpc.sock", conn.RemoteAddr().String())

			var version string
			require.NoError(t, client.Call("rpc.Version", false, &version))
			require.Equal(t, cfg.RRVersion(), version)
			var raw []byte
			require.NoError(t, client.Call("rpc.Config", false, &raw))
			var whole struct {
				RPC struct{ Listen string }
			}
			require.NoError(t, json.Unmarshal(raw, &whole))
			require.Equal(t, decoded.Listen, whole.RPC.Listen)
		})
	}
}

func unixSocketConfig(t *testing.T, listen, options string, flags []string) *config.Plugin {
	t.Helper()
	contents := fmt.Sprintf("version: \"3\"\nrpc:\n  listen: %q\n", listen)
	if options != "" {
		contents += "  unix_socket: " + options + "\n"
	}
	path := filepath.Join(t.TempDir(), ".rr.yaml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	cfg := &config.Plugin{Path: path, Flags: flags}
	require.NoError(t, cfg.Init())
	return cfg
}
