package start

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/AkihiroSuda/lima/pkg/cidata"
	hostagentapi "github.com/AkihiroSuda/lima/pkg/hostagent/api"
	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/qemu"
	"github.com/AkihiroSuda/lima/pkg/store"
	"github.com/AkihiroSuda/lima/pkg/store/filenames"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func ensureDisk(ctx context.Context, instName, instDir string, y *limayaml.LimaYAML) error {
	cidataISOPath := filepath.Join(instDir, filenames.CIDataISO)
	if err := cidata.GenerateISO9660(cidataISOPath, instName, y); err != nil {
		return err
	}
	qCfg := qemu.Config{
		Name:        instName,
		InstanceDir: instDir,
		LimaYAML:    y,
	}
	if err := qemu.EnsureDisk(qCfg); err != nil {
		return err
	}

	return nil
}

func Start(ctx context.Context, inst *store.Instance) error {
	haPIDPath := filepath.Join(inst.Dir, filenames.HostAgentPID)
	if _, err := os.Stat(haPIDPath); !errors.Is(err, os.ErrNotExist) {
		return errors.Errorf("instance %q seems running (hint: remove %q if the instance is not actually running)", inst.Name, haPIDPath)
	}

	y, err := inst.LoadYAML()
	if err != nil {
		return err
	}

	if err := ensureDisk(ctx, inst.Name, inst.Dir, y); err != nil {
		return err
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}
	haStdoutPath := filepath.Join(inst.Dir, filenames.HostAgentStdoutLog)
	haStderrPath := filepath.Join(inst.Dir, filenames.HostAgentStderrLog)
	if err := os.RemoveAll(haStdoutPath); err != nil {
		return err
	}
	if err := os.RemoveAll(haStderrPath); err != nil {
		return err
	}
	haStdoutW, err := os.Create(haStdoutPath)
	if err != nil {
		return err
	}
	// no defer haStdoutW.Close()
	haStderrW, err := os.Create(haStderrPath)
	if err != nil {
		return err
	}
	// no defer haStderrW.Close()

	haCmd := exec.CommandContext(ctx, self,
		"hostagent",
		"--pidfile", haPIDPath,
		inst.Name)
	haCmd.Stdout = haStdoutW
	haCmd.Stderr = haStderrW

	if err := haCmd.Start(); err != nil {
		return err
	}

	if err := waitHostAgentStart(ctx, haPIDPath, haStderrPath); err != nil {
		return err
	}

	return watchHostAgentEvents(ctx, inst.Name, haStdoutPath, haStderrPath)
	// leave the hostagent process running
}

func waitHostAgentStart(ctx context.Context, haPIDPath, haStderrPath string) error {
	begin := time.Now()
	deadlineDuration := 5 * time.Second
	deadline := begin.Add(deadlineDuration)
	for {
		if _, err := os.Stat(haPIDPath); !errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.Errorf("hostagent (%q) did not start up in %v (hint: see %q)", haPIDPath, deadlineDuration, haStderrPath)
		}
	}
}

func watchHostAgentEvents(ctx context.Context, instName, haStdoutPath, haStderrPath string) error {
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	var (
		printedSSHLocalPort  bool
		receivedRunningEvent bool
		err                  error
	)
	onEvent := func(ev hostagentapi.Event) bool {
		if !printedSSHLocalPort && ev.Status.SSHLocalPort != 0 {
			logrus.Infof("SSH Local Port: %d", ev.Status.SSHLocalPort)
			printedSSHLocalPort = true
		}

		if len(ev.Status.Errors) > 0 {
			logrus.Errorf("%+v", ev.Status.Errors)
		}
		if ev.Status.Exiting {
			err = errors.Errorf("exiting, status=%+v (hint: see %q)", ev.Status, haStderrPath)
			return true
		} else if ev.Status.Running {
			receivedRunningEvent = true
			if ev.Status.Degraded {
				logrus.Warnf("DEGRADED. The VM seems running, but file sharing and port forwarding may not work. (hint: see %q)", haStderrPath)
				err = errors.Errorf("degraded, status=%+v", ev.Status)
				return true
			}

			shellCmd := fmt.Sprintf("limactl shell %s", instName)
			if instName == "default" {
				shellCmd = "lima"
			}
			logrus.Infof("READY. Run `%s` to open the shell.", shellCmd)
			err = nil
			return true
		}
		return false
	}

	if xerr := hostagentapi.WatchEvents(ctx2, haStdoutPath, haStderrPath, onEvent); xerr != nil {
		return xerr
	}

	if err != nil {
		return err
	}

	if !receivedRunningEvent {
		return errors.New("did not receive an event with the \"running\" status")
	}

	return nil
}
