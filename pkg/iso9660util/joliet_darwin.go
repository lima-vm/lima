// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package iso9660util

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func writeJoliet(isoPath, label string, layout []Entry) error {
	if err := os.RemoveAll(isoPath); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "joliet")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	for _, entry := range layout {
		path := filepath.Join(tmpDir, entry.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, entry.Reader); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	ctx := context.TODO()
	cmd := exec.CommandContext(ctx, "hdiutil", "makehybrid", "-o", isoPath, "-iso", "-joliet", "-default-volume-name", label, tmpDir)
	logrus.Debugf("Executing %v", cmd.Args)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run %v: %w (output=%q)", cmd.Args, err, string(b))
	}
	return nil
}
