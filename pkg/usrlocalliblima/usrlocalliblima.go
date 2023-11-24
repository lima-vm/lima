package usrlocalliblima

import (
	"io/fs"
	"os"
	"path/filepath"
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

	// self:  /usr/local/bin/limactl
	selfDir := filepath.Dir(self)
	selfDirDir := filepath.Dir(selfDir)
	libLimaDir := filepath.Join(selfDirDir, "lib", "lima")
	if _, err := os.Stat(libLimaDir); err == nil {
		return libLimaDir, nil
	} else {
		return "", err
	}
}
