// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package lockutil

import "errors"

// ErrDirRemoved is returned by WithDirLock when the directory was removed, and
// possibly recreated, while waiting for its lock, so the lock no longer guards
// the directory at that path.
var ErrDirRemoved = errors.New("directory was removed while waiting for its lock")
