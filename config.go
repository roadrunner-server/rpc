package rpc

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/roadrunner-server/tcplisten"
)

const defaultRequestTimeout = 30 * time.Second

// TLS holds optional TLS material for the rpc listener.
type TLS struct {
	Cert string `mapstructure:"cert"`
	Key  string `mapstructure:"key"`
}

// Config defines RPC service config.
type Config struct {
	// Listen - address string (tcp://host:port or unix://file.sock).
	Listen string `mapstructure:"listen"`
	// RequestTimeout caps the read phases of an RPC request (header + body
	// reads). Handler execution itself is bounded per-call by the request's
	// context deadline. Streaming RPCs are not bounded by this value.
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	// TLS material; when set, both Cert and Key are required.
	TLS *TLS `mapstructure:"tls"`
}

// InitDefaults allows init blank config with a pre-defined set of default values.
func (c *Config) InitDefaults() {
	if c.Listen == "" {
		c.Listen = "tcp://127.0.0.1:6001"
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = defaultRequestTimeout
	}
}

// Valid returns nil if config is valid.
func (c *Config) Valid() error {
	if dsn := strings.Split(c.Listen, "://"); len(dsn) != 2 {
		return errors.New("invalid socket DSN (tcp://:6001, unix://file.sock)")
	}
	if c.RequestTimeout < 0 {
		return errors.New("rpc request_timeout must be non-negative")
	}
	if c.TLS != nil {
		if c.TLS.Cert == "" || c.TLS.Key == "" {
			return errors.New("rpc tls config: both cert and key must be provided")
		}
	}
	return nil
}

// Listener creates new rpc socket Listener.
func (c *Config) Listener() (net.Listener, error) {
	return tcplisten.CreateListener(c.Listen)
}

// Dialer creates rpc socket Dialer.
func (c *Config) Dialer() (net.Conn, error) {
	dsn := strings.Split(c.Listen, "://")
	if len(dsn) != 2 {
		return nil, errors.New("invalid socket DSN (tcp://:6001, unix://file.sock)")
	}
	var d net.Dialer
	return d.DialContext(context.Background(), dsn[0], dsn[1])
}
