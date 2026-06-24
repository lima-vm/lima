// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/lima-vm/lima/v2/pkg/hostagent/api"
	"github.com/lima-vm/lima/v2/pkg/httputil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// Agent is the subset of *hostagent.HostAgent that the API server depends on.
type Agent interface {
	Info(context.Context) (*api.Info, error)
	MountAdd(ctx context.Context, hostPath, guestPath string, mountType limatype.MountType, writable bool) (*api.Mount, error)
	MountRemove(ctx context.Context, guestPath string) error
	MountList() []api.Mount
}

type Backend struct {
	Agent Agent
}

func (b *Backend) onError(w http.ResponseWriter, err error, ec int) {
	w.WriteHeader(ec)
	w.Header().Set("Content-Type", "application/json")
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

func (b *Backend) writeJSON(w http.ResponseWriter, code int, v any) {
	m, err := json.Marshal(v)
	if err != nil {
		b.onError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(m)
}

// Mounts is the handler for /v1/mounts (GET list, POST add, DELETE remove).
func (b *Backend) Mounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		b.writeJSON(w, http.StatusOK, b.Agent.MountList())
	case http.MethodPost:
		var req api.MountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			b.onError(w, err, http.StatusBadRequest)
			return
		}
		m, err := b.Agent.MountAdd(ctx, req.HostPath, req.MountPoint, req.Type, req.Writable)
		if err != nil {
			b.onError(w, err, http.StatusInternalServerError)
			return
		}
		b.writeJSON(w, http.StatusOK, m)
	case http.MethodDelete:
		var req api.MountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			b.onError(w, err, http.StatusBadRequest)
			return
		}
		if err := b.Agent.MountRemove(ctx, req.MountPoint); err != nil {
			b.onError(w, err, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func AddRoutes(r *http.ServeMux, b *Backend) {
	r.Handle("/v1/info", http.HandlerFunc(b.GetInfo))
	r.Handle("/v1/mounts", http.HandlerFunc(b.Mounts))
}
