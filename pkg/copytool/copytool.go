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

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
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
	// Command builds and returns the exec.Cmd for the copy operation. If opts is set, it is used for this invocation instead of the tool's Options set during initialization, without modifying the stored Options.
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
		// On Windows, detect local absolute paths (e.g. C:\..., C:/..., UNC
		// paths) before splitting on ":" so a drive letter is not mistaken
		// for an instance name. filepath.IsAbs is deliberate here: a
		// drive-relative path like "C:foo.txt" is not absolute, so it still
		// flows through the colon-split below and is interpreted as
		// instance "C" path "foo.txt". This preserves pre-PR behaviour for
		// single-letter instance names, at the cost of "C:foo" as a
		// local-cwd-relative path no longer being recognized — an obscure
		// form not supported by the pre-PR code either. UNC paths pass
		// IsAbs and flow through PathForSSH; native ssh accepts the
		// //server/share/... form produced by filepath.ToSlash.
		isLocalAbs := runtime.GOOS == "windows" && filepath.IsAbs(path)
		if isLocalAbs {
			sshExe, err := sshutil.NewSSHExe()
			if err != nil {
				return nil, err
			}
			path, err = sshutil.PathForSSH(ctx, sshExe, path)
			if err != nil {
				return nil, err
			}
			cp.Path = path
			cp.IsRemote = false
			copyPaths = append(copyPaths, cp)
			continue
		}
		if runtime.GOOS == "windows" {
			path = filepath.ToSlash(path)
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
