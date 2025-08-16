// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package envutil

import (
	"os"
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func isUsingDefaultBlockList() bool {
	blockEnv := os.Getenv("LIMA_SHELLENV_BLOCK")
	return blockEnv == "" || strings.HasPrefix(blockEnv, "+")
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"PATH", "PATH", true},
		{"PATH", "HOME", false},
		{"SSH_AUTH_SOCK", "SSH_*", true},
		{"SSH_AGENT_PID", "SSH_*", true},
		{"HOME", "SSH_*", false},
		{"XDG_CONFIG_HOME", "XDG_*", true},
		{"_LIMA_TEST", "_*", true},
		{"LIMA_HOME", "_*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_matches_"+tt.pattern, func(t *testing.T) {
			result := matchesPattern(tt.name, tt.pattern)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestMatchesAnyPattern(t *testing.T) {
	patterns := []string{"PATH", "SSH_*", "XDG_*"}

	tests := []struct {
		name     string
		expected bool
	}{
		{"PATH", true},
		{"HOME", false},
		{"SSH_AUTH_SOCK", true},
		{"XDG_CONFIG_HOME", true},
		{"USER", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesAnyPattern(tt.name, patterns)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestParseEnvList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"PATH", []string{"PATH"}},
		{"PATH,HOME", []string{"PATH", "HOME"}},
		{"PATH, HOME , USER", []string{"PATH", "HOME", "USER"}},
		{" , , ", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseEnvList(tt.input)
			assert.DeepEqual(t, result, tt.expected)
		})
	}
}

func TestGetBlockAndAllowLists(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_BLOCK", "")
		t.Setenv("LIMA_SHELLENV_ALLOW", "")

		blockList := getBlockList()
		allowList := getAllowList()

		assert.Assert(t, isUsingDefaultBlockList())
		assert.DeepEqual(t, blockList, defaultBlockList)
		assert.Equal(t, len(allowList), 0)
	})

	t.Run("custom blocklist", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_BLOCK", "PATH,HOME")

		blockList := getBlockList()
		assert.Assert(t, !isUsingDefaultBlockList())
		expected := []string{"PATH", "HOME"}
		assert.DeepEqual(t, blockList, expected)
	})

	t.Run("additive blocklist", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_BLOCK", "+CUSTOM_VAR")

		blockList := getBlockList()
		assert.Assert(t, isUsingDefaultBlockList())
		expected := slices.Concat(GetDefaultBlockList(), []string{"CUSTOM_VAR"})
		assert.DeepEqual(t, blockList, expected)
	})

	t.Run("allowlist", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_ALLOW", "FOO,BAR")

		allowList := getAllowList()
		expected := []string{"FOO", "BAR"}
		assert.DeepEqual(t, allowList, expected)
	})
}

func TestFilterEnvironment(t *testing.T) {
	testEnv := []string{
		"PATH=/usr/bin",
		"HOME=/home/user",
		"USER=testuser",
		"FOO=bar",
		"SSH_AUTH_SOCK=/tmp/ssh",
		"XDG_CONFIG_HOME=/config",
		"BASH_VERSION=5.0",
		"_INTERNAL=secret",
		"CUSTOM_VAR=value",
	}

	t.Run("default blocklist", func(t *testing.T) {
		result := filterEnvironmentWithLists(testEnv, nil, defaultBlockList)

		expected := []string{"FOO=bar", "CUSTOM_VAR=value"}
		assert.Assert(t, containsAll(result, expected))

		blockedPrefixes := []string{
			"PATH=",
			"HOME=",
			"SSH_AUTH_SOCK=",
			"XDG_CONFIG_HOME=",
			"BASH_VERSION=",
			"_INTERNAL=",
		}
		for _, prefix := range blockedPrefixes {
			for _, envVar := range result {
				assert.Assert(t, !strings.HasPrefix(envVar, prefix), "Expected result to not contain variable with prefix %q, but found %q", prefix, envVar)
			}
		}
	})

	t.Run("custom blocklist", func(t *testing.T) {
		result := filterEnvironmentWithLists(testEnv, nil, []string{"FOO"})

		assert.Assert(t, !slices.Contains(result, "FOO=bar"))

		expected := []string{"PATH=/usr/bin", "HOME=/home/user", "USER=testuser"}
		assert.Assert(t, containsAll(result, expected))
	})

	t.Run("allowlist", func(t *testing.T) {
		result := filterEnvironmentWithLists(testEnv, []string{"FOO", "USER"}, nil)

		expected := []string{"FOO=bar", "USER=testuser"}
		assert.Equal(t, len(result), len(expected))
		assert.Assert(t, containsAll(result, expected))
	})

	t.Run("allowlist takes precedence over blocklist", func(t *testing.T) {
		result := filterEnvironmentWithLists(testEnv, []string{"FOO", "CUSTOM_VAR"}, []string{"FOO", "USER"})

		expected := []string{"FOO=bar", "CUSTOM_VAR=value"}
		assert.Assert(t, containsAll(result, expected))

		assert.Assert(t, !slices.Contains(result, "USER=testuser"))
	})
}

func containsAll(slice, items []string) bool {
	for _, item := range items {
		if !slices.Contains(slice, item) {
			return false
		}
	}
	return true
}

func TestGetDefaultBlockList(t *testing.T) {
	blocklist := GetDefaultBlockList()

	if &blocklist[0] == &defaultBlockList[0] {
		t.Error("GetDefaultBlockList should return a copy, not the original slice")
	}

	assert.DeepEqual(t, blocklist, defaultBlockList)

	expectedItems := []string{"PATH", "HOME", "SSH_*"}
	for _, item := range expectedItems {
		found := slices.Contains(blocklist, item)
		assert.Assert(t, found, "Expected builtin blocklist to contain %q", item)
	}
}
