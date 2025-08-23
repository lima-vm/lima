// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nxadm/tail"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/logrusutil"
)

func Watch(ctx context.Context, haStdoutPath, haStderrPath string, begin time.Time, onEvent func(Event) bool) error {
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
		// Do NOT call haStdoutTail.Cleanup(), it prevents the process from ever tailing the file again
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
		// Do NOT call haStderrTail.Cleanup(), it prevents the process from ever tailing the file again
	}()

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case line := <-haStdoutTail.Lines:
			if line == nil {
				break loop
			}
			if line.Err != nil {
				logrus.Error(line.Err)
			}
			if line.Text == "" {
				continue
			}
			var ev Event
			if err := json.Unmarshal([]byte(line.Text), &ev); err != nil {
				return fmt.Errorf("failed to unmarshal %q as %T: %w", line.Text, ev, err)
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
