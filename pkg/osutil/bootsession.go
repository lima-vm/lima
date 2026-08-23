// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import "errors"

// ErrBootSessionNotSupported is returned by BootSessionID on platforms that do not expose
// an identifier of the current boot.
var ErrBootSessionNotSupported = errors.New("boot session ID is not supported on this platform")
