package progressbar

import (
	"os"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/mattn/go-isatty"
)

func New(size int64) (*pb.ProgressBar, error) {
	bar := pb.New64(size)

	bar.Set(pb.Bytes, true)

	// Both logrous and pb use stderr by default.
	logFd := os.Stderr.Fd()

	// Show progress only when logging to terminal.
	if isatty.IsTerminal(logFd) || isatty.IsCygwinTerminal(logFd) {
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
