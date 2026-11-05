// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/copytool"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/uiutil"
)

// TestPathDepth covers both host path forms, because `limactl shell --sync`
// compares the depth against rsyncMinimumSrcDirDepth to refuse a directory too
// close to the root. A Windows path must count the same as its Unix equivalent,
// or the same threshold means two different things. The windows column is what
// the call site passes for runtime.GOOS, so both platforms are covered wherever
// this runs.
func TestPathDepth(t *testing.T) {
	tests := []struct {
		path     string
		windows  bool
		expected int
	}{
		// A home directory sits at 3 on either platform, below the minimum of 4.
		{"/Users/jan", false, 3},
		{`C:\Users\jan`, true, 3},
		{"/Users/jan/proj", false, 4},
		{`C:\Users\jan\proj`, true, 4},
		{"/", false, 2},
		{`C:\`, true, 2},
		// filepath.Abs normalises this form away before Lima ever sees it.
		// Counting it anyway keeps the helper independent of the host separator.
		{"C:/Users/jan/proj", true, 4},
		// On Unix a backslash is an ordinary filename character. Counting it as
		// a separator would score this single directory just below the root at
		// 4 and pass the guard.
		{`/x\y\z`, false, 2},
		{`/foo\..\..\..\bar`, false, 2},
		// The guard must refuse a share root and allow a directory inside it;
		// counting both leading separators would pass the root.
		{`\\server\share`, true, 3},
		{`\\server\share\proj`, true, 4},
		// Go's Clean keeps a separator run inside a UNC volume, and whether
		// Windows collapses it earlier is unverified, so the guard collapses
		// the run itself.
		{`\\server\\share`, true, 3},
		{`\\\server\share`, true, 3},
		// An extended-length or device prefix spells a path the plain form
		// also spells.
		{`\\?\C:\`, true, 2},
		{`\\?\C:\Users\jan\proj`, true, 4},
		{`\\.\C:\`, true, 2},
		{`\\?\UNC\server\share`, true, 3},
		// filepath.Clean keeps a root's trailing separator and strips one below
		// a root, so only the first form reaches Lima. Counting both alike
		// keeps the trim from changing an ordinary path's depth.
		{`\\server\share\`, true, 3},
		{`C:\Users\jan\proj\`, true, 4},
		// An empty path returns 1, so the guard refuses one if it ever reaches
		// here.
		{"", false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, pathDepth(tt.path, tt.windows), tt.expected)
		})
	}
}

func TestBuildSyncExcludeArgs(t *testing.T) {
	// rsyncDir stands in for the converted form a Windows host passes to rsync,
	// so the ignore file must be found through the native dir yet named by it.
	const rsyncDir = "/cygdrive/c/Users/jan/proj"
	tests := []struct {
		name      string
		excludes  []string
		hasIgnore bool
		expected  []string
	}{
		{
			name:     "empty excludes, no ignore file",
			excludes: nil,
			expected: nil,
		},
		{
			name:     "single exclude",
			excludes: []string{"node_modules"},
			expected: []string{"--exclude", "node_modules"},
		},
		{
			name:     "multiple excludes",
			excludes: []string{"node_modules", ".git", "vendor"},
			expected: []string{"--exclude", "node_modules", "--exclude", ".git", "--exclude", "vendor"},
		},
		{
			name:      "ignore file only",
			excludes:  nil,
			hasIgnore: true,
			expected:  []string{"--exclude-from", rsyncDir + "/.limasyncignore"},
		},
		{
			name:      "excludes and ignore file",
			excludes:  []string{".git"},
			hasIgnore: true,
			expected:  []string{"--exclude", ".git", "--exclude-from", rsyncDir + "/.limasyncignore"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.hasIgnore {
				err := os.WriteFile(filepath.Join(dir, ".limasyncignore"), []byte("build\ndist\n"), 0o644)
				assert.NilError(t, err)
			}
			got, err := buildSyncExcludeArgs(tt.excludes, dir, rsyncDir)
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.expected)
		})
	}
}

// TestBuildSyncExcludeArgsRefusesSymlink checks that a symlinked .limasyncignore
// is refused, because the guest could rewrite the file it leads to.
func TestBuildSyncExcludeArgsRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644))
	assert.NilError(t, os.Symlink(".gitignore", filepath.Join(dir, ".limasyncignore")))
	_, err := buildSyncExcludeArgs(nil, dir, dir)
	assert.ErrorContains(t, err, "must be a regular file")
}

func TestParseRsyncStats(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected *rsyncStats
	}{
		{
			name: "mixed output",
			output: `
>f+++++++++ new-file.txt
cd+++++++++ dir/
cL+++++++++ new-symlink -> target
*deleting deleted.txt
`,
			expected: &rsyncStats{
				Added:    3,
				Deleted:  1,
				Modified: 0,
			},
		},
		{
			name: "metadata-only changes",
			output: `
.d..t...... ./
.f...p..... existing.txt
`,
			expected: &rsyncStats{
				Added:    0,
				Deleted:  0,
				Modified: 0,
				Metadata: 2,
			},
		},
		{
			name: "many changes",
			output: `
<f+++++++++ file1
<f+++++++++ file2
*deleting file3
*deleting file4
*deleting file5
<f..T...... file6
`,
			expected: &rsyncStats{
				Added:    2,
				Deleted:  3,
				Modified: 1,
			},
		},
		{
			name: "openrsync one-character names",
			output: `
>f.s..... a
>f+++++++ b
*deleting c
.d..t.... ./
`,
			expected: &rsyncStats{
				Added:    1,
				Deleted:  1,
				Modified: 1,
				Metadata: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRsyncStats(tt.output)
			assert.DeepEqual(t, got, tt.expected)
		})
	}
}

// TestHostTreeForDiff checks that the "View" diff compares the guest copy with a
// host tree that lacks the excluded files too.
func TestHostTreeForDiff(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not found:", err)
	}
	host := filepath.Join(t.TempDir(), "proj")
	assert.NilError(t, os.MkdirAll(host, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(host, "foo.txt"), []byte("host foo\n"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(host, "bar.txt"), []byte("host bar\n"), 0o644))

	excludeArgs, err := buildSyncExcludeArgs([]string{"foo.txt"}, host, host)
	assert.NilError(t, err)
	rsync, err := copytool.New(t.Context(), string(copytool.BackendRsync), []string{host + "/", t.TempDir()}, &copytool.Options{
		AdditionalArgs: append([]string{"--delete"}, excludeArgs...),
	})
	assert.NilError(t, err)

	got, err := hostTreeForDiff(t.Context(), &cobra.Command{}, rsync, host, t.TempDir(), excludeArgs)
	assert.NilError(t, err)
	_, err = os.Stat(filepath.Join(got, "foo.txt"))
	assert.Assert(t, errors.Is(err, fs.ErrNotExist), "excluded file in the diffed host tree: %v", err)
	_, err = os.Stat(filepath.Join(got, "bar.txt"))
	assert.NilError(t, err)

	// Without user filters the diff reads the host directory in place.
	excludeArgs, err = buildSyncExcludeArgs(nil, host, host)
	assert.NilError(t, err)
	got, err = hostTreeForDiff(t.Context(), &cobra.Command{}, rsync, host, t.TempDir(), excludeArgs)
	assert.NilError(t, err)
	assert.Equal(t, got, host)
}

func TestValidateSyncFlagValue(t *testing.T) {
	flags := newShellCommand().Flags()
	for _, val := range []string{"--sync-exclude=.git", "--sync-exclude", "--workdir"} {
		assert.ErrorContains(t, validateSyncFlagValue(flags, val), "requires a directory argument", val)
	}
	// Directories, including ones whose names merely look like flags, are accepted.
	for _, val := range []string{".", "/tmp/proj", "./--sync-exclude", "--data", "-"} {
		assert.NilError(t, validateSyncFlagValue(flags, val), val)
	}
}

func TestRsyncStatsString(t *testing.T) {
	tests := []struct {
		name     string
		stats    *rsyncStats
		expected string
	}{
		{
			name:     "empty",
			stats:    &rsyncStats{},
			expected: "",
		},
		{
			name: "all stats",
			stats: &rsyncStats{
				Added:    1,
				Deleted:  2,
				Modified: 3,
				Metadata: 4,
			},
			expected: "added: 1, deleted: 2, modified: 3, metadata: 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.stats.String(), tt.expected)
		})
	}
}

func TestWindowsShellCommandFlag(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"cmd.exe", "/c"},
		{"CMD.EXE", "/c"},
		{`C:\Windows\System32\cmd.exe`, "/c"},
		{`C:/Windows/System32/cmd.exe`, "/c"},
		{"powershell.exe", "-Command"},
		{"pwsh.exe", "-Command"},
		{`C:\Program Files\PowerShell\7\pwsh.exe`, "-Command"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			assert.Equal(t, tt.want, windowsShellCommandFlag(tt.shell))
		})
	}
}

func TestWindowsQuoteShell(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"cmd.exe", "cmd.exe"},
		{`C:\Windows\System32\cmd.exe`, `C:\Windows\System32\cmd.exe`},
		{`C:\Program Files\PowerShell\7\pwsh.exe`, `"C:\Program Files\PowerShell\7\pwsh.exe"`},
		{`"C:\already\quoted.exe"`, `"C:\already\quoted.exe"`},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			assert.Equal(t, tt.want, windowsQuoteShell(tt.shell))
		})
	}
}

// TestGetRsyncStatsMatchesSyncBack checks that the dry run behind the sync-back
// prompt counts what the sync-back copies, for patterns that rsync anchors at the
// top of the transfer.
func TestGetRsyncStatsMatchesSyncBack(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not found:", err)
	}
	// The guest copy changed bar.txt and lacks foo.txt.
	tests := []struct {
		exclude  string
		expected rsyncStats
	}{
		{"foo.txt", rsyncStats{Modified: 1}},
		{"/foo.txt", rsyncStats{Modified: 1}},
		{"proj", rsyncStats{Deleted: 1, Modified: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.exclude, func(t *testing.T) {
			base := t.TempDir()
			host := filepath.Join(base, "host", "proj")
			guest := filepath.Join(base, "guest", "proj")
			assert.NilError(t, os.MkdirAll(host, 0o755))
			assert.NilError(t, os.MkdirAll(guest, 0o755))
			assert.NilError(t, os.WriteFile(filepath.Join(host, "foo.txt"), []byte("host foo\n"), 0o644))
			assert.NilError(t, os.WriteFile(filepath.Join(host, "bar.txt"), []byte("host bar\n"), 0o644))
			assert.NilError(t, os.WriteFile(filepath.Join(guest, "bar.txt"), []byte("modified bar\n"), 0o644))
			excludeArgs, err := buildSyncExcludeArgs([]string{tt.exclude}, host, host)
			assert.NilError(t, err)
			_, got, err := getRsyncStats(t.Context(), []string{guest + "/", host}, excludeArgs)
			assert.NilError(t, err)
			got.Metadata = 0 // the directory timestamps depend on when the test wrote them
			assert.DeepEqual(t, *got, tt.expected)
		})
	}
}

// TestSyncBackSkipsIgnoreFile checks that the guest can neither create nor
// rewrite .limasyncignore through the sync-back, including under a name that a
// case-insensitive host folds onto it.
func TestSyncBackSkipsIgnoreFile(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not found:", err)
	}
	tests := []struct {
		name       string
		guestFile  string
		hostFilter string // the host's .limasyncignore, if any
	}{
		{"guest rewrites the host's file", ".limasyncignore", "*.log\n"},
		{"guest creates the file", ".limasyncignore", ""},
		{"guest writes an upper-case alias", ".LIMASYNCIGNORE", "*.log\n"},
		{"guest writes a long-s alias", ".lima\u017fyncignore", "*.log\n"},
		{"host's file includes dotfiles", ".limasyncignore", "+ .*\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, guest := t.TempDir(), t.TempDir()
			if tt.hostFilter != "" {
				assert.NilError(t, os.WriteFile(filepath.Join(host, ".limasyncignore"), []byte(tt.hostFilter), 0o644))
			}
			assert.NilError(t, os.WriteFile(filepath.Join(guest, tt.guestFile), []byte("!\n"), 0o644))
			excludeArgs, err := buildSyncExcludeArgs(nil, host, host)
			assert.NilError(t, err)
			_, got, err := getRsyncStats(t.Context(), []string{guest + "/", host}, excludeArgs)
			assert.NilError(t, err)
			got.Metadata = 0 // the directory timestamps depend on when the test wrote them
			assert.DeepEqual(t, *got, rsyncStats{})
		})
	}
}

func TestMountedGuestPath(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	assert.NilError(t, os.MkdirAll(filepath.Join(a, "sub"), 0o755))
	assert.NilError(t, os.MkdirAll(filepath.Join(root, "ab"), 0o755))
	assert.NilError(t, os.Symlink(a, filepath.Join(root, "link")))
	resolvedSub, err := filepath.EvalSymlinks(filepath.Join(a, "sub"))
	assert.NilError(t, err)
	tests := []struct {
		name       string
		location   string
		mountPoint string
		hostDir    string // relative to root
		want       string // "" when no mount contains hostDir
	}{
		{"same mount point", a, filepath.ToSlash(a), "a/sub", filepath.ToSlash(a) + "/sub"},
		{"remapped mount point", a, "/home/user.linux/a", "a", "/home/user.linux/a"},
		{"nested dir", a, "/home/user.linux/a", "a/sub", "/home/user.linux/a/sub"},
		{"root mount", "/", "/mnt/host", "a/sub", path.Join("/mnt/host", resolvedSub)},
		{"symlinked host dir", a, "/home/user.linux/a", "link/sub", "/home/user.linux/a/sub"},
		{"not under any mount", a, "/home/user.linux/a", ".", ""},
		{"sibling with the same prefix", a, "/home/user.linux/a", "ab", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.location == "/" && runtime.GOOS == "windows" {
				t.Skip(`on Windows "/" is the root of the current drive, which may not hold the temp dir`)
			}
			mounts := []limatype.Mount{{Location: tt.location, MountPoint: new(tt.mountPoint)}}
			assert.Equal(t, tt.want, mountedGuestPath(mounts, filepath.Join(root, tt.hostDir)))
		})
	}
}

// localRsyncTool serves "inst:DIR" from DIR on the host instead of from a guest,
// and records the options of every read of the guest.
type localRsyncTool struct {
	guestReads [][]string
}

func (*localRsyncTool) Name() string { return "rsync" }

func (*localRsyncTool) IsAvailableOnGuest(context.Context, []string) bool { return true }

func (l *localRsyncTool) Command(ctx context.Context, paths []string, opts *copytool.Options) (*exec.Cmd, error) {
	extra := syncArgs(nil)
	if opts != nil {
		extra = opts.AdditionalArgs
	}
	args := slices.Concat([]string{"-a", "-q"}, extra, []string{"--"})
	for _, p := range paths {
		if guestPath, ok := strings.CutPrefix(p, "inst:"); ok {
			l.guestReads = append(l.guestReads, extra)
			p = guestPath
		}
		args = append(args, p)
	}
	return exec.CommandContext(ctx, "rsync", args...), nil
}

// newSyncDirs returns a host and a guest directory named alike, as for `limactl shell --sync`.
func newSyncDirs(t *testing.T) (hostDir, guestDir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("localRsyncTool passes native paths to rsync")
	}
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not found:", err)
	}
	hostDir = filepath.Join(t.TempDir(), "project")
	guestDir = filepath.Join(t.TempDir(), "project")
	assert.NilError(t, os.Mkdir(hostDir, 0o755))
	assert.NilError(t, os.Mkdir(guestDir, 0o755))
	t.Cleanup(func() { selectPrompt = uiutil.Select })
	return hostDir, guestDir
}

func askUserForRsyncBackLocally(t *testing.T, hostDir, guestDir string, rsync *localRsyncTool) error {
	t.Helper()
	inst := &limatype.Instance{
		Name:   "inst",
		Config: &limatype.LimaYAML{User: limatype.User{Home: new("/home/user.linux")}},
	}
	// Stands in for ssh, which only removes the guest workdir afterwards.
	sshCmd := exec.CommandContext(t.Context(), "true")
	return askUserForRsyncBack(t.Context(), &cobra.Command{}, inst, sshCmd, hostDir, hostDir, guestDir, rsync, true, nil)
}

// TestAskUserForRsyncBackAppliesReviewedContents checks that "Yes" applies the
// guest directory as it was reviewed, not as a guest process left it afterwards.
func TestAskUserForRsyncBackAppliesReviewedContents(t *testing.T) {
	hostDir, guestDir := newSyncDirs(t)
	assert.NilError(t, os.WriteFile(filepath.Join(hostDir, "build.sh"), []byte("echo building\n"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(hostDir, "keep.txt"), []byte("keep\n"), 0o644))
	reviewed := "echo building\n# comment added by the agent\n"
	assert.NilError(t, os.WriteFile(filepath.Join(guestDir, "build.sh"), []byte(reviewed), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(guestDir, "keep.txt"), []byte("keep\n"), 0o644))
	// Same content and mtime as on the host, so only the mode tells the copy-dest basis apart.
	assert.NilError(t, os.Chmod(filepath.Join(guestDir, "keep.txt"), 0o755))
	hostKeep, err := os.Stat(filepath.Join(hostDir, "keep.txt"))
	assert.NilError(t, err)
	assert.NilError(t, os.Chtimes(filepath.Join(guestDir, "keep.txt"), hostKeep.ModTime(), hostKeep.ModTime()))

	var prompt string
	selectPrompt = func(message string, _ []string) (int, error) {
		prompt = message
		assert.NilError(t, os.WriteFile(filepath.Join(guestDir, "build.sh"), []byte(reviewed+"echo unreviewed\n"), 0o644))
		assert.NilError(t, os.MkdirAll(filepath.Join(guestDir, ".git", "hooks"), 0o755))
		assert.NilError(t, os.WriteFile(filepath.Join(guestDir, ".git", "hooks", "pre-commit"), []byte("#!/bin/sh\n"), 0o755))
		assert.NilError(t, os.Remove(filepath.Join(guestDir, "keep.txt")))
		return 0, nil // Yes
	}
	rsync := &localRsyncTool{}
	assert.NilError(t, askUserForRsyncBackLocally(t, hostDir, guestDir, rsync))

	got, err := os.ReadFile(filepath.Join(hostDir, "build.sh"))
	assert.NilError(t, err)
	assert.Equal(t, string(got), reviewed)
	_, err = os.Stat(filepath.Join(hostDir, ".git"))
	assert.Assert(t, errors.Is(err, fs.ErrNotExist), "unreviewed file reached the host: %v", err)
	keep, err := os.Stat(filepath.Join(hostDir, "keep.txt"))
	assert.NilError(t, err)
	assert.Equal(t, keep.Mode().Perm(), fs.FileMode(0o755))
	// One read of the guest, which takes unchanged files from the host.
	assert.Equal(t, len(rsync.guestReads), 1)
	assert.Check(t, slices.Contains(rsync.guestReads[0], "--copy-dest="+hostDir), rsync.guestReads[0])
	assert.Check(t, strings.Contains(prompt, "modified: 1"), prompt)
}

// TestAskUserForRsyncBackRemovesSnapshot checks that the snapshot is taken beside
// the project and removed afterwards, even with read-only directories from the guest.
func TestAskUserForRsyncBackRemovesSnapshot(t *testing.T) {
	hostDir, guestDir := newSyncDirs(t)
	readOnly := filepath.Join(guestDir, "cache")
	assert.NilError(t, os.Mkdir(readOnly, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(readOnly, "mod.go"), []byte("package mod\n"), 0o444))
	assert.NilError(t, os.Chmod(readOnly, 0o555))
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })

	listParent := func() []string {
		entries, err := os.ReadDir(filepath.Dir(hostDir))
		assert.NilError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return names
	}
	var duringPrompt []string
	selectPrompt = func(string, []string) (int, error) {
		duringPrompt = listParent()
		return 1, nil // No
	}
	assert.NilError(t, askUserForRsyncBackLocally(t, hostDir, guestDir, &localRsyncTool{}))

	assert.Equal(t, len(duringPrompt), 2, duringPrompt)
	assert.Assert(t, strings.HasPrefix(duringPrompt[0], ".lima-guest-synced-"), duringPrompt)
	assert.DeepEqual(t, listParent(), []string{"project"})
}
