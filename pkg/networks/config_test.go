// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFillDefault(t *testing.T) {
	cfg, err := fillDefaults(Config{})
	assert.NilError(t, err)

	userNet := cfg.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.ParseIP("255.255.255.0"))
	assert.DeepEqual(t, userNet.Gateway, net.ParseIP("192.168.104.1"))
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}

func TestFillDefaultWithV2(t *testing.T) {
	cfg := Config{Networks: map[string]Network{
		"user-v2": {Mode: ModeUserV2},
	}}
	cfg, err := fillDefaults(cfg)
	assert.NilError(t, err)

	userNet := cfg.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.ParseIP("255.255.255.0"))
	assert.DeepEqual(t, userNet.Gateway, net.ParseIP("192.168.104.1"))
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}

func TestFillDefaultWithV2AndGateway(t *testing.T) {
	cfg := Config{Networks: map[string]Network{
		"user-v2": {Mode: ModeUserV2, Gateway: net.ParseIP("192.168.105.1")},
	}}
	cfg, err := fillDefaults(cfg)
	assert.NilError(t, err)

	userNet := cfg.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.IP{})
	assert.DeepEqual(t, userNet.Gateway, net.ParseIP("192.168.105.1"))
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}

func TestValidateExternal(t *testing.T) {
	gw := net.ParseIP("192.168.200.1")
	tests := []struct {
		name    string
		nw      Network
		wantErr string
	}{
		{"user-v2", Network{Mode: ModeUserV2, Gateway: gw, External: true}, ""},
		{"not external", Network{Mode: ModeShared}, ""},
		{"shared", Network{Mode: ModeShared, Gateway: gw, External: true}, "only supported for mode"},
		{"bridged", Network{Mode: ModeBridged, Interface: "en0", External: true}, "only supported for mode"},
		{"no gateway", Network{Mode: ModeUserV2, External: true}, "requires `gateway`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Networks: map[string]Network{"ext": tt.nw}}
			err := cfg.validateExternal()
			if tt.wantErr == "" {
				assert.NilError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestLoadConfigExternal(t *testing.T) {
	loadConfig := func(t *testing.T, networksYAML string) (Config, error) {
		t.Helper()
		limaHome := t.TempDir()
		t.Setenv("LIMA_HOME", limaHome)
		assert.NilError(t, os.MkdirAll(filepath.Join(limaHome, "_config"), 0o755))
		assert.NilError(t, os.WriteFile(filepath.Join(limaHome, "_config", "networks.yaml"), []byte(networksYAML), 0o644))
		// LoadConfig() only loads the file once.
		resetCache := func() {
			cache = struct {
				sync.Once
				cfg Config
				err error
			}{}
		}
		resetCache()
		t.Cleanup(resetCache)
		return LoadConfig()
	}

	t.Run("valid", func(t *testing.T) {
		cfg, err := loadConfig(t, `networks:
  ext:
    mode: user-v2
    external: true
    gateway: 192.168.200.1
    netmask: 255.255.255.0
`)
		assert.NilError(t, err)
		external, err := cfg.External("ext")
		assert.NilError(t, err)
		assert.Assert(t, external)
		_, err = cfg.External("missing")
		assert.ErrorContains(t, err, "not defined")
	})

	t.Run("no gateway", func(t *testing.T) {
		// fillDefaults would overwrite this network with the default one, which is not external.
		_, err := loadConfig(t, `networks:
  user-v2:
    mode: user-v2
    external: true
`)
		assert.ErrorContains(t, err, "requires `gateway`")
	})
}
