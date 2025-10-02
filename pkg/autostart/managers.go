// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package autostart manage start at login unit files for darwin/linux
package autostart

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lima-vm/lima/v2/pkg/autostart/launchd"
	"github.com/lima-vm/lima/v2/pkg/autostart/systemd"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/textutil"
)

type notSupportedManager struct{}

var NotSupportedError = fmt.Errorf("autostart is not supported on %s", runtime.GOOS)

func (*notSupportedManager) IsRegistered(_ context.Context, _ *limatype.Instance) (bool, error) {
	return false, NotSupportedError
}

func (*notSupportedManager) RegisterToStartAtLogin(_ context.Context, _ *limatype.Instance) error {
	return NotSupportedError
}

func (*notSupportedManager) UnregisterFromStartAtLogin(_ context.Context, _ *limatype.Instance) error {
	return NotSupportedError
}

// Launchd is the autostart manager for macOS.
var Launchd = &TemplateFileBasedManager{
	filePath: launchd.GetPlistPath,
	template: launchd.Template,
	enabler:  launchd.EnableDisableService,
}

// Systemd is the autostart manager for Linux.
var Systemd = &TemplateFileBasedManager{
	filePath: systemd.GetUnitPath,
	template: systemd.Template,
	enabler:  systemd.EnableDisableUnit,
}

// TemplateFileBasedManager is an autostart manager that uses a template file to create the autostart entry.
type TemplateFileBasedManager struct {
	enabler  func(ctx context.Context, enable bool, instName string) error
	filePath func(instName string) string
	template string
}

func (t *TemplateFileBasedManager) IsRegistered(ctx context.Context, inst *limatype.Instance) (bool, error) {
	if t.filePath == nil {
		return false, fmt.Errorf("no filePath function available")
	}
	autostartFilePath := t.filePath(inst.Name)
	if _, err := os.Stat(autostartFilePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (m *TemplateFileBasedManager) RegisterToStartAtLogin(ctx context.Context, inst *limatype.Instance) error {
	if _, err := m.IsRegistered(ctx, inst); err != nil {
		return fmt.Errorf("failed to check if the autostart entry for instance %q is registered: %w", inst.Name, err)
	}
	content, err := m.renderTemplate(inst.Name, inst.Dir, os.Executable)
	if err != nil {
		return fmt.Errorf("failed to render the autostart entry for instance %q: %w", inst.Name, err)
	}
	entryFilePath := m.filePath(inst.Name)
	if err := os.MkdirAll(filepath.Dir(entryFilePath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create the directory for the autostart entry for instance %q: %w", inst.Name, err)
	}
	if err := os.WriteFile(entryFilePath, content, 0o644); err != nil {
		return fmt.Errorf("failed to write the autostart entry for instance %q: %w", inst.Name, err)
	}
	if m.enabler != nil {
		return m.enabler(ctx, true, inst.Name)
	} else {
		return nil
	}
}

func (m *TemplateFileBasedManager) UnregisterFromStartAtLogin(ctx context.Context, inst *limatype.Instance) error {
	if registered, err := m.IsRegistered(ctx, inst); err != nil {
		return fmt.Errorf("failed to check if the autostart entry for instance %q is registered: %w", inst.Name, err)
	} else if !registered {
		return nil
	}
	if m.enabler != nil {
		if err := m.enabler(ctx, false, inst.Name); err != nil {
			return fmt.Errorf("failed to disable the autostart entry for instance %q: %w", inst.Name, err)
		}
	}
	if err := os.Remove(m.filePath(inst.Name)); err != nil {
		return fmt.Errorf("failed to remove the autostart entry for instance %q: %w", inst.Name, err)
	}
	return nil
}

func (m *TemplateFileBasedManager) renderTemplate(instName, workDir string, getExecutable func() (string, error)) ([]byte, error) {
	if m.template == "" {
		return nil, fmt.Errorf("no template available")
	}
	selfExeAbs, err := getExecutable()
	if err != nil {
		return nil, err
	}
	return textutil.ExecuteTemplate(
		m.template,
		map[string]string{
			"Binary":   selfExeAbs,
			"Instance": instName,
			"WorkDir":  workDir,
		})
}
