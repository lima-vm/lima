// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package autostart manage start at login unit files for darwin/linux
package autostart

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/textutil"
)

type notSupportedManager struct{}

var _ autoStartManager = (*notSupportedManager)(nil)

var ErrNotSupported = fmt.Errorf("autostart is not supported on %s", runtime.GOOS)

func (*notSupportedManager) IsRegistered(_ context.Context, _ *limatype.Instance) (bool, error) {
	return false, ErrNotSupported
}

func (*notSupportedManager) RegisterToStartAtLogin(_ context.Context, _ *limatype.Instance) error {
	return ErrNotSupported
}

func (*notSupportedManager) UnregisterFromStartAtLogin(_ context.Context, _ *limatype.Instance) error {
	return ErrNotSupported
}

func (*notSupportedManager) AutoStartedIdentifier() string {
	return ""
}

func (*notSupportedManager) RequestStart(_ context.Context, _ *limatype.Instance) error {
	return ErrNotSupported
}

func (*notSupportedManager) RequestStop(_ context.Context, _ *limatype.Instance) (bool, error) {
	return false, ErrNotSupported
}

// TemplateFileBasedManager is an autostart manager that uses a template file to create the autostart entry.
type TemplateFileBasedManager struct {
	enabler               func(ctx context.Context, enable bool, instName string) error
	filePath              func(instName string) string
	template              string
	autoStartedIdentifier func() string
	requestStart          func(ctx context.Context, inst *limatype.Instance) error
	requestStop           func(ctx context.Context, inst *limatype.Instance) (bool, error)
}

var _ autoStartManager = (*TemplateFileBasedManager)(nil)

func (t *TemplateFileBasedManager) IsRegistered(_ context.Context, inst *limatype.Instance) (bool, error) {
	if t.filePath == nil {
		return false, errors.New("no filePath function available")
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

func (t *TemplateFileBasedManager) RegisterToStartAtLogin(ctx context.Context, inst *limatype.Instance) error {
	if _, err := t.IsRegistered(ctx, inst); err != nil {
		return fmt.Errorf("failed to check if the autostart entry for instance %q is registered: %w", inst.Name, err)
	}
	content, err := t.renderTemplate(inst.Name, inst.Dir, os.Executable)
	if err != nil {
		return fmt.Errorf("failed to render the autostart entry for instance %q: %w", inst.Name, err)
	}
	entryFilePath := t.filePath(inst.Name)
	if err := os.MkdirAll(filepath.Dir(entryFilePath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create the directory for the autostart entry for instance %q: %w", inst.Name, err)
	}
	if err := os.WriteFile(entryFilePath, content, 0o644); err != nil {
		return fmt.Errorf("failed to write the autostart entry for instance %q: %w", inst.Name, err)
	}
	if t.enabler != nil {
		return t.enabler(ctx, true, inst.Name)
	}
	return nil
}

func (t *TemplateFileBasedManager) UnregisterFromStartAtLogin(ctx context.Context, inst *limatype.Instance) error {
	if registered, err := t.IsRegistered(ctx, inst); err != nil {
		return fmt.Errorf("failed to check if the autostart entry for instance %q is registered: %w", inst.Name, err)
	} else if !registered {
		return nil
	}
	if t.enabler != nil {
		if err := t.enabler(ctx, false, inst.Name); err != nil {
			return fmt.Errorf("failed to disable the autostart entry for instance %q: %w", inst.Name, err)
		}
	}
	if err := os.Remove(t.filePath(inst.Name)); err != nil {
		return fmt.Errorf("failed to remove the autostart entry for instance %q: %w", inst.Name, err)
	}
	return nil
}

func (t *TemplateFileBasedManager) renderTemplate(instName, workDir string, getExecutable func() (string, error)) ([]byte, error) {
	if t.template == "" {
		return nil, errors.New("no template available")
	}
	selfExeAbs, err := getExecutable()
	if err != nil {
		return nil, err
	}
	return textutil.ExecuteTemplate(
		t.template,
		map[string]string{
			"Binary":   selfExeAbs,
			"Instance": instName,
			"WorkDir":  workDir,
		})
}

func (t *TemplateFileBasedManager) AutoStartedIdentifier() string {
	if t.autoStartedIdentifier != nil {
		return t.autoStartedIdentifier()
	}
	return ""
}

func (t *TemplateFileBasedManager) RequestStart(ctx context.Context, inst *limatype.Instance) error {
	if t.requestStart == nil {
		return errors.New("no RequestStart function available")
	}
	return t.requestStart(ctx, inst)
}

func (t *TemplateFileBasedManager) RequestStop(ctx context.Context, inst *limatype.Instance) (bool, error) {
	if t.requestStop == nil {
		return false, errors.New("no RequestStop function available")
	}
	return t.requestStop(ctx, inst)
}
