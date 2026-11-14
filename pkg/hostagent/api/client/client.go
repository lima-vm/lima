// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

// Forked from https://github.com/rootless-containers/rootlesskit/blob/v0.14.2/pkg/api/client/client.go
// Apache License 2.0

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/hostagent/api"
	"github.com/lima-vm/lima/v2/pkg/httpclientutil"
)

type HostAgentClient interface {
	HTTPClient() *http.Client
	Info(context.Context) (*api.Info, error)
	// Screenshot captures the VM display. format is "png" or "bmp" (default "png").
	Screenshot(context.Context, string) ([]byte, error)
	// ListUSBDevices returns the host USB devices that can be attached to the VM.
	ListUSBDevices(context.Context) ([]driver.USBDevice, error)
	// AttachUSBDevice attaches the host USB device with the given IORegistry ID.
	AttachUSBDevice(context.Context, uint64) error
	// DetachUSBDevice detaches the host USB device with the given IORegistry ID.
	DetachUSBDevice(context.Context, uint64) error
}

// NewHostAgentClient creates a client.
// socketPath is a path to the UNIX socket, without unix:// prefix.
func NewHostAgentClient(socketPath string) (HostAgentClient, error) {
	hc, err := httpclientutil.NewHTTPClientWithSocketPath(socketPath)
	if err != nil {
		return nil, err
	}
	return NewHostAgentClientWithHTTPClient(hc), nil
}

func NewHostAgentClientWithHTTPClient(hc *http.Client) HostAgentClient {
	return &client{
		Client:    hc,
		version:   "v1",
		dummyHost: "lima-hostagent",
	}
}

type client struct {
	*http.Client
	// version is always "v1"
	// TODO(AkihiroSuda): negotiate the version
	version   string
	dummyHost string
}

func (c *client) HTTPClient() *http.Client {
	return c.Client
}

func (c *client) Info(ctx context.Context) (*api.Info, error) {
	u := fmt.Sprintf("http://%s/%s/info", c.dummyHost, c.version)
	resp, err := httpclientutil.Get(ctx, c.HTTPClient(), u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var info api.Info
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *client) Screenshot(ctx context.Context, format string) ([]byte, error) {
	if format == "" {
		format = "png"
	}
	if format != "png" && format != "bmp" {
		return nil, fmt.Errorf("unsupported format %q: must be png or bmp", format)
	}
	params := url.Values{"format": {format}}
	u := fmt.Sprintf("http://%s/%s/screenshot?%s", c.dummyHost, c.version, params.Encode())
	resp, err := httpclientutil.Get(ctx, c.HTTPClient(), u)
	if err != nil {
		if httpErr, ok := errors.AsType[*httpclientutil.HTTPStatusError](err); ok {
			switch httpErr.StatusCode {
			case http.StatusNotFound:
				return nil, errors.New("hostagent does not support screenshot — restart the instance to pick up the latest hostagent binary")
			case http.StatusNotImplemented:
				// httpErr.Error() extracts ErrorJSON.Message from the body, which
				// includes the driver name set by the hostagent.
				return nil, httpErr
			case http.StatusUnprocessableEntity:
				return nil, errors.New("VM has no display configured — set video.display (e.g., \"default\") in the instance config")
			}
		}
		return nil, err
	}
	defer resp.Body.Close()
	const maxScreenshotSize = 32 << 20 // 32 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxScreenshotSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxScreenshotSize {
		return nil, fmt.Errorf("screenshot response exceeds %d MB limit", maxScreenshotSize>>20)
	}
	return data, nil
}

// usbError explains the HTTP errors of the USB endpoints.
func usbError(err error) error {
	if httpErr, ok := errors.AsType[*httpclientutil.HTTPStatusError](err); ok && httpErr.StatusCode == http.StatusNotFound {
		return errors.New("hostagent does not support USB passthrough — restart the instance to pick up the latest hostagent binary")
	}
	return err
}

func (c *client) ListUSBDevices(ctx context.Context) ([]driver.USBDevice, error) {
	u := fmt.Sprintf("http://%s/%s/usb/devices", c.dummyHost, c.version)
	resp, err := httpclientutil.Get(ctx, c.HTTPClient(), u)
	if err != nil {
		return nil, usbError(err)
	}
	defer resp.Body.Close()
	var devs []driver.USBDevice
	if err := json.NewDecoder(resp.Body).Decode(&devs); err != nil {
		return nil, err
	}
	return devs, nil
}

func (c *client) AttachUSBDevice(ctx context.Context, registryID uint64) error {
	return c.postUSBDevice(ctx, registryID, "attach")
}

func (c *client) DetachUSBDevice(ctx context.Context, registryID uint64) error {
	return c.postUSBDevice(ctx, registryID, "detach")
}

func (c *client) postUSBDevice(ctx context.Context, registryID uint64, action string) error {
	u := fmt.Sprintf("http://%s/%s/usb/devices/%d/%s", c.dummyHost, c.version, registryID, action)
	resp, err := httpclientutil.Post(ctx, c.HTTPClient(), u, http.NoBody)
	if err != nil {
		return usbError(err)
	}
	return resp.Body.Close()
}
