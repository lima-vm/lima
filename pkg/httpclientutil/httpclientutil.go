// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package httpclientutil

// Forked from https://github.com/rootless-containers/rootlesskit/blob/v0.14.2/pkg/api/client/client.go
// Apache License 2.0

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/httputil"
)

// Get calls HTTP GET and verifies that the status code is 2XX .
func Get(ctx context.Context, c *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if err := Successful(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func Head(ctx context.Context, c *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if err := Successful(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func Post(ctx context.Context, c *http.Client, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if err := Successful(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func readAtMost(r io.Reader, maxBytes int) ([]byte, error) {
	lr := &io.LimitedReader{
		R: r,
		N: int64(maxBytes),
	}
	b, err := io.ReadAll(lr)
	if err != nil {
		return b, err
	}
	if lr.N == 0 {
		return b, fmt.Errorf("expected at most %d bytes, got more", maxBytes)
	}
	return b, nil
}

// HTTPStatusErrorBodyMaxLength specifies the maximum length of HTTPStatusError.Body.
const HTTPStatusErrorBodyMaxLength = 64 * 1024

// httpStatusErrorBodyMaxPrintLength specifies the maximum length of the body
// that is included in the message returned by HTTPStatusError.Error.
// The body of an error response is often an HTML page, and printing all of
// HTTPStatusErrorBodyMaxLength would flood the terminal.
const httpStatusErrorBodyMaxPrintLength = 256

// HTTPStatusError is created from non-2XX HTTP response.
type HTTPStatusError struct {
	// StatusCode is non-2XX status code
	StatusCode int
	// Body is at most HTTPStatusErrorBodyMaxLength
	Body string
}

// Error implements error.
// If e.Body is a marshalled string of httputil.ErrorJSON, Error returns ErrorJSON.Message .
// Otherwise Error returns a human-readable string that contains e.StatusCode and
// e.Body, with e.Body truncated to httpStatusErrorBodyMaxPrintLength.
func (e *HTTPStatusError) Error() string {
	if e.Body != "" && len(e.Body) < HTTPStatusErrorBodyMaxLength {
		var ej httputil.ErrorJSON
		if json.Unmarshal([]byte(e.Body), &ej) == nil {
			return ej.Message
		}
	}
	body := e.Body
	if len(body) > httpStatusErrorBodyMaxPrintLength {
		body = strings.ToValidUTF8(body[:httpStatusErrorBodyMaxPrintLength], "") + "..."
	}
	return fmt.Sprintf("unexpected HTTP status %s, body=%#q", http.StatusText(e.StatusCode), body)
}

func Successful(resp *http.Response) error {
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

// NewHTTPClientWithDialFn creates a client.
// conn is a raw net.Conn instance.
func NewHTTPClientWithDialFn(dialFn func(ctx context.Context) (net.Conn, error)) (*http.Client, error) {
	hc := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialFn(ctx)
			},
		},
	}
	return hc, nil
}
