package rpc

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/roadrunner-server/errors"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Plugin2 makes a call to the plugin1 via Connect-RPC over HTTP/1.1.
// This simulates an external client; production clients normally use the
// generated Connect stubs.
type Plugin2 struct{}

func (p2 *Plugin2) Init() error {
	return nil
}

func (p2 *Plugin2) Serve() chan error {
	errCh := make(chan error, 1)

	go func() {
		time.Sleep(time.Second * 3)

		client := connect.NewClient[wrapperspb.StringValue, wrapperspb.StringValue](
			http.DefaultClient,
			"http://127.0.0.1:6001/rpc_test.plugin1/Hello",
		)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := client.CallUnary(ctx, connect.NewRequest(&wrapperspb.StringValue{Value: "Valery"}))
		if err != nil {
			errCh <- errors.E(errors.Serve, err)
			return
		}
		if resp.Msg.GetValue() != "Hello, username: Valery" {
			errCh <- errors.E("wrong response")
			return
		}
		// signal end of test
		errCh <- errors.Str("test error")
	}()

	return errCh
}

func (p2 *Plugin2) Stop(context.Context) error {
	return nil
}
