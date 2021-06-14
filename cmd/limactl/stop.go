package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	hostagentapi "github.com/AkihiroSuda/lima/pkg/hostagent/api"
	"github.com/AkihiroSuda/lima/pkg/store"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var stopCommand = &cli.Command{
	Name:      "stop",
	Usage:     "Stop an instance",
	ArgsUsage: "INSTANCE [INSTANCE, ...]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "forcibly kill the processes",
		},
	},
	Action:       stopAction,
	BashComplete: stopBashComplete,
}

func stopAction(clicontext *cli.Context) error {
	if clicontext.NArg() > 1 {
		return errors.Errorf("too many arguments")
	}

	instName := clicontext.Args().First()
	if instName == "" {
		instName = DefaultInstanceName
	}

	inst, err := store.Inspect(instName)
	if err != nil {
		return err
	}

	if clicontext.Bool("force") {
		stopInstanceForcibly(inst)
		return nil
	}

	return stopInstanceGracefully(inst)
}

func stopInstanceGracefully(inst *store.Instance) error {
	if inst.Status != store.StatusRunning {
		return errors.Errorf("expected status %q, got %q", store.StatusRunning, inst.Status)
	}

	logrus.Infof("Sending SIGINT to hostagent process %d", inst.HostAgentPID)
	if err := syscall.Kill(inst.HostAgentPID, syscall.SIGINT); err != nil {
		logrus.Error(err)
	}

	logrus.Info("Waiting for the host agent and the qemu processes to shut down")
	return waitForHostAgentTermination(context.TODO(), inst)
}

func waitForHostAgentTermination(ctx context.Context, inst *store.Instance) error {
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

	haStdoutPath := filepath.Join(inst.Dir, "ha.stdout.log")
	haStderrPath := filepath.Join(inst.Dir, "ha.stderr.log")

	if err := hostagentapi.WatchEvents(ctx2, haStdoutPath, haStderrPath, onEvent); err != nil {
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

func stopBashComplete(clicontext *cli.Context) {
	bashCompleteInstanceNames(clicontext)
}
