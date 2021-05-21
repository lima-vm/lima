package start

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AkihiroSuda/lima/pkg/cidata"
	"github.com/AkihiroSuda/lima/pkg/hostagent"
	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/qemu"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func Start(ctx context.Context, instName, instDir string, y *limayaml.LimaYAML) error {
	cidataISOPath := filepath.Join(instDir, "cidata.iso")
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
	qExe, qArgs, err := qemu.Cmdline(qCfg)
	if err != nil {
		return err
	}
	qCmd := exec.CommandContext(ctx, qExe, qArgs...)
	qCmd.Stdout = os.Stdout
	qCmd.Stderr = os.Stderr
	logrus.Infof("Starting QEMU (hint: to watch the boot progress, see %q)", filepath.Join(instDir, "serial.log"))
	logrus.Debugf("qCmd.Args: %v", qCmd.Args)
	if err := qCmd.Start(); err != nil {
		return err
	}
	defer func() {
		_ = qCmd.Process.Kill()
	}()

	sshFixCmd := exec.Command("ssh-keygen",
		"-R", fmt.Sprintf("[127.0.0.1]:%d", y.SSH.LocalPort),
		"-R", fmt.Sprintf("[localhost]:%d", y.SSH.LocalPort),
	)

	if out, err := sshFixCmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to run %v: %q", sshFixCmd.Args, string(out))
	}
	logrus.Infof("SSH: 127.0.0.1:%d", y.SSH.LocalPort)

	hAgent, err := hostagent.New(y, instDir)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := hAgent.Close(); cErr != nil {
			logrus.WithError(cErr).Warn("An error during shutting down the host agent")
		}
	}()
	if err := hAgent.Run(ctx); err == nil {
		logrus.Info("READY. Run `lima bash` to open the shell.")
	} else {
		logrus.WithError(err).Warn("DEGRADED. The VM seems running, but file sharing and port forwarding may not work.")
	}
	// TODO: daemonize the host process here
	return qCmd.Wait()
}
