// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/digitalocean/go-qemu/qmp/raw"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driver/qemu/entitlementutil"
	"github.com/lima-vm/lima/pkg/executil"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/networks/usernet"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

type LimaQemuDriver struct {
	Instance     *store.Instance
	SSHLocalPort int
	VSockPort    int
	VirtioPort   string

	DriverType driver.DriverType

	qCmd    *exec.Cmd
	qWaitCh chan error

	vhostCmds []*exec.Cmd
}

var _ driver.Driver = (*LimaQemuDriver)(nil)

func New(driverType driver.DriverType) *LimaQemuDriver {
	// virtserialport doesn't seem to work reliably: https://github.com/lima-vm/lima/issues/2064
	// but on Windows default Unix socket forwarding is not available
	var virtioPort string
	virtioPort = filenames.VirtioPort
	if runtime.GOOS != "windows" {
		virtioPort = ""
	}
	return &LimaQemuDriver{
		VSockPort:  0,
		VirtioPort: virtioPort,
		DriverType: driverType,
	}
}

func (l *LimaQemuDriver) SetConfig(inst *store.Instance, sshLocalPort int) {
	l.Instance = inst
	l.SSHLocalPort = sshLocalPort
}

func (l *LimaQemuDriver) Validate() error {
	if runtime.GOOS == "darwin" {
		if err := l.checkBinarySignature(); err != nil {
			return err
		}
	}

	if *l.Instance.Config.MountType == limayaml.VIRTIOFS && runtime.GOOS != "linux" {
		return fmt.Errorf("field `mountType` must be %q or %q for QEMU driver on non-Linux, got %q",
			limayaml.REVSSHFS, limayaml.NINEP, *l.Instance.Config.MountType)
	}
	return nil
}

func (l *LimaQemuDriver) CreateDisk(ctx context.Context) error {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return EnsureDisk(ctx, qCfg)
}

