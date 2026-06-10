package rpc

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubRPCer struct {
	name string
	path string
	h    http.Handler
}

func (s *stubRPCer) Name() string                { return s.name }
func (s *stubRPCer) RPC() (string, http.Handler) { return s.path, s.h }

func TestBuildMuxSkipsDuplicateAndInvalidPaths(t *testing.T) {
	first := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	duplicate := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	p := &Plugin{
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		plugins: map[string]RPCer{
			"a-first":  &stubRPCer{name: "a-first", path: "/svc/", h: first},
			"b-second": &stubRPCer{name: "b-second", path: "/svc/", h: duplicate},
			"empty":    &stubRPCer{name: "empty", path: "", h: first},
			"no-slash": &stubRPCer{name: "no-slash", path: "bad", h: first},
		},
	}

	mux, services := p.buildMux()
	if len(services) != 1 || services[0] != "svc" {
		t.Fatalf("expected exactly one mounted service [svc], got %v", services)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/svc/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected the first plugin in sorted order to own the path, got status %d", rec.Code)
	}
}
