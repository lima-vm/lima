package start

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
	"time"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/qemu"
	"github.com/lima-vm/lima/pkg/qemu/entitlementutil"

	"github.com/lima-vm/lima/pkg/downloader"
	"github.com/lima-vm/lima/pkg/fileutils"
	hostagentevents "github.com/lima-vm/lima/pkg/hostagent/events"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

// DefaultWatchHostAgentEventsTimeout is the duration to wait for the instance
// to be running before timing out.
const DefaultWatchHostAgentEventsTimeout = 10 * time.Minute

// ensureNerdctlArchiveCache prefetches the nerdctl-full-VERSION-GOOS-GOARCH.tar.gz archive
// into the cache before launching the hostagent process, so that we can show the progress in tty.
// https://github.com/lima-vm/lima/issues/326
func ensureNerdctlArchiveCache(y *limayaml.LimaYAML, created bool) (string, error) {
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
		path, err := fileutils.DownloadFile("", f, false, "the nerdctl archive", *y.Arch)
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
	NerdctlArchiveCache string
}

// Prepare ensures the disk, the nerdctl archive, etc.
func Prepare(_ context.Context, inst *store.Instance) (*Prepared, error) {
	y, err := inst.LoadYAML()
	if err != nil {
		return nil, err
	}

	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
		Yaml:     y,
	})

	if err := limaDriver.Validate(); err != nil {
		return nil, err
	}

	// Check if the instance has been created (the base disk already exists)
	created := false
	baseDisk := filepath.Join(inst.Dir, filenames.BaseDisk)
	if _, err := os.Stat(baseDisk); err == nil {
		created = true
	}
	if err := limaDriver.CreateDisk(); err != nil {
		return nil, err
	}
	nerdctlArchiveCache, err := ensureNerdctlArchiveCache(y, created)
	if err != nil {
		return nil, err
	}

	return &Prepared{
		Driver:              limaDriver,
		NerdctlArchiveCache: nerdctlArchiveCache,
	}, nil
}

// Start starts the instance.
// Start calls Prepare by itself, so you do not need to call Prepare manually before calling Start.
func Start(ctx context.Context, inst *store.Instance) error {
	haPIDPath := filepath.Join(inst.Dir, filenames.HostAgentPID)
	if _, err := os.Stat(haPIDPath); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("instance %q seems running (hint: remove %q if the instance is not actually running)", inst.Name, haPIDPath)
	}

	haSockPath := filepath.Join(inst.Dir, filenames.HostAgentSock)

	// Ask the user to sign the qemu binary with the "com.apple.security.hypervisor" if needed.
	// Workaround for https://github.com/lima-vm/lima/issues/1742
	if runtime.GOOS == "darwin" && inst.VMType == limayaml.QEMU {
		qExe, _, err := qemu.Exe(inst.Arch)
		if err != nil {
			return fmt.Errorf("failed to find the QEMU binary for the architecture %q: %w", inst.Arch, err)
		}
		if accel := qemu.Accel(inst.Arch); accel == "hvf" {
			entitlementutil.AskToSignIfNotSignedProperly(qExe)
		}
	}

	prepared, err := Prepare(ctx, inst)
	if err != nil {
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
	if prepared.NerdctlArchiveCache != "" {
		args = append(args, "--nerdctl-archive", prepared.NerdctlArchiveCache)
	}
	args = append(args, inst.Name)
	haCmd := exec.CommandContext(ctx, self, args...)
	haCmd.SysProcAttr = SysProcAttr

	haCmd.Stdout = haStdoutW
	haCmd.Stderr = haStderrW

	begin := time.Now() // used for logrus propagation

	if err := haCmd.Start(); err != nil {
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

			if *inst.Config.Plain {
				logrus.Infof("READY. Run `ssh -F %q lima-%s` to open the shell.", inst.SSHConfigFile, inst.Name)
			} else {
				logrus.Infof("READY. Run `%s` to open the shell.", LimactlShellCmd(inst.Name))
			}
			ShowMessage(inst)
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

// WithWatchHostAgentEventsTimeout sets the value of the timeout to use for
// watchHostAgentEvents in the given Context.
func WithWatchHostAgentTimeout(ctx context.Context, timeout time.Duration) context.Context {
	return context.WithValue(ctx, watchHostAgentEventsTimeoutKey{}, timeout)
}

// watchHostAgentEventsTimeout returns the value of the timeout to use for
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
		// Avoid prepending logrus "INFO" header, for ease of copypasting
		fmt.Println(scanner.Text())
	}
	return scanner.Err()
}

func Register(ctx context.Context, inst *store.Instance) error {
	y, err := inst.LoadYAML()
	if err != nil {
		return err
	}

	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
		Yaml:     y,
	})

	return limaDriver.Register(ctx)
}
