// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/networks"
)

func TestCheckExternalUsernetDaemon(t *testing.T) {
	home := t.TempDir()
	limaDir := filepath.Join(home, ".lima")
	assert.NilError(t, os.MkdirAll(filepath.Join(limaDir, "_config"), 0o755))
	project := filepath.Join(home, "src")
	assert.NilError(t, os.Mkdir(project, 0o755))
	link := filepath.Join(t.TempDir(), "home-link")
	assert.NilError(t, os.Symlink(home, link))
	// On Windows, "/" is the root of the current drive, which may not hold home.
	root := filepath.VolumeName(home) + string(filepath.Separator)

	nwCfg := networks.Config{Networks: map[string]networks.Network{
		"builtin": {Mode: networks.ModeUserV2},
		"vpn":     {Mode: networks.ModeUserV2, Daemon: "/usr/local/libexec/vpn-usernet"},
	}}

	cases := []struct {
		name    string
		y       limatype.LimaYAML
		wantErr []string
	}{
		{
			name: "built-in daemon allows anything",
			y: limatype.LimaYAML{
				Networks:     []limatype.Network{{Lima: "builtin"}, {VZNAT: new(true)}},
				Mounts:       []limatype.Mount{{Location: home}},
				PortForwards: []limatype.PortForward{{Reverse: true}},
			},
		},
		{
			name: "external daemon alone with an unrelated mount",
			y: limatype.LimaYAML{
				Networks: []limatype.Network{{Lima: "vpn"}},
				Mounts:   []limatype.Mount{{Location: project}},
			},
		},
		{
			name:    "second interface",
			y:       limatype.LimaYAML{Networks: []limatype.Network{{Lima: "vpn"}, {VZNAT: new(true)}}},
			wantErr: []string{"field `networks[0].lima`", "must be the only network"},
		},
		{
			name: "reverse port forward",
			y: limatype.LimaYAML{
				Networks:     []limatype.Network{{Lima: "vpn"}},
				PortForwards: []limatype.PortForward{{}, {Reverse: true}},
			},
			wantErr: []string{"field `portForwards[1].reverse` must be false"},
		},
		{
			name: "mount of a parent of LIMA_HOME",
			y: limatype.LimaYAML{
				Networks: []limatype.Network{{Lima: "vpn"}},
				Mounts:   []limatype.Mount{{Location: project}, {Location: home}},
			},
			wantErr: []string{"field `mounts[1].location`", "exposes LIMA_HOME"},
		},
		{
			name: "mount of a parent of LIMA_HOME through a symlink",
			y: limatype.LimaYAML{
				Networks: []limatype.Network{{Lima: "vpn"}},
				Mounts:   []limatype.Mount{{Location: link}},
			},
			wantErr: []string{"field `mounts[0].location`", "exposes LIMA_HOME"},
		},
		{
			name: "mount inside LIMA_HOME",
			y: limatype.LimaYAML{
				Networks: []limatype.Network{{Lima: "vpn"}},
				Mounts:   []limatype.Mount{{Location: filepath.Join(limaDir, "_config")}},
			},
			wantErr: []string{"field `mounts[0].location`", "exposes LIMA_HOME"},
		},
		{
			name: "mount of the filesystem root",
			y: limatype.LimaYAML{
				Networks: []limatype.Network{{Lima: "vpn"}},
				Mounts:   []limatype.Mount{{Location: root}},
			},
			wantErr: []string{"field `mounts[0].location`", "exposes LIMA_HOME"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := filepath.EvalSymlinks(limaDir)
			assert.NilError(t, err)
			err = checkExternalUsernetDaemon(&tc.y, nwCfg, resolved)
			if len(tc.wantErr) == 0 {
				assert.NilError(t, err)
				return
			}
			for _, want := range tc.wantErr {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

func TestIsWithin(t *testing.T) {
	assert.Assert(t, isWithin("/a", "/a"))
	assert.Assert(t, isWithin("/a", "/a/b"))
	assert.Assert(t, isWithin("/", "/a"))
	assert.Assert(t, !isWithin("/a/b", "/a"))
	assert.Assert(t, !isWithin("/a", "/ab"))
	assert.Assert(t, !isWithin("/a", "/a/../b"))
}
