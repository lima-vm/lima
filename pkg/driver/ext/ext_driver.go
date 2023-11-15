// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ext

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

type LimaExtDriver struct {
	Instance *limatype.Instance
}

var _ driver.Driver = (*LimaExtDriver)(nil)

func New() *LimaExtDriver {
	return &LimaExtDriver{}
}

func (l *LimaExtDriver) Configure(inst *limatype.Instance) *driver.ConfiguredDriver {
	l.Instance = inst

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaExtDriver) Validate(ctx context.Context) error {
	return validateConfig(ctx, l.Instance.Config)
}

func validateConfig(_ context.Context, cfg *limatype.LimaYAML) error {
	if cfg == nil {
		return errors.New("configuration is nil")
	}
	if err := validateMountType(cfg); err != nil {
		return err
	}

	return nil
}

// Helper method for mount type validation.
func validateMountType(cfg *limatype.LimaYAML) error {
	if cfg.MountTypesUnsupported != nil && cfg.MountType != nil && slices.Contains(cfg.MountTypesUnsupported, *cfg.MountType) {
		return fmt.Errorf("mount type %q is explicitly unsupported", *cfg.MountType)
	}

	return nil
}

func (l *LimaExtDriver) FillConfig(_ context.Context, cfg *limatype.LimaYAML, _ string) error {
	if cfg.VMType == nil {
		cfg.VMType = ptr.Of(limatype.EXT)
	}

	if cfg.MountType == nil || *cfg.MountType == "" || *cfg.MountType == "default" {
		cfg.MountType = ptr.Of(limatype.REVSSHFS)
	}

	mountTypesUnsupported := make(map[string]struct{})
	mountTypesUnsupported[limatype.NINEP] = struct{}{}
	mountTypesUnsupported[limatype.VIRTIOFS] = struct{}{}

	if _, ok := mountTypesUnsupported[*cfg.MountType]; ok {
		return fmt.Errorf("mount type %q is explicitly unsupported", *cfg.MountType)
	}

	return nil
}

func (l *LimaExtDriver) Start(_ context.Context) (chan error, error) {
	errCh := make(chan error)

	return errCh, nil
}

func (l *LimaExtDriver) BootScripts() (map[string][]byte, error) {
	return nil, nil
}

func (l *LimaExtDriver) RunGUI() error {
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "ext", *l.Instance.Config.Video.Display)
}

func (l *LimaExtDriver) Stop(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) GuestAgentConn(_ context.Context) (net.Conn, string, error) {
	return nil, "", nil
}

func (l *LimaExtDriver) Info() driver.Info {
	var info driver.Info
	info.Name = "ext"
	if l.Instance != nil && l.Instance.Dir != "" {
		info.InstanceDir = l.Instance.Dir
	}

	info.Features = driver.DriverFeatures{
		DynamicSSHAddress:    false,
		SkipSocketForwarding: false,
		CanRunGUI:            false,
	}
	return info
}

func (l *LimaExtDriver) SSHAddress(_ context.Context) (string, error) {
	return l.Instance.SSHAddress, nil
}

func (l *LimaExtDriver) InspectStatus(_ context.Context, _ *limatype.Instance) string {
	return ""
}

func (l *LimaExtDriver) Create(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) Delete(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) CreateDisk(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) Register(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) Unregister(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return nil
}

func (l *LimaExtDriver) DisplayConnection(_ context.Context) (string, error) {
	return "", nil
}

func (l *LimaExtDriver) CreateSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaExtDriver) ApplySnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaExtDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaExtDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", errUnimplemented
}

func (l *LimaExtDriver) ForwardGuestAgent() bool {
	// If driver is not providing, use host agent
	return true
}

func (l *LimaExtDriver) AdditionalSetupForSSH(_ context.Context) error {
	return nil
}
