// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usrlocalsharelima

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/debugutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// ExecutableViaArgs0 returns the absolute path to the executable used to start this process.
// It will also append the file extension on Windows, if necessary.
// This function is different from os.Executable(), which will use /proc/self/exe on Linux
// and therefore will resolve any symlink used to locate the executable. This function will
// return the symlink instead because we want to be able to locate ../share/lima relative
// to the location of the symlink, and not the actual executable. This is important when
// using Homebrew.
var ExecutableViaArgs0 = sync.OnceValues(func() (string, error) {
	if os.Args[0] == "" {
		return "", errors.New("os.Args[0] has not been set")
	}
	executable, err := exec.LookPath(os.Args[0])
	if err == nil {
		executable, err = filepath.Abs(executable)
	}
	if err != nil {
		return "", fmt.Errorf("os.Args[0] is invalid: %w", err)
	}

	return executable, nil
})

// SelfDirs returns a list of directory paths where the current executable might be located.
// It checks both os.Args[0] and os.Executable() methods and returns directories containing
// the executable, resolving symlinks as needed.
func SelfDirs() []string {
	var selfPaths []string

	selfViaArgs0, err := ExecutableViaArgs0()
	if err != nil {
		logrus.WithError(err).Warn("failed to find executable from os.Args[0]")
	} else {
		selfPaths = append(selfPaths, filepath.Dir(selfViaArgs0))
	}

	selfViaOS, err := os.Executable()
	if err != nil {
		logrus.WithError(err).Warn("failed to find os.Executable()")
	} else {
		selfFinalPathViaOS, err := filepath.EvalSymlinks(selfViaOS)
		if err != nil {
			logrus.WithError(err).Warn("failed to resolve symlinks")
			selfFinalPathViaOS = selfViaOS // fallback to the original path
		}

		selfDir := filepath.Dir(selfFinalPathViaOS)
		if len(selfPaths) == 0 || selfDir != selfPaths[0] {
			selfPaths = append(selfPaths, selfDir)
		}
	}

	return selfPaths
}

// Dir returns the location of the <PREFIX>/lima/share directory, relative to the location
// of the current executable. It checks for multiple possible filesystem layouts and returns
// the first candidate that contains the native guest agent binary.
func Dir() (string, error) {
	selfDirs := SelfDirs()

	ostype := limatype.NewOS("linux")
	arch := limatype.NewArch(runtime.GOARCH)
	if arch == "" {
		return "", fmt.Errorf("failed to get arch for %q", runtime.GOARCH)
	}

	gaCandidates := []string{}
	for _, selfDir := range selfDirs {
		// selfDir:  /usr/local/bin
		selfDirDir := filepath.Dir(selfDir)
		gaCandidates = append(gaCandidates,
			// candidate 0:
			// - self:  /Applications/Lima.app/Contents/MacOS/limactl
			// - agent: /Applications/Lima.app/Contents/MacOS/lima-guestagent.Linux-x86_64
			// - dir:   /Applications/Lima.app/Contents/MacOS
			filepath.Join(selfDir, "lima-guestagent."+ostype+"-"+arch),
			// candidate 1:
			// - self:  /usr/local/bin/limactl
			// - agent: /usr/local/share/lima/lima-guestagent.Linux-x86_64
			// - dir:   /usr/local/share/lima
			filepath.Join(selfDirDir, "share/lima/lima-guestagent."+ostype+"-"+arch),
			// TODO: support custom path
		)
		if debugutil.Debug {
			// candidate 2: launched by `~/go/bin/dlv dap`
			// - self: ${workspaceFolder}/cmd/limactl/__debug_bin_XXXXXX
			// - agent: ${workspaceFolder}/_output/share/lima/lima-guestagent.Linux-x86_64
			// - dir:  ${workspaceFolder}/_output/share/lima
			candidateForDebugBuild := filepath.Join(filepath.Dir(selfDirDir), "_output/share/lima/lima-guestagent."+ostype+"-"+arch)
			gaCandidates = append(gaCandidates, candidateForDebugBuild)
			logrus.Infof("debug mode detected, adding more guest agent candidates: %v", candidateForDebugBuild)
		}
	}

	for _, gaCandidate := range gaCandidates {
		if _, err := os.Stat(gaCandidate); err == nil {
			return filepath.Dir(gaCandidate), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if _, err := os.Stat(gaCandidate + ".gz"); err == nil {
			return filepath.Dir(gaCandidate), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	return "", fmt.Errorf("failed to find \"lima-guestagent.%s-%s\" binary for %v, attempted %v",
		ostype, arch, selfDirs, gaCandidates)
}

// GuestAgentBinary returns the absolute path of the guest agent binary, possibly with ".gz" suffix.
func GuestAgentBinary(ostype limatype.OS, arch limatype.Arch) (string, error) {
	if ostype == "" {
		return "", errors.New("os must be set")
	}
	if arch == "" {
		return "", errors.New("arch must be set")
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	uncomp := filepath.Join(dir, "lima-guestagent."+ostype+"-"+arch)
	comp := uncomp + ".gz"
	res, err := chooseGABinary([]string{comp, uncomp})
	if err != nil {
		logrus.Debug(err)
		return "", fmt.Errorf("guest agent binary could not be found for %s-%s: %w (Hint: try installing `lima-additional-guestagents` package)", ostype, arch, err)
	}
	return res, nil
}

func chooseGABinary(candidates []string) (string, error) {
	var entries []string
	for _, f := range candidates {
		if _, err := os.Stat(f); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				logrus.WithError(err).Warnf("failed to stat %q", f)
			}
			continue
		}
		entries = append(entries, f)
	}
	switch len(entries) {
	case 0:
		return "", fmt.Errorf("%w: attempted %v", fs.ErrNotExist, candidates)
	case 1:
		return entries[0], nil
	default:
		logrus.Warnf("multiple files found, choosing %q from %v; consider removing the other ones",
			entries[0], candidates)
		return entries[0], nil
	}
}

// LibexecLima returns the <PREFIX>/libexec/lima directories.
// For Homebrew compatibility, it also checks <PREFIX>/lib/lima.
func LibexecLima() ([]string, error) {
	var candidates []string
	selfDirs := SelfDirs()
	for _, selfDir := range selfDirs {
		// selfDir:  /usr/local/bin
		// prefix: /usr/local
		// candidate: /usr/local/libexec/lima
		prefix := filepath.Dir(selfDir)
		candidate := filepath.Join(prefix, "libexec", "lima")
		if ents, err := os.ReadDir(candidate); err == nil && len(ents) > 0 {
			candidates = append(candidates, candidate)
		}
		// selfDir: /opt/homebrew/bin
		// prefix: /opt/homebrew
		// candidate: /opt/homebrew/lib/lima
		//
		// Note that there is no /opt/homebrew/libexec directory,
		// as Homebrew reserves libexec for private use.
		// https://github.com/lima-vm/lima/issues/4295#issuecomment-3490680651
		candidate = filepath.Join(prefix, "lib", "lima")
		if ents, err := os.ReadDir(candidate); err == nil && len(ents) > 0 {
			candidates = append(candidates, candidate)
		}
	}
	return candidates, nil
}
