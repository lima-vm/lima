// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sethvargo/go-password/password"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lima-vm/lima/v2/pkg/autostart"
	"github.com/lima-vm/lima/v2/pkg/cidata"
	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/driverutil"
	"github.com/lima-vm/lima/v2/pkg/freeport"
	guestagentapi "github.com/lima-vm/lima/v2/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/v2/pkg/guestagent/api/client"
	hostagentapi "github.com/lima-vm/lima/v2/pkg/hostagent/api"
	"github.com/lima-vm/lima/v2/pkg/hostagent/dns"
	"github.com/lima-vm/lima/v2/pkg/hostagent/events"
	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/portfwd"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/version/versionutil"
)

type HostAgent struct {
	instConfig        *limatype.LimaYAML
	sshLocalPort      int
	udpDNSLocalPort   int
	tcpDNSLocalPort   int
	instDir           string
	instName          string
	instSSHAddress    string
	sshConfig         *ssh.SSHConfig
	portForwarder     *portForwarder // legacy SSH port forwarder
	grpcPortForwarder *portfwd.Forwarder

	onClose   []func() error // LIFO
	onCloseMu sync.Mutex

	driver   driver.Driver
	signalCh chan os.Signal

	eventEnc   *json.Encoder
	eventEncMu sync.Mutex

	vSockPort  int
	virtioPort string

	clientMu sync.RWMutex
	client   *guestagentclient.GuestAgentClient

	guestAgentAliveCh     chan struct{} // closed on establishing the connection
	guestAgentAliveChOnce sync.Once

	showProgress bool // whether to show cloud-init progress

	statusMu      sync.RWMutex
	currentStatus events.Status
}

type options struct {
	guestAgentBinary string
	nerdctlArchive   string // local path, not URL
	showProgress     bool
}

type Opt func(*options) error

func WithGuestAgentBinary(s string) Opt {
	return func(o *options) error {
		o.guestAgentBinary = s
		return nil
	}
}

func WithNerdctlArchive(s string) Opt {
	return func(o *options) error {
		o.nerdctlArchive = s
		return nil
	}
}

func WithCloudInitProgress(enabled bool) Opt {
	return func(o *options) error {
		o.showProgress = enabled
		return nil
	}
}

