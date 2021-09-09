package hostagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/lima-vm/lima/pkg/osutil"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
	"github.com/digitalocean/go-qemu/qmp/raw"
	"github.com/hashicorp/go-multierror"
	guestagentapi "github.com/lima-vm/lima/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/pkg/guestagent/api/client"
	hostagentapi "github.com/lima-vm/lima/pkg/hostagent/api"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/qemu"
	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/sshocker/pkg/ssh"

	"github.com/sirupsen/logrus"
)

type HostAgent struct {
	l             *logrus.Logger
	y             *limayaml.LimaYAML
	instDir       string
	sshConfig     *ssh.SSHConfig
	portForwarder *portForwarder
	onClose       []func() error // LIFO

	qExe     string
	qArgs    []string
	sigintCh chan os.Signal

	eventEnc   *json.Encoder
	eventEncMu sync.Mutex
}

// New creates the HostAgent.
//
// stdout is for emitting JSON lines of Events.
// stderr is for printing human-readable logs.
func New(instName string, stdout, stderr io.Writer, sigintCh chan os.Signal) (*HostAgent, error) {
	l := &logrus.Logger{
		Out:       stderr,
		Formatter: new(logrus.JSONFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}

	inst, err := store.Inspect(instName)
	if err != nil {
		return nil, err
	}

	y, err := inst.LoadYAML()
	if err != nil {
		return nil, err
	}
	// y is loaded with FillDefault() already, so no need to care about nil pointers.

	qCfg := qemu.Config{
		Name:        instName,
		InstanceDir: inst.Dir,
		LimaYAML:    y,
	}
	qExe, qArgs, err := qemu.Cmdline(qCfg)
	if err != nil {
		return nil, err
	}

	sshArgs, err := sshutil.SSHArgs(inst.Dir, *y.SSH.LoadDotSSHPubKeys)
	if err != nil {
		return nil, err
	}
	sshConfig := &ssh.SSHConfig{
		AdditionalArgs: sshArgs,
	}

	rules := make([]limayaml.PortForward, 0, 3+len(y.PortForwards))
	// Block ports 22 and sshLocalPort on all IPs
	for _, port := range []int{sshGuestPort, y.SSH.LocalPort} {
		rule := limayaml.PortForward{GuestIP: net.IPv4zero, GuestPort: port, Ignore: true}
		limayaml.FillPortForwardDefaults(&rule)
		rules = append(rules, rule)
	}
	rules = append(rules, y.PortForwards...)
	// Default forwards for all non-privileged ports from "127.0.0.1" and "::1"
	rule := limayaml.PortForward{GuestIP: guestagentapi.IPv4loopback1}
	limayaml.FillPortForwardDefaults(&rule)
	rules = append(rules, rule)

	a := &HostAgent{
		l:             l,
		y:             y,
		instDir:       inst.Dir,
		sshConfig:     sshConfig,
		portForwarder: newPortForwarder(l, sshConfig, y.SSH.LocalPort, rules),
		qExe:          qExe,
		qArgs:         qArgs,
		sigintCh:      sigintCh,
		eventEnc:      json.NewEncoder(stdout),
	}
	return a, nil
}

func (a *HostAgent) emitEvent(ctx context.Context, ev hostagentapi.Event) {
	a.eventEncMu.Lock()
	defer a.eventEncMu.Unlock()
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	if err := a.eventEnc.Encode(ev); err != nil {
		a.l.WithField("event", ev).WithError(err).Error("failed to emit an event")
	}
}

func logPipeRoutine(l *logrus.Logger, r io.Reader, header string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		l.Debugf("%s: %s", header, line)
	}
}

