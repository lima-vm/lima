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

// Options contains common options for copy operations. This might not be a complete list; more options can be added as needed.
type Options struct {
	Recursive      bool
	Verbose        bool
	AdditionalArgs []string // Make sure that the additional args are valid for a specific tool and escaped before passing them here.
}

// CopyTool is the interface for copy tool implementations.
type CopyTool interface {
	// Name returns the name of the copy tool.
	Name() string
	// Command builds and returns the exec.Cmd for the copy operation. If opts is set, it completely overrides the tool's Options which were set during initialization.
	Command(ctx context.Context, paths []string, opts *Options) (*exec.Cmd, error)
	// IsAvailableOnGuest checks if the tool is available on the specified guest instance.
	IsAvailableOnGuest(ctx context.Context, paths []string) bool
}

// New creates a new CopyTool based on the specified backend.
func New(ctx context.Context, backend string, paths []string, opts *Options) (CopyTool, error) {
	switch Backend(backend) {
	case BackendSCP:
		return newSCPTool(opts)
	case BackendRsync:
		rsync, err := newRsyncTool(opts)
		if err != nil {
			return nil, err
		}

		if !rsync.IsAvailableOnGuest(ctx, paths) {
			return nil, errors.New("rsync not available on guest(s)")
		}
		return rsync, nil
	case BackendAuto:
		var (
			tool CopyTool
			err  error
		)
		tool, err = newRsyncTool(opts)
		if err == nil {
			if tool.IsAvailableOnGuest(ctx, paths) {
				return tool, nil
			}
			logrus.Debugf("rsync not available on guest(s), falling back to scp")
		} else {
			logrus.Debugf("rsync not found on host, falling back to scp: %v", err)
		}

		tool, err = newSCPTool(opts)
		if err != nil {
			return nil, fmt.Errorf("neither rsync nor scp found on host: %w", err)
		}
		return tool, nil
	default:
		return nil, fmt.Errorf("invalid backend %q, must be one of: scp, rsync, auto", backend)
	}
}

func parseCopyPaths(ctx context.Context, paths []string) ([]*Path, error) {
	var copyPaths []*Path

	for _, path := range paths {
		cp := &Path{}
		if runtime.GOOS == "windows" {
			if filepath.IsAbs(path) {
				var err error
				path, err = ioutilx.WindowsSubsystemPath(ctx, path)
				if err != nil {
					return nil, err
				}
			} else {
				path = filepath.ToSlash(path)
			}
		}

		parts := strings.SplitN(path, ":", 2)
		switch len(parts) {
		case 1:
			cp.Path = path
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
			return nil, fmt.Errorf("path %q contains multiple colons", path)
		}

		copyPaths = append(copyPaths, cp)
	}

	return copyPaths, nil
}
