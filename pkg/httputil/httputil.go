// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package httputil

// ErrorJSON is returned with "application/json" content type and non-2XX status code.
type ErrorJSON struct {
	Message string `json:"message"`
}
