package rpc

import (
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// API is the rpc plugin's own RPC surface, exposing runtime configuration and
// the host RoadRunner version.
type API struct {
	cfg     []byte
	version string
}

// Config returns the whole RoadRunner configuration as JSON-encoded bytes.
func (a *API) Config(_ *emptypb.Empty, out *wrapperspb.BytesValue) error {
	out.Value = a.cfg
	return nil
}

// Version returns the RoadRunner version string.
func (a *API) Version(_ *emptypb.Empty, out *wrapperspb.StringValue) error {
	out.Value = a.version
	return nil
}
