// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package external

// WarningsTrailerKey is the gRPC trailer key used for relaying the warnings
// logged by an external driver during an RPC (currently only Validate) to limactl.
//
// An external driver process writes its logs only to driver.stderr.log,
// so warnings about the user's config would not be visible without this relay.
//
// Each value is a JSON-encoded [Warning].
// The "-bin" suffix allows non-ASCII bytes in the values.
const WarningsTrailerKey = "lima-driver-warnings-bin"

// Warning is a log entry relayed from an external driver.
type Warning struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}