func (l *LimaQemuDriver) Start(ctx context.Context) (chan error, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if l.qCmd == nil {
			cancel()
		}
	}()

	qCfg := Config{
		Name:         l.Instance.Name,
		InstanceDir:  l.Instance.Dir,
		LimaYAML:     l.Instance.Config,
		SSHLocalPort: l.SSHLocalPort,
		SSHAddress:   l.Instance.SSHAddress,
		VirtioGA:     l.VirtioPort != "",
	}
	qExe, qArgs, err := Cmdline(ctx, qCfg)
	if err != nil {
		return nil, err
	}

	var vhostCmds []*exec.Cmd
	if *l.Instance.Config.MountType == limayaml.VIRTIOFS {
		vhostExe, err := FindVirtiofsd(qExe)
		if err != nil {
			return nil, err
		}

		for i := range l.Instance.Config.Mounts {
			args, err := VirtiofsdCmdline(qCfg, i)
			if err != nil {
				return nil, err
			}

			vhostCmds = append(vhostCmds, exec.CommandContext(ctx, vhostExe, args...))
		}
	}

	var qArgsFinal []string
	applier := &qArgTemplateApplier{}
	for _, unapplied := range qArgs {
		applied, err := applier.applyTemplate(unapplied)
		if err != nil {
			return nil, err
		}
		qArgsFinal = append(qArgsFinal, applied)
	}
	qCmd := exec.CommandContext(ctx, qExe, qArgsFinal...)
	qCmd.ExtraFiles = append(qCmd.ExtraFiles, applier.files...)
	qCmd.SysProcAttr = executil.BackgroundSysProcAttr
	qStdout, err := qCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	go logPipeRoutine(qStdout, "qemu[stdout]")
	qStderr, err := qCmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	go logPipeRoutine(qStderr, "qemu[stderr]")

	for i, vhostCmd := range vhostCmds {
		vhostStdout, err := vhostCmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		go logPipeRoutine(vhostStdout, fmt.Sprintf("virtiofsd-%d[stdout]", i))
		vhostStderr, err := vhostCmd.StderrPipe()
		if err != nil {
			return nil, err
		}
		go logPipeRoutine(vhostStderr, fmt.Sprintf("virtiofsd-%d[stderr]", i))
	}

	for i, vhostCmd := range vhostCmds {
		logrus.Debugf("vhostCmd[%d].Args: %v", i, vhostCmd.Args)
		if err := vhostCmd.Start(); err != nil {
			return nil, err
		}

		vhostWaitCh := make(chan error)
		go func() {
			vhostWaitCh <- vhostCmd.Wait()
		}()

		vhostSock := filepath.Join(l.Instance.Dir, fmt.Sprintf(filenames.VhostSock, i))
		vhostSockExists := false
		for attempt := range 5 {
			logrus.Debugf("Try waiting for %s to appear (attempt %d)", vhostSock, attempt)

			if _, err := os.Stat(vhostSock); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					logrus.Warnf("Failed to check for vhost socket: %v", err)
				}
			} else {
				vhostSockExists = true
				break
			}

			retry := time.NewTimer(200 * time.Millisecond)
			select {
			case err = <-vhostWaitCh:
				return nil, fmt.Errorf("virtiofsd never created vhost socket: %w", err)
			case <-retry.C:
			}
		}

		if !vhostSockExists {
			return nil, fmt.Errorf("vhost socket %s never appeared", vhostSock)
		}

		go func() {
			if err := <-vhostWaitCh; err != nil {
				logrus.Errorf("Error from virtiofsd instance #%d: %v", i, err)
			}
		}()
	}

	logrus.Infof("Starting QEMU (hint: to watch the boot progress, see %q)", filepath.Join(qCfg.InstanceDir, "serial*.log"))
	logrus.Debugf("qCmd.Args: %v", qCmd.Args)
	if err := qCmd.Start(); err != nil {
		return nil, err
	}
	l.qCmd = qCmd
	l.qWaitCh = make(chan error)
	go func() {
		l.qWaitCh <- qCmd.Wait()
	}()
	l.vhostCmds = vhostCmds
	go func() {
		if usernetIndex := limayaml.FirstUsernetIndex(l.Instance.Config); usernetIndex != -1 {
			client := usernet.NewClientByName(l.Instance.Config.Networks[usernetIndex].Lima)
			err := client.ConfigureDriver(ctx, l.Instance, l.SSHLocalPort)
			if err != nil {
				l.qWaitCh <- err
			}
		}
	}()
	return l.qWaitCh, nil
}

func (l *LimaQemuDriver) Stop(ctx context.Context) error {
	return l.shutdownQEMU(ctx, 3*time.Minute, l.qCmd, l.qWaitCh)
}

func (l *LimaQemuDriver) ChangeDisplayPassword(_ context.Context, password string) error {
	return l.changeVNCPassword(password)
}

func (l *LimaQemuDriver) GetDisplayConnection(_ context.Context) (string, error) {
	return l.getVNCDisplayPort()
}

