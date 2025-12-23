// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/hostagent/events"
	"github.com/lima-vm/lima/v2/pkg/limatype"
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

If no instance is specified, events from all running instances are watched.

The command will continue watching until interrupted (Ctrl+C).`,
		Example: `  # Watch events from all instances:
  $ limactl watch

  # Watch events from a specific instance:
  $ limactl watch default

  # Watch events in JSON format (for scripting):
  $ limactl watch --json default`,
		Args:              WrapArgsError(cobra.ArbitraryArgs),
		RunE:              watchAction,
		ValidArgsFunction: watchBashComplete,
		GroupID:           advancedCommand,
	}
	watchCommand.Flags().Bool("json", false, "Output events as newline-delimited JSON")
	return watchCommand
}

// watchEvent wraps an event with its instance name for JSON output.
type watchEvent struct {
	Instance string       `json:"instance"`
	Event    events.Event `json:"event"`
}

func watchAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	jsonFormat, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}

	// Determine which instances to watch
	var instNames []string
	if len(args) > 0 {
		instNames = args
	} else {
		// Watch all instances
		allInstances, err := store.Instances()
		if err != nil {
			return err
		}
		if len(allInstances) == 0 {
			logrus.Warn("No instances found.")
			return nil
		}
		instNames = allInstances
	}

	// Validate instances and collect their log paths
	type instanceInfo struct {
		name         string
		haStdoutPath string
		haStderrPath string
	}
	var instances []instanceInfo

	for _, instName := range instNames {
		inst, err := store.Inspect(ctx, instName)
		if err != nil {
			return err
		}
		if inst.Status != limatype.StatusRunning {
			logrus.Warnf("Instance %q is not running (status: %s). Watching for events anyway...", instName, inst.Status)
		}
		instances = append(instances, instanceInfo{
			name:         instName,
			haStdoutPath: filepath.Join(inst.Dir, filenames.HostAgentStdoutLog),
			haStderrPath: filepath.Join(inst.Dir, filenames.HostAgentStderrLog),
		})
	}

	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	// If only one instance, watch it directly
	if len(instances) == 1 {
		inst := instances[0]
		return events.Watch(ctx, inst.haStdoutPath, inst.haStderrPath, time.Now(), !jsonFormat, func(ev events.Event) bool {
			if jsonFormat {
				we := watchEvent{Instance: inst.name, Event: ev}
				j, err := json.Marshal(we)
				if err != nil {
					fmt.Fprintf(stderr, "error marshaling event: %v\n", err)
					return false
				}
				fmt.Fprintln(stdout, string(j))
			} else {
				printHumanReadableEvent(stdout, inst.name, ev)
			}
			return false
		})
	}

	// Watch multiple instances concurrently
	type eventWithInstance struct {
		instance string
		event    events.Event
	}
	eventCh := make(chan eventWithInstance)
	errCh := make(chan error, len(instances))

	for _, inst := range instances {
		go func() {
			err := events.Watch(ctx, inst.haStdoutPath, inst.haStderrPath, time.Now(), !jsonFormat, func(ev events.Event) bool {
				select {
				case eventCh <- eventWithInstance{instance: inst.name, event: ev}:
				case <-ctx.Done():
					return true
				}
				return false
			})
			if err != nil {
				errCh <- fmt.Errorf("instance %s: %w", inst.name, err)
			}
		}()
	}

	// Process events from all instances
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			return err
		case ev := <-eventCh:
			if jsonFormat {
				we := watchEvent{Instance: ev.instance, Event: ev.event}
				j, err := json.Marshal(we)
				if err != nil {
					fmt.Fprintf(stderr, "error marshaling event: %v\n", err)
					continue
				}
				fmt.Fprintln(stdout, string(j))
			} else {
				printHumanReadableEvent(stdout, ev.instance, ev.event)
			}
		}
	}
}

func printHumanReadableEvent(out io.Writer, instName string, ev events.Event) {
	timestamp := ev.Time.Format("2006-01-02 15:04:05")

	printEvent := func(msg string) {
		fmt.Fprintf(out, "%s %s | %s\n", timestamp, instName, msg)
	}

	// Status changes
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

	// SSH port
	if ev.Status.SSHLocalPort != 0 {
		printEvent(fmt.Sprintf("ssh available on port %d", ev.Status.SSHLocalPort))
	}

	// Errors
	for _, e := range ev.Status.Errors {
		printEvent(fmt.Sprintf("error: %s", e))
	}

	// Cloud-init progress
	if ev.Status.CloudInitProgress != nil {
		if ev.Status.CloudInitProgress.Completed {
			printEvent("cloud-init completed")
		} else if ev.Status.CloudInitProgress.LogLine != "" {
			printEvent(fmt.Sprintf("cloud-init: %s", ev.Status.CloudInitProgress.LogLine))
		}
	}

	// Port forwarding events
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

	// Vsock events
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
