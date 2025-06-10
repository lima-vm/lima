// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"text/template"
	"time"

	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/downloader"
	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/executil"
	"github.com/lima-vm/lima/pkg/fileutils"
	hostagentevents "github.com/lima-vm/lima/pkg/hostagent/events"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/nativeimgutil"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/qemu/imgutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
)

// DefaultWatchHostAgentEventsTimeout is the duration to wait for the instance
// to be running before timing out.
const DefaultWatchHostAgentEventsTimeout = 10 * time.Minute

// ensureNerdctlArchiveCache prefetches the nerdctl-full-VERSION-GOOS-GOARCH.tar.gz archive
// into the cache before launching the hostagent process, so that we can show the progress in tty.
// https://github.com/lima-vm/lima/issues/326
func ensureNerdctlArchiveCache(ctx context.Context, y *limayaml.LimaYAML, created bool) (string, error) {
	if !*y.Containerd.System && !*y.Containerd.User {
		// nerdctl archive is not needed
		return "", nil
	}

	errs := make([]error, len(y.Containerd.Archives))
	for i, f := range y.Containerd.Archives {
		// Skip downloading again if the file is already in the cache
		if created && f.Arch == *y.Arch && !downloader.IsLocal(f.Location) {
			path, err := fileutils.CachedFile(f)
			if err == nil {
				return path, nil
			}
		}
		path, err := fileutils.DownloadFile(ctx, "", f, false, "the nerdctl archive", *y.Arch)
		if err != nil {
			errs[i] = err
			continue
		}
		if path == "" {
			if downloader.IsLocal(f.Location) {
				return f.Location, nil
			}
			return "", fmt.Errorf("cache did not contain %q", f.Location)
		}
		return path, nil
	}

	return "", fileutils.Errors(errs)
}

type Prepared struct {
	Driver              driver.Driver
	GuestAgent          string
	NerdctlArchiveCache string
}

// Prepare ensures the disk, the nerdctl archive, etc.
func Prepare(ctx context.Context, inst *store.Instance) (*Prepared, error) {
	var guestAgent string
	if !*inst.Config.Plain {
		var err error
		guestAgent, err = usrlocalsharelima.GuestAgentBinary(*inst.Config.OS, *inst.Config.Arch)
		if err != nil {
			return nil, err
		}
	}
	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
	})

	if err := limaDriver.Validate(); err != nil {
		return nil, err
	}

	if err := limaDriver.Initialize(ctx); err != nil {
		return nil, err
	}

	// Check if the instance has been created (the base disk already exists)
	baseDisk := filepath.Join(inst.Dir, filenames.BaseDisk)
	_, err := os.Stat(baseDisk)
	created := err == nil

	if err := limaDriver.CreateDisk(ctx); err != nil {
		return nil, err
	}

	// Ensure diffDisk size matches the store
	if err := prepareDiffDisk(inst); err != nil {
		return nil, err
	}

	nerdctlArchiveCache, err := ensureNerdctlArchiveCache(ctx, inst.Config, created)
	if err != nil {
		return nil, err
	}

	return &Prepared{
		Driver:              limaDriver,
		GuestAgent:          guestAgent,
		NerdctlArchiveCache: nerdctlArchiveCache,
	}, nil
}

