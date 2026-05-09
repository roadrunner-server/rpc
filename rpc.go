package rpc

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// newSelfHandlers builds the rpc plugin's own /rpc/ surface (Config, Version)
// as Connect unary handlers grouped under one sub-mux. Both procedures take
// emptypb.Empty and return a wrapper proto so the wire format stays
// well-formed protobuf without needing a generated service stub.
func newSelfHandlers(cfg []byte, version string) (string, http.Handler) {
	sub := http.NewServeMux()

	sub.Handle("/rpc/Config", connect.NewUnaryHandler(
		"/rpc/Config",
		func(_ context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[wrapperspb.BytesValue], error) {
			return connect.NewResponse(&wrapperspb.BytesValue{Value: cfg}), nil
		},
	))
	sub.Handle("/rpc/Version", connect.NewUnaryHandler(
		"/rpc/Version",
		func(_ context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[wrapperspb.StringValue], error) {
			return connect.NewResponse(&wrapperspb.StringValue{Value: version}), nil
		},
	))

	return "/rpc/", sub
}
