// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ticker

import (
	"time"
)

func NewSimpleTicker(ticker *time.Ticker) Ticker {
	return &simpleTicker{Ticker: ticker}
}

type simpleTicker struct {
	*time.Ticker
}

func (ticker *simpleTicker) Chan() <-chan time.Time {
	return ticker.Ticker.C
}