func (a *HostAgent) Run(ctx context.Context) error {
	defer func() {
		exitingEv := hostagentapi.Event{
			Status: hostagentapi.Status{
				Exiting: true,
			},
		}
		a.emitEvent(ctx, exitingEv)
	}()

	qCmd := exec.CommandContext(ctx, a.qExe, a.qArgs...)
	qStdout, err := qCmd.StdoutPipe()
	if err != nil {
		return err
	}
	go logPipeRoutine(a.l, qStdout, "qemu[stdout]")
	qStderr, err := qCmd.StderrPipe()
	if err != nil {
		return err
	}
	go logPipeRoutine(a.l, qStderr, "qemu[stderr]")

	a.l.Infof("Starting QEMU (hint: to watch the boot progress, see %q)", filepath.Join(a.instDir, filenames.SerialLog))
	a.l.Debugf("qCmd.Args: %v", qCmd.Args)
	if err := qCmd.Start(); err != nil {
		return err
	}
	qWaitCh := make(chan error)
	go func() {
		qWaitCh <- qCmd.Wait()
	}()

	sshLocalPort := a.y.SSH.LocalPort
	if sshLocalPort == 0 {
		sshLocalPort = osutil.CheckOrGetFreePort()
	}
	stBase := hostagentapi.Status{
		SSHLocalPort: sshLocalPort,
	}
	stBooting := stBase
	a.emitEvent(ctx, hostagentapi.Event{Status: stBooting})

	go func() {
		stRunning := stBase
		if haErr := a.startHostAgentRoutines(ctx); haErr != nil {
			stRunning.Degraded = true
			stRunning.Errors = append(stRunning.Errors, haErr.Error())
		}
		stRunning.Running = true
		a.emitEvent(ctx, hostagentapi.Event{Status: stRunning})
	}()

	for {
		select {
		case <-a.sigintCh:
			a.l.Info("Received SIGINT, shutting down the host agent")
			if closeErr := a.close(); closeErr != nil {
				a.l.WithError(closeErr).Warn("an error during shutting down the host agent")
			}
			return a.shutdownQEMU(ctx, 3*time.Minute, qCmd, qWaitCh)
		case qWaitErr := <-qWaitCh:
			a.l.WithError(qWaitErr).Info("QEMU has exited")
			return qWaitErr
		}
	}
}