// New creates the HostAgent.
//
// stdout is for emitting JSON lines of Events.
func New(ctx context.Context, instName string, stdout io.Writer, signalCh chan os.Signal, opts ...Opt) (*HostAgent, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
	}
	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return nil, err
	}

	var limaVersion string
	limaVersionFile := filepath.Join(inst.Dir, filenames.LimaVersion)
	if b, err := os.ReadFile(limaVersionFile); err == nil {
		limaVersion = strings.TrimSpace(string(b))
	} else if !errors.Is(err, os.ErrNotExist) {
		logrus.WithError(err).Warnf("Failed to read %q", limaVersionFile)
	}

	// inst.Config is loaded with FillDefault() already, so no need to care about nil pointers.
	sshLocalPort, err := determineSSHLocalPort(*inst.Config.SSH.LocalPort, instName, limaVersion)
	if err != nil {
		return nil, err
	}

	var udpDNSLocalPort, tcpDNSLocalPort int
	if *inst.Config.HostResolver.Enabled {
		udpDNSLocalPort, err = freeport.UDP()
		if err != nil {
			return nil, err
		}
		tcpDNSLocalPort, err = freeport.TCP()
		if err != nil {
			return nil, err
		}
	}

	limaDriver, err := driverutil.CreateConfiguredDriver(inst, sshLocalPort)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver instance: %w", err)
	}
	sshLocalPort = inst.SSHLocalPort

	vSockPort := limaDriver.Info().VsockPort
	virtioPort := limaDriver.Info().VirtioPort
	noCloudInit := limaDriver.Info().Features.NoCloudInit
	rosettaEnabled := limaDriver.Info().Features.RosettaEnabled
	rosettaBinFmt := limaDriver.Info().Features.RosettaBinFmt

	// Disable Rosetta in Plain mode
	if *inst.Config.Plain {
		rosettaEnabled = false
		rosettaBinFmt = false
	}

	if err := cidata.GenerateCloudConfig(ctx, inst.Dir, instName, inst.Config); err != nil {
		return nil, err
	}
	if err := cidata.GenerateISO9660(ctx, limaDriver, inst.Dir, instName, inst.Config, udpDNSLocalPort, tcpDNSLocalPort, o.guestAgentBinary, o.nerdctlArchive, vSockPort, virtioPort, noCloudInit, rosettaEnabled, rosettaBinFmt); err != nil {
		return nil, err
	}

	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		return nil, err
	}
	sshOpts, err := sshutil.SSHOpts(
		ctx,
		sshExe,
		inst.Dir,
		*inst.Config.User.Name,
		*inst.Config.SSH.LoadDotSSHPubKeys,
		*inst.Config.SSH.ForwardAgent,
		*inst.Config.SSH.ForwardX11,
		*inst.Config.SSH.ForwardX11Trusted)
	if err != nil {
		return nil, err
	}
	if err = writeSSHConfigFile("ssh", inst.Name, inst.Dir, inst.SSHAddress, sshLocalPort, sshOpts); err != nil {
		return nil, err
	}
	sshConfig := &ssh.SSHConfig{
		AdditionalArgs: sshutil.SSHArgsFromOpts(sshOpts),
	}

	ignoreTCP := false
	ignoreUDP := false
	for _, rule := range inst.Config.PortForwards {
		if rule.Ignore && rule.GuestPortRange[0] == 1 && rule.GuestPortRange[1] == 65535 {
			switch rule.Proto {
			case limatype.ProtoTCP:
				ignoreTCP = true
				logrus.Info("TCP port forwarding is disabled (except for SSH)")
			case limatype.ProtoUDP:
				ignoreUDP = true
				logrus.Info("UDP port forwarding is disabled")
			case limatype.ProtoAny:
				ignoreTCP = true
				ignoreUDP = true
				logrus.Info("TCP (except for SSH) and UDP port forwarding is disabled")
			}
		} else {
			break
		}
	}
	rules := make([]limatype.PortForward, 0, 3+len(inst.Config.PortForwards))
	// Block ports 22 and sshLocalPort on all IPs
	for _, port := range []int{sshGuestPort, sshLocalPort} {
		rule := limatype.PortForward{GuestIP: net.IPv4zero, GuestPort: port, Ignore: true}
		limayaml.FillPortForwardDefaults(&rule, inst.Dir, inst.Config.User, inst.Param)
		rules = append(rules, rule)
	}
	rules = append(rules, inst.Config.PortForwards...)
	// Default forwards for all non-privileged ports from "127.0.0.1" and "::1"
	rule := limatype.PortForward{}
	limayaml.FillPortForwardDefaults(&rule, inst.Dir, inst.Config.User, inst.Param)
	rules = append(rules, rule)

	a := &HostAgent{
		instConfig:        inst.Config,
		sshLocalPort:      sshLocalPort,
		udpDNSLocalPort:   udpDNSLocalPort,
		tcpDNSLocalPort:   tcpDNSLocalPort,
		instDir:           inst.Dir,
		instName:          instName,
		instSSHAddress:    inst.SSHAddress,
		sshConfig:         sshConfig,
		driver:            limaDriver,
		signalCh:          signalCh,
		eventEnc:          json.NewEncoder(stdout),
		vSockPort:         vSockPort,
		virtioPort:        virtioPort,
		guestAgentAliveCh: make(chan struct{}),
		showProgress:      o.showProgress,
	}
	a.grpcPortForwarder = portfwd.NewPortForwarder(rules, ignoreTCP, ignoreUDP, func(ev *events.PortForwardEvent) {
		a.emitPortForwardEvent(context.Background(), ev)
	})
	a.portForwarder = newPortForwarder(sshConfig, a.sshAddressPort, rules, ignoreTCP, inst.VMType, func(ev *events.PortForwardEvent) {
		a.emitPortForwardEvent(context.Background(), ev)
	})

	// Set up vsock event callback if the driver supports it
	if vsockEmitter, ok := limaDriver.Driver.(driver.VsockEventEmitter); ok {
		vsockEmitter.SetVsockEventCallback(func(ev *events.VsockEvent) {
			a.emitVsockEvent(context.Background(), ev)
		})
	}

	return a, nil
}

func writeSSHConfigFile(sshPath, instName, instDir, instSSHAddress string, sshLocalPort int, sshOpts []string) error {
	if instDir == "" {
		return fmt.Errorf("directory is unknown for the instance %q", instName)
	}
	b := bytes.NewBufferString(`# This SSH config file can be passed to 'ssh -F'.
# This file is created by Lima, but not used by Lima itself currently.
# Modifications to this file will be lost on restarting the Lima instance.
`)
	if runtime.GOOS == "windows" {
		// Remove ControlMaster, ControlPath, and ControlPersist options,
		// because Cygwin-based SSH clients do not support multiplexing when executing commands.
		// References:
		//   https://inbox.sourceware.org/cygwin/c98988a5-7e65-4282-b2a1-bb8e350d5fab@acm.org/T/
		//   https://stackoverflow.com/questions/20959792/is-ssh-controlmaster-with-cygwin-on-windows-actually-possible
		// By removing these options:
		//   - Avoids execution failures when the control master is not yet available.
		//   - Prevents error messages such as:
		//     > mux_client_request_session: read from master failed: Connection reset by peer
		//     > ControlSocket ....sock already exists, disabling multiplexing
		// Only remove these options when writing the SSH config file and executing `limactl shell`, since multiplexing seems to work with port forwarding.
		sshOpts = sshutil.SSHOptsRemovingControlPath(sshOpts)
	}
	if err := sshutil.Format(b, sshPath, instName, sshutil.FormatConfig,
		append(sshOpts,
			fmt.Sprintf("Hostname=%s", instSSHAddress),
			fmt.Sprintf("Port=%d", sshLocalPort),
		)); err != nil {
		return err
	}
	fileName := filepath.Join(instDir, filenames.SSHConfig)
	return os.WriteFile(fileName, b.Bytes(), 0o600)
}

