// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/store"
)

type Backend string

const (
	BackendAuto  Backend = "auto"
	BackendRsync Backend = "rsync"
	BackendSCP   Backend = "scp"
)

type Path struct {
	InstanceName string
	Path         string
	IsRemote     bool
	Instance     *limatype.Instance
}

// Options contains common options for copy operations. This might not be a complete list;
// more options can be added as needed.
type Options struct {
	Recursive bool
	Verbose   bool
}

// CopyTool is the interface for copy tool implementations.
type CopyTool interface {
	// Name returns the name of the copy tool.
	Name() string
	// Command builds and returns the exec.Cmd for the copy operation.
	Command(ctx context.Context, opts *Options) (*exec.Cmd, error)
	// IsAvailableOnGuest checks if the tool is available on the specified guest instance.
	IsAvailableOnGuest(ctx context.Context) bool
}

func New(ctx context.Context, backend string, args []string) (CopyTool, error) {
	copyPaths, err := parseCopyArgs(ctx, args)
	if err != nil {
		return nil, err
	}

	switch Backend(backend) {
	case BackendSCP:
		return newSCPTool(copyPaths)
	case BackendRsync:
		rsync, err := newRsyncTool(copyPaths)
		if err != nil {
			return nil, err
		}

		if !rsync.IsAvailableOnGuest(ctx) {
			return nil, errors.New("rsync not available on guest(s)")
		}
		return rsync, nil
	case BackendAuto:
		var tool CopyTool
		tool, err = newRsyncTool(copyPaths)
		if err == nil {
			if tool.IsAvailableOnGuest(ctx) {
				return tool, nil
			}
			logrus.Debugf("rsync not available on guest(s), falling back to scp")
		} else {
			logrus.Debugf("rsync not found on host, falling back to scp: %v", err)
		}

		tool, err = newSCPTool(copyPaths)
		if err != nil {
			return nil, fmt.Errorf("neither rsync nor scp found on host: %w", err)
		}
		return tool, nil
	default:
		return nil, fmt.Errorf("invalid backend %q, must be one of: scp, rsync, auto", backend)
	}
}

func parseCopyArgs(ctx context.Context, args []string) ([]*Path, error) {
	var copyPaths []*Path

	for _, arg := range args {
		cp := &Path{}
		if runtime.GOOS == "windows" {
			if filepath.IsAbs(arg) {
				var err error
				arg, err = ioutilx.WindowsSubsystemPath(ctx, arg)
				if err != nil {
					return nil, err
				}
			} else {
				arg = filepath.ToSlash(arg)
			}
		}

		parts := strings.SplitN(arg, ":", 2)
		switch len(parts) {
		case 1:
			cp.Path = arg
			cp.IsRemote = false
		case 2:
			cp.InstanceName = parts[0]
			cp.Path = parts[1]
			cp.IsRemote = true

			inst, err := store.Inspect(ctx, cp.InstanceName)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil, fmt.Errorf("instance %q does not exist, run `limactl create %s` to create a new instance", cp.InstanceName, cp.InstanceName)
				}
				return nil, err
			}
			if inst.Status == limatype.StatusStopped {
				return nil, fmt.Errorf("instance %q is stopped, run `limactl start %s` to start the instance", cp.InstanceName, cp.InstanceName)
			}
			cp.Instance = inst
		default:
			return nil, fmt.Errorf("path %q contains multiple colons", arg)
		}

		copyPaths = append(copyPaths, cp)
	}

	return copyPaths, nil
}
