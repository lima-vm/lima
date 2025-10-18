// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package krunkit

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/executil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

type LimaKrunDriver struct {
	Instance     *limatype.Instance
	SSHLocalPort int

	krunkitCmd    *exec.Cmd
	krunkitWaitCh chan error
}

var _ driver.Driver = (*LimaKrunDriver)(nil)

func New() *LimaKrunDriver {
	return &LimaKrunDriver{}
}

func (l *LimaKrunDriver) Configure(inst *limatype.Instance) *driver.ConfiguredDriver {
	l.Instance = inst
	l.SSHLocalPort = inst.SSHLocalPort

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaKrunDriver) CreateDisk(ctx context.Context) error {
	return EnsureDisk(ctx, l.Instance)
}

func (l *LimaKrunDriver) Start(ctx context.Context) (chan error, error) {
	krunCmd, err := Cmdline(l.Instance)
	if err != nil {
		return nil, fmt.Errorf("failed to construct krunkit command line: %w", err)
	}
	krunCmd.SysProcAttr = executil.BackgroundSysProcAttr

	logPath := filepath.Join(l.Instance.Dir, "krun.log")
	logfile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open krunkit logfile: %w", err)
	}
	krunCmd.Stderr = logfile

	logrus.Infof("Starting krun VM (hint: to watch the progress, see %q)", logPath)
	logrus.Debugf("krunCmd.Args: %v", krunCmd.Args)

	if err := krunCmd.Start(); err != nil {
		logfile.Close()
		return nil, err
	}

	pidPath := filepath.Join(l.Instance.Dir, filenames.PIDFile(*l.Instance.Config.VMType))
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", krunCmd.Process.Pid)), 0644); err != nil {
		logrus.WithError(err).Warn("Failed to write PID file")
	}

	l.krunkitCmd = krunCmd
	l.krunkitWaitCh = make(chan error, 1)
	go func() {
		defer func() {
			logfile.Close()
			os.RemoveAll(pidPath)
			close(l.krunkitWaitCh)
		}()
		l.krunkitWaitCh <- krunCmd.Wait()
	}()

	return l.krunkitWaitCh, nil
}

func (l *LimaKrunDriver) Stop(ctx context.Context) error {
	if l.krunkitCmd == nil {
		return nil
	}

	if err := l.krunkitCmd.Process.Signal(syscall.SIGTERM); err != nil {
		logrus.WithError(err).Warn("Failed to send interrupt signal")
	}

	timeout := time.After(30 * time.Second)
	select {
	case <-l.krunkitWaitCh:
		return nil
	case <-timeout:
		if err := l.krunkitCmd.Process.Kill(); err != nil {
			return err
		}

		<-l.krunkitWaitCh
		return nil
	}
}

func (l *LimaKrunDriver) Delete(ctx context.Context) error {
	return nil
}

func (l *LimaKrunDriver) InspectStatus(ctx context.Context, inst *limatype.Instance) string {
	return ""
}

func (l *LimaKrunDriver) RunGUI() error {
	return nil
}

func (l *LimaKrunDriver) ChangeDisplayPassword(ctx context.Context, password string) error {
	return fmt.Errorf("display password change not supported by krun driver")
}

func (l *LimaKrunDriver) DisplayConnection(ctx context.Context) (string, error) {
	return "", fmt.Errorf("display connection not supported by krun driver")
}

func (l *LimaKrunDriver) CreateSnapshot(ctx context.Context, tag string) error {
	return fmt.Errorf("snapshots not supported by krun driver")
}

func (l *LimaKrunDriver) ApplySnapshot(ctx context.Context, tag string) error {
	return fmt.Errorf("snapshots not supported by krun driver")
}

func (l *LimaKrunDriver) DeleteSnapshot(ctx context.Context, tag string) error {
	return fmt.Errorf("snapshots not supported by krun driver")
}

func (l *LimaKrunDriver) ListSnapshots(ctx context.Context) (string, error) {
	return "", fmt.Errorf("snapshots not supported by krun driver")
}

func (l *LimaKrunDriver) Register(ctx context.Context) error {
	return nil
}

func (l *LimaKrunDriver) Unregister(ctx context.Context) error {
	return nil
}

func (l *LimaKrunDriver) ForwardGuestAgent() bool {
	return true
}

func (l *LimaKrunDriver) GuestAgentConn(ctx context.Context) (net.Conn, string, error) {
	return nil, "", fmt.Errorf("guest agent connection not implemented for krun driver")
}

func (l *LimaKrunDriver) Validate(ctx context.Context) error {
	return nil
}

func (l *LimaKrunDriver) FillConfig(ctx context.Context, cfg *limatype.LimaYAML, filePath string) error {
	if cfg.MountType == nil {
		cfg.MountType = ptr.Of(limatype.VIRTIOFS)
	} else {
		*cfg.MountType = limatype.VIRTIOFS
	}

	cfg.VMType = ptr.Of("krunkit")

	return nil
}

func (l *LimaKrunDriver) BootScripts() (map[string][]byte, error) {
	return nil, nil
}

func (l *LimaKrunDriver) Create(ctx context.Context) error {
	return nil
}

func (l *LimaKrunDriver) Info() driver.Info {
	var info driver.Info
	info.Name = "krunkit"
	if l.Instance != nil && l.Instance.Dir != "" {
		info.InstanceDir = l.Instance.Dir
	}

	info.Features = driver.DriverFeatures{
		DynamicSSHAddress:    false,
		SkipSocketForwarding: false,
		CanRunGUI:            false,
	}
	return info
}

func (l *LimaKrunDriver) SSHAddress(ctx context.Context) (string, error) {
	return "127.0.0.1", nil
}