func determineSSHLocalPort(confLocalPort int, instName, limaVersion string) (int, error) {
	if confLocalPort > 0 {
		return confLocalPort, nil
	}
	if confLocalPort < 0 {
		return 0, fmt.Errorf("invalid ssh local port %d", confLocalPort)
	}
	if versionutil.LessThan(limaVersion, "2.0.0") && instName == "default" {
		// use hard-coded value for "default" instance, for backward compatibility
		return 60022, nil
	}
	sshLocalPort, err := freeport.TCP()
	if err != nil {
		return 0, fmt.Errorf("failed to find a free port, try setting `ssh.localPort` manually: %w", err)
	}
	return sshLocalPort, nil
}

func (a *HostAgent) emitEvent(_ context.Context, ev events.Event) {
	a.eventEncMu.Lock()
	defer a.eventEncMu.Unlock()

	a.statusMu.Lock()
	a.currentStatus = ev.Status
	a.statusMu.Unlock()

	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	if err := a.eventEnc.Encode(ev); err != nil {
		logrus.WithField("event", ev).WithError(err).Error("failed to emit an event")
	}
}

func (a *HostAgent) emitCloudInitProgressEvent(ctx context.Context, progress *events.CloudInitProgress) {
	a.statusMu.RLock()
	currentStatus := a.currentStatus
	a.statusMu.RUnlock()

	currentStatus.CloudInitProgress = progress

	ev := events.Event{Status: currentStatus}
	a.emitEvent(ctx, ev)
}

func (a *HostAgent) emitPortForwardEvent(ctx context.Context, pfEvent *events.PortForwardEvent) {
	a.statusMu.RLock()
	currentStatus := a.currentStatus
	a.statusMu.RUnlock()

	currentStatus.PortForward = pfEvent

	ev := events.Event{Status: currentStatus}
	a.emitEvent(ctx, ev)
}

func (a *HostAgent) emitVsockEvent(ctx context.Context, vsockEvent *events.VsockEvent) {
	a.statusMu.RLock()
	currentStatus := a.currentStatus
	a.statusMu.RUnlock()

	currentStatus.Vsock = vsockEvent

	ev := events.Event{Status: currentStatus}
	a.emitEvent(ctx, ev)
}

func generatePassword(length int) (string, error) {
	// avoid any special symbols, to make it easier to copy/paste
	return password.Generate(length, length/4, 0, false, false)
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
	adjustNofileRlimit()

	if limayaml.FirstUsernetIndex(a.instConfig) == -1 && *a.instConfig.HostResolver.Enabled {
		hosts := a.instConfig.HostResolver.Hosts
		if hosts == nil {
			hosts = make(map[string]string)
		}
		hosts["host.lima.internal"] = networks.SlirpGateway
		name := hostname.FromInstName(a.instName) // TODO: support customization
		hosts[name] = networks.SlirpIPAddress
		srvOpts := dns.ServerOptions{
			UDPPort: a.udpDNSLocalPort,
			TCPPort: a.tcpDNSLocalPort,
			Address: "127.0.0.1",
			HandlerOptions: dns.HandlerOptions{
				IPv6:        *a.instConfig.HostResolver.IPv6,
				StaticHosts: hosts,
			},
		}
		dnsServer, err := dns.Start(srvOpts)
		if err != nil {
			return fmt.Errorf("cannot start DNS server: %w", err)
		}
		defer dnsServer.Shutdown()
	}

	errCh, err := a.driver.Start(ctx)
	if err != nil {
		return err
	}

	if err := a.driver.AdditionalSetupForSSH(ctx); err != nil {
		return err
	}

	// WSL instance SSH address isn't known until after VM start
	if a.driver.Info().Features.DynamicSSHAddress {
		sshAddr, err := a.driver.SSHAddress(ctx)
		if err != nil {
			return err
		}
		a.instSSHAddress = sshAddr
	}

	if a.instConfig.Video.Display != nil && *a.instConfig.Video.Display == "vnc" {
		vncdisplay, vncoptions, _ := strings.Cut(*a.instConfig.Video.VNC.Display, ",")
		vnchost, vncnum, err := net.SplitHostPort(vncdisplay)
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(vncnum)
		if err != nil {
			return err
		}
		vncport := strconv.Itoa(5900 + n)
		vncpwdfile := filepath.Join(a.instDir, filenames.VNCPasswordFile)
		vncpasswd, err := generatePassword(8)
		if err != nil {
			return err
		}
		if err := a.driver.ChangeDisplayPassword(ctx, vncpasswd); err != nil {
			return err
		}
		if err := os.WriteFile(vncpwdfile, []byte(vncpasswd), 0o600); err != nil {
			return err
		}
		if strings.Contains(vncoptions, "to=") {
			vncport, err = a.driver.DisplayConnection(ctx)
			if err != nil {
				return err
			}
			p, err := strconv.Atoi(vncport)
			if err != nil {
				return err
			}
			vncnum = strconv.Itoa(p - 5900)
			vncdisplay = net.JoinHostPort(vnchost, vncnum)
		}
		vncfile := filepath.Join(a.instDir, filenames.VNCDisplayFile)
		if err := os.WriteFile(vncfile, []byte(vncdisplay), 0o600); err != nil {
			return err
		}
		vncurl := "vnc://" + net.JoinHostPort(vnchost, vncport)
		logrus.Infof("VNC server running at %s <%s>", vncdisplay, vncurl)
		logrus.Infof("VNC Display: `%s`", vncfile)
		logrus.Infof("VNC Password: `%s`", vncpwdfile)
	}

	if a.driver.Info().Features.CanRunGUI {
		go func() {
			err = a.startRoutinesAndWait(ctx, errCh)
			if err != nil {
				logrus.Error(err)
			}
		}()
		return a.driver.RunGUI()
	}
	return a.startRoutinesAndWait(ctx, errCh)
}

