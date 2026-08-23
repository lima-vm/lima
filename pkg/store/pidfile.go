// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

// pidFileSuffix is the suffix of the PID files of an instance.
const pidFileSuffix = ".pid"

// bootSessionID is a variable so that it can be replaced in the tests.
var bootSessionID = osutil.BootSessionID

// currentBootSessionID returns the ID of the current boot of the host, or an empty string
// on the platforms that do not provide one. Any other failure is returned as an error:
// guessing would either void a PID file that is current, or trust one that is not.
func currentBootSessionID() (string, error) {
	id, err := bootSessionID()
	if errors.Is(err, osutil.ErrBootSessionNotSupported) {
		// PID files are handled the way they were before the marker was introduced.
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to determine the boot session ID: %w", err)
	}
	return id, nil
}

// WritePIDFile writes pid to path.
//
// The directory of path holds a marker with the boot session ID of the host, so that
// ReadPIDFile can tell whether a PID file was written during the current boot. When the
// marker is from a previous boot, every PID file of the directory is removed before the
// marker is refreshed: those PIDs are meaningless, and refreshing the marker would
// otherwise make them look current.
func WritePIDFile(path string, pid int) error {
	dir := filepath.Dir(path)
	current, err := currentBootSessionID()
	if err != nil {
		return err
	}
	if current != "" {
		stale, err := staleBootSession(dir, current)
		if err != nil {
			return err
		}
		if stale {
			if err := removePIDFiles(dir); err != nil {
				return err
			}
		}
		if err := writeBootSessionMarker(dir, current); err != nil {
			return err
		}
	}
	return writePIDFileAtomically(path, pid)
}

// writePIDFileAtomically renames the PID file into place, so that a reader never sees a partially
// written one.
func writePIDFileAtomically(path string, pid int) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, fmt.Appendf(nil, "%d\n", pid), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// pidFileFromPreviousBoot returns true when the PID file at path was written during a
// previous boot of the host.
func pidFileFromPreviousBoot(path string) (bool, error) {
	current, err := currentBootSessionID()
	if err != nil || current == "" {
		return false, err
	}
	return staleBootSession(filepath.Dir(path), current)
}

// staleBootSession returns true when dir holds a boot session marker of a boot other than
// the current one.
func staleBootSession(dir, current string) (bool, error) {
	b, err := os.ReadFile(filepath.Join(dir, filenames.BootSession))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No marker: the PID files were written by an older version of Lima.
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(b)) != current, nil
}

func writeBootSessionMarker(dir, id string) error {
	return os.WriteFile(filepath.Join(dir, filenames.BootSession), []byte(id+"\n"), 0o644)
}

func removePIDFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), pidFileSuffix) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		logrus.Infof("Removing PID file %#q left behind by a previous boot", path)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
