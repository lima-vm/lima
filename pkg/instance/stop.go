// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	hostagentevents "github.com/lima-vm/lima/v2/pkg/hostagent/events"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func StopGracefully(ctx context.Context, inst *limatype.Instance, isRestart bool) error {
	if inst.Status != limatype.StatusRunning {
		if isRestart {
			logrus.Warn("The instance is not running, continuing with the restart")
			return nil
		}
		return fmt.Errorf("expected status %q, got %q (maybe use `limactl stop -f`?)", limatype.StatusRunning, inst.Status)
	}

	begin := time.Now() // used for logrus propagation
	logrus.Infof("Sending SIGINT to hostagent process %d", inst.HostAgentPID)
	if err := osutil.SysKill(inst.HostAgentPID, osutil.SigInt); err != nil {
		logrus.Error(err)
	}

	logrus.Info("Waiting for the host agent and the driver processes to shut down")
	err := waitForHostAgentTermination(ctx, inst, begin)
	if err != nil {
		return err
	}

	logrus.Info("Waiting for the instance to shut down")
	return waitForInstanceShutdown(ctx, inst)
}

func waitForHostAgentTermination(ctx context.Context, inst *limatype.Instance, begin time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute+10*time.Second)
	defer cancel()

	var receivedExitingEvent bool
	onEvent := func(ev hostagentevents.Event) bool {
		if len(ev.Status.Errors) > 0 {
			logrus.Errorf("%+v", ev.Status.Errors)
		}
		if ev.Status.Exiting {
			receivedExitingEvent = true
			return true
		}
		return false
	}

	haStdoutPath := filepath.Join(inst.Dir, filenames.HostAgentStdoutLog)
	haStderrPath := filepath.Join(inst.Dir, filenames.HostAgentStderrLog)

	if err := hostagentevents.Watch(ctx, haStdoutPath, haStderrPath, begin, onEvent); err != nil {
		return err
	}

	if !receivedExitingEvent {
		return errors.New("did not receive an event with the \"exiting\" status")
	}

	return nil
}

func waitForInstanceShutdown(ctx context.Context, inst *limatype.Instance) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			updatedInst, err := store.Inspect(ctx, inst.Name)
			if err != nil {
				return fmt.Errorf("failed to inspect instance status: %w", err)
			}

			if updatedInst.Status == limatype.StatusStopped {
				logrus.Infof("The instance %s has shut down", updatedInst.Name)
				return nil
			}
		case <-ctx.Done():
			return errors.New("timed out waiting for instance to shut down after 3 minutes")
		}
	}
}

func StopForcibly(inst *limatype.Instance) {
	if inst.DriverPID > 0 {
		logrus.Infof("Sending SIGKILL to the %s driver process %d", inst.VMType, inst.DriverPID)
		if err := osutil.SysKill(inst.DriverPID, osutil.SigKill); err != nil {
			logrus.Error(err)
		}
	} else {
		logrus.Infof("The %s driver process seems already stopped", inst.VMType)
	}

	for _, d := range inst.AdditionalDisks {
		diskName := d.Name
		disk, err := store.InspectDisk(diskName)
		if err != nil {
			logrus.Warnf("Disk %q does not exist", diskName)
			continue
		}
		if err := disk.Unlock(); err != nil {
			logrus.Warnf("Failed to unlock disk %q. To use, run `limactl disk unlock %v`", diskName, diskName)
		}
	}

	if inst.HostAgentPID > 0 {
		logrus.Infof("Sending SIGKILL to the host agent process %d", inst.HostAgentPID)
		if err := osutil.SysKill(inst.HostAgentPID, osutil.SigKill); err != nil {
			logrus.Error(err)
		}
	} else {
		logrus.Info("The host agent process seems already stopped")
	}

	globPatterns := strings.ReplaceAll(strings.Join(filenames.TmpFileSuffixes, " "), ".", "*.")
	logrus.Infof("Removing %s under %q", globPatterns, inst.Dir)

	fi, err := os.ReadDir(inst.Dir)
	if err != nil {
		logrus.Error(err)
		return
	}
	for _, f := range fi {
		path := filepath.Join(inst.Dir, f.Name())
		for _, suffix := range filenames.TmpFileSuffixes {
			if strings.HasSuffix(path, suffix) {
				logrus.Infof("Removing %q", path)
				if err := os.Remove(path); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						logrus.Debug(err.Error())
					} else {
						logrus.Error(err)
					}
				}
			}
		}
	}
}
