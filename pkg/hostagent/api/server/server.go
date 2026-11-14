// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/hostagent"
	"github.com/lima-vm/lima/v2/pkg/httputil"
)

type Backend struct {
	Agent *hostagent.HostAgent
}

func (b *Backend) onError(w http.ResponseWriter, err error, ec int) {
	// Set headers before WriteHeader — calling WriteHeader flushes headers.
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

// GetScreenshot is the handler for GET /v1/screenshot.
func (b *Backend) GetScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "png"
	}
	if format != "png" && format != "bmp" {
		b.onError(w, fmt.Errorf("unsupported format %q: must be png or bmp", format), http.StatusBadRequest)
		return
	}
	data, err := b.Agent.Screenshot(ctx, format)
	if err != nil {
		ec := http.StatusInternalServerError
		switch {
		case errors.Is(err, driver.ErrDriverNotScreenshotter):
			ec = http.StatusNotImplemented
		case errors.Is(err, driver.ErrNoDisplay):
			ec = http.StatusUnprocessableEntity
		}
		b.onError(w, err, ec)
		return
	}
	ct := "image/png"
	if format == "bmp" {
		ct = "image/bmp"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func usbErrorCode(err error) int {
	if errors.Is(err, driver.ErrDriverNotUSBPassthrough) {
		return http.StatusNotImplemented
	}
	return http.StatusInternalServerError
}

// GetUSBDevices is the handler for GET /v1/usb/devices.
func (b *Backend) GetUSBDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	devs, err := b.Agent.ListUSBDevices(r.Context())
	if err != nil {
		b.onError(w, err, usbErrorCode(err))
		return
	}
	if devs == nil {
		devs = []driver.USBDevice{}
	}
	m, err := json.Marshal(devs)
	if err != nil {
		b.onError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(m)
}

// PostUSBDevice is the handler for POST /v1/usb/devices/{registryID}/{attach,detach}.
func (b *Backend) PostUSBDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	registryID, err := strconv.ParseUint(r.PathValue("registryID"), 0, 64)
	if err != nil {
		b.onError(w, fmt.Errorf("invalid USB device registry ID: %w", err), http.StatusBadRequest)
		return
	}
	switch action := r.PathValue("action"); action {
	case "attach":
		err = b.Agent.AttachUSBDevice(r.Context(), registryID)
	case "detach":
		err = b.Agent.DetachUSBDevice(r.Context(), registryID)
	default:
		b.onError(w, fmt.Errorf("unknown action %q: must be attach or detach", action), http.StatusNotFound)
		return
	}
	if err != nil {
		b.onError(w, err, usbErrorCode(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func AddRoutes(r *http.ServeMux, b *Backend) {
	r.Handle("/v1/info", http.HandlerFunc(b.GetInfo))
	r.Handle("/v1/screenshot", http.HandlerFunc(b.GetScreenshot))
	r.Handle("/v1/usb/devices", http.HandlerFunc(b.GetUSBDevices))
	r.Handle("/v1/usb/devices/{registryID}/{action}", http.HandlerFunc(b.PostUSBDevice))
}
