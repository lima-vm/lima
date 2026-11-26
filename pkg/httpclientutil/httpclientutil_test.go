// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package httpclientutil

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSuccessfulErrorMessage(t *testing.T) {
	// GitHub serves an HTML page of about 64 KiB when a release asset does not exist.
	longHTML := "<!DOCTYPE html>\n<html lang=\"en\">\n" +
		strings.Repeat("<p>filler</p>\n", 100) +
		"<!-- deep-in-the-body -->\n" +
		strings.Repeat("<p>filler</p>\n", 8192) +
		"</html>\n"

	for _, tc := range []struct {
		name        string
		contentType string
		body        string
		contains    string
		notContains string
	}{
		{
			name:        "short body is kept verbatim",
			contentType: "text/plain",
			body:        "no such release asset",
			contains:    "no such release asset",
		},
		{
			name:        "ErrorJSON body is reduced to its message",
			contentType: "application/json",
			body:        `{"message":"driver \"vz\" does not support screenshot"}`,
			contains:    `driver "vz" does not support screenshot`,
		},
		{
			name:        "long HTML body does not reach the message",
			contentType: "text/html",
			body:        longHTML,
			contains:    "unexpected HTTP status Not Found",
			notContains: "deep-in-the-body",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(ts.Close)

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, http.NoBody)
			assert.NilError(t, err)
			resp, err := ts.Client().Do(req)
			assert.NilError(t, err)
			defer resp.Body.Close()

			err = Successful(resp)
			assert.ErrorContains(t, err, tc.contains)
			if tc.notContains != "" {
				assert.Assert(t, !strings.Contains(err.Error(), tc.notContains),
					"message should not contain %q", tc.notContains)
			}
			// The message is printed to the terminal, so it has to stay readable
			// whatever the server returns.
			assert.Assert(t, len(err.Error()) < 1024, "message is %d bytes long", len(err.Error()))
		})
	}
}
