// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package envutil

import (
	"fmt"
	"os"
	"regexp"
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

func validatePattern(pattern string) error {
	invalidChar := regexp.MustCompile(`([^a-zA-Z0-9_*])`)
	if matches := invalidChar.FindStringSubmatch(pattern); matches != nil {
		invalidCharacter := matches[1]
		pos := strings.Index(pattern, invalidCharacter)
		return fmt.Errorf("pattern %#q contains invalid character %#q at position %d",
			pattern, invalidCharacter, pos)
	}
	return nil
}

// getBlockList returns the list of environment variable patterns to be blocked.
// The extra patterns are appended to the list built from LIMA_SHELLENV_BLOCK,
// or to the default block list when LIMA_SHELLENV_BLOCK is empty.
func getBlockList(extra []string) []string {
	blockEnv := os.Getenv("LIMA_SHELLENV_BLOCK")
	if blockEnv == "" {
		return slices.Concat(defaultBlockList, extra)
	}

	shouldAppend := strings.HasPrefix(blockEnv, "+")
	patterns := parseEnvList(strings.TrimPrefix(blockEnv, "+"))

	for _, pattern := range patterns {
		if err := validatePattern(pattern); err != nil {
			logrus.Fatalf("Invalid LIMA_SHELLENV_BLOCK pattern: %v", err)
		}
	}

	if shouldAppend {
		return slices.Concat(defaultBlockList, patterns, extra)
	}
	return slices.Concat(patterns, extra)
}

// getAllowList returns the list of environment variable patterns to be allowed.
// The extra patterns are appended to the list built from LIMA_SHELLENV_ALLOW.
func getAllowList(extra []string) []string {
	allowEnv := os.Getenv("LIMA_SHELLENV_ALLOW")
	if allowEnv == "" {
		if len(extra) == 0 {
			return nil
		}
		return slices.Clone(extra)
	}

	patterns := parseEnvList(allowEnv)

	for _, pattern := range patterns {
		if err := validatePattern(pattern); err != nil {
			logrus.Fatalf("Invalid LIMA_SHELLENV_ALLOW pattern: %v", err)
		}
	}

	return slices.Concat(patterns, extra)
}

// ValidatePatterns returns an error if any of the patterns contains characters
// other than letters, digits, underscores and asterisks.
func ValidatePatterns(patterns []string) error {
	for _, pattern := range patterns {
		if err := validatePattern(pattern); err != nil {
			return err
		}
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

	regexPattern := strings.ReplaceAll(pattern, "*", ".*")
	regexPattern = "^" + regexPattern + "$"

	match, err := regexp.MatchString(regexPattern, name)
	if err != nil {
		return false
	}
	return match
}

func matchesAnyPattern(name string, patterns []string) bool {
	return slices.ContainsFunc(patterns, func(pattern string) bool {
		return matchesPattern(name, pattern)
	})
}

// FilterEnvironment filters environment variables based on configuration from environment variables.
// It returns a slice of environment variables that are not blocked by the current configuration.
// The filtering is controlled by LIMA_SHELLENV_BLOCK and LIMA_SHELLENV_ALLOW environment variables,
// extended with the extraAllow and extraBlock patterns.
func FilterEnvironment(extraAllow, extraBlock []string) []string {
	return filterEnvironmentWithLists(
		os.Environ(),
		getAllowList(extraAllow),
		getBlockList(extraBlock),
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

		if len(allowList) > 0 && matchesAnyPattern(name, allowList) {
			filtered = append(filtered, envVar)
			continue
		}

		if matchesAnyPattern(name, blockList) {
			logrus.Debugf("Blocked env variable %#q", name)
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
