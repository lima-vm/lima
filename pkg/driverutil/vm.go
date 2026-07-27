// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/reflectutil"
	"github.com/lima-vm/lima/v2/pkg/registry"
)

// ResolveVMType sets y.VMType to the appropriate default if it is nil.
// It validates the VMType is a known type (built-in or discovered external driver)
// but does NOT require the driver to be available on the current platform.
func ResolveVMType(y *limatype.LimaYAML) error {
	if y.VMType == nil {
		y.VMType = ptr.Of(limatype.DefaultDriver())
		if y.Arch != nil && !limatype.IsNativeArch(*y.Arch) {
			y.VMType = ptr.Of(limatype.DefaultNonNativeArchDriver())
		}
	}

	// Accept built-in VMType regardless of current platform.
	if slices.Contains(limatype.VMTypes, *y.VMType) {
		return nil
	}

	// Also accept external drivers discovered in the registry.
	_, _, exists := registry.Get(*y.VMType)
	if !exists {
		return fmt.Errorf("vmType %#q is not a registered driver", *y.VMType)
	}

	return nil
}

func InspectStatus(ctx context.Context, inst *limatype.Instance) (string, error) {
	if inst == nil || inst.Config == nil || inst.Config.VMType == nil {
		return "", errors.New("instance or its configuration is not properly initialized")
	}

	extDriver, intDriver, exists := registry.Get(*inst.Config.VMType)
	if !exists {
		return "", fmt.Errorf("unknown or unsupported VM type: %s", *inst.Config.VMType)
	}

	if extDriver != nil {
		status, err := handleInspectStatusAction(ctx, inst, extDriver.Path)
		if err != nil {
			extDriver.Logger.Errorf("Failed to inspect status for instance %#q: %v", inst.Name, err)
			return "", err
		}
		extDriver.Logger.Debugf("Instance %#q inspected successfully with status: %s", inst.Name, inst.Status)
		return status, nil
	}

	return intDriver.InspectStatus(ctx, inst), nil
}

func handleInspectStatusAction(ctx context.Context, inst *limatype.Instance, extDriverPath string) (string, error) {
	cmd := exec.CommandContext(ctx, extDriverPath, "--inspect-status")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	encoder := json.NewEncoder(stdin)
	payload, err := inst.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("failed to marshal instance config: %w", err)
	}
	if err := encoder.Encode(payload); err != nil {
		return "", err
	}
	stdin.Close()

	decoder := json.NewDecoder(stdout)
	var response []byte
	if err := decoder.Decode(&response); err != nil {
		return "", err
	}

	var respInst limatype.Instance
	if err := respInst.UnmarshalJSON(response); err != nil {
		return "", fmt.Errorf("failed to unmarshal instance response: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		if stderrBuf.Len() > 0 {
			return "", fmt.Errorf("inspect status command failed: %w; stderr: %s", err, stderrBuf.String())
		}
		return "", fmt.Errorf("inspect status command failed: %w", err)
	}

	if stderrBuf.Len() > 0 {
		logrus.Debugf("external driver stderr: %s", stderrBuf.String())
	}

	*inst = respInst
	logrus.Debugf("Inspecting instance status action completed successfully for %#q", extDriverPath)
	return inst.Status, nil
}

var knownContainerYamlProperties = []string{
	"Arch",
	"Audio",
	"Containerd",
	"CopyToHost",
	"CPUType",
	"Disk",
	"DNS",
	"Env",
	"HostResolver",
	"Images",
	"Message",
	"Mounts",
	"MountType",
	"Networks",
	"OS",
	"Param",
	"Plain",
	"PortForwards",
	"Probes",
	"PropagateProxyEnv",
	"Provision",
	"SSH",
	"TPM",
	"VMType",
}

