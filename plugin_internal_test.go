package rpc

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubRPCer struct {
	name string
	path string
	h    http.Handler
}

func (s *stubRPCer) Name() string                { return s.name }
func (s *stubRPCer) RPC() (string, http.Handler) { return s.path, s.h }

func TestBuildMuxSkipsInvalidPaths(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	p := &Plugin{
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		plugins: map[string]RPCer{
			"svc":      &stubRPCer{name: "svc", path: "/svc/", h: ok},
			"empty":    &stubRPCer{name: "empty", path: "", h: ok},
			"no-slash": &stubRPCer{name: "no-slash", path: "bad", h: ok},
		},
	}

	mux, services, err := p.buildMux()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 || services[0] != "svc" {
		t.Fatalf("expected exactly one mounted service [svc], got %v", services)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/svc/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected the mounted handler to serve the path, got status %d", rec.Code)
	}
}

func TestBuildMuxRejectsDuplicatePaths(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	p := &Plugin{
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		plugins: map[string]RPCer{
			"a-first":  &stubRPCer{name: "a-first", path: "/svc/", h: ok},
			"b-second": &stubRPCer{name: "b-second", path: "/svc/", h: ok},
		},
	}

	_, _, err := p.buildMux()
	if err == nil {
		t.Fatal("expected an error for a duplicate mount path")
	}
	if !strings.Contains(err.Error(), "a-first") || !strings.Contains(err.Error(), "b-second") {
		t.Fatalf("error should name both conflicting plugins, got: %v", err)
	}
}
