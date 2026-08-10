// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hcs

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/lima-vm/go-qcow2reader/image/vhdx"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func newTestInstance(t *testing.T, cfg *limatype.LimaYAML) *limatype.Instance {
	t.Helper()
	return &limatype.Instance{
		Name:         "hcs",
		Dir:          t.TempDir(),
		SSHLocalPort: 60022,
		Config:       cfg,
	}
}

func TestValidateConfig(t *testing.T) {
	arch := limatype.NewArch(runtime.GOARCH)

	tests := []struct {
		name        string
		cfg         *limatype.LimaYAML
		expectedErr string
	}{
		{
			name:        "nil config is rejected",
			cfg:         nil,
			expectedErr: "configuration is nil",
		},
		{
			name: "minimal config is accepted",
			cfg: &limatype.LimaYAML{
				VMType: new(limatype.HCS),
				Arch:   &arch,
			},
		},
		{
			name: "Linux guest is accepted",
			cfg: &limatype.LimaYAML{
				VMType: new(limatype.HCS),
				Arch:   &arch,
				OS:     new(limatype.LINUX),
			},
		},
		{
			name: "Windows guest is rejected",
			cfg: &limatype.LimaYAML{
				VMType: new(limatype.HCS),
				Arch:   &arch,
				OS:     new(limatype.WINDOWS),
			},
			expectedErr: "currently Windows guest OS is only supported on QEMU",
		},
		{
			name: "emulation of a non-native arch is rejected",
			cfg: &limatype.LimaYAML{
				VMType: new(limatype.HCS),
				Arch:   new(limatype.Arch("unknown")),
			},
			expectedErr: "unsupported arch",
		},
		{
			name: "tpm is rejected",
			cfg: &limatype.LimaYAML{
				VMType: new(limatype.HCS),
				Arch:   &arch,
				TPM:    new(true),
			},
			expectedErr: "field `tpm` is not supported on HCS driver",
		},
		{
			name: "tpm=false is accepted",
			cfg: &limatype.LimaYAML{
				VMType: new(limatype.HCS),
				Arch:   &arch,
				TPM:    new(false),
			},
		},
		{
			name: "unsupported fields are only warned about, not rejected",
			cfg: &limatype.LimaYAML{
				VMType: new(limatype.HCS),
				Arch:   &arch,
				Audio:  limatype.Audio{Device: new("none")},
				Mounts: []limatype.Mount{{Location: "~"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(t.Context(), tt.cfg)
			if tt.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}

func TestKnownYamlProperties(t *testing.T) {
	yamlType := reflect.TypeFor[limatype.LimaYAML]()
	for _, name := range knownYamlProperties {
		t.Run(name, func(t *testing.T) {
			_, ok := yamlType.FieldByName(name)
			assert.Assert(t, ok, "limatype.LimaYAML has no field %#q", name)
		})
	}
}

func TestConfigure(t *testing.T) {
	arch := limatype.NewArch(runtime.GOARCH)

	t.Run("fills in the HCS defaults", func(t *testing.T) {
		inst := newTestInstance(t, &limatype.LimaYAML{Arch: &arch})

		l := New()
		configured, err := l.Configure(t.Context(), inst)
		assert.NilError(t, err)
		assert.Assert(t, configured != nil)

		assert.Equal(t, *inst.Config.VMType, limatype.HCS)
		assert.Equal(t, *inst.Config.MountType, limatype.REVSSHFS)
		assert.Equal(t, l.Instance, inst)
		assert.Equal(t, l.SSHLocalPort, inst.SSHLocalPort)
	})

	t.Run("keeps the values that are already set", func(t *testing.T) {
		inst := newTestInstance(t, &limatype.LimaYAML{
			Arch:      &arch,
			VMType:    new(limatype.HCS),
			MountType: new(limatype.NINEP),
		})

		l := New()
		_, err := l.Configure(t.Context(), inst)
		assert.NilError(t, err)
		assert.Equal(t, *inst.Config.MountType, limatype.NINEP)
	})

	t.Run("propagates a validation error", func(t *testing.T) {
		inst := newTestInstance(t, &limatype.LimaYAML{
			Arch: &arch,
			TPM:  new(true),
		})

		l := New()
		configured, err := l.Configure(t.Context(), inst)
		assert.ErrorContains(t, err, "field `tpm` is not supported")
		assert.Assert(t, configured == nil)
	})
}

func TestValidate(t *testing.T) {
	arch := limatype.NewArch(runtime.GOARCH)

	l := New()
	l.Instance = newTestInstance(t, &limatype.LimaYAML{
		Arch:   &arch,
		VMType: new(limatype.HCS),
	})
	assert.NilError(t, l.Validate(t.Context()))

	l.Instance.Config.TPM = new(true)
	assert.ErrorContains(t, l.Validate(t.Context()), "field `tpm` is not supported")
}

func TestInfo(t *testing.T) {
	l := New()

	info := l.Info(t.Context())
	assert.Equal(t, info.Name, "hcs")
	assert.Equal(t, info.InstanceDir, "")

	assert.Equal(t, info.Features.NoCloudInit, false)
	assert.Equal(t, info.Features.CanRunGUI, false)

	assert.Equal(t, info.Features.DynamicSSHAddress, true)
	assert.Equal(t, info.Features.StaticSSHPort, true)
	assert.DeepEqual(t, info.Features.SupportedImageFormats, []string{string(vhdx.Type)})

	l.Instance = newTestInstance(t, &limatype.LimaYAML{})
	assert.Equal(t, l.Info(t.Context()).InstanceDir, l.Instance.Dir)
}

func TestSSHAddress(t *testing.T) {
	l := New()

	_, err := l.SSHAddress(t.Context())
	assert.ErrorContains(t, err, "not known yet")

	l.sshAddress = "192.168.10.5"
	got, err := l.SSHAddress(t.Context())
	assert.NilError(t, err)
	assert.Equal(t, got, "192.168.10.5")
}

func TestCreateDisk(t *testing.T) {
	l := New()
	l.Instance = newTestInstance(t, &limatype.LimaYAML{})

	diskPath := filepath.Join(l.Instance.Dir, filenames.Disk)
	const content = "not a real disk"
	assert.NilError(t, os.WriteFile(diskPath, []byte(content), 0o600))

	assert.NilError(t, l.CreateDisk(t.Context()))

	st, err := os.Stat(diskPath + ".vhdx")
	assert.NilError(t, err)
	assert.Equal(t, st.Size(), int64(len(content)))

	assert.NilError(t, l.CreateDisk(t.Context()))
}

func TestGuestAgentIsForwardedByHostAgent(t *testing.T) {
	l := New()
	assert.Equal(t, l.ForwardGuestAgent(t.Context()), true)

	conn, addr, err := l.GuestAgentConn(t.Context())
	assert.NilError(t, err)
	assert.Assert(t, conn == nil)
	assert.Equal(t, addr, "")
}

func TestStopBeforeStart(t *testing.T) {
	l := New()
	l.Instance = newTestInstance(t, &limatype.LimaYAML{})

	assert.NilError(t, l.Stop(t.Context()))
}
