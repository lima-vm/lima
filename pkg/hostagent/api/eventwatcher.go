package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/AkihiroSuda/lima/pkg/logrusutil"
	"github.com/nxadm/tail"
	"github.com/sirupsen/logrus"
)

func WatchEvents(ctx context.Context, haStdoutPath, haStderrPath string, begin time.Time, onEvent func(Event) bool) error {
	haStdoutTail, err := tail.TailFile(haStdoutPath,
		tail.Config{
			Follow:    true,
			MustExist: true,
		})
	if err != nil {
		return err
	}
	defer func() {
		_ = haStdoutTail.Stop()
		haStdoutTail.Cleanup()
	}()

	haStderrTail, err := tail.TailFile(haStderrPath,
		tail.Config{
			Follow:    true,
			MustExist: true,
		})
	if err != nil {
		return err
	}
	defer func() {
		_ = haStderrTail.Stop()
		haStderrTail.Cleanup()
	}()

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case line := <-haStdoutTail.Lines:
			if line.Err != nil {
				logrus.Error(line.Err)
			}
			if line.Text == "" {
				continue
			}
			var ev Event
			if err := json.Unmarshal([]byte(line.Text), &ev); err != nil {
				return err
			}
			logrus.WithField("event", ev).Debugf("received an event")
			if stop := onEvent(ev); stop {
				return nil
			}
		case line := <-haStderrTail.Lines:
			if line.Err != nil {
				logrus.Error(line.Err)
			}
			logrusutil.PropagateJSON(logrus.StandardLogger(), []byte(line.Text), "[hostagent] ", begin)
		}
	}

	return nil
}
