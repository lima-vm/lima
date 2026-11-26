// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func TestFindSnapshotTagByID(t *testing.T) {
	snapshots := []driver.Snapshot{
		{ID: "1", Tag: "initial"},
		{ID: "2", Tag: "duplicate"},
		{ID: "3", Tag: "duplicate"},
	}

	tag, err := findSnapshotTagByID(snapshots, "1")
	assert.NilError(t, err)
	assert.Equal(t, tag, "initial")

	_, err = findSnapshotTagByID(snapshots, "2")
	assert.ErrorContains(t, err, `snapshot ID "2" has non-unique tag "duplicate"`)

	_, err = findSnapshotTagByID(snapshots, "missing")
	assert.ErrorContains(t, err, `snapshot ID "missing" not found`)
}

func TestHmpErrorMessage(t *testing.T) {
	testCases := map[string]string{
		"warning: something\r\n":            "",
		"the word Error: inside a line\r\n": "",
		"Error: Migration is disabled when VirtFS export path '/home/user' is mounted\r\n":      "Migration is disabled when VirtFS export path '/home/user' is mounted",
		"warning: something\r\nError: Snapshot 'foo' does not exist in one or more devices\r\n": "Snapshot 'foo' does not exist in one or more devices",
	}
	for output, expected := range testCases {
		assert.Equal(t, hmpErrorMessage(output), expected)
	}
}

func TestArgValue(t *testing.T) {
	type testCase struct {
		key           string
		expectedValue string
		expectedOK    bool
	}
	args := []string{"-cpu", "foo", "-no-reboot", "-m", "2G", "-s"}
	testCases := []testCase{
		{
			key:           "-cpu",
			expectedValue: "foo",
			expectedOK:    true,
		},
		{
			key:           "-no-reboot",
			expectedValue: "",
			expectedOK:    true,
		},
		{
			key:           "-m",
			expectedValue: "2G",
			expectedOK:    true,
		},
		{
			key:           "-machine",
			expectedValue: "",
			expectedOK:    false,
		},
		{
			key:           "-s",
			expectedValue: "",
			expectedOK:    true,
		},
	}

	for _, tc := range testCases {
		v, ok := argValue(args, tc.key)
		assert.Equal(t, tc.expectedValue, v)
		assert.Equal(t, tc.expectedOK, ok)
	}
}

func TestParseQemuVersion(t *testing.T) {
	type testCase struct {
		versionOutput string
		expectedValue string
		expectedError string
	}
	testCases := []testCase{
		{
			// old one line version
			versionOutput: "QEMU emulator version 1.5.3 (qemu-kvm-1.5.3-175.el7_9.6), " +
				"Copyright (c) 2003-2008 Fabrice Bellard\n",
			expectedValue: "1.5.3",
			expectedError: "",
		},
		{
			// new two line version
			versionOutput: "QEMU emulator version 8.0.0 (v8.0.0)\n" +
				"Copyright (c) 2003-2022 Fabrice Bellard and the QEMU Project developers\n",
			expectedValue: "8.0.0",
			expectedError: "",
		},
		{
			versionOutput: "foobar",
			expectedValue: "0.0.0",
			expectedError: "failed to parse",
		},
	}

	for _, tc := range testCases {
		v, err := parseQemuVersion(tc.versionOutput)
		if tc.expectedError == "" {
			assert.NilError(t, err)
		} else {
			assert.ErrorContains(t, err, tc.expectedError)
		}
		assert.Equal(t, tc.expectedValue, v.String())
	}
}

func TestExeRelative(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake QEMU has no .exe suffix")
	}
	cwd, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)
	t.Chdir(cwd)
	assert.NilError(t, os.WriteFile("qemu", nil, 0o755))
	t.Setenv("QEMU_SYSTEM_X86_64", "./qemu")

	exe, _, err := Exe(limatype.X8664)
	assert.NilError(t, err)
	assert.Equal(t, exe, filepath.Join(cwd, "qemu"))
}

