package hostagent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
	"github.com/digitalocean/go-qemu/qmp/raw"
	"github.com/hashicorp/go-multierror"
	"github.com/lima-vm/lima/pkg/cidata"
	guestagentapi "github.com/lima-vm/lima/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/pkg/guestagent/api/client"
	hostagentapi "github.com/lima-vm/lima/pkg/hostagent/api"
	"github.com/lima-vm/lima/pkg/hostagent/dns"
	"github.com/lima-vm/lima/pkg/hostagent/events"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/qemu"
	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

type HostAgent struct {
	y               *limayaml.LimaYAML
	sshLocalPort    int
	udpDNSLocalPort int
	tcpDNSLocalPort int
	instDir         string
	sshConfig       *ssh.SSHConfig
	portForwarder   *portForwarder
	onClose         []func() error // LIFO

	qExe     string
	qArgs    []string
	sigintCh chan os.Signal

	eventEnc   *json.Encoder
	eventEncMu sync.Mutex
}

type options struct {
	nerdctlArchive string // local path, not URL
}

type Opt func(*options) error

func WithNerdctlArchive(s string) Opt {
	return func(o *options) error {
		o.nerdctlArchive = s
		return nil
	}
}

// New creates the HostAgent.
//
// stdout is for emitting JSON lines of Events.
func New(instName string, stdout io.Writer, sigintCh chan os.Signal, opts ...Opt) (*HostAgent, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
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

	sshLocalPort, err := determineSSHLocalPort(y, instName)
	if err != nil {
		return nil, err
	}

	var udpDNSLocalPort, tcpDNSLocalPort int
	if *y.HostResolver.Enabled {
		udpDNSLocalPort, err = findFreeUDPLocalPort()
		if err != nil {
			return nil, err
		}
		tcpDNSLocalPort, err = findFreeTCPLocalPort()
		if err != nil {
			return nil, err
		}
	}

	if err := cidata.GenerateISO9660(inst.Dir, instName, y, udpDNSLocalPort, tcpDNSLocalPort, o.nerdctlArchive); err != nil {
		return nil, err
	}

	qCfg := qemu.Config{
		Name:         instName,
		InstanceDir:  inst.Dir,
		LimaYAML:     y,
		SSHLocalPort: sshLocalPort,
	}
	qExe, qArgs, err := qemu.Cmdline(qCfg)
	if err != nil {
		return nil, err
	}

	sshOpts, err := sshutil.SSHOpts(inst.Dir, *y.SSH.LoadDotSSHPubKeys, *y.SSH.ForwardAgent)
	if err != nil {
		return nil, err
	}
	sshConfig := &ssh.SSHConfig{
		AdditionalArgs: sshutil.SSHArgsFromOpts(sshOpts),
	}

	rules := make([]limayaml.PortForward, 0, 3+len(y.PortForwards))
	// Block ports 22 and sshLocalPort on all IPs
	for _, port := range []int{sshGuestPort, sshLocalPort} {
		rule := limayaml.PortForward{GuestIP: net.IPv4zero, GuestPort: port, Ignore: true}
		limayaml.FillPortForwardDefaults(&rule, inst.Dir)
		rules = append(rules, rule)
	}
	rules = append(rules, y.PortForwards...)
	// Default forwards for all non-privileged ports from "127.0.0.1" and "::1"
	rule := limayaml.PortForward{GuestIP: guestagentapi.IPv4loopback1}
	limayaml.FillPortForwardDefaults(&rule, inst.Dir)
	rules = append(rules, rule)

	a := &HostAgent{
		y:               y,
		sshLocalPort:    sshLocalPort,
		udpDNSLocalPort: udpDNSLocalPort,
		tcpDNSLocalPort: tcpDNSLocalPort,
		instDir:         inst.Dir,
		sshConfig:       sshConfig,
		portForwarder:   newPortForwarder(sshConfig, sshLocalPort, rules),
		qExe:            qExe,
		qArgs:           qArgs,
		sigintCh:        sigintCh,
		eventEnc:        json.NewEncoder(stdout),
	}
	return a, nil
}

