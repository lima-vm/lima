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
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/digitalocean/go-qemu/qmp/raw"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/driver/qemu/entitlementutil"
	"github.com/lima-vm/lima/v2/pkg/executil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/networks/usernet"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/version/versionutil"
)

type LimaQemuDriver struct {
	Instance     *limatype.Instance
	SSHLocalPort int
	vSockPort    int
	virtioPort   string

	qCmd    *exec.Cmd
	qWaitCh chan error

	vhostCmds []*exec.Cmd
}

var _ driver.Driver = (*LimaQemuDriver)(nil)

func New() *LimaQemuDriver {
	// virtserialport doesn't seem to work reliably: https://github.com/lima-vm/lima/issues/2064
	// but on Windows default Unix socket forwarding is not available
	var virtioPort string
	virtioPort = filenames.VirtioPort
	if runtime.GOOS != "windows" {
		virtioPort = ""
	}
	return &LimaQemuDriver{
		vSockPort:  0,
		virtioPort: virtioPort,
	}
}

func (l *LimaQemuDriver) Configure(inst *limatype.Instance) *driver.ConfiguredDriver {
	l.Instance = inst
	l.SSHLocalPort = inst.SSHLocalPort

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaQemuDriver) Validate(ctx context.Context) error {
	if runtime.GOOS == "darwin" {
		if err := l.checkBinarySignature(ctx); err != nil {
			return err
		}
	}
	if err := l.validateMountType(); err != nil {
		return err
	}

	return nil
}

// Helper method for mount type validation.
func (l *LimaQemuDriver) validateMountType() error {
	if l.Instance == nil || l.Instance.Config == nil {
		return errors.New("instance configuration is not set")
	}

	cfg := l.Instance.Config

	if cfg.MountType != nil && *cfg.MountType == limatype.VIRTIOFS && runtime.GOOS != "linux" {
		return fmt.Errorf("field `mountType` must be %q or %q for QEMU driver on non-Linux, got %q",
			limatype.REVSSHFS, limatype.NINEP, *cfg.MountType)
	}
	if cfg.MountTypesUnsupported != nil && cfg.MountType != nil && slices.Contains(cfg.MountTypesUnsupported, *cfg.MountType) {
		return fmt.Errorf("mount type %q is explicitly unsupported", *cfg.MountType)
	}
	if runtime.GOOS == "windows" && cfg.MountType != nil && *cfg.MountType == limatype.NINEP {
		return fmt.Errorf("mount type %q is not supported on Windows", limatype.NINEP)
	}

	return nil
}

func (l *LimaQemuDriver) FillConfig(cfg *limatype.LimaYAML, filePath string) error {
	if cfg.VMType == nil {
		cfg.VMType = ptr.Of(limatype.QEMU)
	}

	instDir := filepath.Dir(filePath)

	if cfg.Video.VNC.Display == nil || *cfg.Video.VNC.Display == "" {
		cfg.Video.VNC.Display = ptr.Of("127.0.0.1:0,to=9")
	}

	if cfg.VMOpts.QEMU.CPUType == nil {
		cfg.VMOpts.QEMU.CPUType = limatype.CPUType{}
	}

	//nolint:staticcheck // Migration of top-level CPUTYPE if specified
	if len(cfg.CPUType) > 0 {
		logrus.Warn("The top-level `cpuType` field is deprecated and will be removed in a future release. Please migrate to `vmOpts.qemu.cpuType`.")
		for arch, v := range cfg.CPUType {
			if v == "" {
				continue
			}
			if existing, ok := cfg.VMOpts.QEMU.CPUType[arch]; ok && existing != "" && existing != v {
				logrus.Warnf("Conflicting cpuType for arch %q: top-level=%q, vmOpts.qemu=%q; using vmOpts.qemu value", arch, v, existing)
				continue
			}
			cfg.VMOpts.QEMU.CPUType[arch] = v
		}
		cfg.CPUType = nil
	}

	mountTypesUnsupported := make(map[string]struct{})
	for _, f := range cfg.MountTypesUnsupported {
		mountTypesUnsupported[f] = struct{}{}
	}

	if runtime.GOOS == "windows" {
		// QEMU for Windows does not support 9p
		mountTypesUnsupported[limatype.NINEP] = struct{}{}
	}

	if cfg.MountType == nil || *cfg.MountType == "" || *cfg.MountType == "default" {
		cfg.MountType = ptr.Of(limatype.NINEP)
		if _, ok := mountTypesUnsupported[limatype.NINEP]; ok {
			// Use REVSSHFS if the instance does not support 9p
			cfg.MountType = ptr.Of(limatype.REVSSHFS)
		} else if limayaml.IsExistingInstanceDir(instDir) && !versionutil.GreaterEqual(limayaml.ExistingLimaVersion(instDir), "1.0.0") {
			// Use REVSSHFS if the instance was created with Lima prior to v1.0
			cfg.MountType = ptr.Of(limatype.REVSSHFS)
		}
	}

	for i := range cfg.Mounts {
		mount := &cfg.Mounts[i]
		if mount.Virtiofs.QueueSize == nil && *cfg.MountType == limatype.VIRTIOFS {
			cfg.Mounts[i].Virtiofs.QueueSize = ptr.Of(limayaml.DefaultVirtiofsQueueSize)
		}
	}

	if _, ok := mountTypesUnsupported[*cfg.MountType]; ok {
		return fmt.Errorf("mount type %q is explicitly unsupported", *cfg.MountType)
	}

	return nil
}

