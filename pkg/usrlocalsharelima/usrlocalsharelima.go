/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package usrlocalsharelima

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lima-vm/lima/pkg/debugutil"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/sirupsen/logrus"
)

func Dir() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	selfSt, err := os.Stat(self)
	if err != nil {
		return "", err
	}
	if selfSt.Mode()&fs.ModeSymlink != 0 {
		self, err = os.Readlink(self)
		if err != nil {
			return "", err
		}
	}

	ostype := limayaml.NewOS("linux")
	arch := limayaml.NewArch(runtime.GOARCH)
	if arch == "" {
		return "", fmt.Errorf("failed to get arch for %q", runtime.GOARCH)
	}

	// self:  /usr/local/bin/limactl
	selfDir := filepath.Dir(self)
	selfDirDir := filepath.Dir(selfDir)
	gaCandidates := []string{
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
	}
	if debugutil.Debug {
		// candidate 2: launched by `~/go/bin/dlv dap`
		// - self: ${workspaceFolder}/cmd/limactl/__debug_bin_XXXXXX
		// - agent: ${workspaceFolder}/_output/share/lima/lima-guestagent.Linux-x86_64
		// - dir:  ${workspaceFolder}/_output/share/lima
		candidateForDebugBuild := filepath.Join(filepath.Dir(selfDirDir), "_output/share/lima/lima-guestagent."+ostype+"-"+arch)
		gaCandidates = append(gaCandidates, candidateForDebugBuild)
		logrus.Infof("debug mode detected, adding more guest agent candidates: %v", candidateForDebugBuild)
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

	return "", fmt.Errorf("failed to find \"lima-guestagent.%s-%s\" binary for %q, attempted %v",
		ostype, arch, self, gaCandidates)
}

func GuestAgentBinary(ostype limayaml.OS, arch limayaml.Arch) (string, error) {
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
	return filepath.Join(dir, "lima-guestagent."+ostype+"-"+arch), nil
}