func (a *HostAgent) startRoutinesAndWait(ctx context.Context, errCh <-chan error) error {
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
	// wait for either the driver to stop or a signal to shut down
	select {
	case driverErr := <-errCh:
		logrus.Infof("Driver stopped due to error: %q", driverErr)
	case sig := <-a.signalCh:
		logrus.Infof("Received %s, shutting down the host agent", osutil.SignalName(sig))
	}
	// close the host agent routines before cancelling the context
	if closeErr := a.close(); closeErr != nil {
		logrus.WithError(closeErr).Warn("an error during shutting down the host agent")
	}
	cancelHA()
	return a.driver.Stop(ctx)
}

func (a *HostAgent) Info(_ context.Context) (*hostagentapi.Info, error) {
	info := &hostagentapi.Info{
		AutoStartedIdentifier: autostart.AutoStartedIdentifier(),
		SSHLocalPort:          a.sshLocalPort,
	}
	return info, nil
}

func (a *HostAgent) sshAddressPort() (sshAddress string, sshPort int) {
	sshAddress = a.instSSHAddress
	sshPort = a.sshLocalPort
	return sshAddress, sshPort
}

func (a *HostAgent) startHostAgentRoutines(ctx context.Context) error {
	if *a.instConfig.Plain {
		msg := "Running in plain mode. Mounts, dynamic port forwarding, containerd, etc. will be ignored. Guest agent will not be running."
		for _, port := range a.instConfig.PortForwards {
			if port.Static {
				msg += " Static port forwarding is allowed." //nolint:modernize // stringsbuilder is not needed
				break
			}
		}
		logrus.Info(msg)
	}
	a.cleanUp(func() error {
		// Skip ExitMaster when the control socket does not exist.
		// On Windows, the ControlMaster is used only for SSH port forwarding.
		if !sshutil.IsControlMasterExisting(a.instDir) {
			return nil
		}
		logrus.Debugf("shutting down the SSH master")
		if exitMasterErr := ssh.ExitMaster(a.instSSHAddress, a.sshLocalPort, a.sshConfig); exitMasterErr != nil {
			logrus.WithError(exitMasterErr).Warn("failed to exit SSH master")
		}
		return nil
	})
	var errs []error
	if err := a.waitForRequirements("essential", a.essentialRequirements()); err != nil {
		errs = append(errs, err)
	}
	if *a.instConfig.SSH.ForwardAgent {
		faScript := `#!/bin/bash
set -eux -o pipefail
sudo mkdir -p -m 700 /run/host-services
sudo ln -sf "${SSH_AUTH_SOCK}" /run/host-services/ssh-auth.sock
sudo chown -R "${USER}" /run/host-services`
		faDesc := "linking ssh auth socket to static location /run/host-services/ssh-auth.sock"
		stdout, stderr, err := ssh.ExecuteScript(a.instSSHAddress, a.sshLocalPort, a.sshConfig, faScript, faDesc)
		logrus.Debugf("stdout=%q, stderr=%q, err=%v", stdout, stderr, err)
		if err != nil {
			errs = append(errs, fmt.Errorf("stdout=%q, stderr=%q: %w", stdout, stderr, err))
		}
	}
	if *a.instConfig.MountType == limatype.REVSSHFS && !*a.instConfig.Plain {
		mounts, err := a.setupMounts(ctx)
		if err != nil {
			errs = append(errs, err)
		}
		a.cleanUp(func() error {
			var unmountErrs []error
			for _, m := range mounts {
				if unmountErr := m.close(); unmountErr != nil {
					unmountErrs = append(unmountErrs, unmountErr)
				}
			}
			return errors.Join(unmountErrs...)
		})
	}
	if len(a.instConfig.AdditionalDisks) > 0 {
		a.cleanUp(func() error {
			var unlockErrs []error
			for _, d := range a.instConfig.AdditionalDisks {
				disk, inspectErr := store.InspectDisk(d.Name, d.FSType)
				if inspectErr != nil {
					unlockErrs = append(unlockErrs, inspectErr)
					continue
				}
				logrus.Infof("Unmounting disk %q", disk.Name)
				if unlockErr := disk.Unlock(); unlockErr != nil {
					unlockErrs = append(unlockErrs, unlockErr)
				}
			}
			return errors.Join(unlockErrs...)
		})
	}

	staticPortForwards := a.separateStaticPortForwards()
	a.addStaticPortForwardsFromList(ctx, staticPortForwards)

	hasGuestAgentDaemon := !*a.instConfig.Plain && *a.instConfig.OS == limatype.LINUX
	if hasGuestAgentDaemon {
		go a.watchGuestAgentEvents(ctx)
		go a.startTimeSync(ctx)
		if a.showProgress {
			cloudInitDone := make(chan struct{})
			go func() {
				timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
				defer cancel()

				a.watchCloudInitProgress(timeoutCtx)
				close(cloudInitDone)
			}()

			go func() {
				<-cloudInitDone
				logrus.Debug("Cloud-init monitoring completed, VM is fully ready")
			}()
		}
	}
	if err := a.waitForRequirements("optional", a.optionalRequirements()); err != nil {
		errs = append(errs, err)
	}
	if hasGuestAgentDaemon {
		logrus.Info("Waiting for the guest agent to be running")
		select {
		case <-a.guestAgentAliveCh:
			// NOP
		case <-time.After(time.Minute):
			errs = append(errs, errors.New("guest agent does not seem to be running; port forwards will not work"))
		}
	}
	if err := a.waitForRequirements("final", a.finalRequirements()); err != nil {
		errs = append(errs, err)
	}
	// Copy all config files _after_ the requirements are done
	for _, rule := range a.instConfig.CopyToHost {
		sshAddress, sshPort := a.sshAddressPort()
		if err := copyToHost(ctx, a.sshConfig, sshAddress, sshPort, rule.HostFile, rule.GuestFile); err != nil {
			errs = append(errs, err)
		}
	}
	a.cleanUp(func() error {
		var rmErrs []error
		for _, rule := range a.instConfig.CopyToHost {
			if rule.DeleteOnStop {
				logrus.Infof("Deleting %s", rule.HostFile)
				if err := os.RemoveAll(rule.HostFile); err != nil {
					rmErrs = append(rmErrs, err)
				}
			}
		}
		return errors.Join(rmErrs...)
	})
	return errors.Join(errs...)
}

