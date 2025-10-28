// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/lima-vm/lima/v2/pkg/hostagent"
	"github.com/lima-vm/lima/v2/pkg/httputil"
)

type Backend struct {
	Agent *hostagent.HostAgent
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

// GetMemory is the handler for GET /v1/memory.
func (b *Backend) GetMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	memory, err := b.Agent.GetCurrentMemory()
	if err != nil {
		b.onError(w, err, http.StatusInternalServerError)
		return
	}
	s := strconv.FormatInt(memory, 10)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(s))
}

// SetMemory is the handler for PUT /v1/memory.
func (b *Backend) SetMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		b.onError(w, err, http.StatusInternalServerError)
		return
	}
	memory, err := strconv.ParseInt(string(body), 10, 64)
	if err != nil {
		b.onError(w, err, http.StatusInternalServerError)
		return
	}

	err = b.Agent.SetTargetMemory(memory)
	if err != nil {
		b.onError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func AddRoutes(r *http.ServeMux, b *Backend) {
	r.Handle("/v1/info", http.HandlerFunc(b.GetInfo))
	r.Handle("GET /v1/memory", http.HandlerFunc(b.GetMemory))
	r.Handle("PUT /v1/memory", http.HandlerFunc(b.SetMemory))
}
