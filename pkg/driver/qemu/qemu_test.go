// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

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

func TestGetFirmwareFromDescriptorFiles(t *testing.T) {
	dir := t.TempDir()
	secureCode2M := filepath.Join(dir, "OVMF_CODE.secboot.fd")
	secureVars2M := filepath.Join(dir, "OVMF_VARS.secboot.fd")
	secureCode4M := filepath.Join(dir, "OVMF_CODE_4M.secboot.qcow2")
	secureVarsWithKeys := filepath.Join(dir, "OVMF_VARS_4M.secboot.qcow2")
	secureVarsWithoutKeys := filepath.Join(dir, "OVMF_VARS_4M.qcow2")
	plainCode := filepath.Join(dir, "OVMF_CODE_4M.qcow2")
	plainVars := filepath.Join(dir, "OVMF_VARS_4M.qcow2")
	for _, path := range []string{secureCode2M, secureVars2M, secureCode4M, secureVarsWithKeys, secureVarsWithoutKeys, plainCode, plainVars} {
		assert.NilError(t, os.WriteFile(path, nil, 0o644))
	}

	secureWithKeys2M := filepath.Join(dir, "secure-with-keys-2m.json")
	secureWithKeys4M := filepath.Join(dir, "secure-with-keys-4m.json")
	secureWithoutKeys := filepath.Join(dir, "secure-without-keys.json")
	plain := filepath.Join(dir, "plain-4m.json")
	assert.NilError(t, os.WriteFile(secureWithKeys2M, []byte(testQemuFirmwareDescriptorJSON(secureCode2M, secureVars2M, []string{"secure-boot", "requires-smm", "enrolled-keys"})), 0o644))
	assert.NilError(t, os.WriteFile(secureWithKeys4M, []byte(testQemuFirmwareDescriptorJSON(secureCode4M, secureVarsWithKeys, []string{"secure-boot", "requires-smm", "enrolled-keys"})), 0o644))
	assert.NilError(t, os.WriteFile(secureWithoutKeys, []byte(testQemuFirmwareDescriptorJSON(secureCode4M, secureVarsWithoutKeys, []string{"secure-boot", "requires-smm"})), 0o644))
	assert.NilError(t, os.WriteFile(plain, []byte(testQemuFirmwareDescriptorJSON(plainCode, plainVars, nil)), 0o644))

	testCases := []struct {
		name                    string
		descriptors             []string
		secureBoot              bool
		preEnrollSecureBootKeys bool
		expectedDescriptor      string
		expectedVars            string
		expectedErr             string
	}{
		{
			name:                    "secure boot requires pre-enrolled keys",
			descriptors:             []string{secureWithoutKeys, secureWithKeys4M},
			secureBoot:              true,
			preEnrollSecureBootKeys: true,
			expectedDescriptor:      secureWithKeys4M,
			expectedVars:            secureVarsWithKeys,
		},
		{
			name:                    "secure boot does not fallback to empty vars when pre-enrolled keys are required",
			descriptors:             []string{secureWithoutKeys},
			secureBoot:              true,
			preEnrollSecureBootKeys: true,
			expectedErr:             "no QEMU firmware descriptor",
		},
		{
			name:                    "secure boot rejects pre-enrolled keys when false",
			descriptors:             []string{secureWithKeys4M, secureWithoutKeys},
			secureBoot:              true,
			preEnrollSecureBootKeys: false,
			expectedDescriptor:      secureWithoutKeys,
			expectedVars:            secureVarsWithoutKeys,
		},
		{
			name:                    "prefers 4M firmware",
			descriptors:             []string{secureWithKeys2M, secureWithKeys4M},
			secureBoot:              true,
			preEnrollSecureBootKeys: true,
			expectedDescriptor:      secureWithKeys4M,
			expectedVars:            secureVarsWithKeys,
		},
		{
			name:               "rejects secure boot when disabled",
			descriptors:        []string{secureWithKeys4M, plain},
			secureBoot:         false,
			expectedDescriptor: plain,
			expectedVars:       plainVars,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			firmware, err := getFirmwareFromDescriptorFiles(tc.descriptors, limatype.X8664, tc.secureBoot, tc.preEnrollSecureBootKeys)
			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tc.expectedDescriptor, firmware.DescriptorPath)
			assert.Assert(t, firmware.Vars != nil)
			assert.Equal(t, tc.expectedVars, firmware.Vars.Path)
		})
	}
}

func TestEnsureFirmwareFileIfMissingPreservesExistingVars(t *testing.T) {
	dir := t.TempDir()
	template := filepath.Join(dir, "template.fd")
	vars := filepath.Join(dir, "vars.fd")
	assert.NilError(t, os.WriteFile(template, []byte("template"), 0o644))
	assert.NilError(t, os.WriteFile(vars, []byte("guest-nvram"), 0o644))

	err := ensureFirmwareFileIfMissing(vars, firmwareFile{Path: template, Format: "raw"})
	assert.NilError(t, err)
	b, err := os.ReadFile(vars)
	assert.NilError(t, err)
	assert.Equal(t, "guest-nvram", string(b))
}

func testQemuFirmwareDescriptorJSON(code, vars string, features []string) string {
	featuresJSON, err := json.Marshal(features)
	if err != nil {
		panic(err)
	}
	raw := fmt.Sprintf(`{
		"description": "test secure boot firmware",
		"interface-types": ["uefi"],
		"mapping": {
			"device": "flash",
			"executable": {
				"filename": %q,
				"format": ""
			},
			"nvram-template": {
				"filename": %q,
				"format": ""
			}
		},
		"targets": [
			{
				"architecture": "x86_64",
				"machines": ["pc-q35-*"]
			}
		],
		"features": %s
	}`, code, vars, string(featuresJSON))
	return raw
}