func determineSSHLocalPort(y *limayaml.LimaYAML, instName string) (int, error) {
	if *y.SSH.LocalPort > 0 {
		return *y.SSH.LocalPort, nil
	}
	if *y.SSH.LocalPort < 0 {
		return 0, fmt.Errorf("invalid ssh local port %d", y.SSH.LocalPort)
	}
	switch instName {
	case "default":
		// use hard-coded value for "default" instance, for backward compatibility
		return 60022, nil
	default:
		sshLocalPort, err := findFreeTCPLocalPort()
		if err != nil {
			return 0, fmt.Errorf("failed to find a free port, try setting `ssh.localPort` manually: %w", err)
		}
		return sshLocalPort, nil
	}
}

func findFreeTCPLocalPort() (int, error) {
	lAddr0, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp4", lAddr0)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	lAddr := l.Addr()
	lTCPAddr, ok := lAddr.(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("expected *net.TCPAddr, got %v", lAddr)
	}
	port := lTCPAddr.Port
	if port <= 0 {
		return 0, fmt.Errorf("unexpected port %d", port)
	}
	return port, nil
}

func findFreeUDPLocalPort() (int, error) {
	lAddr0, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenUDP("udp4", lAddr0)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	lAddr := l.LocalAddr()
	lUDPAddr, ok := lAddr.(*net.UDPAddr)
	if !ok {
		return 0, fmt.Errorf("expected *net.UDPAddr, got %v", lAddr)
	}
	port := lUDPAddr.Port
	if port <= 0 {
		return 0, fmt.Errorf("unexpected port %d", port)
	}
	return port, nil
}

func (a *HostAgent) emitEvent(ctx context.Context, ev events.Event) {
	a.eventEncMu.Lock()
	defer a.eventEncMu.Unlock()
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	if err := a.eventEnc.Encode(ev); err != nil {
		logrus.WithField("event", ev).WithError(err).Error("failed to emit an event")
	}
}

func logPipeRoutine(r io.Reader, header string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		logrus.Debugf("%s: %s", header, line)
	}
}

func (a *HostAgent) Run(ctx context.Context) error {
	defer func() {
		exitingEv := events.Event{
			Status: events.Status{
				Exiting: true,
			},
		}
		a.emitEvent(ctx, exitingEv)
	}()

	if *a.y.HostResolver.Enabled {
		dnsServer, err := dns.Start(a.udpDNSLocalPort, a.tcpDNSLocalPort, *a.y.HostResolver.IPv6)
		if err != nil {
			return fmt.Errorf("cannot start DNS server: %w", err)
		}
		defer dnsServer.Shutdown()
	}

	qCmd := exec.CommandContext(ctx, a.qExe, a.qArgs...)
	qStdout, err := qCmd.StdoutPipe()
	if err != nil {
		return err
	}
	go logPipeRoutine(qStdout, "qemu[stdout]")
	qStderr, err := qCmd.StderrPipe()
	if err != nil {
		return err
	}
	go logPipeRoutine(qStderr, "qemu[stderr]")

	logrus.Infof("Starting QEMU (hint: to watch the boot progress, see %q)", filepath.Join(a.instDir, filenames.SerialLog))
	logrus.Debugf("qCmd.Args: %v", qCmd.Args)
	if err := qCmd.Start(); err != nil {
		return err
	}
	qWaitCh := make(chan error)
	go func() {
		qWaitCh <- qCmd.Wait()
	}()

	stBase := events.Status{
		SSHLocalPort: a.sshLocalPort,
	}
	stBooting := stBase
	a.emitEvent(ctx, events.Event{Status: stBooting})

	ctxHA, cancelHA := context.WithCancel(ctx)
	go func() {
		stRunning := stBase
		if haErr := a.startHostAgentRoutines(ctxHA); haErr != nil {
			stRunning.Degraded = true
			stRunning.Errors = append(stRunning.Errors, haErr.Error())
		}
		stRunning.Running = true
		a.emitEvent(ctx, events.Event{Status: stRunning})
	}()

	for {
		select {
		case <-a.sigintCh:
			logrus.Info("Received SIGINT, shutting down the host agent")
			cancelHA()
			if closeErr := a.close(); closeErr != nil {
				logrus.WithError(closeErr).Warn("an error during shutting down the host agent")
			}
			return a.shutdownQEMU(ctx, 3*time.Minute, qCmd, qWaitCh)
		case qWaitErr := <-qWaitCh:
			logrus.WithError(qWaitErr).Info("QEMU has exited")
			// lint insists that we need to call cancelHA() on all possible codepaths
			cancelHA()
			return qWaitErr
		}
	}
}
func (a *HostAgent) Info(ctx context.Context) (*hostagentapi.Info, error) {
	info := &hostagentapi.Info{
		SSHLocalPort: a.sshLocalPort,
	}
	return info, nil
}

