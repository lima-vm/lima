// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

const (
	qemuFirmwareInterfaceUEFI = "uefi"

	qemuFirmwareDeviceFlash = "flash"

	qemuFirmwareFlashModeSplit = "split"

	qemuFirmwareFeatureEnrolledKeys = "enrolled-keys"
	qemuFirmwareFeatureRequiresSMM  = "requires-smm"
	qemuFirmwareFeatureSecureBoot   = "secure-boot"

	qemuFirmwareFormatRaw   = "raw"
	qemuFirmwareFormatQCOW2 = "qcow2"
)

type qemuFirmwareDescriptor struct {
	InterfaceTypes []string `json:"interface-types"`
	Mapping        struct {
		Device string `json:"device"`
		// Mode is optional in QEMU descriptors. For flash mappings, an empty
		// mode is treated like "split".
		Mode string `json:"mode"`

		Executable struct {
			Filename string `json:"filename"`
			Format   string `json:"format"`
		} `json:"executable"`

		NVRAMTemplate struct {
			Filename string `json:"filename"`
			Format   string `json:"format"`
		} `json:"nvram-template"`

		// Other mapping variants, not currently used by Lima's Secure Boot path.
		Filename string `json:"filename"`
		UEFIVars struct {
			Template string `json:"template"`
		} `json:"uefi-vars"`
	} `json:"mapping"`
	Targets []struct {
		Architecture string   `json:"architecture"`
		Machines     []string `json:"machines"`
	} `json:"targets"`
	Features []string `json:"features"`
}

type firmwareFile struct {
	Path   string
	Format string
}

type qemuFirmware struct {
	Code           firmwareFile
	Vars           *firmwareFile
	DescriptorPath string
}

func getFirmwareTemplate(qemuExe string, arch limatype.Arch, secureBoot, preEnrollSecureBootKeys bool, descriptorPaths []string) (qemuFirmware, error) {
	descriptors, err := qemuFirmwareDescriptorPaths(qemuExe)
	if err != nil {
		return qemuFirmware{}, err
	}
	descriptorPaths = append(slices.Clone(descriptorPaths), descriptors...)
	firmware, err := getFirmwareFromDescriptorFiles(descriptorPaths, arch, secureBoot, preEnrollSecureBootKeys)
	if err == nil {
		return firmware, nil
	}
	if secureBoot {
		return qemuFirmware{}, err
	}
	logrus.WithError(err).Debug("failed to find firmware via QEMU descriptors")

	code, err := getFirmwareCode(qemuExe, arch)
	if err != nil {
		return qemuFirmware{}, err
	}
	return qemuFirmware{Code: firmwareFile{Path: code, Format: qemuFirmwareFormatRaw}}, nil
}

func (f qemuFirmware) instanceCodePath(instanceDir string) string {
	if f.Code.Format == qemuFirmwareFormatQCOW2 {
		return filepath.Join(instanceDir, filenames.QemuEfiCodeQCOW2)
	}
	return filepath.Join(instanceDir, filenames.QemuEfiCodeFD)
}

func (f qemuFirmware) instanceVarsPath(instanceDir string) string {
	if f.Vars != nil && f.Vars.Format == qemuFirmwareFormatQCOW2 {
		return filepath.Join(instanceDir, filenames.QemuEfiVarsQCOW2)
	}
	return filepath.Join(instanceDir, filenames.QemuEfiVarsFD)
}

func qemuFirmwareDescriptorDirs(qemuExe string) []string {
	var dirs []string

	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		dirs = append(dirs, filepath.Join(xdgConfigHome, "qemu", "firmware"))
	} else if homeDir, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(homeDir, ".config", "qemu", "firmware"))
	}
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		dirs = append(dirs, filepath.Join(xdgDataHome, "qemu", "firmware"))
	} else if homeDir, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(homeDir, ".local", "share", "qemu", "firmware"))
	}

	dirs = append(dirs, filepath.Join(string(os.PathSeparator), "etc", "qemu", "firmware"))

	binDir := filepath.Dir(qemuExe)  // "/usr/local/bin"
	localDir := filepath.Dir(binDir) // "/usr/local"

	dirs = append(dirs,
		filepath.Join(localDir, "share", "qemu", "firmware"), // macOS Homebrew, Linux custom prefixes
		filepath.Join(binDir, "share", "firmware"),           // Windows installer
	)
	if localDir != "/usr" {
		dirs = append(dirs, filepath.Join(string(os.PathSeparator), "usr", "share", "qemu", "firmware"))
	}

	return dirs
}

