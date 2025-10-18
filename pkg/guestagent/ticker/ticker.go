// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ticker

import (
	"time"
)

type Ticker interface {
	// similar to time.Ticker.C, but must be closed when Stop() is called
	Chan() <-chan time.Time
	Stop()
}
