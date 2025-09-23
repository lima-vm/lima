// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ticker

import (
	"time"
)

func NewCompoundTicker(t1, t2 Ticker) Ticker {
	return &compoundTicker{t1, t2}
}

type compoundTicker struct {
	t1, t2 Ticker
}

func (ticker *compoundTicker) Chan() <-chan time.Time {
	ch := make(chan time.Time)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case v, ok := <-ticker.t1.Chan():
				if !ok {
					return
				}
				ch <- v
			case v, ok := <-ticker.t2.Chan():
				if !ok {
					return
				}
				ch <- v
			}
		}
	}()
	return ch
}

func (ticker *compoundTicker) Stop() {
	ticker.t1.Stop()
	ticker.t2.Stop()
}
