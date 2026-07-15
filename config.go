package rpc

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/roadrunner-server/tcplisten"
)

// Config defines RPC service config.
type Config struct {
	// Listen - address string (tcp://host:port or unix://file.sock).
	Listen string `mapstructure:"listen"`
}

// InitDefaults allows init blank config with a pre-defined set of default values.
func (c *Config) InitDefaults() {
	if c.Listen == "" {
		c.Listen = "tcp://127.0.0.1:6001"
	}
}

// dsn is a parsed "scheme://address" RPC listen string.
type dsn struct {
	scheme string
	addr   string
}

// parseDSN splits a "scheme://address" listen string into its scheme and
// address. It errors unless the string contains exactly one "://" separator.
func parseDSN(listen string) (dsn, error) {
	scheme, addr, ok := strings.Cut(listen, "://")
	if !ok || strings.Contains(addr, "://") {
		return dsn{}, errors.New("invalid socket DSN (tcp://:6001, unix://file.sock)")
	}
	return dsn{scheme: scheme, addr: addr}, nil
}

// Valid returns nil if config is valid.
func (c *Config) Valid() error {
	_, err := parseDSN(c.Listen)
	return err
}

// Listener creates new rpc socket Listener.
func (c *Config) Listener() (net.Listener, error) {
	return tcplisten.CreateListener(c.Listen)
}

// Dialer creates rpc socket Dialer.
func (c *Config) Dialer() (net.Conn, error) {
	parsed, err := parseDSN(c.Listen)
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	return d.DialContext(context.Background(), parsed.scheme, parsed.addr)
}
