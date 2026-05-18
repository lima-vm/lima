// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"reflect"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

var (
	setFlagProperties     []string
	setFlagPropertiesOnce sync.Once
)

func getSetFlagProperties() []string {
	setFlagPropertiesOnce.Do(func() {
		setFlagProperties = extractYAMLPaths(reflect.TypeFor[limatype.LimaYAML](), "", 0)
	})
	return setFlagProperties
}

// extractYAMLPaths recursively traverses a struct, reading `yaml` tags
// Slice and map element types are not traversed; array-index paths like
// .mounts[0].mountPoint must be typed manually by the user.
func extractYAMLPaths(t reflect.Type, prefix string, depth int) []string {
	var paths []string

	// Limit recursion to top-level and one level deep to avoid autocomplete noise
	if depth > 1 {
		return paths
	}

	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return paths
	}

	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "-" {
			continue
		}

		name, _, _ := strings.Cut(yamlTag, ",")
		if name == "" && strings.Contains(yamlTag, "inline") {
			paths = append(paths, extractYAMLPaths(field.Type, prefix, depth)...)
			continue
		}
		if name == "" {
			continue
		}

		currentPath := prefix + "." + name
		paths = append(paths, currentPath)

		// Recurse into nested structs to get deep paths like .ssh.localPort
		ft := field.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			paths = append(paths, extractYAMLPaths(ft, currentPath, depth+1)...)
		}
	}
	return paths
}

var setFlagEnumValues = map[string][]string{
	".vmType": {
		limatype.QEMU, limatype.VZ, limatype.WSL2,
	},
	".os": {
		limatype.LINUX, limatype.DARWIN, limatype.FREEBSD,
	},
	".arch": {
		limatype.X8664, limatype.AARCH64, limatype.ARMV7L,
		limatype.PPC64LE, limatype.RISCV64, limatype.S390X,
	},
	".mountType": {
		limatype.REVSSHFS, limatype.NINEP,
		limatype.VIRTIOFS, limatype.WSLMount,
	},
	".mountInotify":         {"true", "false"},
	".upgradePackages":      {"true", "false"},
	".propagateProxyEnv":    {"true", "false"},
	".plain":                {"true", "false"},
	".nestedVirtualization": {"true", "false"},
	".rosetta.enabled":      {"true", "false"},
	".rosetta.binfmt":       {"true", "false"},
	".containerd.system":    {"true", "false"},
	".containerd.user":      {"true", "false"},
	".hostResolver.enabled": {"true", "false"},
	".hostResolver.ipv6":    {"true", "false"},
}

func setFlagCompletion(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	noFile := cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp

	if prop, valuePrefix, found := strings.Cut(toComplete, "="); found {
		if values, ok := setFlagEnumValues[prop]; ok {
			var completions []string
			for _, v := range values {
				if strings.HasPrefix(v, valuePrefix) {
					completions = append(completions, prop+"="+v)
				}
			}
			return completions, noFile
		}
		return nil, noFile
	}

	var completions []string
	for _, p := range getSetFlagProperties() {
		if strings.HasPrefix(p, toComplete) {
			completions = append(completions, p+"=")
		}
	}
	return completions, noFile
}
