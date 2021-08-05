package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lima-vm/lima/pkg/guestagent"
	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/sirupsen/logrus"
)

type Backend struct {
	Agent guestagent.Agent
}

func (b *Backend) onError(w http.ResponseWriter, r *http.Request, err error, ec int) {
	w.WriteHeader(ec)
	w.Header().Set("Content-Type", "application/json")
	// it is safe to return the err to the client, because the client is reliable
	e := api.ErrorJSON{
		Message: err.Error(),
	}
	_ = json.NewEncoder(w).Encode(e)
}

// GetInfo is the handler for GET /v{N}/info
func (b *Backend) GetInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	info, err := b.Agent.Info(ctx)
	if err != nil {
		b.onError(w, r, err, http.StatusInternalServerError)
		return
	}
	m, err := json.Marshal(info)
	if err != nil {
		b.onError(w, r, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(m)
}

// GetEvents is the handler for GET /v{N}/events.
func (b *Backend) GetEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("http.ResponseWriter has to implement http.Flusher")
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := make(chan api.Event)
	go b.Agent.Events(ctx, ch)

	enc := json.NewEncoder(w)
	for ev := range ch {
		if err := enc.Encode(ev); err != nil {
			logrus.Warn(err)
			return
		}
		flusher.Flush()
	}
}

func AddRoutes(r *mux.Router, b *Backend) {
	v1 := r.PathPrefix("/v1").Subrouter()
	v1.Path("/info").Methods("GET").HandlerFunc(b.GetInfo)
	v1.Path("/events").Methods("GET").HandlerFunc(b.GetEvents)
}
