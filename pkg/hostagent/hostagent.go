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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sethvargo/go-password/password"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

	onClose []func() error // LIFO

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
	if *inst.Config.VMType == limatype.WSL2 {
		sshLocalPort = inst.SSHLocalPort
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

	vSockPort := limaDriver.Info().VsockPort
	virtioPort := limaDriver.Info().VirtioPort

	if err := cidata.GenerateCloudConfig(ctx, inst.Dir, instName, inst.Config); err != nil {
		return nil, err
	}
	if err := cidata.GenerateISO9660(ctx, inst.Dir, instName, inst.Config, udpDNSLocalPort, tcpDNSLocalPort, o.guestAgentBinary, o.nerdctlArchive, vSockPort, virtioPort); err != nil {
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
		portForwarder:     newPortForwarder(sshConfig, sshLocalPort, rules, ignoreTCP, inst.VMType),
		grpcPortForwarder: portfwd.NewPortForwarder(rules, ignoreTCP, ignoreUDP),
		driver:            limaDriver,
		signalCh:          signalCh,
		eventEnc:          json.NewEncoder(stdout),
		vSockPort:         vSockPort,
		virtioPort:        virtioPort,
		guestAgentAliveCh: make(chan struct{}),
		showProgress:      o.showProgress,
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
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	if err := a.eventEnc.Encode(ev); err != nil {
		logrus.WithField("event", ev).WithError(err).Error("failed to emit an event")
	}
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

	// WSL instance SSH address isn't known until after VM start
	if *a.instConfig.VMType == limatype.WSL2 {
		sshAddr, err := store.GetSSHAddress(ctx, a.instName)
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

	if a.driver.Info().CanRunGUI {
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
	for {
		select {
		case driverErr := <-errCh:
			logrus.Infof("Driver stopped due to error: %q", driverErr)
			cancelHA()
			if closeErr := a.close(); closeErr != nil {
				logrus.WithError(closeErr).Warn("an error during shutting down the host agent")
			}
			err := a.driver.Stop(ctx)
			return err
		case sig := <-a.signalCh:
			logrus.Infof("Received %s, shutting down the host agent", osutil.SignalName(sig))
			cancelHA()
			if closeErr := a.close(); closeErr != nil {
				logrus.WithError(closeErr).Warn("an error during shutting down the host agent")
			}
			err := a.driver.Stop(ctx)
			return err
		}
	}
}

func (a *HostAgent) Info(_ context.Context) (*hostagentapi.Info, error) {
	info := &hostagentapi.Info{
		SSHLocalPort: a.sshLocalPort,
	}
	return info, nil
}

func (a *HostAgent) startHostAgentRoutines(ctx context.Context) error {
	if *a.instConfig.Plain {
		logrus.Info("Running in plain mode. Mounts, port forwarding, containerd, etc. will be ignored. Guest agent will not be running.")
	}
	a.onClose = append(a.onClose, func() error {
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
		a.onClose = append(a.onClose, func() error {
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
		a.onClose = append(a.onClose, func() error {
			var unlockErrs []error
			for _, d := range a.instConfig.AdditionalDisks {
				disk, inspectErr := store.InspectDisk(d.Name)
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
	if !*a.instConfig.Plain {
		go a.watchGuestAgentEvents(ctx)
		if a.showProgress {
			cloudInitDone := make(chan struct{})
			go func() {
				a.watchCloudInitProgress(ctx)
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
	if !*a.instConfig.Plain {
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
		if err := copyToHost(ctx, a.sshConfig, a.sshLocalPort, rule.HostFile, rule.GuestFile); err != nil {
			errs = append(errs, err)
		}
	}
	a.onClose = append(a.onClose, func() error {
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

func (a *HostAgent) close() error {
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
	if *a.instConfig.VMType != limatype.WSL2 {
		logrus.Debugf("Forwarding unix sockets")
		for _, rule := range a.instConfig.PortForwards {
			if rule.GuestSocket != "" {
				local := hostAddress(rule, &guestagentapi.IPPort{})
				_ = forwardSSH(ctx, a.sshConfig, a.sshLocalPort, local, rule.GuestSocket, verbForward, rule.Reverse)
			}
		}
	}

	localUnix := filepath.Join(a.instDir, filenames.GuestAgentSock)
	remoteUnix := "/run/lima-guestagent.sock"

	a.onClose = append(a.onClose, func() error {
		logrus.Debugf("Stop forwarding unix sockets")
		var errs []error
		for _, rule := range a.instConfig.PortForwards {
			if rule.GuestSocket != "" {
				local := hostAddress(rule, &guestagentapi.IPPort{})
				// using ctx.Background() because ctx has already been cancelled
				if err := forwardSSH(context.Background(), a.sshConfig, a.sshLocalPort, local, rule.GuestSocket, verbCancel, rule.Reverse); err != nil {
					errs = append(errs, err)
				}
			}
		}
		if a.driver.ForwardGuestAgent() {
			if err := forwardSSH(context.Background(), a.sshConfig, a.sshLocalPort, localUnix, remoteUnix, verbCancel, false); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	})

	go func() {
		if a.instConfig.MountInotify != nil && *a.instConfig.MountInotify {
			if a.client == nil || !isGuestAgentSocketAccessible(ctx, a.client) {
				if a.driver.ForwardGuestAgent() {
					_ = forwardSSH(ctx, a.sshConfig, a.sshLocalPort, localUnix, remoteUnix, verbForward, false)
				}
			}
			err := a.startInotify(ctx)
			if err != nil {
				logrus.WithError(err).Warn("failed to start inotify")
			}
		}
	}()

	for {
		if a.client == nil || !isGuestAgentSocketAccessible(ctx, a.client) {
			if a.driver.ForwardGuestAgent() {
				_ = forwardSSH(ctx, a.sshConfig, a.sshLocalPort, localUnix, remoteUnix, verbForward, false)
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
			a.grpcPortForwarder.OnEvent(ctx, client, ev)
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

func executeSSH(ctx context.Context, sshConfig *ssh.SSHConfig, port int, command ...string) error {
	args := sshConfig.Args()
	args = append(args,
		"-p", strconv.Itoa(port),
		"127.0.0.1",
		"--",
	)
	args = append(args, command...)
	cmd := exec.CommandContext(ctx, sshConfig.Binary(), args...)
	if out, err := cmd.Output(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
}

func forwardSSH(ctx context.Context, sshConfig *ssh.SSHConfig, port int, local, remote, verb string, reverse bool) error {
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
		"-p", strconv.Itoa(port),
		"127.0.0.1",
		"--",
	)
	if strings.HasPrefix(local, "/") {
		switch verb {
		case verbForward:
			if reverse {
				logrus.Infof("Forwarding %q (host) to %q (guest)", local, remote)
				if err := executeSSH(ctx, sshConfig, port, "rm", "-f", remote); err != nil {
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
				if err := executeSSH(ctx, sshConfig, port, "rm", "-f", remote); err != nil {
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
				if err := executeSSH(ctx, sshConfig, port, "rm", "-f", remote); err != nil {
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
	logrus.Debug("Starting cloud-init progress monitoring")

	a.emitEvent(ctx, events.Event{
		Status: events.Status{
			SSHLocalPort: a.sshLocalPort,
			CloudInitProgress: &events.CloudInitProgress{
				Active: true,
			},
		},
	})

	maxRetries := 30
	retryDelay := time.Second
	var sshReady bool

	for i := 0; i < maxRetries && !sshReady; i++ {
		if i > 0 {
			time.Sleep(retryDelay)
		}

		// Test SSH connectivity
		args := a.sshConfig.Args()
		args = append(args,
			"-p", strconv.Itoa(a.sshLocalPort),
			"127.0.0.1",
			"echo 'SSH Ready'",
		)

		cmd := exec.CommandContext(ctx, a.sshConfig.Binary(), args...)
		if err := cmd.Run(); err == nil {
			sshReady = true
			logrus.Debug("SSH ready for cloud-init monitoring")
		}
	}

	if !sshReady {
		logrus.Warn("SSH not ready for cloud-init monitoring, proceeding anyway")
	}

	args := a.sshConfig.Args()
	args = append(args,
		"-p", strconv.Itoa(a.sshLocalPort),
		"127.0.0.1",
		"sudo", "tail", "-n", "+1", "-f", "/var/log/cloud-init-output.log",
	)

	cmd := exec.CommandContext(ctx, a.sshConfig.Binary(), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logrus.WithError(err).Warn("Failed to create stdout pipe for cloud-init monitoring")
		return
	}

	if err := cmd.Start(); err != nil {
		logrus.WithError(err).Warn("Failed to start cloud-init monitoring command")
		return
	}

	scanner := bufio.NewScanner(stdout)
	cloudInitFinished := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.Contains(line, "Cloud-init") && strings.Contains(line, "finished") {
			cloudInitFinished = true
		}

		a.emitEvent(ctx, events.Event{
			Status: events.Status{
				SSHLocalPort: a.sshLocalPort,
				CloudInitProgress: &events.CloudInitProgress{
					Active:    !cloudInitFinished,
					LogLine:   line,
					Completed: cloudInitFinished,
				},
			},
		})
	}

	if err := cmd.Wait(); err != nil {
		logrus.WithError(err).Debug("SSH command finished (expected when cloud-init completes)")
	}

	if !cloudInitFinished {
		logrus.Debug("Connection dropped, checking for any remaining cloud-init logs")

		finalArgs := a.sshConfig.Args()
		finalArgs = append(finalArgs,
			"-p", strconv.Itoa(a.sshLocalPort),
			"127.0.0.1",
			"sudo", "tail", "-n", "20", "/var/log/cloud-init-output.log",
		)

		finalCmd := exec.CommandContext(ctx, a.sshConfig.Binary(), finalArgs...)
		if finalOutput, err := finalCmd.Output(); err == nil {
			lines := strings.Split(string(finalOutput), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					if strings.Contains(line, "Cloud-init") && strings.Contains(line, "finished") {
						cloudInitFinished = true
					}

					a.emitEvent(ctx, events.Event{
						Status: events.Status{
							SSHLocalPort: a.sshLocalPort,
							CloudInitProgress: &events.CloudInitProgress{
								Active:    !cloudInitFinished,
								LogLine:   line,
								Completed: cloudInitFinished,
							},
						},
					})
				}
			}
		}
	}

	a.emitEvent(ctx, events.Event{
		Status: events.Status{
			SSHLocalPort: a.sshLocalPort,
			CloudInitProgress: &events.CloudInitProgress{
				Active:    false,
				Completed: true,
			},
		},
	})

	logrus.Debug("Cloud-init progress monitoring completed")
}

func copyToHost(ctx context.Context, sshConfig *ssh.SSHConfig, port int, local, remote string) error {
	args := sshConfig.Args()
	args = append(args,
		"-p", strconv.Itoa(port),
		"127.0.0.1",
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