// cleanUp registers a cleanup function to be called when the host agent is stopped.
// The cleanup functions are called before the context is cancelled, in the reverse order of their registration.
func (a *HostAgent) cleanUp(fn func() error) {
	a.onCloseMu.Lock()
	defer a.onCloseMu.Unlock()
	a.onClose = append(a.onClose, fn)
}

func (a *HostAgent) close() error {
	a.onCloseMu.Lock()
	defer a.onCloseMu.Unlock()
	logrus.Infof("Shutting down the host agent")
	var errs []error
	for i := len(a.onClose) - 1; i >= 0; i-- {
		f := a.onClose[i]
		if err := f(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (a *HostAgent) watchGuestAgentEvents(ctx context.Context) {
	// TODO: use vSock (when QEMU for macOS gets support for vSock)

	// Setup all socket forwards and defer their teardown
	if !(a.driver.Info().Features.SkipSocketForwarding) {
		logrus.Debugf("Forwarding unix sockets")
		sshAddress, sshPort := a.sshAddressPort()
		for _, rule := range a.instConfig.PortForwards {
			if rule.GuestSocket != "" {
				local := hostAddress(rule, &guestagentapi.IPPort{})
				_ = forwardSSH(ctx, a.sshConfig, sshAddress, sshPort, local, rule.GuestSocket, verbForward, rule.Reverse)
			}
		}
	}

	localUnix := filepath.Join(a.instDir, filenames.GuestAgentSock)
	remoteUnix := "/run/lima-guestagent.sock"

	a.cleanUp(func() error {
		logrus.Debugf("Stop forwarding unix sockets")
		var errs []error
		sshAddress, sshPort := a.sshAddressPort()
		for _, rule := range a.instConfig.PortForwards {
			if rule.GuestSocket != "" {
				local := hostAddress(rule, &guestagentapi.IPPort{})
				// using ctx.Background() because ctx has already been cancelled
				if err := forwardSSH(context.Background(), a.sshConfig, sshAddress, sshPort, local, rule.GuestSocket, verbCancel, rule.Reverse); err != nil {
					errs = append(errs, err)
				}
			}
		}
		if a.driver.ForwardGuestAgent() {
			if err := forwardSSH(context.Background(), a.sshConfig, sshAddress, sshPort, localUnix, remoteUnix, verbCancel, false); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	})

	go func() {
		if a.instConfig.MountInotify != nil && *a.instConfig.MountInotify {
			if a.client == nil || !isGuestAgentSocketAccessible(ctx, a.client) {
				if a.driver.ForwardGuestAgent() {
					sshAddress, sshPort := a.sshAddressPort()
					_ = forwardSSH(ctx, a.sshConfig, sshAddress, sshPort, localUnix, remoteUnix, verbForward, false)
				}
			}
			err := a.startInotify(ctx)
			if err != nil {
				logrus.WithError(err).Warn("failed to start inotify")
			}
		}
	}()

	// ensure close before ctx is cancelled
	a.cleanUp(a.grpcPortForwarder.Close)

	for {
		if a.client == nil || !isGuestAgentSocketAccessible(ctx, a.client) {
			if a.driver.ForwardGuestAgent() {
				sshAddress, sshPort := a.sshAddressPort()
				_ = forwardSSH(ctx, a.sshConfig, sshAddress, sshPort, localUnix, remoteUnix, verbForward, false)
			}
		}
		client, err := a.getOrCreateClient(ctx)
		if err == nil {
			if err := a.processGuestAgentEvents(ctx, client); err != nil {
				if !errors.Is(err, context.Canceled) {
					logrus.WithError(err).Warn("guest agent events closed unexpectedly")
				}
			}
		} else {
			if !strings.Contains(err.Error(), context.Canceled.Error()) {
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

func (a *HostAgent) addStaticPortForwardsFromList(ctx context.Context, staticPortForwards []limatype.PortForward) {
	sshAddress, sshPort := a.sshAddressPort()
	for _, rule := range staticPortForwards {
		if rule.GuestSocket == "" {
			guest := &guestagentapi.IPPort{
				Ip:       rule.GuestIP.String(),
				Port:     int32(rule.GuestPort),
				Protocol: rule.Proto,
			}
			local, remote := a.portForwarder.forwardingAddresses(guest)
			if local != "" {
				logrus.Infof("Setting up static TCP forwarding from %s to %s", remote, local)
				if err := forwardTCP(ctx, a.sshConfig, sshAddress, sshPort, local, remote, verbForward); err != nil {
					logrus.WithError(err).Warnf("failed to set up static TCP forwarding %s -> %s", remote, local)
				}
			}
		}
	}
}

// separateStaticPortForwards separates static port forwards from a.instConfig.PortForwards,
// updates a.instConfig.PortForwards to contain only non-static port forwards,
// and returns the list of static port forwards.
func (a *HostAgent) separateStaticPortForwards() []limatype.PortForward {
	staticPortForwards := make([]limatype.PortForward, 0, len(a.instConfig.PortForwards))
	nonStaticPortForwards := make([]limatype.PortForward, 0, len(a.instConfig.PortForwards))

	for i := range len(a.instConfig.PortForwards) {
		rule := a.instConfig.PortForwards[i]
		if rule.Static {
			logrus.Debugf("Found static port forward: guest=%d host=%d", rule.GuestPort, rule.HostPort)
			staticPortForwards = append(staticPortForwards, rule)
		} else {
			logrus.Debugf("Found non-static port forward: guest=%d host=%d", rule.GuestPort, rule.HostPort)
			nonStaticPortForwards = append(nonStaticPortForwards, rule)
		}
	}

	logrus.Debugf("Static port forwards: %d, Non-static port forwards: %d", len(staticPortForwards), len(nonStaticPortForwards))

	a.instConfig.PortForwards = nonStaticPortForwards
	return staticPortForwards
}

func isGuestAgentSocketAccessible(ctx context.Context, client *guestagentclient.GuestAgentClient) bool {
	_, err := client.Info(ctx)
	return err == nil
}

func (a *HostAgent) getOrCreateClient(ctx context.Context) (*guestagentclient.GuestAgentClient, error) {
	a.clientMu.Lock()
	defer a.clientMu.Unlock()
	if a.client != nil && isGuestAgentSocketAccessible(ctx, a.client) {
		return a.client, nil
	}
	var err error
	a.client, err = guestagentclient.NewGuestAgentClient(a.createConnection)
	return a.client, err
}

func (a *HostAgent) createConnection(ctx context.Context) (net.Conn, error) {
	conn, _, err := a.driver.GuestAgentConn(ctx)
	// default to forwarded sock
	if conn == nil && err == nil {
		var d net.Dialer
		conn, err = d.DialContext(ctx, "unix", filepath.Join(a.instDir, filenames.GuestAgentSock))
	}
	return conn, err
}

func (a *HostAgent) processGuestAgentEvents(ctx context.Context, client *guestagentclient.GuestAgentClient) error {
	info, err := client.Info(ctx)
	if err != nil {
		return err
	}
	logrus.Info("Guest agent is running")
	a.guestAgentAliveChOnce.Do(func() {
		close(a.guestAgentAliveCh)
	})

	logrus.Debugf("guest agent info: %+v", info)

	onEvent := func(ev *guestagentapi.Event) {
		logrus.Debugf("guest agent event: %+v", ev)
		for _, f := range ev.Errors {
			logrus.Warnf("received error from the guest: %q", f)
		}
		// History of the default value of useSSHFwd:
		// - v0.1.0:        true  (effectively)
		// - v1.0.0:        false
		// - v1.0.1:        true
		// - v1.1.0-beta.0: false
		useSSHFwd := false
		if envVar := os.Getenv("LIMA_SSH_PORT_FORWARDER"); envVar != "" {
			b, err := strconv.ParseBool(envVar)
			if err != nil {
				logrus.WithError(err).Warnf("invalid LIMA_SSH_PORT_FORWARDER value %q", envVar)
			} else {
				useSSHFwd = b
			}
		}
		if useSSHFwd {
			a.portForwarder.OnEvent(ctx, ev)
		} else {
			dialContext := portfwd.DialContextToGRPCTunnel(client)
			a.grpcPortForwarder.OnEvent(ctx, dialContext, ev)
		}
	}

	if err := client.Events(ctx, onEvent); err != nil {
		if status.Code(err) == codes.Canceled {
			return context.Canceled
		}
		return err
	}
	return io.EOF
}

const (
	verbForward = "forward"
	verbCancel  = "cancel"
)

func executeSSH(ctx context.Context, sshConfig *ssh.SSHConfig, sshAddress string, sshPort int, command ...string) error {
	args := sshConfig.Args()
	args = append(args,
		"-p", strconv.Itoa(sshPort),
		sshAddress,
		"--",
	)
	args = append(args, command...)
	cmd := exec.CommandContext(ctx, sshConfig.Binary(), args...)
	if out, err := cmd.Output(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
}

func forwardSSH(ctx context.Context, sshConfig *ssh.SSHConfig, sshAddress string, sshPort int, local, remote, verb string, reverse bool) error {
	args := sshConfig.Args()
	args = append(args,
		"-T",
		"-O", verb,
	)
	if reverse {
		args = append(args,
			"-R", remote+":"+local,
		)
	} else {
		args = append(args,
			"-L", local+":"+remote,
		)
	}
	args = append(args,
		"-N",
		"-f",
		"-p", strconv.Itoa(sshPort),
		sshAddress,
		"--",
	)
	if strings.HasPrefix(local, "/") {
		switch verb {
		case verbForward:
			if reverse {
				logrus.Infof("Forwarding %q (host) to %q (guest)", local, remote)
				if err := executeSSH(ctx, sshConfig, sshAddress, sshPort, "rm", "-f", remote); err != nil {
					logrus.WithError(err).Warnf("Failed to clean up %q (guest) before setting up forwarding", remote)
				}
			} else {
				logrus.Infof("Forwarding %q (guest) to %q (host)", remote, local)
				if err := os.RemoveAll(local); err != nil {
					logrus.WithError(err).Warnf("Failed to clean up %q (host) before setting up forwarding", local)
				}
			}
			if err := os.MkdirAll(filepath.Dir(local), 0o750); err != nil {
				return fmt.Errorf("can't create directory for local socket %q: %w", local, err)
			}
		case verbCancel:
			if reverse {
				logrus.Infof("Stopping forwarding %q (host) to %q (guest)", local, remote)
				if err := executeSSH(ctx, sshConfig, sshAddress, sshPort, "rm", "-f", remote); err != nil {
					logrus.WithError(err).Warnf("Failed to clean up %q (guest) after stopping forwarding", remote)
				}
			} else {
				logrus.Infof("Stopping forwarding %q (guest) to %q (host)", remote, local)
				defer func() {
					if err := os.RemoveAll(local); err != nil {
						logrus.WithError(err).Warnf("Failed to clean up %q (host) after stopping forwarding", local)
					}
				}()
			}
		default:
			panic(fmt.Errorf("invalid verb %q", verb))
		}
	}
	cmd := exec.CommandContext(ctx, sshConfig.Binary(), args...)
	logrus.Debugf("Running %q", cmd)
	if out, err := cmd.Output(); err != nil {
		if verb == verbForward && strings.HasPrefix(local, "/") {
			if reverse {
				logrus.WithError(err).Warnf("Failed to set up forward from %q (host) to %q (guest)", local, remote)
				if err := executeSSH(ctx, sshConfig, sshAddress, sshPort, "rm", "-f", remote); err != nil {
					logrus.WithError(err).Warnf("Failed to clean up %q (guest) after forwarding failed", remote)
				}
			} else {
				logrus.WithError(err).Warnf("Failed to set up forward from %q (guest) to %q (host)", remote, local)
				if removeErr := os.RemoveAll(local); removeErr != nil {
					logrus.WithError(removeErr).Warnf("Failed to clean up %q (host) after forwarding failed", local)
				}
			}
		}
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
}

func (a *HostAgent) watchCloudInitProgress(ctx context.Context) {
	exitReason := "Cloud-init monitoring completed successfully"
	var cmd *exec.Cmd

	defer func() {
		a.emitCloudInitProgressEvent(context.Background(), &events.CloudInitProgress{
			Active:    false,
			Completed: true,
			LogLine:   exitReason,
		})
		logrus.Debug("Cloud-init progress monitoring completed")
	}()

	logrus.Debug("Starting cloud-init progress monitoring")

	a.emitCloudInitProgressEvent(ctx, &events.CloudInitProgress{
		Active: true,
	})

	sshAddress, sshPort := a.sshAddressPort()
	args := a.sshConfig.Args()
	args = append(args,
		"-p", strconv.Itoa(sshPort),
		sshAddress,
		"sh", "-c",
		`"if command -v systemctl >/dev/null 2>&1 && systemctl is-enabled -q cloud-init-main.service; then
			sudo journalctl -u cloud-init-main.service -b -S @0 -o cat -f
		else
			sudo tail -n +$(sudo awk '
				BEGIN{b=1; e=1}
				/^Cloud-init.* finished/{e=NR}
				/.*/{if(NR>e){b=e+1}}
				END{print b}
			' /var/log/cloud-init-output.log) -f /var/log/cloud-init-output.log
		fi"`,
	)

	cmd = exec.CommandContext(ctx, a.sshConfig.Binary(), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logrus.WithError(err).Warn("Failed to create stdout pipe for cloud-init monitoring")
		exitReason = "Failed to create stdout pipe for cloud-init monitoring"
		return
	}

	if err := cmd.Start(); err != nil {
		logrus.WithError(err).Warn("Failed to start cloud-init monitoring command")
		exitReason = "Failed to start cloud-init monitoring command"
		return
	}

	scanner := bufio.NewScanner(stdout)
	cloudInitMainServiceStarted := false
	cloudInitFinished := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if !cloudInitMainServiceStarted {
			if isStartedCloudInitMainService(line) {
				logrus.Debug("cloud-init-main.service started detected via log pattern")
				cloudInitMainServiceStarted = true
			} else if !cloudInitFinished {
				if isCloudInitFinished(line) {
					logrus.Debug("Cloud-init completion detected via log pattern")
					cloudInitFinished = true
				}
			}
		} else if !cloudInitFinished && isDeactivatedCloudInitMainService(line) {
			logrus.Debug("cloud-init-main.service deactivated detected via log pattern")
			cloudInitFinished = true
		}

		a.emitCloudInitProgressEvent(ctx, &events.CloudInitProgress{
			Active:    !cloudInitFinished,
			LogLine:   line,
			Completed: cloudInitFinished,
		})

		if cloudInitFinished {
			logrus.Debug("Breaking from cloud-init monitoring loop - completion detected")
			if cmd.Process != nil {
				logrus.Debug("Killing cloud-init monitoring process after completion")
				if err := cmd.Process.Kill(); err != nil {
					logrus.WithError(err).Debug("Failed to kill cloud-init monitoring process")
				}
			}
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logrus.Warn("Cloud-init monitoring timed out after 10 minutes")
			exitReason = "Cloud-init monitoring timed out after 10 minutes"
			return
		}
		logrus.WithError(err).Debug("SSH command finished (expected when cloud-init completes)")
	}

	if !cloudInitFinished {
		logrus.Debug("Connection dropped, checking for any remaining cloud-init logs")

		finalArgs := a.sshConfig.Args()
		finalArgs = append(finalArgs,
			"-p", strconv.Itoa(sshPort),
			sshAddress,
			"sudo", "tail", "-n", "20", "/var/log/cloud-init-output.log",
		)

		finalCmd := exec.CommandContext(ctx, a.sshConfig.Binary(), finalArgs...)
		if finalOutput, err := finalCmd.Output(); err == nil {
			for line := range strings.SplitSeq(string(finalOutput), "\n") {
				if strings.TrimSpace(line) != "" {
					if !cloudInitFinished {
						cloudInitFinished = isCloudInitFinished(line)
					}

					a.emitCloudInitProgressEvent(ctx, &events.CloudInitProgress{
						Active:    !cloudInitFinished,
						LogLine:   line,
						Completed: cloudInitFinished,
					})
				}
			}
		}
	}
}

func isCloudInitFinished(line string) bool {
	line = strings.ToLower(strings.TrimSpace(line))
	return strings.Contains(line, "cloud-init") && strings.Contains(line, "finished")
}

func isStartedCloudInitMainService(line string) bool {
	line = strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(line, "started cloud-init-main.service")
}

func isDeactivatedCloudInitMainService(line string) bool {
	line = strings.ToLower(strings.TrimSpace(line))
	// Deactivated event lines end with a line reporting consumed CPU time, etc.
	return strings.HasPrefix(line, "cloud-init-main.service: consumed")
}

func copyToHost(ctx context.Context, sshConfig *ssh.SSHConfig, sshAddress string, sshPort int, local, remote string) error {
	args := sshConfig.Args()
	args = append(args,
		"-p", strconv.Itoa(sshPort),
		sshAddress,
		"--",
	)
	args = append(args,
		"sudo",
		"cat",
		remote,
	)
	logrus.Infof("Copying config from %s to %s", remote, local)
	if err := os.MkdirAll(filepath.Dir(local), 0o700); err != nil {
		return fmt.Errorf("can't create directory for local file %q: %w", local, err)
	}
	cmd := exec.CommandContext(ctx, sshConfig.Binary(), args...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	if err := os.WriteFile(local, out, 0o600); err != nil {
		return fmt.Errorf("can't write to local file %q: %w", local, err)
	}
	return nil
}
