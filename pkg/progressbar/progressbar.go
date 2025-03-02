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

package progressbar

import (
	"os"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
)

// ProgressBar adapts pb.ProgressBar to go-qcow2reader.convert.Updater interface.
type ProgressBar struct {
	*pb.ProgressBar
}

func (b *ProgressBar) Update(n int64) {
	b.Add64(n)
}

func New(size int64) (*ProgressBar, error) {
	bar := &ProgressBar{pb.New64(size)}

	bar.Set(pb.Bytes, true)

	if showProgress() {
		bar.SetTemplateString(`{{counters . }} {{bar . | green }} {{percent .}} {{speed . "%s/s"}}`)
		bar.SetRefreshRate(200 * time.Millisecond)
	} else {
		bar.Set(pb.Static, true)
	}

	bar.SetWidth(80)
	if err := bar.Err(); err != nil {
		return nil, err
	}

	return bar, nil
}

func showProgress() bool {
	// Progress supports only text format fow now.
	if _, ok := logrus.StandardLogger().Formatter.(*logrus.TextFormatter); !ok {
		return false
	}

	// Both logrus and pb use stderr by default.
	logFd := os.Stderr.Fd()
	return isatty.IsTerminal(logFd) || isatty.IsCygwinTerminal(logFd)
}
