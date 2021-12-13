package start

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lima-vm/lima/pkg/downloader"
	hostagentevents "github.com/lima-vm/lima/pkg/hostagent/events"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/qemu"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

func ensureDisk(ctx context.Context, instName, instDir string, y *limayaml.LimaYAML) error {
	qCfg := qemu.Config{
		Name:        instName,
		InstanceDir: instDir,
		LimaYAML:    y,
	}
	if err := qemu.EnsureDisk(qCfg); err != nil {
		return err
	}

	return nil
}

// ensureNerdctlArchiveCache prefetches the nerdctl-full-VERSION-linux-GOARCH.tar.gz archive
// into the cache before launching the hostagent process, so that we can show the progress in tty.
// https://github.com/lima-vm/lima/issues/326
func ensureNerdctlArchiveCache(y *limayaml.LimaYAML) (string, error) {
	if !*y.Containerd.System && !*y.Containerd.User {
		// nerdctl archive is not needed
		return "", nil
	}

	errs := make([]error, len(y.Containerd.Archives))
	for i := range y.Containerd.Archives {
		f := &y.Containerd.Archives[i]
		if f.Arch != *y.Arch {
			errs[i] = fmt.Errorf("unsupported arch: %q", f.Arch)
			continue
		}
		logrus.WithField("digest", f.Digest).Infof("Attempting to download the nerdctl archive from %q", f.Location)
		res, err := downloader.Download("", f.Location, downloader.WithCache(), downloader.WithExpectedDigest(f.Digest))
		if err != nil {
			errs[i] = fmt.Errorf("failed to download %q: %w", f.Location, err)
			continue
		}
		switch res.Status {
		case downloader.StatusDownloaded:
			logrus.Infof("Downloaded the nerdctl archive from %q", f.Location)
		case downloader.StatusUsedCache:
			logrus.Infof("Using cache %q", res.CachePath)
		default:
			logrus.Warnf("Unexpected result from downloader.Download(): %+v", res)
		}
		if res.CachePath == "" {
			if downloader.IsLocal(f.Location) {
				return f.Location, nil
			}
			return "", fmt.Errorf("cache did not contain %q", f.Location)
		}
		return res.CachePath, nil
	}

	return "", fmt.Errorf("failed to download the nerdctl archive, attempted %d candidates, errors=%v",
		len(y.Containerd.Archives), errs)
}

func Start(ctx context.Context, inst *store.Instance) error {
	haPIDPath := filepath.Join(inst.Dir, filenames.HostAgentPID)
	if _, err := os.Stat(haPIDPath); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("instance %q seems running (hint: remove %q if the instance is not actually running)", inst.Name, haPIDPath)
	}

	haSockPath := filepath.Join(inst.Dir, filenames.HostAgentSock)

	y, err := inst.LoadYAML()
	if err != nil {
		return err
	}

	if err := ensureDisk(ctx, inst.Name, inst.Dir, y); err != nil {
		return err
	}
	nerdctlArchiveCache, err := ensureNerdctlArchiveCache(y)
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
	if nerdctlArchiveCache != "" {
		args = append(args, "--nerdctl-archive", nerdctlArchiveCache)
	}
	args = append(args, inst.Name)
	haCmd := exec.CommandContext(ctx, self, args...)

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
		watchErrCh <- watchHostAgentEvents(ctx, inst.Name, haStdoutPath, haStderrPath, begin)
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

func waitHostAgentStart(ctx context.Context, haPIDPath, haStderrPath string) error {
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

func watchHostAgentEvents(ctx context.Context, instName, haStdoutPath, haStderrPath string, begin time.Time) error {
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Minute)
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

			logrus.Infof("READY. Run `%s` to open the shell.", LimactlShellCmd(instName))
			err = nil
			return true
		}
		return false
	}

	if xerr := hostagentevents.Watch(ctx2, haStdoutPath, haStderrPath, begin, onEvent); xerr != nil {
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

func LimactlShellCmd(instName string) string {
	shellCmd := fmt.Sprintf("limactl shell %s", instName)
	if instName == "default" {
		shellCmd = "lima"
	}
	return shellCmd
}
