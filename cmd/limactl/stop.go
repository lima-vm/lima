package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	hostagentapi "github.com/lima-vm/lima/pkg/hostagent/api"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newStopCommand() *cobra.Command {
	var stopCmd = &cobra.Command{
		Use:               "stop INSTANCE",
		Short:             "Stop an instance",
		Args:              cobra.ExactArgs(1),
		RunE:              stopAction,
		ValidArgsFunction: stopBashComplete,
	}

	stopCmd.Flags().BoolP("force", "f", false, "force stop the instance")
	return stopCmd
}

func stopAction(cmd *cobra.Command, args []string) error {
	instName := args[0]
	if instName == "" {
		instName = DefaultInstanceName
	}

	inst, err := store.Inspect(instName)
	if err != nil {
		return err
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}
	if force {
		stopInstanceForcibly(inst)
		return nil
	}

	return stopInstanceGracefully(inst)
}

func stopInstanceGracefully(inst *store.Instance) error {
	if inst.Status != store.StatusRunning {
		return fmt.Errorf("expected status %q, got %q", store.StatusRunning, inst.Status)
	}

	begin := time.Now() // used for logrus propagation
	logrus.Infof("Sending SIGINT to hostagent process %d", inst.HostAgentPID)
	if err := syscall.Kill(inst.HostAgentPID, syscall.SIGINT); err != nil {
		logrus.Error(err)
	}

	logrus.Info("Waiting for the host agent and the qemu processes to shut down")
	return waitForHostAgentTermination(context.TODO(), inst, begin)
}

func waitForHostAgentTermination(ctx context.Context, inst *store.Instance, begin time.Time) error {
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	var receivedExitingEvent bool
	onEvent := func(ev hostagentapi.Event) bool {
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

	if err := hostagentapi.WatchEvents(ctx2, haStdoutPath, haStderrPath, begin, onEvent); err != nil {
		return err
	}

	if !receivedExitingEvent {
		return errors.New("did not receive an event with the \"exiting\" status")
	}

	return nil
}

func stopInstanceForcibly(inst *store.Instance) {
	if inst.QemuPID > 0 {
		logrus.Infof("Sending SIGKILL to the QEMU process %d", inst.QemuPID)
		if err := syscall.Kill(inst.QemuPID, syscall.SIGKILL); err != nil {
			logrus.Error(err)
		}
	} else {
		logrus.Info("The QEMU process seems already stopped")
	}

	if inst.HostAgentPID > 0 {
		logrus.Infof("Sending SIGKILL to the host agent process %d", inst.HostAgentPID)
		if err := syscall.Kill(inst.HostAgentPID, syscall.SIGKILL); err != nil {
			logrus.Error(err)
		}
	} else {
		logrus.Info("The host agent process seems already stopped")
	}

	logrus.Infof("Removing *.pid *.sock under %q", inst.Dir)
	fi, err := os.ReadDir(inst.Dir)
	if err != nil {
		logrus.Error(err)
		return
	}
	for _, f := range fi {
		path := filepath.Join(inst.Dir, f.Name())
		if strings.HasSuffix(path, ".pid") || strings.HasSuffix(path, ".sock") {
			logrus.Infof("Removing %q", path)
			if err := os.Remove(path); err != nil {
				logrus.Error(err)
			}
		}
	}
}

func stopBashComplete(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
