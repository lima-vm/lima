// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

// TestMain intercepts the test binary when invoked as a mock QEMU.
// The mock is activated only when both LIMA_TEST_MOCK_QEMU=1 and
// LIMA_TEST_MOCK_QEMU_OWNER=tpm_test are set, so a stray env var
// cannot silently hijack other tests in this package.
func TestMain(m *testing.M) {
	if os.Getenv("LIMA_TEST_MOCK_QEMU") == "1" && os.Getenv("LIMA_TEST_MOCK_QEMU_OWNER") == "tpm_test" {
		// Mock QEMU: respond to feature-probe flags and exit.
		if len(os.Args) > 1 {
			for i, arg := range os.Args {
				if arg == "--version" {
					fmt.Println("QEMU emulator version 8.2.1")
					os.Exit(0)
				}
				if arg == "-accel" && i+1 < len(os.Args) && os.Args[i+1] == "help" {
					fmt.Println("Accelerators: kvm, hvf, whpx, nvmm, tcg")
					os.Exit(0)
				}
				if arg == "-cpu" && i+1 < len(os.Args) && os.Args[i+1] == "help" {
					fmt.Println("Available CPUs:")
					fmt.Println("  qemu64")
					fmt.Println("  max")
					fmt.Println("  host")
					os.Exit(0)
				}
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestSwtpmCmdline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("swtpm Unix socket mode is not supported on Windows hosts")
	}
	tmpDir := t.TempDir()

	// 1. Create a mock swtpm executable in a temporary bin directory
	binDir := filepath.Join(tmpDir, "bin")
	err := os.MkdirAll(binDir, 0o755)
	assert.NilError(t, err)

	swtpmPath := filepath.Join(binDir, "swtpm")
	err = os.WriteFile(swtpmPath, []byte{}, 0o755)
	assert.NilError(t, err)

	// 2. Prepend the temporary bin directory to PATH
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)
	os.Setenv("PATH", binDir+":"+oldPath)

	cfg := Config{
		Name:        "test-tpm",
		InstanceDir: tmpDir,
		LimaYAML:    &limatype.LimaYAML{},
	}

	exe, args, err := SwtpmCmdline(cfg)
	assert.NilError(t, err)
	assert.Equal(t, exe, swtpmPath)

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
	assert.DeepEqual(t, args, expectedArgs)

	// Verify that state directory was created
	_, err = os.Stat(stateDir)
	assert.NilError(t, err)

	// Verify stale socket removal: pre-create a stale socket and call again
	err = os.WriteFile(swtpmSock, []byte("stale"), 0o644)
	assert.NilError(t, err)
	_, _, err = SwtpmCmdline(cfg)
	assert.NilError(t, err)
	_, statErr := os.Stat(swtpmSock)
	assert.Assert(t, os.IsNotExist(statErr), "stale socket should have been removed")
}

func TestTPMQEMUArgs(t *testing.T) {
	// Stage the mock QEMU binary under t.TempDir() so we don't pollute
	// $GOCACHE or leave directories behind (S5).
	mockDir := t.TempDir()
	mockBinDir := filepath.Join(mockDir, "bin")
	err := os.MkdirAll(mockBinDir, 0o755)
	assert.NilError(t, err)

	// Copy the test binary to act as mock QEMU
	absSelf, err := filepath.Abs(os.Args[0])
	assert.NilError(t, err)
	selfBytes, err := os.ReadFile(absSelf)
	assert.NilError(t, err)

	mockQemuX86 := filepath.Join(mockBinDir, "qemu-system-x86_64")
	mockQemuAarch64 := filepath.Join(mockBinDir, "qemu-system-aarch64")
	mockQemuArm := filepath.Join(mockBinDir, "qemu-system-arm")
	mockQemuRiscv64 := filepath.Join(mockBinDir, "qemu-system-riscv64")
	if runtime.GOOS == "windows" {
		mockQemuX86 += ".exe"
		mockQemuAarch64 += ".exe"
		mockQemuArm += ".exe"
		mockQemuRiscv64 += ".exe"
	}
	for _, p := range []string{mockQemuX86, mockQemuAarch64, mockQemuArm, mockQemuRiscv64} {
		err = os.WriteFile(p, selfBytes, 0o755)
		assert.NilError(t, err)
	}

	t.Setenv("QEMU_SYSTEM_X86_64", filepath.ToSlash(mockQemuX86))
	t.Setenv("QEMU_SYSTEM_AARCH64", filepath.ToSlash(mockQemuAarch64))
	t.Setenv("QEMU_SYSTEM_ARM", filepath.ToSlash(mockQemuArm))
	t.Setenv("QEMU_SYSTEM_RISCV64", filepath.ToSlash(mockQemuRiscv64))
	t.Setenv("LIMA_TEST_MOCK_QEMU", "1")
	t.Setenv("LIMA_TEST_MOCK_QEMU_OWNER", "tpm_test")

	// Create firmware files under the same TempDir tree (S5)
	shareDir := filepath.Join(mockBinDir, "share")
	err = os.MkdirAll(shareDir, 0o755)
	assert.NilError(t, err)
	localShareDir := filepath.Join(mockDir, "share", "qemu")
	err = os.MkdirAll(localShareDir, 0o755)
	assert.NilError(t, err)

	for _, f := range []string{
		filepath.Join(shareDir, "edk2-x86_64-code.fd"),
		filepath.Join(localShareDir, "edk2-x86_64-code.fd"),
		filepath.Join(shareDir, "edk2-aarch64-code.fd"),
		filepath.Join(localShareDir, "edk2-aarch64-code.fd"),
		filepath.Join(shareDir, "edk2-arm-code.fd"),
		filepath.Join(localShareDir, "edk2-arm-code.fd"),
		filepath.Join(shareDir, "edk2-riscv-code.fd"),
		filepath.Join(localShareDir, "edk2-riscv-code.fd"),
		filepath.Join(shareDir, "edk2-i386-vars.fd"),
		filepath.Join(localShareDir, "edk2-i386-vars.fd"),
	} {
		err = os.WriteFile(f, []byte("mock-firmware-content"), 0o644)
		assert.NilError(t, err)
	}

	testCases := []struct {
		name         string
		arch         limatype.Arch
		tpmEnabled   bool
		expectedArgs []string
		excludedArgs []string
		expectError  bool
	}{
		{
			name:       "x86_64 with TPM enabled",
			arch:       limatype.X8664,
			tpmEnabled: true,
			expectedArgs: []string{
				"-chardev",
				"socket,id=chrtpm,",
				"-tpmdev",
				"emulator,id=tpm0,chardev=chrtpm",
				"-device",
				"tpm-crb,tpmdev=tpm0",
			},
		},
		{
			name:       "x86_64 with TPM disabled",
			arch:       limatype.X8664,
			tpmEnabled: false,
			excludedArgs: []string{
				"id=chrtpm",
				"emulator,id=tpm0",
				"tpm-crb",
				"tpm-tis-device",
			},
		},
		{
			name:       "aarch64 with TPM enabled",
			arch:       limatype.AARCH64,
			tpmEnabled: true,
			expectedArgs: []string{
				"-chardev",
				"socket,id=chrtpm,",
				"-tpmdev",
				"emulator,id=tpm0,chardev=chrtpm",
				"-device",
				"tpm-tis-device,tpmdev=tpm0",
			},
		},
		{
			name:       "aarch64 with TPM disabled",
			arch:       limatype.AARCH64,
			tpmEnabled: false,
			excludedArgs: []string{
				"id=chrtpm",
				"emulator,id=tpm0",
				"tpm-crb",
				"tpm-tis-device",
			},
		},
		{
			name:       "armv7l with TPM enabled",
			arch:       limatype.ARMV7L,
			tpmEnabled: true,
			expectedArgs: []string{
				"tpm-tis-device,tpmdev=tpm0",
			},
		},
		{
			name:        "riscv64 with TPM enabled (unsupported)",
			arch:        limatype.RISCV64,
			tpmEnabled:  true,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create stub files that Cmdline might access
			err = os.WriteFile(filepath.Join(tmpDir, filenames.CIDataISO), []byte{}, 0o644)
			assert.NilError(t, err)

			cfg := Config{
				Name:        "test-tpm-vm",
				InstanceDir: tmpDir,
				LimaYAML: &limatype.LimaYAML{
					Arch:   ptr.Of(tc.arch),
					CPUs:   ptr.Of(1),
					Memory: ptr.Of("512MiB"),
					MountType: ptr.Of(limatype.REVSSHFS),
					Audio: limatype.Audio{
						Device: ptr.Of(""),
					},
					Video: limatype.Video{
						Display: ptr.Of("none"),
					},
					VMType: ptr.Of(limatype.QEMU),
					Firmware: limatype.Firmware{
						LegacyBIOS: ptr.Of(false),
					},
					TPM: limatype.TPM{
						Enabled: ptr.Of(tc.tpmEnabled),
					},
				},
			}

			_, args, err := Cmdline(context.Background(), cfg)

			if tc.expectError {
				assert.Assert(t, err != nil, "expected error for unsupported arch %q", tc.arch)
				return
			}
			assert.NilError(t, err)

			// Validate expected arguments are present
			for _, expected := range tc.expectedArgs {
				found := false
				for _, arg := range args {
					if strings.Contains(arg, expected) {
						found = true
						break
					}
				}
				assert.Assert(t, found, "expected arg %q to be present in %v", expected, args)
			}

			// Validate excluded arguments are not present
			for _, excluded := range tc.excludedArgs {
				for _, arg := range args {
					assert.Assert(t, !strings.Contains(arg, excluded), "did not expect arg %q to be present, but found in %v", excluded, args)
				}
			}
		})
	}
}