// ValidateContainerDriverConfig validates the YAML config for container-based/rootfs-based drivers.
func ValidateContainerDriverConfig(cfg *limatype.LimaYAML, driverName string, allowedMountTypes []limatype.MountType) error {
	if cfg == nil {
		return errors.New("configuration is nil")
	}

	if cfg.MountType == nil && len(allowedMountTypes) > 0 {
		cfg.MountType = ptr.Of(allowedMountTypes[0])
	}

	if cfg.MountType != nil {
		if !slices.Contains(allowedMountTypes, *cfg.MountType) {
			if len(allowedMountTypes) == 1 {
				return fmt.Errorf("field `mountType` must be %#q for %s driver, got %#q", allowedMountTypes[0], driverName, *cfg.MountType)
			}
			return fmt.Errorf("field `mountType` must be one of %v for %s driver, got %#q", allowedMountTypes, driverName, *cfg.MountType)
		}
	}

	if cfg.VMType != nil {
		if unknown := reflectutil.UnknownNonEmptyFields(cfg, knownContainerYamlProperties...); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: %+v", *cfg.VMType, unknown)
		}
	}

	if cfg.OS != nil && *cfg.OS != limatype.LINUX {
		return fmt.Errorf("guest OS %q is not supported for %s driver; only Linux guest OS is supported", *cfg.OS, driverName)
	}

	if cfg.Arch != nil && !limatype.IsNativeArch(*cfg.Arch) {
		return fmt.Errorf("unsupported arch: %#q", *cfg.Arch)
	}

	if cfg.TPM != nil && *cfg.TPM {
		return fmt.Errorf("field `tpm` is not supported on %s driver", driverName)
	}

	if cfg.Images != nil && cfg.Arch != nil {
		tarFileRegex := regexp.MustCompile(`\.(tar|tgz|txz|tbz2|tzst|tar\.(gz|xz|bz2|zstd|zst))$`)
		unsupportedVMImgRegex := regexp.MustCompile(`\.(qcow2|raw|img|iso|ipsw)(\.(gz|xz|bz2|zstd|zst))?$`)
		squashfsRegex := regexp.MustCompile(`\.squashfs(\.(gz|xz|bz2|zstd|zst))?$`)
		for i, image := range cfg.Images {
			if unknown := reflectutil.UnknownNonEmptyFields(image, "File", "Variant"); len(unknown) > 0 {
				logrus.Warnf("Ignoring: vmType %s: images[%d]: %+v", driverName, i, unknown)
			}
			if image.Arch == *cfg.Arch {
				location := image.Location
				if !tarFileRegex.MatchString(location) {
					if unsupportedVMImgRegex.MatchString(location) {
						return fmt.Errorf("unsupported image type for %s: %q. %s only supports importing tar archive root filesystems, not standard VM disk images", driverName, location, driverName)
					}
					if squashfsRegex.MatchString(location) {
						return fmt.Errorf("unsupported image type for %s: %q. %s does not natively support importing SquashFS images; please convert the image to a tar archive before importing", driverName, location, driverName)
					}
					return fmt.Errorf("unsupported image type for %s: %q. A tar archive root filesystem (.tar, .tar.gz, .tar.xz, etc.) is required", driverName, location)
				}
			}
		}
	}

	if cfg.Mounts != nil {
		for i, mount := range cfg.Mounts {
			if unknown := reflectutil.UnknownNonEmptyFields(mount); len(unknown) > 0 {
				logrus.Warnf("Ignoring: vmType %s: mounts[%d]: %+v", driverName, i, unknown)
			}
		}
	}

	if cfg.Networks != nil {
		for i, network := range cfg.Networks {
			if unknown := reflectutil.UnknownNonEmptyFields(network); len(unknown) > 0 {
				logrus.Warnf("Ignoring: vmType %s: networks[%d]: %+v", driverName, i, unknown)
			}
		}
	}

	if cfg.Audio.Device != nil {
		audioDevice := *cfg.Audio.Device
		if audioDevice != "" {
			logrus.Warnf("Ignoring: vmType %s: `audio.device`: %+v", driverName, audioDevice)
		}
	}

	return nil
}

// DetectInitSystem detects the init system (systemd or openrc) in a tarball rootfs.
func DetectInitSystem(ctx context.Context, baseDisk string) string {
	baseDiskLower := strings.ToLower(filepath.Base(baseDisk))
	if strings.Contains(baseDiskLower, "alpine") {
		return "openrc"
	}
	if strings.Contains(baseDiskLower, "ubuntu") ||
		strings.Contains(baseDiskLower, "debian") ||
		strings.Contains(baseDiskLower, "fedora") ||
		strings.Contains(baseDiskLower, "rocky") ||
		strings.Contains(baseDiskLower, "alma") ||
		strings.Contains(baseDiskLower, "centos") {
		return "systemd"
	}

	cmd := exec.CommandContext(ctx, "tar", "-tf", baseDisk, "usr/lib/systemd/systemd", "lib/systemd/systemd", "sbin/openrc-init", "usr/sbin/openrc-init")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run()

	output := stdout.String()
	if strings.Contains(output, "systemd/systemd") {
		return "systemd"
	}
	if strings.Contains(output, "openrc-init") {
		return "openrc"
	}

	return "systemd"
}
