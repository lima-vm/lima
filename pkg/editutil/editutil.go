// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package editutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/editutil/editorcmd"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func fileWarning(filename string) string {
	b, err := os.ReadFile(filename)
	if err != nil || len(b) == 0 {
		return ""
	}
	s := "# WARNING: " + filename + " includes the following settings,\n"
	s += "# which are applied before applying this YAML:\n"
	s += "# -----------\n"
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		s += "#"
		if line != "" {
			s += " " + line
		}
		s += "\n"
	}
	s += "# -----------\n"
	s += "\n"
	return s
}

// GenerateEditorWarningHeader generates the editor warning header.
func GenerateEditorWarningHeader() string {
	var s string
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		s += "# WARNING: failed to load the config dir\n"
		s += "\n"
		return s
	}

	s += fileWarning(filepath.Join(configDir, filenames.Default))
	s += fileWarning(filepath.Join(configDir, filenames.Override))
	return s
}

// OpenEditor opens an editor, and returns the content (not path) of the modified yaml.
//
// OpenEditor returns nil when the file was saved as an empty file, optionally with whitespaces.
func OpenEditor(ctx context.Context, content []byte, hdr string) ([]byte, error) {
	editor := editorcmd.Detect()
	if editor == "" {
		return nil, errors.New("could not detect a text editor binary, try setting $EDITOR")
	}
	tmpYAMLFile, err := os.CreateTemp("", "lima-editor-")
	if err != nil {
		return nil, err
	}
	tmpYAMLPath := tmpYAMLFile.Name()
	defer os.RemoveAll(tmpYAMLPath)
	if _, err := tmpYAMLFile.Write(append([]byte(hdr), content...)); err != nil {
		tmpYAMLFile.Close()
		return nil, err
	}
	if err := tmpYAMLFile.Close(); err != nil {
		return nil, err
	}

	editorCmd := exec.CommandContext(ctx, editor, tmpYAMLPath)
	editorCmd.Env = os.Environ()
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	logrus.Debugf("opening editor %q for a file %q", editor, tmpYAMLPath)
	if err := editorCmd.Run(); err != nil {
		return nil, fmt.Errorf("could not execute editor %q for a file %q: %w", editor, tmpYAMLPath, err)
	}
	b, err := os.ReadFile(tmpYAMLPath)
	if err != nil {
		return nil, err
	}
	modifiedInclHdr := string(b)
	modifiedExclHdr := strings.TrimPrefix(modifiedInclHdr, hdr)
	if strings.TrimSpace(modifiedExclHdr) == "" {
		return nil, nil
	}
	return []byte(modifiedExclHdr), nil
}
