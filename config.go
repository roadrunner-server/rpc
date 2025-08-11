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
	// Listen - address string
	Listen string `mapstructure:"listen"`
}

// InitDefaults allows init blank config with a pre-defined set of default values.
func (c *Config) InitDefaults() {
	if c.Listen == "" {
		c.Listen = "tcp://127.0.0.1:6001"
	}
}

// Valid returns nil if config is valid.
func (c *Config) Valid() error {
	if dsn := strings.Split(c.Listen, "://"); len(dsn) != 2 {
		return errors.New("invalid socket DSN (tcp://:6001, unix://file.sock)")
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