func (a *HostAgent) shutdownQEMU(ctx context.Context, timeout time.Duration, qCmd *exec.Cmd, qWaitCh <-chan error) error {
	logrus.Info("Shutting down QEMU with ACPI")
	qmpSockPath := filepath.Join(a.instDir, filenames.QMPSock)
	qmpClient, err := qmp.NewSocketMonitor("unix", qmpSockPath, 5*time.Second)
	if err != nil {
		logrus.WithError(err).Warnf("failed to open the QMP socket %q, forcibly killing QEMU", qmpSockPath)
		return a.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
	if err := qmpClient.Connect(); err != nil {
		logrus.WithError(err).Warnf("failed to connect to the QMP socket %q, forcibly killing QEMU", qmpSockPath)
		return a.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
	defer func() { _ = qmpClient.Disconnect() }()
	rawClient := raw.NewMonitor(qmpClient)
	logrus.Info("Sending QMP system_powerdown command")
	if err := rawClient.SystemPowerdown(); err != nil {
		logrus.WithError(err).Warnf("failed to send system_powerdown command via the QMP socket %q, forcibly killing QEMU", qmpSockPath)
		return a.killQEMU(ctx, timeout, qCmd, qWaitCh)
	}
	deadline := time.After(timeout)
	select {
	case qWaitErr := <-qWaitCh:
		logrus.WithError(qWaitErr).Info("QEMU has exited")
		return qWaitErr
	case <-deadline:
	}
	logrus.Warnf("QEMU did not exit in %v, forcibly killing QEMU", timeout)
	return a.killQEMU(ctx, timeout, qCmd, qWaitCh)
}

func (a *HostAgent) killQEMU(ctx context.Context, timeout time.Duration, qCmd *exec.Cmd, qWaitCh <-chan error) error {
	if killErr := qCmd.Process.Kill(); killErr != nil {
		logrus.WithError(killErr).Warn("failed to kill QEMU")
	}
	qWaitErr := <-qWaitCh
	logrus.WithError(qWaitErr).Info("QEMU has exited, after killing forcibly")
	qemuPIDPath := filepath.Join(a.instDir, filenames.QemuPID)
	_ = os.RemoveAll(qemuPIDPath)
	return qWaitErr
}

func (a *HostAgent) startHostAgentRoutines(ctx context.Context) error {
	a.onClose = append(a.onClose, func() error {
		logrus.Debugf("shutting down the SSH master")
		if exitMasterErr := ssh.ExitMaster("127.0.0.1", a.sshLocalPort, a.sshConfig); exitMasterErr != nil {
			logrus.WithError(exitMasterErr).Warn("failed to exit SSH master")
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
	if err := a.waitForRequirements(ctx, "final", a.finalRequirements()); err != nil {
		mErr = multierror.Append(mErr, err)
	}
	return mErr
}

func (a *HostAgent) close() error {
	logrus.Infof("Shutting down the host agent")
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

	// Setup all socket forwards and defer their teardown
	logrus.Debugf("Forwarding unix sockets")
	for _, rule := range a.y.PortForwards {
		if rule.GuestSocket != "" {
			local := hostAddress(rule, guestagentapi.IPPort{})
			_ = forwardSSH(ctx, a.sshConfig, a.sshLocalPort, local, rule.GuestSocket, verbForward)
		}
	}

	localUnix := filepath.Join(a.instDir, filenames.GuestAgentSock)
	remoteUnix := "/run/lima-guestagent.sock"

	a.onClose = append(a.onClose, func() error {
		logrus.Debugf("Stop forwarding unix sockets")
		var mErr error
		for _, rule := range a.y.PortForwards {
			if rule.GuestSocket != "" {
				local := hostAddress(rule, guestagentapi.IPPort{})
				// using ctx.Background() because ctx has already been cancelled
				if err := forwardSSH(context.Background(), a.sshConfig, a.sshLocalPort, local, rule.GuestSocket, verbCancel); err != nil {
					mErr = multierror.Append(mErr, err)
				}
			}
		}
		if err := forwardSSH(context.Background(), a.sshConfig, a.sshLocalPort, localUnix, remoteUnix, verbCancel); err != nil {
			mErr = multierror.Append(mErr, err)
		}
		return mErr
	})

	for {
		if !isGuestAgentSocketAccessible(ctx, localUnix) {
			_ = forwardSSH(ctx, a.sshConfig, a.sshLocalPort, localUnix, remoteUnix, verbForward)
		}
		if err := a.processGuestAgentEvents(ctx, localUnix); err != nil {
			if !errors.Is(err, context.Canceled) {
				logrus.WithError(err).Warn("connection to the guest agent was closed unexpectedly")
			}
		}
		select {
		case <-ctx.Done():
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

	logrus.Debugf("guest agent info: %+v", info)

	onEvent := func(ev guestagentapi.Event) {
		logrus.Debugf("guest agent event: %+v", ev)
		for _, f := range ev.Errors {
			logrus.Warnf("received error from the guest: %q", f)
		}
		a.portForwarder.OnEvent(ctx, ev)
	}

	if err := client.Events(ctx, onEvent); err != nil {
		return err
	}
	return io.EOF
}

const (
	verbForward = "forward"
	verbCancel  = "cancel"
)

func forwardSSH(ctx context.Context, sshConfig *ssh.SSHConfig, port int, local, remote string, verb string) error {
	args := sshConfig.Args()
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
	if strings.HasPrefix(local, "/") {
		switch verb {
		case verbForward:
			logrus.Infof("Forwarding %q (guest) to %q (host)", remote, local)
			if err := os.RemoveAll(local); err != nil {
				logrus.WithError(err).Warnf("Failed to clean up %q (host) before setting up forwarding", local)
			}
			if err := os.MkdirAll(filepath.Dir(local), 0750); err != nil {
				return fmt.Errorf("can't create directory for local socket %q: %w", local, err)
			}
		case verbCancel:
			logrus.Infof("Stopping forwarding %q (guest) to %q (host)", remote, local)
			defer func() {
				if err := os.RemoveAll(local); err != nil {
					logrus.WithError(err).Warnf("Failed to clean up %q (host) after stopping forwarding", local)
				}
			}()
		default:
			panic(fmt.Errorf("invalid verb %q", verb))
		}
	}
	cmd := exec.CommandContext(ctx, sshConfig.Binary(), args...)
	if out, err := cmd.Output(); err != nil {
		if verb == verbForward && strings.HasPrefix(local, "/") {
			logrus.WithError(err).Warnf("Failed to set up forward from %q (guest) to %q (host)", remote, local)
			if removeErr := os.RemoveAll(local); err != nil {
				logrus.WithError(removeErr).Warnf("Failed to clean up %q (host) after forwarding failed", local)
			}
		}
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
}