func (a *HostAgent) shutdownQEMU(ctx context.Context, timeout time.Duration, qCmd *exec.Cmd, qWaitCh <-chan error) error {
	a.l.Info("Shutting down QEMU with ACPI")
	qmpSockPath := filepath.Join(a.instDir, filenames.QMPSock)
	qmpClient, err := qmp.NewSocketMonitor("unix", qmpSockPath, 5*time.Second)
	if err != nil {
		a.l.WithError(err).Warnf("failed to open the QMP socket %q, forcibly killing QEMU", qmpSockPath)
		return a.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
	if err := qmpClient.Connect(); err != nil {
		a.l.WithError(err).Warnf("failed to connect to the QMP socket %q, forcibly killing QEMU", qmpSockPath)
		return a.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
	defer func() { _ = qmpClient.Disconnect() }()
	rawClient := raw.NewMonitor(qmpClient)
	a.l.Info("Sending QMP system_powerdown command")
	if err := rawClient.SystemPowerdown(); err != nil {
		a.l.WithError(err).Warnf("failed to send system_powerdown command via the QMP socket %q, forcibly killing QEMU", qmpSockPath)
		return a.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
	deadline := time.After(timeout)
	select {
	case qWaitErr := <-qWaitCh:
		a.l.WithError(qWaitErr).Info("QEMU has exited")
		return qWaitErr
	case <-deadline:
	}
	a.l.Warnf("QEMU did not exit in %v, forcibly killing QEMU", timeout)
	return a.killQEMU(ctx, timeout, qCmd, qWaitCh)
}

func (a *HostAgent) killQEMU(ctx context.Context, timeout time.Duration, qCmd *exec.Cmd, qWaitCh <-chan error) error {
	if killErr := qCmd.Process.Kill(); killErr != nil {
		a.l.WithError(killErr).Warn("failed to kill QEMU")
	}
	qWaitErr := <-qWaitCh
	a.l.WithError(qWaitErr).Info("QEMU has exited, after killing forcibly")
	qemuPIDPath := filepath.Join(a.instDir, filenames.QemuPID)
	_ = os.RemoveAll(qemuPIDPath)
	return qWaitErr
}

func (a *HostAgent) startHostAgentRoutines(ctx context.Context) error {
	a.onClose = append(a.onClose, func() error {
		a.l.Debugf("shutting down the SSH master")
		if exitMasterErr := ssh.ExitMaster("127.0.0.1", a.y.SSH.LocalPort, a.sshConfig); exitMasterErr != nil {
			a.l.WithError(exitMasterErr).Warn("failed to exit SSH master")
		}
		return nil
	})
	var mErr error
	if err := a.waitForRequirements(ctx, "essential", a.essentialRequirements()); err != nil {
		mErr = multierror.Append(mErr, err)
	}
	mounts, err := a.setupMounts(ctx)
	if err != nil {
		mErr = multierror.Append(mErr, err)
	}
	a.onClose = append(a.onClose, func() error {
		var unmountMErr error
		for _, m := range mounts {
			if unmountErr := m.close(); unmountErr != nil {
				unmountMErr = multierror.Append(unmountMErr, unmountErr)
			}
		}
		return unmountMErr
	})
	go a.watchGuestAgentEvents(ctx)
	if err := a.waitForRequirements(ctx, "optional", a.optionalRequirements()); err != nil {
		mErr = multierror.Append(mErr, err)
	}
	return mErr
}

func (a *HostAgent) close() error {
	a.l.Infof("Shutting down the host agent")
	var mErr error
	for i := len(a.onClose) - 1; i >= 0; i-- {
		f := a.onClose[i]
		if err := f(); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}
	return mErr
}

func (a *HostAgent) watchGuestAgentEvents(ctx context.Context) {
	// TODO: use vSock (when QEMU for macOS gets support for vSock)

	localUnix := filepath.Join(a.instDir, filenames.GuestAgentSock)
	// guest should have same UID as the host (specified in cidata)
	remoteUnix := fmt.Sprintf("/run/user/%d/lima-guestagent.sock", os.Getuid())

	for {
		if !isGuestAgentSocketAccessible(ctx, localUnix) {
			if err := os.RemoveAll(localUnix); err != nil {
				a.l.WithError(err).Warnf("failed to clean up %q (host) before setting up forwarding", localUnix)
			}
			a.l.Infof("Forwarding %q (guest) to %q (host)", remoteUnix, localUnix)
			if err := forwardSSH(ctx, a.sshConfig, a.y.SSH.LocalPort, localUnix, remoteUnix, false); err != nil {
				a.l.WithError(err).Warnf("failed to setting up forward from %q (guest) to %q (host)", remoteUnix, localUnix)
			}
		}
		if err := a.processGuestAgentEvents(ctx, localUnix); err != nil {
			a.l.WithError(err).Warn("connection to the guest agent was closed unexpectedly")
		}
		select {
		case <-ctx.Done():
			a.l.Infof("Stopping forwarding %q to %q", remoteUnix, localUnix)
			verbCancel := true
			if err := forwardSSH(ctx, a.sshConfig, a.y.SSH.LocalPort, localUnix, remoteUnix, verbCancel); err != nil {
				a.l.WithError(err).Warnf("failed to stop forwarding %q (remote) to %q (local)", remoteUnix, localUnix)
			}
			if err := os.RemoveAll(localUnix); err != nil {
				a.l.WithError(err).Warnf("failed to clean up %q (host) after stopping forwarding", localUnix)
			}
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func isGuestAgentSocketAccessible(ctx context.Context, localUnix string) bool {
	client, err := guestagentclient.NewGuestAgentClient(localUnix)
	if err != nil {
		return false
	}
	_, err = client.Info(ctx)
	return err == nil
}

func (a *HostAgent) processGuestAgentEvents(ctx context.Context, localUnix string) error {
	client, err := guestagentclient.NewGuestAgentClient(localUnix)
	if err != nil {
		return err
	}

	info, err := client.Info(ctx)
	if err != nil {
		return err
	}

	a.l.Debugf("guest agent info: %+v", info)

	onEvent := func(ev guestagentapi.Event) {
		a.l.Debugf("guest agent event: %+v", ev)
		for _, f := range ev.Errors {
			a.l.Warnf("received error from the guest: %q", f)
		}
		a.portForwarder.OnEvent(ctx, ev)
	}

	if err := client.Events(ctx, onEvent); err != nil {
		return err
	}
	return io.EOF
}

func forwardSSH(ctx context.Context, sshConfig *ssh.SSHConfig, port int, local, remote string, cancel bool) error {
	args := sshConfig.Args()
	verb := "forward"
	if cancel {
		verb = "cancel"
	}
	args = append(args,
		"-T",
		"-O", verb,
		"-L", local+":"+remote,
		"-N",
		"-f",
		"-p", strconv.Itoa(port),
		"127.0.0.1",
		"--",
	)
	cmd := exec.CommandContext(ctx, sshConfig.Binary(), args...)
	if out, err := cmd.Output(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
}
