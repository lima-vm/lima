// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ticker

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

func NewSimpleTicker(ticker *time.Ticker) Ticker {
	return &simpleTicker{
		Ticker:     ticker,
		closableCh: make(chan any),
		exposeCh:   make(chan time.Time),
	}
}

var _ Ticker = (*simpleTicker)(nil)

type simpleTicker struct {
	*time.Ticker
	closableCh chan any
	exposeCh   chan time.Time
	once       sync.Once
}

func (ticker *simpleTicker) Chan() <-chan time.Time {
	// We cannot directly expose ticker.Ticker.C because it won't be closed on Stop()
	ticker.once.Do(func() {
		go func() {
			defer close(ticker.exposeCh)
			for {
				select {
				case v, ok := <-ticker.Ticker.C:
					if !ok {
						// should not happen as time.Ticker.C is never closed as per docs
						return
					}
					ticker.exposeCh <- v
				case <-ticker.closableCh:
					logrus.Debug("simpleTicker: exiting")
					return
				}
			}
		}()
		logrus.Debug("simpleTicker: starting")
	})
	return ticker.exposeCh
}

func (ticker *simpleTicker) Stop() {
	ticker.Ticker.Stop()
	// Since ticker.Ticker.Stop() does not close ticker.Ticker.C,
	// we need to close the goroutine created in Chan().
	close(ticker.closableCh)
}
