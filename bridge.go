package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// build inspects each plugin's RPC() struct and registers Connect handlers
// for every method that follows the proto-shaped contract:
//
//	func(*A, *B) error  where A and B implement proto.Message.
//
// Methods that don't match are skipped with a warning. Returns the configured
// mux and the list of registered procedure paths.
func build(plugins map[string]RPCer, log *slog.Logger) (*http.ServeMux, []string) {
	mux := http.NewServeMux()
	routes := make([]string, 0)

	for name, rpcer := range plugins {
		if rpcer == nil {
			continue
		}
		svc := rpcer.RPC()
		if svc == nil {
			log.Warn("plugin returned nil from RPC()", "plugin", name)
			continue
		}

		registerService(mux, &routes, name, svc, log)
	}

	return mux, routes
}

func registerService(mux *http.ServeMux, routes *[]string, pluginName string, svc any, log *slog.Logger) {
	protoMessageType := reflect.TypeOf((*proto.Message)(nil)).Elem()
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	val := reflect.ValueOf(svc)
	typ := val.Type()

	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}

		mt := method.Type
		// signature must be: receiver + 2 inputs + 1 output (error)
		if mt.NumIn() != 3 || mt.NumOut() != 1 {
			log.Warn("rpc method skipped: wrong arity",
				"plugin", pluginName, "method", method.Name,
				"in", mt.NumIn()-1, "out", mt.NumOut())
			continue
		}
		if mt.Out(0) != errorType {
			log.Warn("rpc method skipped: must return error",
				"plugin", pluginName, "method", method.Name)
			continue
		}

		inType, outType := mt.In(1), mt.In(2)
		if inType.Kind() != reflect.Pointer || !inType.Implements(protoMessageType) {
			log.Warn("rpc method skipped: input is not a proto.Message pointer",
				"plugin", pluginName, "method", method.Name, "in", inType.String())
			continue
		}
		if outType.Kind() != reflect.Pointer || !outType.Implements(protoMessageType) {
			log.Warn("rpc method skipped: output is not a proto.Message pointer",
				"plugin", pluginName, "method", method.Name, "out", outType.String())
			continue
		}

		procedure := "/" + pluginName + "/" + method.Name
		callable := val.MethodByName(method.Name)

		mux.Handle(procedure, newUnaryHandler(procedure, callable, inType.Elem(), outType.Elem()))
		*routes = append(*routes, procedure)
	}
}

func newUnaryHandler(procedure string, callable reflect.Value, inElem, outElem reflect.Type) http.Handler {
	initReq := func(_ connect.Spec, msg any) error {
		ptr, ok := msg.(*proto.Message)
		if !ok {
			return fmt.Errorf("rpc bridge: request initializer received unexpected type %T", msg)
		}
		*ptr = reflect.New(inElem).Interface().(proto.Message)
		return nil
	}

	handle := func(_ context.Context, req *connect.Request[proto.Message]) (*connect.Response[proto.Message], error) {
		if req.Msg == nil || *req.Msg == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("nil request message"))
		}
		// req.Msg is *proto.Message (pointer to interface); dereference once so
		// reflect.ValueOf unwraps the interface down to the concrete pointer the
		// target method expects.
		inMsg := *req.Msg
		outVal := reflect.New(outElem)

		results := callable.Call([]reflect.Value{
			reflect.ValueOf(inMsg),
			outVal,
		})

		if errVal := results[0]; !errVal.IsNil() {
			return nil, errVal.Interface().(error)
		}

		outMsg := outVal.Interface().(proto.Message)
		return connect.NewResponse(&outMsg), nil
	}

	return connect.NewUnaryHandler[proto.Message, proto.Message](
		procedure,
		handle,
		connect.WithCodec(protoBridgeCodec{}),
		connect.WithCodec(jsonBridgeCodec{}),
		connect.WithRequestInitializer(initReq),
	)
}

// protoBridgeCodec marshals/unmarshals proto messages where the runtime value
// may be either a proto.Message directly (typical for the response path) or a
// *proto.Message — the slot Connect's request initializer fills with the
// concrete instance allocated by reflection.
type protoBridgeCodec struct{}

func (protoBridgeCodec) Name() string { return "proto" }

func (protoBridgeCodec) Marshal(v any) ([]byte, error) {
	m, err := unwrapProto(v)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(m)
}

func (protoBridgeCodec) Unmarshal(data []byte, v any) error {
	m, err := unwrapProto(v)
	if err != nil {
		return err
	}
	return proto.Unmarshal(data, m)
}

type jsonBridgeCodec struct{}

func (jsonBridgeCodec) Name() string { return "json" }

func (jsonBridgeCodec) Marshal(v any) ([]byte, error) {
	m, err := unwrapProto(v)
	if err != nil {
		return nil, err
	}
	return protojson.Marshal(m)
}

func (jsonBridgeCodec) Unmarshal(data []byte, v any) error {
	m, err := unwrapProto(v)
	if err != nil {
		return err
	}
	return protojson.Unmarshal(data, m)
}

func unwrapProto(v any) (proto.Message, error) {
	if mp, ok := v.(*proto.Message); ok {
		if *mp == nil {
			return nil, errors.New("rpc bridge: proto.Message is nil; request initializer did not run")
		}
		return *mp, nil
	}
	if m, ok := v.(proto.Message); ok {
		return m, nil
	}
	return nil, fmt.Errorf("rpc bridge: not a proto.Message (got %T)", v)
}
