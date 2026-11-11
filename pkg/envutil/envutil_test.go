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

		blockList := getBlockList(nil)
		allowList := getAllowList(nil)

		assert.Assert(t, isUsingDefaultBlockList())
		assert.DeepEqual(t, blockList, defaultBlockList)
		assert.Equal(t, len(allowList), 0)
	})

	t.Run("custom blocklist", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_BLOCK", "PATH,HOME")

		blockList := getBlockList(nil)
		assert.Assert(t, !isUsingDefaultBlockList())
		expected := []string{"PATH", "HOME"}
		assert.DeepEqual(t, blockList, expected)
	})

	t.Run("additive blocklist", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_BLOCK", "+CUSTOM_VAR")

		blockList := getBlockList(nil)
		assert.Assert(t, isUsingDefaultBlockList())
		expected := slices.Concat(GetDefaultBlockList(), []string{"CUSTOM_VAR"})
		assert.DeepEqual(t, blockList, expected)
	})

	t.Run("allowlist", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_ALLOW", "FOO,BAR")

		allowList := getAllowList(nil)
		expected := []string{"FOO", "BAR"}
		assert.DeepEqual(t, allowList, expected)
	})

	t.Run("extra block patterns extend the default blocklist", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_BLOCK", "")

		blockList := getBlockList([]string{"PRIVATE*"})
		expected := slices.Concat(GetDefaultBlockList(), []string{"PRIVATE*"})
		assert.DeepEqual(t, blockList, expected)
	})

	t.Run("extra block patterns extend a custom blocklist", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_BLOCK", "PATH,HOME")

		blockList := getBlockList([]string{"PRIVATE*"})
		assert.DeepEqual(t, blockList, []string{"PATH", "HOME", "PRIVATE*"})
	})

	t.Run("extra block patterns extend an additive blocklist", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_BLOCK", "+CUSTOM_VAR")

		blockList := getBlockList([]string{"PRIVATE*"})
		expected := slices.Concat(GetDefaultBlockList(), []string{"CUSTOM_VAR", "PRIVATE*"})
		assert.DeepEqual(t, blockList, expected)
	})

	t.Run("extra allow patterns without LIMA_SHELLENV_ALLOW", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_ALLOW", "")

		allowList := getAllowList([]string{"GITHUB_TOKEN"})
		assert.DeepEqual(t, allowList, []string{"GITHUB_TOKEN"})
	})

	t.Run("extra allow patterns extend LIMA_SHELLENV_ALLOW", func(t *testing.T) {
		t.Setenv("LIMA_SHELLENV_ALLOW", "FOO,BAR")

		allowList := getAllowList([]string{"GITHUB_TOKEN"})
		assert.DeepEqual(t, allowList, []string{"FOO", "BAR", "GITHUB_TOKEN"})
	})
}

func TestValidatePatterns(t *testing.T) {
	assert.NilError(t, ValidatePatterns(nil))
	assert.NilError(t, ValidatePatterns([]string{"GITHUB_TOKEN", "PRIVATE*"}))
	assert.ErrorContains(t, ValidatePatterns([]string{"FOO", "BAR-BAZ"}), "invalid character")
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

	t.Run("allowlist without blocklist passes everything", func(t *testing.T) {
		result := filterEnvironmentWithLists(testEnv, []string{"FOO", "USER"}, nil)

		// The allow list only exempts variables from the block list.
		// With no block list, nothing is blocked, so all variables pass through.
		assert.Equal(t, len(result), len(testEnv))
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

	expectedItems := []string{"PATH", "HOME", "SSH_*", "USER"}
	for _, item := range expectedItems {
		found := slices.Contains(blocklist, item)
		assert.Assert(t, found, "Expected builtin blocklist to contain %#q", item)
	}
}

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		valid   bool
	}{
		{"simple alphanumeric uppercase", "FOO", true},
		{"simple alphanumeric lowercase", "foo", true},
		{"mixed case", "FooBar", true},
		{"with underscore", "FOO_BAR", true},
		{"with numbers", "VAR123", true},
		{"with trailing asterisk", "FOO*", true},
		{"with multiple asterisks", "FOO*BAR*", true},
		{"asterisk at beginning", "*FOO", true},
		{"asterisk in middle", "FOO*BAR", true},
		{"only asterisk", "*", true},
		{"with dash", "FOO-BAR", false},
		{"with space", "FOO BAR", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePattern(tt.pattern)
			if tt.valid {
				assert.NilError(t, err, "Expected pattern %#q to be valid", tt.pattern)
			} else {
				assert.Assert(t, err != nil, "Expected pattern %#q to be invalid", tt.pattern)
			}
		})
	}
}

func TestValidatePatternErrorMessage(t *testing.T) {
	err := validatePattern("FOO-BAR")
	assert.Assert(t, err != nil, "Expected pattern with dash to be invalid")
	expectedMsg := "pattern `FOO-BAR` contains invalid character `-` at position 3"
	assert.Equal(t, err.Error(), expectedMsg)
}
