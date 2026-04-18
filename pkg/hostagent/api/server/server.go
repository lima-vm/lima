// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/lima-vm/lima/v2/pkg/hostagent"
	"github.com/lima-vm/lima/v2/pkg/httputil"
)

type Backend struct {
	Agent *hostagent.HostAgent
}

func (b *Backend) onError(w http.ResponseWriter, err error, ec int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ec)
	// err may potentially contain credential info (in a future version),
	// but it is safe to return the err to the client, because we do not expose the socket to the internet
	e := httputil.ErrorJSON{
		Message: err.Error(),
	}
	_ = json.NewEncoder(w).Encode(e)
}

// GetInfo is the handler for GET /v1/info.
func (b *Backend) GetInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	info, err := b.Agent.Info(ctx)
	if err != nil {
		b.onError(w, err, http.StatusInternalServerError)
		return
	}
	m, err := json.Marshal(info)
	if err != nil {
		b.onError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(m)
}

func AddRoutes(r *http.ServeMux, b *Backend) {
	r.Handle("/v1/info", http.HandlerFunc(b.GetInfo))
	r.Handle("/v1/resume", http.HandlerFunc(b.PostResume))
	r.Handle("/v1/pause", http.HandlerFunc(b.PostPause))
}

// PostResume is the handler for POST /v1/resume.
// It triggers auto-pause resume if the VM is currently paused.
func (b *Backend) PostResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	triggered := b.Agent.Resume()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]bool{"resumed": triggered}
	_ = json.NewEncoder(w).Encode(resp)
}

// PostPause is the handler for POST /v1/pause.
// It triggers an immediate pause of the VM via the auto-pause manager.
func (b *Backend) PostPause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	if err := b.Agent.Pause(ctx); err != nil {
		b.onError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]bool{"paused": true}
	_ = json.NewEncoder(w).Encode(resp)
}
