package client

// Forked from https://github.com/rootless-containers/rootlesskit/blob/v0.14.2/pkg/api/client/client.go
// Apache License 2.0

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"

	"github.com/AkihiroSuda/lima/pkg/guestagent/api"
	"github.com/pkg/errors"
)

type GuestAgentClient interface {
	HTTPClient() *http.Client
	Info(context.Context) (*api.Info, error)
	Events(context.Context, func(api.Event)) error
}

// NewGuestAgentClient creates a client.
// socketPath is a path to the UNIX socket, without unix:// prefix.
func NewGuestAgentClient(socketPath string) (GuestAgentClient, error) {
	if _, err := os.Stat(socketPath); err != nil {
		return nil, err
	}
	hc := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}
	return NewGuestAgentClientWithHTTPClient(hc), nil
}

func NewGuestAgentClientWithHTTPClient(hc *http.Client) GuestAgentClient {
	return &client{
		Client:    hc,
		version:   "v1",
		dummyHost: "lima-guestagent",
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
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := successful(resp); err != nil {
		return nil, err
	}
	var info api.Info
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *client) Events(ctx context.Context, onEvent func(api.Event)) error {
	u := fmt.Sprintf("http://%s/%s/events", c.dummyHost, c.version)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := successful(resp); err != nil {
		return err
	}
	var ev api.Event
	dec := json.NewDecoder(resp.Body)
	for {
		if err := dec.Decode(&ev); err != nil {
			return err
		}
		onEvent(ev)
	}
}

func readAtMost(r io.Reader, maxBytes int) ([]byte, error) {
	lr := &io.LimitedReader{
		R: r,
		N: int64(maxBytes),
	}
	b, err := ioutil.ReadAll(lr)
	if err != nil {
		return b, err
	}
	if lr.N == 0 {
		return b, errors.Errorf("expected at most %d bytes, got more", maxBytes)
	}
	return b, nil
}

// HTTPStatusErrorBodyMaxLength specifies the maximum length of HTTPStatusError.Body
const HTTPStatusErrorBodyMaxLength = 64 * 1024

// HTTPStatusError is created from non-2XX HTTP response
type HTTPStatusError struct {
	// StatusCode is non-2XX status code
	StatusCode int
	// Body is at most HTTPStatusErrorBodyMaxLength
	Body string
}

// Error implements error.
// If e.Body is a marshalled string of api.ErrorJSON, Error returns ErrorJSON.Message .
// Otherwise Error returns a human-readable string that contains e.StatusCode and e.Body.
func (e *HTTPStatusError) Error() string {
	if e.Body != "" && len(e.Body) < HTTPStatusErrorBodyMaxLength {
		var ej api.ErrorJSON
		if json.Unmarshal([]byte(e.Body), &ej) == nil {
			return ej.Message
		}
	}
	return fmt.Sprintf("unexpected HTTP status %s, body=%q", http.StatusText(e.StatusCode), e.Body)
}

func successful(resp *http.Response) error {
	if resp == nil {
		return errors.New("nil response")
	}
	if resp.StatusCode/100 != 2 {
		b, _ := readAtMost(resp.Body, HTTPStatusErrorBodyMaxLength)
		return &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Body:       string(b),
		}
	}
	return nil
}
