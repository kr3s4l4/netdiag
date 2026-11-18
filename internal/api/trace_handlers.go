package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kr3s4l4/netdiag/internal/trace"
)

// TraceRequest is the payload for an on-demand traceroute.
type TraceRequest struct {
	Target  string `json:"target"`
	Hops    int    `json:"hops,omitempty"`
	Probes  int    `json:"probes,omitempty"`
	Timeout int    `json:"timeout_ms,omitempty"`
	Resolve *bool  `json:"resolve,omitempty"`
}

func (s *Server) handleTrace(w http.ResponseWriter, r *http.Request) {
	var req TraceRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	if err := dec.Decode(&req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if req.Target == "" {
		s.fail(w, r, http.StatusBadRequest, "falta el destino")
		return
	}
	opts := trace.Options{
		MaxHops: req.Hops,
		Probes:  req.Probes,
		Timeout: time.Duration(req.Timeout) * time.Millisecond,
		Resolve: true,
	}
	if req.Resolve != nil {
		opts.Resolve = *req.Resolve
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := trace.Run(ctx, req.Target, opts)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
