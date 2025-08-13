package pprof

import (
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"
)

type Handler struct {
	mux *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(prefix string) *Handler {
	mux := &http.ServeMux{}

	mux.HandleFunc(fmt.Sprintf("%s/", prefix), pprof.Index)
	mux.HandleFunc(fmt.Sprintf("%s/cmdline", prefix), pprof.Cmdline)
	mux.HandleFunc(fmt.Sprintf("%s/profile", prefix), pprof.Profile)
	mux.HandleFunc(fmt.Sprintf("%s/symbol", prefix), pprof.Symbol)
	mux.HandleFunc(fmt.Sprintf("%s/trace", prefix), pprof.Trace)
	mux.Handle(fmt.Sprintf("%s/vars", prefix), expvar.Handler())

	mux.HandleFunc(fmt.Sprintf("%s/{name}", prefix), func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		pprof.Handler(name).ServeHTTP(w, r)
	})

	return &Handler{mux}
}

var _ http.Handler = &Handler{}
