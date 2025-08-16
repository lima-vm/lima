// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package envutil

import (
	"os"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"
)

// defaultBlockList contains environment variables that should not be propagated by default.
var defaultBlockList = []string{
	"BASH*",
	"DISPLAY",
	"DYLD_*",
	"EUID",
	"FPATH",
	"GID",
	"GROUP",
	"HOME",
	"HOSTNAME",
	"LD_*",
	"LOGNAME",
	"OLDPWD",
	"PATH",
	"PWD",
	"SHELL",
	"SHLVL",
	"SSH_*",
	"TERM",
	"TERMINFO",
	"TMPDIR",
	"UID",
	"USER",
	"XAUTHORITY",
	"XDG_*",
	"ZDOTDIR",
	"ZSH*",
	"_*", // Variables starting with underscore are typically internal
}

func getBlockList() []string {
	blockEnv := os.Getenv("LIMA_SHELLENV_BLOCK")
	if blockEnv == "" {
		return defaultBlockList
	}
	after, found := strings.CutPrefix(blockEnv, "+")
	if !found {
		return parseEnvList(blockEnv)
	}
	return slices.Concat(defaultBlockList, parseEnvList(after))
}

func getAllowList() []string {
	if allowEnv := os.Getenv("LIMA_SHELLENV_ALLOW"); allowEnv != "" {
		return parseEnvList(allowEnv)
	}
	return nil
}

func parseEnvList(envList string) []string {
	parts := strings.Split(envList, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func matchesPattern(name, pattern string) bool {
	if pattern == name {
		return true
	}

	prefix, found := strings.CutSuffix(pattern, "*")
	return found && strings.HasPrefix(name, prefix)
}

func matchesAnyPattern(name string, patterns []string) bool {
	return slices.ContainsFunc(patterns, func(pattern string) bool {
		return matchesPattern(name, pattern)
	})
}

// FilterEnvironment filters environment variables based on configuration from environment variables.
// It returns a slice of environment variables that are not blocked by the current configuration.
// The filtering is controlled by LIMA_SHELLENV_BLOCK and LIMA_SHELLENV_ALLOW environment variables.
func FilterEnvironment() []string {
	return filterEnvironmentWithLists(
		os.Environ(),
		getAllowList(),
		getBlockList(),
	)
}

func filterEnvironmentWithLists(env, allowList, blockList []string) []string {
	var filtered []string

	for _, envVar := range env {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			continue
		}

		name := parts[0]

		if len(allowList) > 0 {
			if !matchesAnyPattern(name, allowList) {
				continue
			}
			filtered = append(filtered, envVar)
			continue
		}

		if matchesAnyPattern(name, blockList) {
			logrus.Debugf("Blocked env variable %q", name)
			continue
		}

		filtered = append(filtered, envVar)
	}

	return filtered
}

// GetDefaultBlockList returns a copy of the default block list.
func GetDefaultBlockList() []string {
	return slices.Clone(defaultBlockList)
}
