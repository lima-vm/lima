package progressbar

import (
	"io"
	"os"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
)

type ProxyReaderAt struct {
	io.ReaderAt
	Bar *pb.ProgressBar
}

func (r *ProxyReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := r.ReaderAt.ReadAt(p, off)
	r.Bar.Add(n)
	return n, err
}

func New(size int64) (*pb.ProgressBar, error) {
	bar := pb.New64(size)

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
