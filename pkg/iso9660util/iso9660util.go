package iso9660util

import (
	"io"
	"os"
	"strings"

	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
	"github.com/pkg/errors"
)

type Entry struct {
	Path   string
	Reader io.Reader
}

func Write(isoPath, label string, layout []Entry) error {
	if err := os.RemoveAll(isoPath); err != nil {
		return err
	}

	isoFile, err := os.Create(isoPath)
	if err != nil {
		return err
	}

	defer isoFile.Close()

	fs, err := iso9660.Create(isoFile, 0, 0, 0, "")
	if err != nil {
		return err
	}

	for _, f := range layout {
		if _, err := WriteFile(fs, f.Path, f.Reader); err != nil {
			return err
		}
	}

	finalizeOptions := iso9660.FinalizeOptions{
		RockRidge:        true,
		VolumeIdentifier: label,
	}
	if err := fs.Finalize(finalizeOptions); err != nil {
		return err
	}

	return isoFile.Close()
}

func WriteFile(fs filesystem.FileSystem, path string, r io.Reader) (int64, error) {
	if strings.Contains(path, "/") || strings.Contains(path, "\\") {
		return 0, errors.Errorf("directory not supported yet: %q", path)
	}
	f, err := fs.OpenFile(path, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}