func waitFileExists(path string, timeout time.Duration) error {
	startWaiting := time.Now()
	for {
		_, err := os.Stat(path)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if time.Since(startWaiting) > timeout {
			return fmt.Errorf("timeout waiting for %s", path)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// Ask the user to sign the qemu binary with the "com.apple.security.hypervisor" if needed.
// Workaround for https://github.com/lima-vm/lima/issues/1742
func (l *LimaQemuDriver) checkBinarySignature() error {
	macOSProductVersion, err := osutil.ProductVersion()
	if err != nil {
		return err
	}
	// The codesign --xml option is only available on macOS Monterey and later
	if !macOSProductVersion.LessThan(*semver.New("12.0.0")) {
		qExe, _, err := Exe(l.Instance.Arch)
		if err != nil {
			return fmt.Errorf("failed to find the QEMU binary for the architecture %q: %w", l.Instance.Arch, err)
		}
		if accel := Accel(l.Instance.Arch); accel == "hvf" {
			entitlementutil.AskToSignIfNotSignedProperly(qExe)
		}
	}

	return nil
}

func (l *LimaQemuDriver) changeVNCPassword(password string) error {
	qmpSockPath := filepath.Join(l.Instance.Dir, filenames.QMPSock)
	err := waitFileExists(qmpSockPath, 30*time.Second)
	if err != nil {
		return err
	}
	qmpClient, err := qmp.NewSocketMonitor("unix", qmpSockPath, 5*time.Second)
	if err != nil {
		return err
	}
	if err := qmpClient.Connect(); err != nil {
		return err
	}
	defer func() { _ = qmpClient.Disconnect() }()
	rawClient := raw.NewMonitor(qmpClient)
	err = rawClient.ChangeVNCPassword(password)
	if err != nil {
		return err
	}
	return nil
}

func (l *LimaQemuDriver) getVNCDisplayPort() (string, error) {
	qmpSockPath := filepath.Join(l.Instance.Dir, filenames.QMPSock)
	qmpClient, err := qmp.NewSocketMonitor("unix", qmpSockPath, 5*time.Second)
	if err != nil {
		return "", err
	}
	if err := qmpClient.Connect(); err != nil {
		return "", err
	}
	defer func() { _ = qmpClient.Disconnect() }()
	rawClient := raw.NewMonitor(qmpClient)
	info, err := rawClient.QueryVNC()
	if err != nil {
		return "", err
	}
	return *info.Service, nil
}

func (l *LimaQemuDriver) removeVNCFiles() error {
	vncfile := filepath.Join(l.Instance.Dir, filenames.VNCDisplayFile)
	err := os.RemoveAll(vncfile)
	if err != nil {
		return err
	}
	vncpwdfile := filepath.Join(l.Instance.Dir, filenames.VNCPasswordFile)
	err = os.RemoveAll(vncpwdfile)
	if err != nil {
		return err
	}
	return nil
}

func (l *LimaQemuDriver) killVhosts() error {
	var errs []error
	for i, vhost := range l.vhostCmds {
		if err := vhost.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			errs = append(errs, fmt.Errorf("failed to kill virtiofsd instance #%d: %w", i, err))
		}
	}

	return errors.Join(errs...)
}

func (l *LimaQemuDriver) shutdownQEMU(ctx context.Context, timeout time.Duration, qCmd *exec.Cmd, qWaitCh <-chan error) error {
	// "power button" refers to ACPI on the most archs, except RISC-V
	logrus.Info("Shutting down QEMU with the power button")
	if usernetIndex := limayaml.FirstUsernetIndex(l.Instance.Config); usernetIndex != -1 {
		client := usernet.NewClientByName(l.Instance.Config.Networks[usernetIndex].Lima)
		err := client.UnExposeSSH(l.SSHLocalPort)
		if err != nil {
			logrus.Warnf("Failed to remove SSH binding for port %d", l.SSHLocalPort)
		}
	}
	qmpSockPath := filepath.Join(l.Instance.Dir, filenames.QMPSock)
	qmpClient, err := qmp.NewSocketMonitor("unix", qmpSockPath, 5*time.Second)
	if err != nil {
		logrus.WithError(err).Warnf("failed to open the QMP socket %q, forcibly killing QEMU", qmpSockPath)
		return l.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
	if err := qmpClient.Connect(); err != nil {
		logrus.WithError(err).Warnf("failed to connect to the QMP socket %q, forcibly killing QEMU", qmpSockPath)
		return l.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
	defer func() { _ = qmpClient.Disconnect() }()
	rawClient := raw.NewMonitor(qmpClient)
	logrus.Info("Sending QMP system_powerdown command")
	if err := rawClient.SystemPowerdown(); err != nil {
		logrus.WithError(err).Warnf("failed to send system_powerdown command via the QMP socket %q, forcibly killing QEMU", qmpSockPath)
		return l.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
	deadline := time.After(timeout)
	select {
	case qWaitErr := <-qWaitCh:
		entry := logrus.NewEntry(logrus.StandardLogger())
		if qWaitErr != nil {
			entry = entry.WithError(qWaitErr)
		}
		entry.Info("QEMU has exited")
		_ = l.removeVNCFiles()
		return errors.Join(qWaitErr, l.killVhosts())
	case <-deadline:
	}
	logrus.Warnf("QEMU did not exit in %v, forcibly killing QEMU", timeout)
	return l.killQEMU(ctx, timeout, qCmd, qWaitCh)
}

func (l *LimaQemuDriver) killQEMU(_ context.Context, _ time.Duration, qCmd *exec.Cmd, qWaitCh <-chan error) error {
	var qWaitErr error
	if qCmd.ProcessState == nil {
		if killErr := qCmd.Process.Kill(); killErr != nil {
			logrus.WithError(killErr).Warn("failed to kill QEMU")
		}
		qWaitErr = <-qWaitCh
		logrus.WithError(qWaitErr).Info("QEMU has exited, after killing forcibly")
	} else {
		logrus.Info("QEMU has already exited")
	}
	qemuPIDPath := filepath.Join(l.Instance.Dir, filenames.PIDFile(*l.Instance.Config.VMType))
	_ = os.RemoveAll(qemuPIDPath)
	_ = l.removeVNCFiles()
	return errors.Join(qWaitErr, l.killVhosts())
}

func logPipeRoutine(r io.Reader, header string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		logrus.Debugf("%s: %s", header, line)
	}
}

func (l *LimaQemuDriver) DeleteSnapshot(_ context.Context, tag string) error {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return Del(qCfg, l.Instance.Status == store.StatusRunning, tag)
}

func (l *LimaQemuDriver) CreateSnapshot(_ context.Context, tag string) error {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return Save(qCfg, l.Instance.Status == store.StatusRunning, tag)
}

func (l *LimaQemuDriver) ApplySnapshot(_ context.Context, tag string) error {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return Load(qCfg, l.Instance.Status == store.StatusRunning, tag)
}

func (l *LimaQemuDriver) ListSnapshots(_ context.Context) (string, error) {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return List(qCfg, l.Instance.Status == store.StatusRunning)
}

func (l *LimaQemuDriver) GuestAgentConn(ctx context.Context) (net.Conn, error) {
	var d net.Dialer
	dialContext, err := d.DialContext(ctx, "unix", filepath.Join(l.Instance.Dir, filenames.GuestAgentSock))
	return dialContext, err
}

type qArgTemplateApplier struct {
	files []*os.File
}

func (a *qArgTemplateApplier) applyTemplate(qArg string) (string, error) {
	if !strings.Contains(qArg, "{{") {
		return qArg, nil
	}
	funcMap := template.FuncMap{
		"fd_connect": func(v any) string {
			fn := func(v any) (string, error) {
				s, ok := v.(string)
				if !ok {
					return "", fmt.Errorf("non-string argument %+v", v)
				}
				addr, err := net.ResolveUnixAddr("unix", s)
				if err != nil {
					return "", err
				}
				conn, err := net.DialUnix("unix", nil, addr)
				if err != nil {
					return "", err
				}
				f, err := conn.File()
				if err != nil {
					return "", err
				}
				if err := conn.Close(); err != nil {
					return "", err
				}
				a.files = append(a.files, f)
				fd := len(a.files) + 2 // the first FD is 3
				return strconv.Itoa(fd), nil
			}
			res, err := fn(v)
			if err != nil {
				panic(fmt.Errorf("fd_connect: %w", err))
			}
			return res
		},
	}
	tmpl, err := template.New("").Funcs(funcMap).Parse(qArg)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, nil); err != nil {
		return "", err
	}
	return b.String(), nil
}

func (l *LimaQemuDriver) GetInfo() driver.Info {
	var info driver.Info
	if l.Instance != nil && l.Instance.Dir != "" {
		info.InstanceDir = l.Instance.Dir
	}
	info.DriverName = "qemu"
	info.CanRunGUI = false
	info.VirtioPort = l.VirtioPort
	info.VsockPort = l.VSockPort
	return info
}

func (l *LimaQemuDriver) Initialize(_ context.Context) error {
	return nil
}

func (l *LimaQemuDriver) RunGUI() error {
	return nil
}

func (l *LimaQemuDriver) Register(_ context.Context) error {
	return nil
}

func (l *LimaQemuDriver) Unregister(_ context.Context) error {
	return nil
}

func (l *LimaQemuDriver) ForwardGuestAgent() bool {
	// if driver is not providing, use host agent
	return l.VSockPort == 0 && l.VirtioPort == ""
}