func getFirmwareFromDescriptorFiles(descriptorPaths []string, arch limatype.Arch, secureBoot, preEnrollSecureBootKeys bool) (qemuFirmware, error) {
	var matches []qemuFirmware
	for _, descriptorPath := range descriptorPaths {
		if descriptorPath == "" {
			continue
		}
		b, err := os.ReadFile(descriptorPath)
		if err != nil {
			return qemuFirmware{}, err
		}
		var descriptor qemuFirmwareDescriptor
		if err = json.Unmarshal(b, &descriptor); err != nil {
			return qemuFirmware{}, err
		}
		firmware, ok := firmwareFromDescriptor(descriptor, descriptorPath, arch, secureBoot, preEnrollSecureBootKeys)
		if ok {
			matches = append(matches, firmware)
		}
	}
	for _, firmware := range matches {
		if firmwareUses4M(firmware) {
			return firmware, nil
		}
	}
	if len(matches) > 0 {
		return matches[0], nil
	}
	return qemuFirmware{}, fmt.Errorf("no QEMU firmware descriptor from %v matches arch %q secureBoot=%v preEnrollSecureBootKeys=%v", descriptorPaths, arch, secureBoot, preEnrollSecureBootKeys)
}

func qemuFirmwareDescriptorPaths(qemuExe string) ([]string, error) {
	// QEMU descriptor lookup supports overriding descriptors by filename. More
	// specific directories are listed first here. A zero-length descriptor file
	// masks descriptors with the same basename from less-specific directories.
	seen := map[string]bool{}
	var descriptors []string

	for _, dir := range qemuFirmwareDescriptorDirs(qemuExe) {
		matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil {
			return nil, err
		}
		slices.Sort(matches)

		for _, descriptorPath := range matches {
			base := filepath.Base(descriptorPath)
			if seen[base] {
				continue
			}
			seen[base] = true

			info, err := os.Stat(descriptorPath)
			if err != nil {
				logrus.WithError(err).Debugf("failed to stat QEMU firmware descriptor %q", descriptorPath)
				continue
			}
			if info.Size() == 0 {
				logrus.Debugf("QEMU firmware descriptor %q masks less-specific descriptors", descriptorPath)
				continue
			}

			descriptors = append(descriptors, descriptorPath)
		}
	}

	return descriptors, nil
}

func firmwareFromDescriptor(descriptor qemuFirmwareDescriptor, descriptorPath string, arch limatype.Arch, secureBoot, preEnrollSecureBootKeys bool) (qemuFirmware, bool) {
	if !slices.Contains(descriptor.InterfaceTypes, qemuFirmwareInterfaceUEFI) {
		return qemuFirmware{}, false
	}
	if secureBoot && !slices.Contains(descriptor.Features, qemuFirmwareFeatureSecureBoot) {
		return qemuFirmware{}, false
	}
	if !secureBoot && slices.Contains(descriptor.Features, qemuFirmwareFeatureSecureBoot) {
		return qemuFirmware{}, false
	}
	if secureBoot && !slices.Contains(descriptor.Features, qemuFirmwareFeatureRequiresSMM) {
		return qemuFirmware{}, false
	}
	if preEnrollSecureBootKeys && !slices.Contains(descriptor.Features, qemuFirmwareFeatureEnrolledKeys) {
		return qemuFirmware{}, false
	}
	if !preEnrollSecureBootKeys && slices.Contains(descriptor.Features, qemuFirmwareFeatureEnrolledKeys) {
		return qemuFirmware{}, false
	}
	if !firmwareTargetsQ35(descriptor.Targets, arch) {
		return qemuFirmware{}, false
	}
	if descriptor.Mapping.Device != qemuFirmwareDeviceFlash {
		return qemuFirmware{}, false
	}
	if descriptor.Mapping.Mode != "" && descriptor.Mapping.Mode != qemuFirmwareFlashModeSplit {
		return qemuFirmware{}, false
	}

	code := descriptor.Mapping.Executable.Filename
	vars := descriptor.Mapping.NVRAMTemplate.Filename
	if code == "" || vars == "" {
		return qemuFirmware{}, false
	}

	codeFormat := firmwareFormat(descriptor.Mapping.Executable.Format, code)
	if !supportedFirmwareFormat(codeFormat) {
		return qemuFirmware{}, false
	}
	varsFormat := firmwareFormat(descriptor.Mapping.NVRAMTemplate.Format, vars)
	if !supportedFirmwareFormat(varsFormat) {
		return qemuFirmware{}, false
	}

	if !osutil.FileExists(code) || !osutil.FileExists(vars) {
		return qemuFirmware{}, false
	}

	firmware := qemuFirmware{
		Code:           firmwareFile{Path: code, Format: codeFormat},
		Vars:           &firmwareFile{Path: vars, Format: varsFormat},
		DescriptorPath: descriptorPath,
	}
	return firmware, true
}

