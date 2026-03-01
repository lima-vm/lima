// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestTranslateHostPath(t *testing.T) {
	mountPoint1 := "/mnt/home-user"
	mountPoint2 := "/mnt/tmp"

	tests := []struct {
		name          string
		hostPath      string
		toolSet       ToolSet
		wantGuestPath string
		wantWarnings  bool
		wantErr       bool
	}{
		{
			name:     "file in mounted directory",
			hostPath: "/home/user/documents/file.txt",
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: "/home/user", MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "/mnt/home-user/documents/file.txt",
			wantWarnings:  false,
			wantErr:       false,
		},
		{
			name:     "path outside mount - fallback to guest path",
			hostPath: "/other/path/file.txt",
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: "/home/user", MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "/other/path/file.txt",
			wantWarnings:  true,
			wantErr:       false,
		},
		{
			name:     "similar prefix but not under mount",
			hostPath: "/home/user2/file.txt",
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: "/home/user", MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "/home/user2/file.txt",
			wantWarnings:  true,
			wantErr:       false,
		},
		{
			name:     "multiple mounts",
			hostPath: "/tmp/myfile",
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: "/home/user", MountPoint: &mountPoint1},
							{Location: "/tmp", MountPoint: &mountPoint2},
						},
					},
				},
			},
			wantGuestPath: "/mnt/tmp/myfile",
			wantWarnings:  false,
			wantErr:       false,
		},
		{
			name:     "relative path should error",
			hostPath: "relative/path",
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: "/home/user", MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "",
			wantWarnings:  false,
			wantErr:       true,
		},
		{
			name:     "empty path should error",
			hostPath: "",
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: "/home/user", MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "",
			wantWarnings:  false,
			wantErr:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, logs, err := test.toolSet.TranslateHostPath(test.hostPath)
			if test.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, test.wantGuestPath, got)
				if test.wantWarnings {
					assert.Assert(t, logs != "")
				} else {
					assert.Equal(t, "", logs)
				}
			}
		})
	}
}
