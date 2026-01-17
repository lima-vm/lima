// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/hostagent/events"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newWatchCommand() *cobra.Command {
	watchCommand := &cobra.Command{
		Use:   "watch [INSTANCE]...",
		Short: "Watch events from instances",
		Long: `Watch events from Lima instances.

Events include status changes (starting, running, stopping), port forwarding
events, and other instance lifecycle events.

If no instance is specified, events from all instances are watched,
including newly created instances.

The command will continue watching until interrupted (Ctrl+C).`,
		Example: `  # Watch events from all instances:
  $ limactl watch

  # Watch events from a specific instance:
  $ limactl watch default

  # Include historical events:
  $ limactl watch --history default

  # Show verbose output (host agent logs, etc.):
  $ limactl watch --verbose

  # Watch events in JSON format (for scripting):
  $ limactl watch --json default`,
		Args:              WrapArgsError(cobra.ArbitraryArgs),
		RunE:              watchAction,
		ValidArgsFunction: watchBashComplete,
		GroupID:           advancedCommand,
	}
	watchCommand.Flags().Bool("json", false, "Output events as newline-delimited JSON")
	watchCommand.Flags().Bool("history", false, "Include historical events from before watch started")
	watchCommand.Flags().Bool("verbose", false, "Show verbose output")
	return watchCommand
}

type watchEvent struct {
	Instance string       `json:"instance"`
	Event    events.Event `json:"event"`
}

type eventWatcher struct {
	ctx             context.Context
	begin           time.Time
	propagateStderr bool
	eventCh         chan watchEvent
	watching        sync.Map
}

func (w *eventWatcher) startInstance(instName string) {
	if _, loaded := w.watching.LoadOrStore(instName, true); loaded {
		return
	}

	inst, err := store.Inspect(w.ctx, instName)
	if err != nil {
		logrus.WithError(err).Warnf("Failed to inspect instance %q", instName)
		w.watching.Delete(instName)
		return
	}

	haStdoutPath := filepath.Join(inst.Dir, filenames.HostAgentStdoutLog)
	haStderrPath := filepath.Join(inst.Dir, filenames.HostAgentStderrLog)

	go w.watchInstance(instName, haStdoutPath, haStderrPath)
}

func (w *eventWatcher) watchInstance(instName, haStdoutPath, haStderrPath string) {
	err := events.Watch(w.ctx, haStdoutPath, haStderrPath, w.begin, w.propagateStderr, func(ev events.Event) bool {
		select {
		case w.eventCh <- watchEvent{Instance: instName, Event: ev}:
		case <-w.ctx.Done():
			return true
		}
		return false
	})
	if err != nil && w.ctx.Err() == nil {
		logrus.WithError(err).Warnf("Watcher for instance %q stopped", instName)
	}
}

func watchAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	jsonFormat, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}
	history, err := cmd.Flags().GetBool("history")
	if err != nil {
		return err
	}
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return err
	}

	if !verbose {
		logrus.SetLevel(logrus.WarnLevel)
	}

	var begin time.Time
	if !history {
		begin = time.Now()
	}

	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()
	watchAll := len(args) == 0

	var instNames []string
	if watchAll {
		instNames, err = store.Instances()
		if err != nil {
			return err
		}
		if len(instNames) == 0 {
			printStatus(stderr, "No instances found")
		}
	} else {
		instNames = args
	}

	newInstanceCh := make(chan string, 16)
	w := &eventWatcher{
		ctx:             ctx,
		begin:           begin,
		propagateStderr: verbose,
		eventCh:         make(chan watchEvent, 64),
	}

	for _, instName := range instNames {
		w.startInstance(instName)
	}

	if watchAll {
		go watchLimaDir(ctx, newInstanceCh)
	}

	printStatus(stderr, "Watching for events...")

	for {
		select {
		case <-ctx.Done():
			return nil
		case instName := <-newInstanceCh:
			printStatus(stderr, "New instance detected: "+instName)
			w.startInstance(instName)
		case ev := <-w.eventCh:
			if jsonFormat {
				j, err := json.Marshal(ev)
				if err != nil {
					fmt.Fprintf(stderr, "error marshaling event: %v\n", err)
					continue
				}
				fmt.Fprintln(stdout, string(j))
			} else {
				printHumanReadableEvent(stdout, ev.Instance, ev.Event)
			}
		}
	}
}

