package rpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/roadrunner-server/tcplisten"
)

// Config defines RPC service config.
type Config struct {
	// Listen - address string (tcp://host:port or unix://file.sock).
	Listen     string                       `mapstructure:"listen"`
	UnixSocket *tcplisten.UnixSocketOptions `mapstructure:"unix_socket"`
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
	if err != nil {
		return err
	}
	if err = c.UnixSocket.Validate(c.Listen); err != nil {
		return fmt.Errorf("rpc.unix_socket: %w", err)
	}
	return nil
}

// Listener creates new rpc socket Listener.
func (c *Config) Listener() (net.Listener, error) {
	return tcplisten.CreateListenerWithOptions(c.Listen, c.UnixSocket)
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

// validateUnixSocketIDs rejects values that weak decoding can convert to valid IDs.
func validateUnixSocketIDs(cfg Configurer) error {
	const key = PluginName + ".unix_socket"
	var options map[string]any
	if err := cfg.UnmarshalKey(key, &options); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	for _, field := range []string{"uid", "gid"} {
		if options[field] == nil {
			continue
		}
		value := reflect.ValueOf(options[field])
		valid := false
		switch {
		case value.CanInt():
			id := value.Int()
			valid = id >= 0 && id < math.MaxUint32
		case value.CanUint():
			valid = value.Uint() < math.MaxUint32
		case value.Kind() == reflect.String:
			id, err := strconv.ParseInt(value.String(), 0, strconv.IntSize)
			valid = err == nil && id >= 0 && id < math.MaxUint32
		case value.CanFloat():
			id := value.Float()
			valid = id >= 0 && id < math.MaxUint32 && math.Trunc(id) == id
		}
		if !valid {
			return fmt.Errorf("%s.%s: must be an integer between 0 and 4294967294", key, field)
		}
	}
	return nil
}
