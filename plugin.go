package rpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	"connectrpc.com/grpcreflect"
	"github.com/roadrunner-server/endure/v2/dep"
	"github.com/roadrunner-server/errors"
)

// PluginName contains default plugin name.
const PluginName = "rpc"

// Plugin is an RPC service.
type Plugin struct {
	cfg Config
	log *slog.Logger
	// set of the plugins, which are implement RPCer interface and can be plugged into the RR via RPC
	plugins   map[string]RPCer
	listener  net.Listener
	server    *http.Server
	closed    atomic.Bool
	rrVersion string

	// whole configuration
	wcfg []byte
}

// RPCer declares the ability to expose a Connect-RPC service. Implementations
// typically delegate to a generated connect.NewXxxServiceHandler(impl).
type RPCer interface {
	// Name of the plugin.
	Name() string
	// RPC returns the URL prefix and HTTP handler this plugin wants mounted on
	// the rpc server's mux.
	RPC() (string, http.Handler)
}

type Configurer interface {
	// RRVersion returns current RR version
	RRVersion() string
	// Unmarshal returns the whole configuration
	Unmarshal(out any) error
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error
	// Has checks if config section exists.
	Has(name string) bool
}

type Logger interface {
	NamedLogger(name string) *slog.Logger
}

// Init rpc service. Must return true if service is enabled.
func (s *Plugin) Init(cfg Configurer, log Logger) error {
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
	s.log = log.NamedLogger(PluginName)

	// validate config
	err = s.cfg.Valid()
	if err != nil {
		return errors.E(op, err)
	}

	var wholeCfg any
	err = cfg.Unmarshal(&wholeCfg)
	if err != nil {
		return errors.E(op, err)
	}

	s.wcfg, err = json.Marshal(wholeCfg)
	if err != nil {
		return err
	}

	s.rrVersion = cfg.RRVersion()

	return nil
}

// Serve serves the service.
func (s *Plugin) Serve() chan error {
	const op = errors.Op("rpc_plugin_serve")
	errCh := make(chan error, 1)

	// register the rpc plugin's own API surface alongside discovered plugins
	s.plugins[PluginName] = s

	mux := http.NewServeMux()
	services := make([]string, 0, len(s.plugins))
	for name, rpcer := range s.plugins {
		path, handler := rpcer.RPC()
		if path == "" || handler == nil {
			s.log.Warn("plugin returned empty rpc handler", "plugin", name)
			continue
		}
		// http.ServeMux.Handle panics on patterns missing a leading slash.
		if !strings.HasPrefix(path, "/") {
			s.log.Warn("plugin rpc handler path must start with '/'", "plugin", name, "path", path)
			continue
		}
		mux.Handle(path, handler)
		// derive the gRPC service name from the mount path
		// (`/<service>/<Method>` or `/<service>/`)
		svc, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
		services = append(services, svc)
	}

	// gRPC server reflection so operators can list services with grpcurl
	if len(services) > 0 {
		reflector := grpcreflect.NewStaticReflector(services...)
		rpath, rhandler := grpcreflect.NewHandlerV1(reflector)
		mux.Handle(rpath, rhandler)
		rpath, rhandler = grpcreflect.NewHandlerV1Alpha(reflector)
		mux.Handle(rpath, rhandler)
	}

	listener, err := s.cfg.Listener()
	if err != nil {
		errCh <- errors.E(op, err)
		return errCh
	}
	s.listener = listener

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	s.server = &http.Server{
		Handler:           mux,
		Protocols:         protocols,
		ReadHeaderTimeout: s.cfg.RequestTimeout,
		ReadTimeout:       s.cfg.RequestTimeout,
	}

	useTLS := s.cfg.TLS != nil
	if useTLS {
		cert, err := tls.LoadX509KeyPair(s.cfg.TLS.Cert, s.cfg.TLS.Key)
		if err != nil {
			_ = s.listener.Close()
			errCh <- errors.E(op, err)
			return errCh
		}
		tlsProto := new(http.Protocols)
		tlsProto.SetHTTP1(true)
		tlsProto.SetHTTP2(true)
		s.server.Protocols = tlsProto
		s.server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"h2", "http/1.1"},
		}
	}

	s.log.Debug("plugin was started",
		"address", s.cfg.Listen,
		"tls", useTLS,
		"services", services,
	)

	go func() {
		var serveErr error
		if useTLS {
			serveErr = s.server.ServeTLS(s.listener, "", "")
		} else {
			serveErr = s.server.Serve(s.listener)
		}
		if serveErr != nil && !stderrors.Is(serveErr, http.ErrServerClosed) && !s.closed.Load() {
			errCh <- errors.E(op, serveErr)
		}
	}()

	return errCh
}

// Stop stops the service.
func (s *Plugin) Stop(ctx context.Context) error {
	const op = errors.Op("rpc_plugin_stop")
	s.closed.Store(true)
	if s.server == nil {
		return nil
	}
	if err := s.server.Shutdown(ctx); err != nil {
		return errors.E(op, err)
	}
	return nil
}

func (s *Plugin) Weight() uint {
	return 100
}

// Name contains service name.
func (s *Plugin) Name() string {
	return PluginName
}

// RPC exposes the rpc plugin's own API surface (Config, Version) so it is
// served alongside collected plugins.
func (s *Plugin) RPC() (string, http.Handler) {
	return newSelfHandlers(s.wcfg, s.rrVersion)
}

// Collects all plugins which implement Name + RPCer interfaces
func (s *Plugin) Collects() []*dep.In {
	return []*dep.In{
		dep.Fits(func(p any) {
			rpcer := p.(RPCer)
			s.plugins[rpcer.Name()] = rpcer
		}, (*RPCer)(nil)),
	}
}