// Start starts the hostagent in the background, which in turn will start the instance.
// Start will listen to hostagent events and log them to STDOUT until either the instance
// is running, or has failed to start.
//
// The `limactl` argument allows the caller to specify the full path of the `limactl` executable.
// When called from inside limactl itself it will always be the empty string which uses the name
// of the current executable instead.
//
// The `launchHostAgentForeground` argument makes the hostagent run in the foreground.
// The function will continue to listen and log hostagent events until the instance is
// shut down again.
//
// Start calls Prepare by itself, so you do not need to call Prepare manually before calling Start.
func Start(ctx context.Context, inst *store.Instance, limactl string, launchHostAgentForeground bool) error {
	haPIDPath := filepath.Join(inst.Dir, filenames.HostAgentPID)
	if _, err := os.Stat(haPIDPath); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("instance %q seems running (hint: remove %q if the instance is not actually running)", inst.Name, haPIDPath)
	}
	logrus.Infof("Starting the instance %q with VM driver %q", inst.Name, inst.VMType)

	haSockPath := filepath.Join(inst.Dir, filenames.HostAgentSock)

	prepared, err := Prepare(ctx, inst)
	if err != nil {
		return err
	}

	if limactl == "" {
		limactl, err = os.Executable()
		if err != nil {
			return err
		}
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

	var args []string
	if logrus.GetLevel() >= logrus.DebugLevel {
		args = append(args, "--debug")
	}
	args = append(args,
		"hostagent",
		"--pidfile", haPIDPath,
		"--socket", haSockPath)
	if prepared.Driver.CanRunGUI() {
		args = append(args, "--run-gui")
	}
	if prepared.GuestAgent != "" {
		args = append(args, "--guestagent", prepared.GuestAgent)
	}
	if prepared.NerdctlArchiveCache != "" {
		args = append(args, "--nerdctl-archive", prepared.NerdctlArchiveCache)
	}
	args = append(args, inst.Name)
	haCmd := exec.CommandContext(ctx, limactl, args...)

	if launchHostAgentForeground {
		haCmd.SysProcAttr = executil.ForegroundSysProcAttr
	} else {
		haCmd.SysProcAttr = executil.BackgroundSysProcAttr
	}

	haCmd.Stdout = haStdoutW
	haCmd.Stderr = haStderrW

	begin := time.Now() // used for logrus propagation

	if launchHostAgentForeground {
		logrus.Info("Running the host agent in the foreground")
		if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			// Write message to standard log files to avoid confusing users
			message := "This log file is not used because `limactl start` was launched in the terminal with the `--foreground` option."
			if _, err := haStdoutW.WriteString(message); err != nil {
				return err
			}
			if _, err := haStderrW.WriteString(message); err != nil {
				return err
			}
		} else {
			if err := osutil.Dup2(int(haStdoutW.Fd()), syscall.Stdout); err != nil {
				return err
			}
			if err := osutil.Dup2(int(haStderrW.Fd()), syscall.Stderr); err != nil {
				return err
			}
		}
		if err := syscall.Exec(limactl, haCmd.Args, haCmd.Environ()); err != nil {
			return err
		}
	} else if err := haCmd.Start(); err != nil {
		return err
	}

	if err := waitHostAgentStart(ctx, haPIDPath, haStderrPath); err != nil {
		return err
	}

	watchErrCh := make(chan error)
	go func() {
		watchErrCh <- watchHostAgentEvents(ctx, inst, haStdoutPath, haStderrPath, begin)
		close(watchErrCh)
	}()
	waitErrCh := make(chan error)
	go func() {
		waitErrCh <- haCmd.Wait()
		close(waitErrCh)
	}()

	select {
	case watchErr := <-watchErrCh:
		// watchErr can be nil
		return watchErr
		// leave the hostagent process running
	case waitErr := <-waitErrCh:
		// waitErr should not be nil
		return fmt.Errorf("host agent process has exited: %w", waitErr)
	}
}

func waitHostAgentStart(_ context.Context, haPIDPath, haStderrPath string) error {
	begin := time.Now()
	deadlineDuration := 5 * time.Second
	deadline := begin.Add(deadlineDuration)
	for {
		if _, err := os.Stat(haPIDPath); !errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("hostagent (%q) did not start up in %v (hint: see %q)", haPIDPath, deadlineDuration, haStderrPath)
		}
	}
}

