// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func TestInspectStatus(t *testing.T) {
	tests := []struct {
		name           string
		setupInstance  func(*limatype.Instance)
		expectedStatus string
	}{
		{
			name: "nil instance",
			setupInstance: func(inst *limatype.Instance) {
				*inst = limatype.Instance{}
			},
			expectedStatus: limatype.StatusBroken,
		},
		{
			name: "empty directory",
			setupInstance: func(inst *limatype.Instance) {
				*inst = limatype.Instance{
					Dir:    "",
					Config: &limatype.LimaYAML{},
				}
			},
			expectedStatus: limatype.StatusBroken,
		},
		{
			name: "no PID file",
			setupInstance: func(inst *limatype.Instance) {
				tempDir := t.TempDir()
				*inst = limatype.Instance{
					Dir:    tempDir,
					Config: &limatype.LimaYAML{},
				}
			},
			expectedStatus: "", // fallback should handle this
		},
		{
			name: "PID file with valid process",
			setupInstance: func(inst *limatype.Instance) {
				tempDir := t.TempDir()
				pidFile := filepath.Join(tempDir, filenames.PIDFile(limatype.QEMU))

				// Create a PID file with the current test process's PID (guaranteed to be running)
				pidStr := strconv.Itoa(os.Getpid())
				if err := os.WriteFile(pidFile, []byte(pidStr), 0o644); err != nil {
					assert.NilError(t, err)
				}

				*inst = limatype.Instance{
					Dir:    tempDir,
					Config: &limatype.LimaYAML{},
				}
			},
			expectedStatus: "", // fallback should handle this
		},
		{
			name: "PID file with dead process",
			setupInstance: func(inst *limatype.Instance) {
				tempDir := t.TempDir()
				pidFile := filepath.Join(tempDir, filenames.PIDFile(limatype.QEMU))

				// Create a short-lived child process and use its PID after it exits to ensure it's dead
				var cmd *exec.Cmd
				if runtime.GOOS == "windows" {
					cmd = exec.CommandContext(t.Context(), "cmd", "/C", "exit", "0")
				} else {
					// Use a longer-lived process to avoid immediate PID reuse races
					cmd = exec.CommandContext(t.Context(), "sh", "-c", "sleep 5")
				}
				if err := cmd.Start(); err != nil {
					assert.NilError(t, err)
				}
				pid := cmd.Process.Pid
				if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o644); err != nil {
					assert.NilError(t, err)
				}
				// Ensure the process is dead when InspectStatus reads the PID file
				if runtime.GOOS != "windows" {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}

				*inst = limatype.Instance{
					Dir:    tempDir,
					Config: &limatype.LimaYAML{},
				}
			},
			expectedStatus: func() string {
				// On Windows, ReadPIDFile returns the PID without checking liveness, so InspectStatus returns "".
				if runtime.GOOS == "windows" {
					return ""
				}
				return limatype.StatusBroken
			}(),
		},
		{
			name: "PID file with invalid content",
			setupInstance: func(inst *limatype.Instance) {
				tempDir := t.TempDir()
				pidFile := filepath.Join(tempDir, filenames.PIDFile(limatype.QEMU))

				// Create a PID file with invalid content (non-numeric)
				if err := os.WriteFile(pidFile, []byte("invalid"), 0o644); err != nil {
					assert.NilError(t, err)
				}

				*inst = limatype.Instance{
					Dir:    tempDir,
					Config: &limatype.LimaYAML{},
				}
			},
			expectedStatus: limatype.StatusBroken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inst limatype.Instance
			tt.setupInstance(&inst)

			driver := &LimaQemuDriver{}
			status := driver.InspectStatus(t.Context(), &inst)

			if status != tt.expectedStatus {
				t.Errorf("InspectStatus() = %v, want %v", status, tt.expectedStatus)
			}
		})
	}
}
