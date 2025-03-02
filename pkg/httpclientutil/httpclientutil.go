/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

	"github.com/lima-vm/lima/pkg/httputil"
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

// HTTPStatusError is created from non-2XX HTTP response.
type HTTPStatusError struct {
	// StatusCode is non-2XX status code
	StatusCode int
	// Body is at most HTTPStatusErrorBodyMaxLength
	Body string
}

// Error implements error.
// If e.Body is a marshalled string of httputil.ErrorJSON, Error returns ErrorJSON.Message .
// Otherwise Error returns a human-readable string that contains e.StatusCode and e.Body.
func (e *HTTPStatusError) Error() string {
	if e.Body != "" && len(e.Body) < HTTPStatusErrorBodyMaxLength {
		var ej httputil.ErrorJSON
		if json.Unmarshal([]byte(e.Body), &ej) == nil {
			return ej.Message
		}
	}
	return fmt.Sprintf("unexpected HTTP status %s, body=%q", http.StatusText(e.StatusCode), e.Body)
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