func watchHostAgentEvents(ctx context.Context, inst *store.Instance, haStdoutPath, haStderrPath string, begin time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, watchHostAgentTimeout(ctx))
	defer cancel()

	var (
		printedSSHLocalPort  bool
		receivedRunningEvent bool
		err                  error
	)
	onEvent := func(ev hostagentevents.Event) bool {
		if !printedSSHLocalPort && ev.Status.SSHLocalPort != 0 {
			logrus.Infof("SSH Local Port: %d", ev.Status.SSHLocalPort)
			printedSSHLocalPort = true
		}

		if len(ev.Status.Errors) > 0 {
			logrus.Errorf("%+v", ev.Status.Errors)
		}
		if ev.Status.Exiting {
			err = fmt.Errorf("exiting, status=%+v (hint: see %q)", ev.Status, haStderrPath)
			return true
		} else if ev.Status.Running {
			receivedRunningEvent = true
			if ev.Status.Degraded {
				logrus.Warnf("DEGRADED. The VM seems running, but file sharing and port forwarding may not work. (hint: see %q)", haStderrPath)
				err = fmt.Errorf("degraded, status=%+v", ev.Status)
				return true
			}

			if xerr := runAnsibleProvision(ctx, inst); xerr != nil {
				err = xerr
				return true
			}
			if *inst.Config.Plain {
				logrus.Infof("READY. Run `ssh -F %q %s` to open the shell.", inst.SSHConfigFile, inst.Hostname)
			} else {
				logrus.Infof("READY. Run `%s` to open the shell.", LimactlShellCmd(inst.Name))
			}
			_ = ShowMessage(inst)
			err = nil
			return true
		}
		return false
	}

	if xerr := hostagentevents.Watch(ctx, haStdoutPath, haStderrPath, begin, onEvent); xerr != nil {
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

type watchHostAgentEventsTimeoutKey = struct{}

// WithWatchHostAgentTimeout sets the value of the timeout to use for
// watchHostAgentEvents in the given Context.
func WithWatchHostAgentTimeout(ctx context.Context, timeout time.Duration) context.Context {
	return context.WithValue(ctx, watchHostAgentEventsTimeoutKey{}, timeout)
}

// watchHostAgentTimeout returns the value of the timeout to use for
// watchHostAgentEvents contained in the given Context, or its default value.
func watchHostAgentTimeout(ctx context.Context) time.Duration {
	if timeout, ok := ctx.Value(watchHostAgentEventsTimeoutKey{}).(time.Duration); ok {
		return timeout
	}
	return DefaultWatchHostAgentEventsTimeout
}

func LimactlShellCmd(instName string) string {
	shellCmd := fmt.Sprintf("limactl shell %s", instName)
	if instName == "default" {
		shellCmd = "lima"
	}
	return shellCmd
}

func ShowMessage(inst *store.Instance) error {
	if inst.Message == "" {
		return nil
	}
	t, err := template.New("message").Parse(inst.Message)
	if err != nil {
		return err
	}
	data, err := store.AddGlobalFields(inst)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return err
	}
	scanner := bufio.NewScanner(&b)
	logrus.Infof("Message from the instance %q:", inst.Name)
	for scanner.Scan() {
		// Avoid prepending logrus "INFO" header, for ease of copy pasting
		fmt.Fprintln(logrus.StandardLogger().Out, scanner.Text())
	}
	return scanner.Err()
}

// prepareDiffDisk checks the disk size difference between inst.Disk and yaml.Disk.
// If there is no diffDisk, return nil (the instance has not been initialized or started yet).
func prepareDiffDisk(inst *store.Instance) error {
	diffDisk := filepath.Join(inst.Dir, filenames.DiffDisk)

	// Handle the instance initialization
	_, err := os.Stat(diffDisk)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var diskSize int64
	var format string
	if *inst.Config.VMType != limayaml.VBOX {
		f, err := os.Open(diffDisk)
		if err != nil {
			return err
		}
		defer f.Close()

		img, err := qcow2reader.Open(f)
		if err != nil {
			return err
		}

		diskSize = img.Size()
		format = string(img.Type())
	} else {
		info, err := imgutil.GetInfo(diffDisk)
		if err != nil {
			return err
		}

		diskSize = info.VSize
		format = info.Format
	}

	if inst.Disk == diskSize {
		return nil
	}

	logrus.Infof("Resize instance %s's disk from %s to %s", inst.Name, units.BytesSize(float64(diskSize)), units.BytesSize(float64(inst.Disk)))

	if inst.Disk < diskSize {
		inst.Disk = diskSize
		return errors.New("diffDisk: Shrinking is currently unavailable")
	}

	if format == "raw" {
		err = nativeimgutil.ResizeRawDisk(diffDisk, int(inst.Disk))
	} else {
		err = imgutil.ResizeDisk(diffDisk, format, int(inst.Disk))
	}

	if err != nil {
		return err
	}

	return nil
}