func (l *LimaQemuDriver) AcceptConfig(cfg *limatype.LimaYAML, _ string) error {
	if l.Instance == nil {
		l.Instance = &limatype.Instance{}
	}
	l.Instance.Config = cfg

	if err := l.Validate(context.Background()); err != nil {
		return fmt.Errorf("config not supported by the QEMU driver: %w", err)
	}

	if cfg.VMOpts.QEMU.MinimumVersion != nil {
		if _, err := semver.NewVersion(*cfg.VMOpts.QEMU.MinimumVersion); err != nil {
			return fmt.Errorf("field `vmOpts.qemu.minimumVersion` must be a semvar value, got %q: %w", *cfg.VMOpts.QEMU.MinimumVersion, err)
		}
	}

	if runtime.GOOS == "darwin" {
		if cfg.Arch != nil && limayaml.IsNativeArch(*cfg.Arch) {
			logrus.Debugf("ResolveVMType: resolved VMType %q (non-native arch=%q is specified in []*LimaYAML{o,y,d})", "qemu", *cfg.Arch)
			return nil
		}
		if limayaml.ResolveArch(cfg.Arch) == limatype.X8664 && cfg.Firmware.LegacyBIOS != nil && *cfg.Firmware.LegacyBIOS {
			logrus.Debugf("ResolveVMType: resolved VMType %q (firmware.legacyBIOS is specified in []*LimaYAML{o,y,d} on x86_64)", "qemu")
			return nil
		}
		if cfg.MountType != nil && *cfg.MountType == limatype.NINEP {
			logrus.Debugf("ResolveVMType: resolved VMType %q (mountType=%q is specified in []*LimaYAML{o,y,d})", "qemu", limatype.NINEP)
			return nil
		}
		if cfg.Audio.Device != nil {
			switch *cfg.Audio.Device {
			case "", "none", "default", "vz":
				// NOP
			default:
				logrus.Debugf("ResolveVMType: resolved VMType %q (audio.device=%q is specified in []*LimaYAML{o,y,d})", "qemu", *cfg.Audio.Device)
				return nil
			}
		}
		if cfg.Video.Display != nil {
			display := *cfg.Video.Display
			if display != "" && display != "none" && display != "default" && display != "vz" {
				logrus.Debugf("ResolveVMType: resolved VMType %q (video.display=%q is specified in []*LimaYAML{o,y,d})", "qemu", display)
				return nil
			}
		}
	}

	return nil
}

func (l *LimaQemuDriver) BootScripts() (map[string][]byte, error) {
	return nil, nil
}

func (l *LimaQemuDriver) CreateDisk(ctx context.Context) error {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return EnsureDisk(ctx, qCfg)
}

