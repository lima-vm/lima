// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ticker

import (
	"time"
)

type Ticker interface {
	Chan() <-chan time.Time
	Stop()
}
