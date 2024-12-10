package instance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	hostagentclient "github.com/lima-vm/lima/pkg/hostagent/api/client"
	hostagentevents "github.com/lima-vm/lima/pkg/hostagent/events"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

func StopGracefully(inst *store.Instance, saveOnStop bool) error {
	if inst.Status != store.StatusRunning {
		return fmt.Errorf("expected status %q, got %q (maybe use `limactl stop -f`?)", store.StatusRunning, inst.Status)
	}

	if saveOnStop && inst.Saved {
		logrus.Warn("saved VZ machine state is found. It will be overwritten by the new one.")
	}

	if inst.VMType == limayaml.VZ {
		haSock := filepath.Join(inst.Dir, filenames.HostAgentSock)
		haClient, err := hostagentclient.NewHostAgentClient(haSock)
		if err != nil {
			logrus.WithError(err).Error("Failed to create a host agent client")
		}
		ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
		defer cancel()
		disableSaveOnStopConfig := struct {
			SaveOnStop bool `json:"saveOnStop"`
		}{SaveOnStop: saveOnStop}
		_, err = haClient.DriverConfig(ctx, disableSaveOnStopConfig)
		if err != nil {
			return fmt.Errorf("failed to disable saveOnStop: %w", err)
		}
	} else if saveOnStop {
		return fmt.Errorf("save is not supported for %q", inst.VMType)
	}
	begin := time.Now() // used for logrus propagation
	logrus.Infof("Sending SIGINT to hostagent process %d", inst.HostAgentPID)
	if err := osutil.SysKill(inst.HostAgentPID, osutil.SigInt); err != nil {
		logrus.Error(err)
	}

	logrus.Info("Waiting for the host agent and the driver processes to shut down")
	return waitForHostAgentTermination(context.TODO(), inst, begin)
}

func waitForHostAgentTermination(ctx context.Context, inst *store.Instance, begin time.Time) error {
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Minute+10*time.Second)
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

	if err := hostagentevents.Watch(ctx2, haStdoutPath, haStderrPath, begin, onEvent); err != nil {
		return err
	}

	if !receivedExitingEvent {
		return errors.New("did not receive an event with the \"exiting\" status")
	}

	return nil
}

func StopForcibly(inst *store.Instance) {
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

	suffixesToBeRemoved := []string{".pid", ".sock", ".tmp"}
	globPatterns := strings.ReplaceAll(strings.Join(suffixesToBeRemoved, " "), ".", "*.")
	logrus.Infof("Removing %s under %q", globPatterns, inst.Dir)

	fi, err := os.ReadDir(inst.Dir)
	if err != nil {
		logrus.Error(err)
		return
	}
	for _, f := range fi {
		path := filepath.Join(inst.Dir, f.Name())
		for _, suffix := range suffixesToBeRemoved {
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
