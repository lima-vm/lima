/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bicopy

import (
	"io"
	"sync"

	"github.com/sirupsen/logrus"
)

// Bicopy is from https://github.com/rootless-containers/rootlesskit/blob/v0.10.1/pkg/port/builtin/parent/tcp/tcp.go#L73-L104
// (originally from libnetwork, Apache License 2.0).
func Bicopy(x, y io.ReadWriter, quit <-chan struct{}) {
	type closeReader interface {
		CloseRead() error
	}
	type closeWriter interface {
		CloseWrite() error
	}
	var wg sync.WaitGroup
	broker := func(to, from io.ReadWriter) {
		if _, err := io.Copy(to, from); err != nil {
			logrus.WithError(err).Debug("failed to call io.Copy")
		}
		if fromCR, ok := from.(closeReader); ok {
			if err := fromCR.CloseRead(); err != nil {
				logrus.WithError(err).Debug("failed to call CloseRead")
			}
		}
		if toCW, ok := to.(closeWriter); ok {
			if err := toCW.CloseWrite(); err != nil {
				logrus.WithError(err).Debug("failed to call CloseWrite")
			}
		}
		wg.Done()
	}

	wg.Add(2)
	go broker(x, y)
	go broker(y, x)
	finish := make(chan struct{})
	go func() {
		wg.Wait()
		close(finish)
	}()

	select {
	case <-quit:
	case <-finish:
	}
	if xCloser, ok := x.(io.Closer); ok {
		if err := xCloser.Close(); err != nil {
			logrus.WithError(err).Debug("failed to call xCloser.Close")
		}
	}
	if yCloser, ok := y.(io.Closer); ok {
		if err := yCloser.Close(); err != nil {
			logrus.WithError(err).Debug("failed to call yCloser.Close")
		}
	}
	<-finish
	// TODO: return copied bytes
}
