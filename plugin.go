package rpc

import (
	"net"
	"net/rpc"
	"sync/atomic"

	endure "github.com/roadrunner-server/endure/pkg/container"
	"github.com/roadrunner-server/errors"
	goridgeRpc "github.com/roadrunner-server/goridge/v3/pkg/rpc"
	"go.uber.org/zap"
)

// PluginName contains default plugin name.
const PluginName = "rpc"

// Plugin is RPC service.
type Plugin struct {
	cfg Config
	log *zap.Logger
	rpc *rpc.Server
	// set of the plugins, which are implement RPCer interface and can be plugged into the RR via RPC
	plugins  map[string]RPCer
	listener net.Listener
	closed   uint32
}

// RPCer declares the ability to create set of public RPC methods.
type RPCer interface {
	// RPC Provides methods for the given service.
	RPC() any
}

type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error

	// Has checks if config section exists.
	Has(name string) bool
}

// Init rpc service. Must return true if service is enabled.
func (s *Plugin) Init(cfg Configurer, log *zap.Logger) error {
	const op = errors.Op("rpc_plugin_init")

	if !cfg.Has(PluginName) {
		return errors.E(op, errors.Disabled)
	}

	err := cfg.UnmarshalKey(PluginName, &s.cfg)
	if err != nil {
		return errors.E(op, errors.Disabled, err)
	}

	// Init defaults
	s.cfg.InitDefaults()
	// Init pluggable plugins map
	s.plugins = make(map[string]RPCer, 1)
	// init logs
	s.log = log

	// set up state
	atomic.StoreUint32(&s.closed, 0)

	// validate config
	err = s.cfg.Valid()
	if err != nil {
		return errors.E(op, err)
	}
	return nil
}

// Serve serves the service.
func (s *Plugin) Serve() chan error {
	const op = errors.Op("rpc_plugin_serve")
	errCh := make(chan error, 1)

	s.rpc = rpc.NewServer()

	plugins := make([]string, 0, len(s.plugins))

	// Attach all services
	for name := range s.plugins {
		err := s.Register(name, s.plugins[name].RPC())
		if err != nil {
			errCh <- errors.E(op, err)
			return errCh
		}

		plugins = append(plugins, name)
	}

	var err error
	s.listener, err = s.cfg.Listener()
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}

	s.log.Debug("plugin was started", zap.String("address", s.cfg.Listen), zap.Strings("list of the plugins with RPC methods:", plugins))

	go func() {
		for {
			conn, errA := s.listener.Accept()
			if errA != nil {
				if atomic.LoadUint32(&s.closed) == 1 {
					// just continue, this is not a critical issue, we just called Stop
					return
				}

				s.log.Error("failed to accept the connection", zap.Error(errA))
				continue
			}

			go s.rpc.ServeCodec(goridgeRpc.NewCodec(conn))
		}
	}()

	return errCh
}

// Stop stops the service.
func (s *Plugin) Stop() error {
	const op = errors.Op("rpc_plugin_stop")
	// store closed state
	atomic.StoreUint32(&s.closed, 1)
	err := s.listener.Close()
	if err != nil {
		return errors.E(op, err)
	}
	return nil
}

// Name contains service name.
func (s *Plugin) Name() string {
	return PluginName
}

// Collects all plugins which implement Name + RPCer interfaces
func (s *Plugin) Collects() []any {
	return []any{
		s.RegisterPlugin,
	}
}

// RegisterPlugin registers RPC service plugin.
func (s *Plugin) RegisterPlugin(name endure.Named, p RPCer) {
	s.plugins[name.Name()] = p
}

// Register publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//   - exported method of exported type
//   - two arguments, both of exported type
//   - the second argument is a pointer
//   - one return value, of type error
//
// It returns an error if the receiver is not an exported type or has
// no suitable methods. It also logs the error using package log.
func (s *Plugin) Register(name string, svc any) error {
	if s.rpc == nil {
		return errors.E("RPC service is not configured")
	}

	return s.rpc.RegisterName(name, svc)
}
