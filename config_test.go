package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Listener(t *testing.T) {
	cfg := &Config{Listen: "tcp://:18001"}

	ln, err := cfg.Listener()
	assert.NoError(t, err)
	assert.NotNil(t, ln)
	defer func() {
		err := ln.Close()
		if err != nil {
			t.Errorf("error closing the listener: error %v", err)
		}
	}()

	assert.Equal(t, "tcp", ln.Addr().Network())
	assert.Equal(t, "0.0.0.0:18001", ln.Addr().String())
}

func TestConfig_Listener2(t *testing.T) {
	cfg := &Config{Listen: ":18001"}

	ln, err := cfg.Listener()
	assert.NoError(t, err)
	assert.NotNil(t, ln)
	defer func() {
		err := ln.Close()
		if err != nil {
			t.Errorf("error closing the listener: error %v", err)
		}
	}()

	assert.Equal(t, "tcp", ln.Addr().Network())
	assert.Equal(t, "0.0.0.0:18001", ln.Addr().String())
}

func TestConfig_ListenerIPV6(t *testing.T) {
	cfg := &Config{Listen: "tcp://[::]:18001"}

	ln, err := cfg.Listener()
	assert.NoError(t, err)
	assert.NotNil(t, ln)
	defer func() {
		err := ln.Close()
		if err != nil {
			t.Errorf("error closing the listener: error %v", err)
		}
	}()

	assert.Equal(t, "tcp", ln.Addr().Network())
	assert.Equal(t, "[::]:18001", ln.Addr().String())
}

func TestConfig_ListenerUnix(t *testing.T) {
	cfg := &Config{Listen: "unix://file.sock"}

	ln, err := cfg.Listener()
	assert.NoError(t, err)
	assert.NotNil(t, ln)
	defer func() {
		err := ln.Close()
		if err != nil {
			t.Errorf("error closing the listener: error %v", err)
		}
	}()

	assert.Equal(t, "unix", ln.Addr().Network())
	assert.Equal(t, "file.sock", ln.Addr().String())
}

func Test_Config_Error(t *testing.T) {
	cfg := &Config{Listen: "uni:unix.sock"}
	ln, err := cfg.Listener()
	assert.Nil(t, ln)
	assert.Error(t, err)
}

func Test_Config_ErrorMethod(t *testing.T) {
	cfg := &Config{Listen: "xinu://unix.sock"}

	ln, err := cfg.Listener()
	assert.Nil(t, ln)
	assert.Error(t, err)
}

func TestConfig_Dialer(t *testing.T) {
	cfg := &Config{Listen: "tcp://:18001"}

	ln, _ := cfg.Listener()
	defer func() {
		err := ln.Close()
		if err != nil {
			t.Errorf("error closing the listener: error %v", err)
		}
	}()

	conn, err := cfg.Dialer()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	defer func() {
		err := conn.Close()
		if err != nil {
			t.Errorf("error closing the connection: error %v", err)
		}
	}()

	assert.Equal(t, "tcp", conn.RemoteAddr().Network())
	assert.Equal(t, "127.0.0.1:18001", conn.RemoteAddr().String())
}

func TestConfig_DialerUnix(t *testing.T) {
	cfg := &Config{Listen: "unix://file.sock"}

	ln, _ := cfg.Listener()
	defer func() {
		err := ln.Close()
		if err != nil {
			t.Errorf("error closing the listener: error %v", err)
		}
	}()

	conn, err := cfg.Dialer()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	defer func() {
		err := conn.Close()
		if err != nil {
			t.Errorf("error closing the connection: error %v", err)
		}
	}()

	assert.Equal(t, "unix", conn.RemoteAddr().Network())
	assert.Equal(t, "file.sock", conn.RemoteAddr().String())
}

func Test_Config_DialerError(t *testing.T) {
	cfg := &Config{Listen: "uni:unix.sock"}
	ln, err := cfg.Dialer()
	assert.Nil(t, ln)
	assert.Error(t, err)
	assert.Equal(t, "invalid socket DSN (tcp://:6001, unix://file.sock)", err.Error())
}

func Test_Config_DialerErrorMethod(t *testing.T) {
	cfg := &Config{Listen: "xinu://unix.sock"}

	ln, err := cfg.Dialer()
	assert.Nil(t, ln)
	assert.Error(t, err)
}

func Test_Config_MultipleSeparators(t *testing.T) {
	// A DSN with more than one "://" must be rejected by both Valid and Dialer.
	cfg := &Config{Listen: "tcp://host://6001"}

	assert.Error(t, cfg.Valid())

	conn, err := cfg.Dialer()
	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.Equal(t, "invalid socket DSN (tcp://:6001, unix://file.sock)", err.Error())
}

func Test_Config_Defaults(t *testing.T) {
	c := &Config{}
	c.InitDefaults()
	assert.Equal(t, "tcp://127.0.0.1:6001", c.Listen)
}

func TestParseDSN(t *testing.T) {
	cases := []struct {
		name       string
		listen     string
		wantScheme string
		wantAddr   string
		wantErr    bool
	}{
		{name: "tcp with host", listen: "tcp://127.0.0.1:6001", wantScheme: "tcp", wantAddr: "127.0.0.1:6001"},
		{name: "tcp without host", listen: "tcp://:6001", wantScheme: "tcp", wantAddr: ":6001"},
		{name: "unix socket", listen: "unix://rpc.sock", wantScheme: "unix", wantAddr: "rpc.sock"},
		{name: "ipv6", listen: "tcp://[::1]:6001", wantScheme: "tcp", wantAddr: "[::1]:6001"},
		{name: "no separator", listen: "127.0.0.1:6001", wantErr: true},
		{name: "empty", listen: "", wantErr: true},
		{name: "two separators", listen: "tcp://unix://rpc.sock", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDSN(tc.listen)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.wantScheme, got.scheme)
			assert.Equal(t, tc.wantAddr, got.addr)
		})
	}
}

func TestInitDefaults(t *testing.T) {
	blank := &Config{}
	blank.InitDefaults()
	assert.Equal(t, "tcp://127.0.0.1:6001", blank.Listen)

	custom := &Config{Listen: "unix://custom.sock"}
	custom.InitDefaults()
	assert.Equal(t, "unix://custom.sock", custom.Listen)
}