func TestSwtpmCmdline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("swtpm unix socket mode is not supported on Windows host")
	}

	tmpDir := t.TempDir()

	// Create a mock swtpm binary.
	binDir := filepath.Join(tmpDir, "bin")
	err := os.MkdirAll(binDir, 0o755)
	assert.NilError(t, err)
	swtpmPath := filepath.Join(binDir, "swtpm")
	err = os.WriteFile(swtpmPath, []byte{}, 0o755)
	assert.NilError(t, err)

	// Overwrite PATH so that the function find the mock binary.
	t.Setenv("PATH", binDir)

	// Setup configs and expected value
	cfg := Config{
		Name:        "tpm-test",
		InstanceDir: tmpDir,
		LimaYAML:    &limatype.LimaYAML{},
	}

	stateDir := filepath.Join(tmpDir, filenames.SwtpmDir)
	swtpmSock := filepath.Join(tmpDir, filenames.SwtpmSock)

	expectedArgs := []string{
		"socket",
		"--tpmstate", "dir=" + stateDir,
		"--ctrl", "type=unixio,path=" + swtpmSock,
		"--tpm2",
		"--terminate",
		"--log", "level=1",
	}

	exe, args, err := SwtpmCmdline(cfg)
	assert.NilError(t, err)
	assert.Equal(t, exe, swtpmPath)
	assert.DeepEqual(t, args, expectedArgs)

	// Verify that state directory was created.
	_, err = os.Stat(stateDir)
	assert.NilError(t, err)

	// Verify that stale socket is removed.
	err = os.WriteFile(swtpmSock, []byte("stale socket"), 0o644)
	assert.NilError(t, err)
	// Call again to clean up the stale socket.
	_, _, err = SwtpmCmdline(cfg)
	assert.NilError(t, err)
	_, err = os.Stat(swtpmSock)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestParseKVMNestedParam(t *testing.T) {
	type testCase struct {
		value         string
		expected      bool
		expectedError string
	}
	testCases := []testCase{
		{value: "Y\n", expected: true},  // kvm_intel (bool)
		{value: "N\n", expected: false}, // kvm_intel (bool)
		{value: "1\n", expected: true},  // kvm_amd (int)
		{value: "0\n", expected: false}, // kvm_amd (int)
		{value: "", expectedError: "unexpected value"},
		{value: "foo", expectedError: "unexpected value"},
	}
	for _, tc := range testCases {
		enabled, err := parseKVMNestedParam(tc.value)
		if tc.expectedError == "" {
			assert.NilError(t, err)
		} else {
			assert.ErrorContains(t, err, tc.expectedError)
		}
		assert.Equal(t, tc.expected, enabled)
	}
}

func TestValidateConfigNestedVirtualization(t *testing.T) {
	cfg := &limatype.LimaYAML{
		NestedVirtualization: new(true),
	}
	err := validateConfig(cfg)
	if runtime.GOOS == "darwin" {
		productVer, verErr := osutil.ProductVersion()
		assert.NilError(t, verErr)
		if productVer.LessThan(*semver.New("15.0.0")) {
			assert.ErrorContains(t, err, "nested virtualization requires macOS 15 or newer")
		} else {
			assert.NilError(t, err)
		}
	} else {
		assert.NilError(t, err)
	}
}

func TestNestedVirtualizationEnabled(t *testing.T) {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		// Unsupported arch
		enabled, err := nestedVirtualizationEnabled(limatype.X8664, "hvf", "host", semver.New("11.1.0"))
		assert.NilError(t, err)
		assert.Equal(t, false, enabled)

		// Unsupported accel
		enabled, err = nestedVirtualizationEnabled(limatype.AARCH64, "tcg", "cortex-a72", semver.New("11.1.0"))
		assert.NilError(t, err)
		assert.Equal(t, false, enabled)

		// QEMU too old
		_, err = nestedVirtualizationEnabled(limatype.AARCH64, "hvf", "host", semver.New("11.0.0"))
		assert.ErrorContains(t, err, "QEMU 11.0.0 is too old")

		// Supported
		enabled, err = nestedVirtualizationEnabled(limatype.AARCH64, "hvf", "host", semver.New("11.1.0"))
		assert.NilError(t, err)
		assert.Equal(t, true, enabled)
	}
}

func TestValidateConfigRejectsBlockDevices(t *testing.T) {
	err := validateConfig(&limatype.LimaYAML{
		BlockDevices: []string{"/dev/disk4"},
	})
	assert.ErrorContains(t, err, "field `blockDevices` is not supported for vmType: qemu")
}

func TestValidateConfigVNCDisplay(t *testing.T) {
	display := func(s string) *limatype.LimaYAML {
		return &limatype.LimaYAML{Video: limatype.Video{Display: &s}}
	}
	// "vnc=..." bypasses the ",password=on" enforcement, exposing a passwordless VNC server.
	for _, bad := range []string{"vnc=0.0.0.0:0", "vnc=:0", "vnc,password=off"} {
		err := validateConfig(display(bad))
		assert.ErrorContains(t, err, "field `video.display`")
	}
	// The supported "vnc" value and non-VNC displays stay valid.
	for _, ok := range []string{"vnc", "none", "default", "gtk", "sdl", "cocoa"} {
		assert.NilError(t, validateConfig(display(ok)))
	}
}

