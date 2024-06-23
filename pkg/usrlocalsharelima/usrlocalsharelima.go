package usrlocalsharelima

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

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

func Open(path string) (io.ReadCloser, error) {
	reader, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		reader, err := os.Open(path + ".gz")
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Decompressing %s.gz", path)
		return gzip.NewReader(reader)
	}
	if err != nil {
		return nil, err
	}
	return reader, nil
}