func firmwareTargetsQ35(
	targets []struct {
		Architecture string   `json:"architecture"`
		Machines     []string `json:"machines"`
	},
	arch limatype.Arch,
) bool {
	for _, target := range targets {
		if target.Architecture != "x86_64" || arch != limatype.X8664 {
			continue
		}
		for _, machine := range target.Machines {
			if strings.HasPrefix(machine, "pc-q35-") || strings.Contains(machine, "q35") {
				return true
			}
		}
	}
	return false
}

func firmwareUses4M(firmware qemuFirmware) bool {
	s := strings.ToLower(filepath.Base(firmware.Code.Path))
	if firmware.Vars != nil {
		s += " " + strings.ToLower(filepath.Base(firmware.Vars.Path))
	}
	return strings.Contains(s, "4m")
}

func prepareFirmwareTemplate(instanceDir string, firmware qemuFirmware, secureBoot bool) (codePath, varsPath, codeFormat, varsFormat string, err error) {
	codeFormat = firmware.Code.Format
	varsFormat = qemuFirmwareFormatRaw
	if firmware.DescriptorPath == "" {
		logrus.Infof("Using system firmware %q", firmware.Code.Path)
		return firmware.Code.Path, "", codeFormat, varsFormat, nil
	}
	if firmware.Vars == nil {
		return "", "", "", "", fmt.Errorf("firmware descriptor %q is missing a variable store template", firmware.DescriptorPath)
	}

	codePath = firmware.instanceCodePath(instanceDir)
	varsPath = firmware.instanceVarsPath(instanceDir)
	varsFormat = firmware.Vars.Format
	if err = ensureFirmwareFile(codePath, firmware.Code); err != nil {
		return "", "", "", "", err
	}
	if err = ensureFirmwareFileIfMissing(varsPath, *firmware.Vars); err != nil {
		return "", "", "", "", err
	}
	if secureBoot {
		logrus.Infof("Using Secure Boot firmware %q from %q with variable store %q (template %q, descriptor %q)", codePath, firmware.Code.Path, varsPath, firmware.Vars.Path, firmware.DescriptorPath)
	} else {
		logrus.Infof("Using firmware %q from %q with variable store %q (template %q, descriptor %q)", codePath, firmware.Code.Path, varsPath, firmware.Vars.Path, firmware.DescriptorPath)
	}
	return codePath, varsPath, codeFormat, varsFormat, nil
}

func supportedFirmwareFormat(format string) bool {
	switch format {
	case "", qemuFirmwareFormatRaw, qemuFirmwareFormatQCOW2:
		return true
	default:
		return false
	}
}

func firmwareFormat(format, path string) string {
	if format != "" {
		return format
	}
	if strings.EqualFold(filepath.Ext(path), ".qcow2") {
		return qemuFirmwareFormatQCOW2
	}
	return qemuFirmwareFormatRaw
}

func ensureFirmwareFile(dst string, src firmwareFile) error {
	if !supportedFirmwareFormat(src.Format) {
		return fmt.Errorf("unsupported firmware format %q for %q", src.Format, src.Path)
	}
	return copyFileIfChanged(dst, src.Path)
}

func ensureFirmwareFileIfMissing(dst string, src firmwareFile) error {
	if _, err := os.Stat(dst); err == nil {
		logrus.Infof("Preserving existing firmware variable store %q", dst)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return ensureFirmwareFile(dst, src)
}

func copyFileIfChanged(dst, src string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		_ = dstFile.Close()
		return err
	}
	if err = dstFile.Close(); err != nil {
		return err
	}
	return replaceFileIfChanged(dst, tmp)
}

func replaceFileIfChanged(dst, tmp string) error {
	same, err := sameFileContents(dst, tmp)
	if err != nil {
		return err
	}
	if same {
		return os.Remove(tmp)
	}
	return os.Rename(tmp, dst)
}

func sameFileContents(a, b string) (bool, error) {
	aBytes, err := os.ReadFile(a)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	bBytes, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(aBytes, bBytes), nil
}