func TestValidate9pMountOptions(t *testing.T) {
	tests := []struct {
		name    string
		nineP   limatype.NineP
		wantErr string
	}{
		{
			name: "valid non-default values",
			nineP: limatype.NineP{
				SecurityModel:   new("mapped-xattr"),
				ProtocolVersion: new("9p2000.u"),
				Cache:           new("loose"),
			},
		},
		{
			// A comma in securityModel would inject extra options into the QEMU -virtfs argument.
			name:    "securityModel with injected option",
			nineP:   limatype.NineP{SecurityModel: new("none,readonly=off")},
			wantErr: "mounts[0].9p.securityModel",
		},
		{
			name:    "unknown protocolVersion",
			nineP:   limatype.NineP{ProtocolVersion: new("9p2000.L,foo=bar")},
			wantErr: "mounts[0].9p.protocolVersion",
		},
		{
			name:    "unknown cache",
			nineP:   limatype.NineP{Cache: new("fscache,foo")},
			wantErr: "mounts[0].9p.cache",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &limatype.LimaYAML{
				Mounts: []limatype.Mount{{Location: "/", NineP: tt.nineP}},
			}
			err := validate9pMountOptions(cfg)
			if tt.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRejectsHostLoopbackFalse(t *testing.T) {
	cfg := &limatype.LimaYAML{NetworkOpts: limatype.NetworkOpts{HostLoopback: new(false)}}
	// validateConfig also runs on deletion, which must keep working.
	assert.NilError(t, validateConfig(cfg))
	l := &LimaQemuDriver{Instance: &limatype.Instance{Config: cfg}}
	assert.ErrorContains(t, l.Validate(t.Context()), "field `networkOpts.hostLoopback: false` is not supported for vmType: qemu")
}

func TestCoordinateQEMUWaitNoRaceOrPanic(t *testing.T) {
	t.Run("usernet errors first and waits for qemu exit", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		qWaitCh := make(chan error, 1)
		qWaitErrCh := make(chan error, 1)
		usernetErrCh := make(chan error, 1)

		go coordinateQEMUWait(cancel, qWaitCh, qWaitErrCh, usernetErrCh)

		usernetErrCh <- errors.New("usernet failed")
		<-ctx.Done()
		qWaitErrCh <- errors.New("qemu killed")

		err, ok := <-qWaitCh
		assert.Assert(t, ok, "expected joined error before channel close")
		assert.ErrorContains(t, err, "usernet failed")
		assert.ErrorContains(t, err, "qemu killed")
		_, open := <-qWaitCh
		assert.Assert(t, !open, "expected qWaitCh to be closed")
	})

	t.Run("qemu exits before concurrent usernet error without panic", func(t *testing.T) {
		_, cancel := context.WithCancel(t.Context())
		qWaitCh := make(chan error, 1)
		qWaitErrCh := make(chan error, 1)
		usernetErrCh := make(chan error, 1)

		var wg sync.WaitGroup
		wg.Go(func() {
			coordinateQEMUWait(cancel, qWaitCh, qWaitErrCh, usernetErrCh)
		})

		qWaitErrCh <- errors.New("qemu exited early")
		usernetErrCh <- errors.New("usernet late error")
		wg.Wait()

		err, ok := <-qWaitCh
		assert.Assert(t, ok, "expected error before channel close")
		assert.ErrorContains(t, err, "qemu exited early")
		_, open := <-qWaitCh
		assert.Assert(t, !open, "expected qWaitCh to be closed")
	})

	t.Run("clean exit with nil wait error closes channel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		qWaitCh := make(chan error, 1)
		qWaitErrCh := make(chan error, 1)
		usernetErrCh := make(chan error, 1)

		go coordinateQEMUWait(cancel, qWaitCh, qWaitErrCh, usernetErrCh)
		qWaitErrCh <- nil

		err, ok := <-qWaitCh
		assert.Assert(t, ok, "expected nil result before channel close")
		assert.NilError(t, err)
		<-ctx.Done()
		_, open := <-qWaitCh
		assert.Assert(t, !open, "expected qWaitCh to be closed")
	})
}

func TestResolveCPUType(t *testing.T) {
	arch := limatype.X8664
	y := &limatype.LimaYAML{
		Arch: &arch,
		VMOpts: limatype.VMOpts{
			limatype.QEMU: limatype.QEMUOpts{
				CPUType: map[limatype.Arch]string{
					"amd64":        "host",
					limatype.X8664: "qemu64",
				},
			},
		},
	}

	var logBuf bytes.Buffer
	origOut := logrus.StandardLogger().Out
	logrus.SetOutput(&logBuf)
	t.Cleanup(func() { logrus.SetOutput(origOut) })

	got := resolveCPUType(y, semver.New("9.0.0"))
	assert.Equal(t, got, "qemu64")
	assert.Assert(t, bytes.Contains(logBuf.Bytes(), []byte("uses unsupported arch `amd64`")), "expected warning log, got: %s", logBuf.String())

	qemuOpts, ok := y.VMOpts[limatype.QEMU].(limatype.QEMUOpts)
	assert.Assert(t, ok)
	_, hasInvalid := qemuOpts.CPUType["amd64"]
	assert.Assert(t, !hasInvalid, "expected unsupported arch key to be ignored")
}

func TestInspectFeatures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell script for fake QEMU")
	}

	t.Run("isolates help buffers per command", func(t *testing.T) {
		fakeQemu := filepath.Join(t.TempDir(), "qemu-fake")
		script := `#!/bin/sh
case "$*" in
  *"-accel help"*)
    printf "Accelerators supported in QEMU binary:\ntcg\nkvm\n"
    ;;
  *"-netdev help"*)
    printf "Available netdev backend types:\nuser\nsocket\n"
    ;;
  *"-machine help"*)
    printf "Supported machines are:\nq35\npc\n"
    ;;
  *"-cpu help"*)
    printf "Available CPUs:\nx86 qemu64\n"
    ;;
  *)
    echo "unexpected args: $*" >&2
    exit 1
    ;;
esac
`
		assert.NilError(t, os.WriteFile(fakeQemu, []byte(script), 0o755))

		f, err := inspectFeatures(t.Context(), fakeQemu, "q35")
		assert.NilError(t, err)
		assert.Equal(t, string(f.AccelHelp), "Accelerators supported in QEMU binary:\ntcg\nkvm\n")
		assert.Equal(t, string(f.NetdevHelp), "Available netdev backend types:\nuser\nsocket\n")
		assert.Equal(t, string(f.MachineHelp), "Supported machines are:\nq35\npc\n")
		assert.Equal(t, string(f.CPUHelp), "Available CPUs:\nx86 qemu64\n")
	})

	t.Run("falls back to stderr when prior command wrote stdout", func(t *testing.T) {
		fakeQemu := filepath.Join(t.TempDir(), "qemu-fake")
		script := `#!/bin/sh
case "$*" in
  *"-accel help"*)
    printf "accel-stdout\n"
    ;;
  *"-netdev help"*)
    printf "netdev-stderr\n" >&2
    ;;
  *"-machine help"*)
    printf "machine-stdout\n"
    ;;
  *"-cpu help"*)
    printf "cpu-stderr\n" >&2
    ;;
esac
`
		assert.NilError(t, os.WriteFile(fakeQemu, []byte(script), 0o755))

		f, err := inspectFeatures(t.Context(), fakeQemu, "q35")
		assert.NilError(t, err)
		assert.Equal(t, string(f.AccelHelp), "accel-stdout\n")
		assert.Equal(t, string(f.NetdevHelp), "netdev-stderr\n")
		assert.Equal(t, string(f.MachineHelp), "machine-stdout\n")
		assert.Equal(t, string(f.CPUHelp), "cpu-stderr\n")
	})

	t.Run("accel failure returns error and optional check failure leaves field empty", func(t *testing.T) {
		fakeQemu := filepath.Join(t.TempDir(), "qemu-fake")
		script := `#!/bin/sh
case "$*" in
  *"-accel help"*)
    echo "accel boom" >&2
    exit 1
    ;;
esac
`
		assert.NilError(t, os.WriteFile(fakeQemu, []byte(script), 0o755))
		_, err := inspectFeatures(t.Context(), fakeQemu, "q35")
		assert.ErrorContains(t, err, "accel boom")

		script2 := `#!/bin/sh
case "$*" in
  *"-accel help"*)
    printf "tcg\n"
    ;;
  *)
    echo "unsupported" >&2
    exit 1
    ;;
esac
`
		assert.NilError(t, os.WriteFile(fakeQemu, []byte(script2), 0o755))
		f, err := inspectFeatures(t.Context(), fakeQemu, "q35")
		assert.NilError(t, err)
		assert.Equal(t, string(f.AccelHelp), "tcg\n")
		assert.Equal(t, len(f.NetdevHelp), 0)
		assert.Equal(t, len(f.MachineHelp), 0)
		assert.Equal(t, len(f.CPUHelp), 0)
	})
}
