package rpc

import (
	"net/http"
)

type handler struct {
}

func newHandler() *handler {
	return &handler{}
}

// handleHttp handles requests to the http.
func (h *handler) handleHttp() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	})
}