func (l *LimaQemuDriver) Start(_ context.Context) (chan error, error) {
	ctx, cancel := context.WithCancel(context.Background())
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
		VirtioGA:     l.virtioPort != "",
	}
	qExe, qArgs, err := Cmdline(ctx, qCfg)
	if err != nil {
		return nil, err
	}

	var vhostCmds []*exec.Cmd
	if *l.Instance.Config.MountType == limatype.VIRTIOFS {
		vhostExe, err := FindVirtiofsd(ctx, qExe)
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
	l.qWaitCh = make(chan error, 1)
	go func() {
		defer close(l.qWaitCh)
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

func (l *LimaQemuDriver) DisplayConnection(_ context.Context) (string, error) {
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
func (l *LimaQemuDriver) checkBinarySignature(ctx context.Context) error {
	macOSProductVersion, err := osutil.ProductVersion()
	if err != nil {
		return err
	}
	// The codesign --xml option is only available on macOS Monterey and later
	if !macOSProductVersion.LessThan(*semver.New("12.0.0")) && l.Instance.Arch != "" {
		qExe, _, err := Exe(l.Instance.Arch)
		if err != nil {
			return fmt.Errorf("failed to find the QEMU binary for the architecture %q: %w", l.Instance.Arch, err)
		}
		if accel := Accel(l.Instance.Arch); accel == "hvf" {
			entitlementutil.AskToSignIfNotSignedProperly(ctx, qExe)
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
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), timeout)
	defer timeoutCancel()

	select {
	case qWaitErr, ok := <-qWaitCh:
		if !ok {
			logrus.Info("QEMU wait channel was closed")
			_ = l.removeVNCFiles()
			return l.killVhosts()
		}
		entry := logrus.NewEntry(logrus.StandardLogger())
		if qWaitErr != nil {
			entry = entry.WithError(qWaitErr)
		}
		entry.Info("QEMU has exited")
		_ = l.removeVNCFiles()
		return errors.Join(qWaitErr, l.killVhosts())
	case <-timeoutCtx.Done():
		if qCmd.ProcessState != nil {
			logrus.Info("QEMU has already exited")
			_ = l.removeVNCFiles()
			return l.killVhosts()
		}
		logrus.Warnf("QEMU did not exit in %v, forcibly killing QEMU", timeout)
		return l.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
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

func (l *LimaQemuDriver) DeleteSnapshot(ctx context.Context, tag string) error {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return Del(ctx, qCfg, l.Instance.Status == limatype.StatusRunning, tag)
}

func (l *LimaQemuDriver) CreateSnapshot(ctx context.Context, tag string) error {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return Save(ctx, qCfg, l.Instance.Status == limatype.StatusRunning, tag)
}

func (l *LimaQemuDriver) ApplySnapshot(ctx context.Context, tag string) error {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return Load(ctx, qCfg, l.Instance.Status == limatype.StatusRunning, tag)
}

func (l *LimaQemuDriver) ListSnapshots(ctx context.Context) (string, error) {
	qCfg := Config{
		Name:        l.Instance.Name,
		InstanceDir: l.Instance.Dir,
		LimaYAML:    l.Instance.Config,
	}
	return List(ctx, qCfg, l.Instance.Status == limatype.StatusRunning)
}

func (l *LimaQemuDriver) GuestAgentConn(ctx context.Context) (net.Conn, string, error) {
	var d net.Dialer
	dialContext, err := d.DialContext(ctx, "unix", filepath.Join(l.Instance.Dir, filenames.GuestAgentSock))
	return dialContext, "unix", err
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

func (l *LimaQemuDriver) Info() driver.Info {
	var info driver.Info
	if l.Instance != nil && l.Instance.Dir != "" {
		info.InstanceDir = l.Instance.Dir
	}
	info.DriverName = "qemu"
	info.CanRunGUI = false
	info.VirtioPort = l.virtioPort
	info.VsockPort = l.vSockPort

	info.Features = driver.DriverFeatures{
		DynamicSSHAddress:    false,
		SkipSocketForwarding: false,
	}
	return info
}

func (l *LimaQemuDriver) SSHAddress(_ context.Context) (string, error) {
	return "127.0.0.1", nil
}

func (l *LimaQemuDriver) InspectStatus(_ context.Context, _ *limatype.Instance) string {
	return ""
}

func (l *LimaQemuDriver) Create(_ context.Context) error {
	return nil
}

func (l *LimaQemuDriver) Delete(_ context.Context) error {
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
	return l.vSockPort == 0 && l.virtioPort == ""
}
