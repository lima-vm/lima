// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func absPath(p string) string {
	if runtime.GOOS == "windows" {
		return "C:" + filepath.FromSlash(p)
	}
	return filepath.FromSlash(p)
}

func TestTranslateHostPath(t *testing.T) {
	mountPointParent := "/mnt/parent"
	mountPointChild := "/mnt/child"
	mountPoint1 := "/mnt/home-user"
	mountPoint2 := "/mnt/tmp"

	type translateHostPathTest struct {
		name          string
		hostPath      string
		toolSet       ToolSet
		wantGuestPath string
		wantErr       bool
	}

	tests := []translateHostPathTest{
		{
			name:     "child mount takes precedence",
			hostPath: absPath("/parent/child/file.txt"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/parent"), MountPoint: &mountPointParent},
							{Location: absPath("/parent/child"), MountPoint: &mountPointChild},
						},
					},
				},
			},
			wantGuestPath: "/mnt/child/file.txt",
			wantErr:       false,
		},
		{
			name:     "parent mount used for parent path",
			hostPath: absPath("/parent/file.txt"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/parent"), MountPoint: &mountPointParent},
							{Location: absPath("/parent/child"), MountPoint: &mountPointChild},
						},
					},
				},
			},
			wantGuestPath: "/mnt/parent/file.txt",
			wantErr:       false,
		},
		{
			name:     "child mount listed first",
			hostPath: absPath("/parent/child/file.txt"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/parent/child"), MountPoint: &mountPointChild},
							{Location: absPath("/parent"), MountPoint: &mountPointParent},
						},
					},
				},
			},
			wantGuestPath: "/mnt/child/file.txt",
			wantErr:       false,
		},
		{
			name:     "no mounts configured returns error",
			hostPath: absPath("/foo/bar.txt"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{},
					},
				},
			},
			wantGuestPath: "",
			wantErr:       true,
		},
		{
			name:     "mount with nil MountPoint returns error",
			hostPath: absPath("/foo/bar.txt"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/foo"), MountPoint: nil},
						},
					},
				},
			},
			wantGuestPath: "",
			wantErr:       true,
		},
		{
			name:     "path with trailing slash matches mount",
			hostPath: absPath("/home/user//"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/home/user"), MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "/mnt/home-user",
			wantErr:       false,
		},
		{
			name:     "file inside mounted directory",
			hostPath: absPath("/home/user/documents/file.txt"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/home/user"), MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "/mnt/home-user/documents/file.txt",
			wantErr:       false,
		},
		{
			name:     "exact mount path match",
			hostPath: absPath("/home/user"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/home/user"), MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "/mnt/home-user",
			wantErr:       false,
		},
		{
			name:     "path outside mount returns error",
			hostPath: absPath("/other/path/file.txt"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/home/user"), MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "",
			wantErr:       true,
		},
		{
			name:     "similar prefix but not under mount returns error",
			hostPath: absPath("/home/user2/file.txt"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/home/user"), MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "",
			wantErr:       true,
		},
		{
			name:     "multiple mounts, correct one selected",
			hostPath: absPath("/tmp/myfile"),
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/home/user"), MountPoint: &mountPoint1},
							{Location: absPath("/tmp"), MountPoint: &mountPoint2},
						},
					},
				},
			},
			wantGuestPath: "/mnt/tmp/myfile",
			wantErr:       false,
		},
		{
			name:     "relative path returns error",
			hostPath: "relative/path",
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/home/user"), MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "",
			wantErr:       true,
		},
		{
			name:     "empty path returns error",
			hostPath: "",
			toolSet: ToolSet{
				inst: &limatype.Instance{
					Config: &limatype.LimaYAML{
						Mounts: []limatype.Mount{
							{Location: absPath("/home/user"), MountPoint: &mountPoint1},
						},
					},
				},
			},
			wantGuestPath: "",
			wantErr:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.toolSet.TranslateHostPath(test.hostPath)
			if test.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, test.wantGuestPath, got)
			}
		})
	}
}

func createSymlinkOrSkip(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}
}

func TestTranslateHostPathWithSymlinkedMountLocation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is skipped on windows")
	}

	root := t.TempDir()
	realMount := filepath.Join(root, "real-mount")
	assert.NilError(t, os.MkdirAll(realMount, 0o755))

	symlinkMount := filepath.Join(root, "symlink-mount")
	createSymlinkOrSkip(t, realMount, symlinkMount)

	mountPoint := "/mnt/data"
	ts := ToolSet{
		inst: &limatype.Instance{
			Config: &limatype.LimaYAML{
				Mounts: []limatype.Mount{{Location: symlinkMount, MountPoint: &mountPoint}},
			},
		},
	}

	// non-existing tail simulates write_file target path.
	hostPath := filepath.Join(realMount, "nested", "file.txt")
	guestPath, err := ts.TranslateHostPath(hostPath)
	assert.NilError(t, err)
	assert.Equal(t, "/mnt/data/nested/file.txt", guestPath)
}

func TestTranslateHostPathWithSymlinkedHostPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is skipped on windows")
	}

	root := t.TempDir()
	realMount := filepath.Join(root, "real-mount")
	assert.NilError(t, os.MkdirAll(realMount, 0o755))

	symlinkMount := filepath.Join(root, "symlink-mount")
	createSymlinkOrSkip(t, realMount, symlinkMount)

	mountPoint := "/mnt/data"
	ts := ToolSet{
		inst: &limatype.Instance{
			Config: &limatype.LimaYAML{
				Mounts: []limatype.Mount{{Location: realMount, MountPoint: &mountPoint}},
			},
		},
	}

	hostPath := filepath.Join(symlinkMount, "nested", "file.txt")
	guestPath, err := ts.TranslateHostPath(hostPath)
	assert.NilError(t, err)
	assert.Equal(t, "/mnt/data/nested/file.txt", guestPath)
}
