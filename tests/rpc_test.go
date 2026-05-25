package rpc

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/endure/v2"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/logger/v6"
	"github.com/roadrunner-server/rpc/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

func TestRpcInit(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	err := cont.Register(&Plugin1{})
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Register(&Plugin2{})
	if err != nil {
		t.Fatal(err)
	}

	v := &config.Plugin{
		Version: "v2024.2.0",
		Path:    "configs/.rr.yaml",
	}

	err = cont.Register(v)
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Register(&rpc.Plugin{})
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Register(&logger.Plugin{})
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Init()
	if err != nil {
		t.Fatal(err)
	}

	ch, err := cont.Serve()
	if err != nil {
		t.Fatal(err)
	}

	sig := make(chan os.Signal, 1)

	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	wg := &sync.WaitGroup{}

	tt := time.NewTimer(time.Second * 3)

	wg.Go(func() {
		defer tt.Stop()
		for {
			select {
			case e := <-ch:
				// just stop, this is ok
				assert.Error(t, e.Error)
				_ = cont.Stop()
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-tt.C:
				return
			}
		}
	})

	wg.Wait()
}

func TestRpcReflection(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	require.NoError(t, cont.Register(&Plugin1{}))
	require.NoError(t, cont.Register(&config.Plugin{
		Version: "v2024.2.0",
		Path:    "configs/.rr.yaml",
	}))
	require.NoError(t, cont.Register(&rpc.Plugin{}))
	require.NoError(t, cont.Register(&logger.Plugin{}))
	require.NoError(t, cont.Init())

	ch, err := cont.Serve()
	require.NoError(t, err)

	defer func() { _ = cont.Stop() }()

	// poll the listener until it accepts connections (avoids fixed-sleep flakes)
	require.Eventually(t, func() bool {
		c, err := net.DialTimeout("tcp", "127.0.0.1:6001", 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 5*time.Second, 50*time.Millisecond, "http listener never came up")

	conn, err := grpc.NewClient("127.0.0.1:6001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := reflectionpb.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)

	require.NoError(t, stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{ListServices: ""},
	}))

	resp, err := stream.Recv()
	require.NoError(t, err)

	listResp := resp.GetListServicesResponse()
	require.NotNil(t, listResp, "ServerReflectionInfo did not return a ListServicesResponse")

	got := make([]string, 0, len(listResp.GetService()))
	for _, svc := range listResp.GetService() {
		got = append(got, svc.GetName())
	}
	sort.Strings(got)

	assert.Contains(t, got, "rpc", "rpc plugin's own service should be advertised")
	assert.Contains(t, got, "rpc_test.plugin1", "Plugin1's service should be advertised")

	// drain any backpressure error so endure shutdown stays clean
	select {
	case <-ch:
	default:
	}
}

func TestRpcDisabled(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	err := cont.Register(&Plugin1{})
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Register(&Plugin2{})
	if err != nil {
		t.Fatal(err)
	}

	v := &config.Plugin{}
	v.Path = "configs/.rr-rpc-disabled.yaml"
	v.Prefix = "rr"
	err = cont.Register(v)
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Register(&rpc.Plugin{})
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Register(&logger.Plugin{})
	if err != nil {
		t.Fatal(err)
	}

	err = cont.Init()
	if err != nil {
		t.Fatal(err)
	}

	ch, err := cont.Serve()
	if err != nil {
		t.Fatal(err)
	}

	sig := make(chan os.Signal, 1)

	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	tt := time.NewTimer(time.Second * 20)

	wg := &sync.WaitGroup{}

	wg.Go(func() {
		defer tt.Stop()
		for {
			select {
			case e := <-ch:
				// RPC is turned off, should be and dial error
				if errors.Is(errors.Disabled, e.Error) {
					assert.FailNow(t, "should not be disabled error")
				}
				assert.Error(t, e.Error)
				assert.NoError(t, cont.Stop())
				return
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-tt.C:
				// timeout
				return
			}
		}
	})

	wg.Wait()
}