func watchLimaDir(ctx context.Context, newInstanceCh chan<- string) {
	limaDir := store.Directory()
	if limaDir == "" {
		logrus.Warn("Could not determine lima directory")
		return
	}

	fsEvents := make(chan notify.EventInfo, 128)
	if err := notify.Watch(limaDir, fsEvents, notify.Create); err != nil {
		logrus.WithError(err).Warn("Failed to watch lima directory for new instances")
		return
	}
	defer notify.Stop(fsEvents)

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-fsEvents:
			name := filepath.Base(ev.Path())
			if !isValidInstanceName(name) {
				continue
			}
			if !isInstanceDir(ev.Path()) {
				continue
			}
			select {
			case newInstanceCh <- name:
			case <-ctx.Done():
				return
			}
		}
	}
}

func isValidInstanceName(name string) bool {
	return !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "_")
}

func isInstanceDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	yamlPath := filepath.Join(path, filenames.LimaYAML)
	_, err = os.Stat(yamlPath)
	return err == nil
}

func printStatus(out io.Writer, msg string) {
	fmt.Fprintf(out, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

func printHumanReadableEvent(out io.Writer, instName string, ev events.Event) {
	timestamp := ev.Time.Format("2006-01-02 15:04:05")

	printEvent := func(msg string) {
		fmt.Fprintf(out, "%s %s | %s\n", timestamp, instName, msg)
	}

	if ev.Status.Running {
		if ev.Status.Degraded {
			printEvent("running (degraded)")
		} else {
			printEvent("running")
		}
	}
	if ev.Status.Exiting {
		printEvent("exiting")
	}
	if ev.Status.SSHLocalPort != 0 {
		printEvent(fmt.Sprintf("ssh available on port %d", ev.Status.SSHLocalPort))
	}
	for _, e := range ev.Status.Errors {
		printEvent(fmt.Sprintf("error: %s", e))
	}
	if ev.Status.CloudInitProgress != nil {
		if ev.Status.CloudInitProgress.Completed {
			printEvent("cloud-init completed")
		} else if ev.Status.CloudInitProgress.LogLine != "" {
			printEvent(fmt.Sprintf("cloud-init: %s", ev.Status.CloudInitProgress.LogLine))
		}
	}
	if ev.Status.PortForward != nil {
		pf := ev.Status.PortForward
		switch pf.Type {
		case events.PortForwardEventForwarding:
			printEvent(fmt.Sprintf("forwarding %s %s to %s", pf.Protocol, pf.GuestAddr, pf.HostAddr))
		case events.PortForwardEventNotForwarding:
			printEvent(fmt.Sprintf("not forwarding %s %s", pf.Protocol, pf.GuestAddr))
		case events.PortForwardEventStopping:
			printEvent(fmt.Sprintf("stopping forwarding %s %s", pf.Protocol, pf.GuestAddr))
		case events.PortForwardEventFailed:
			printEvent(fmt.Sprintf("failed to forward %s %s: %s", pf.Protocol, pf.GuestAddr, pf.Error))
		}
	}
	if ev.Status.Vsock != nil {
		vs := ev.Status.Vsock
		switch vs.Type {
		case events.VsockEventStarted:
			printEvent(fmt.Sprintf("started vsock forwarder: %s -> vsock:%d", vs.HostAddr, vs.VsockPort))
		case events.VsockEventSkipped:
			printEvent(fmt.Sprintf("skipped vsock forwarder: %s", vs.Reason))
		case events.VsockEventFailed:
			printEvent(fmt.Sprintf("failed to start vsock forwarder: %s", vs.Reason))
		}
	}
}

func watchBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
