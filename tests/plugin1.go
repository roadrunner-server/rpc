package rpc

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const plugin1HelloPath = "/rpc_test.plugin1/Hello"

type Plugin1 struct {
	config Configurer
}

type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error
	// Has checks if a config section exists.
	Has(name string) bool
}

func (p1 *Plugin1) Init(cfg Configurer) error {
	p1.config = cfg
	return nil
}

func (p1 *Plugin1) Serve() chan error {
	return make(chan error, 1)
}

func (p1 *Plugin1) Stop(context.Context) error {
	return nil
}

func (p1 *Plugin1) Name() string {
	return "rpc_test.plugin1"
}

func (p1 *Plugin1) RPC() (string, http.Handler) {
	return plugin1HelloPath, connect.NewUnaryHandler(
		plugin1HelloPath,
		func(_ context.Context, req *connect.Request[wrapperspb.StringValue]) (*connect.Response[wrapperspb.StringValue], error) {
			return connect.NewResponse(&wrapperspb.StringValue{Value: "Hello, username: " + req.Msg.GetValue()}), nil
		},
	)
}
