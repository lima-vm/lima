// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package editflags

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/localpathutil"
)

func TestCompleteCPUs(t *testing.T) {
	assert.DeepEqual(t, []int{1}, completeCPUs(1))
	assert.DeepEqual(t, []int{1, 2}, completeCPUs(2))
	assert.DeepEqual(t, []int{1, 2, 4, 8}, completeCPUs(8))
	assert.DeepEqual(t, []int{1, 2, 4, 8, 16, 20}, completeCPUs(20))
}

func TestCompleteMemoryGiB(t *testing.T) {
	assert.DeepEqual(t, []float32{0.5}, completeMemoryGiB(1<<30))
	assert.DeepEqual(t, []float32{1}, completeMemoryGiB(2<<30))
	assert.DeepEqual(t, []float32{1, 2}, completeMemoryGiB(4<<30))
	assert.DeepEqual(t, []float32{1, 2, 4}, completeMemoryGiB(8<<30))
	assert.DeepEqual(t, []float32{1, 2, 4, 8, 10}, completeMemoryGiB(20<<30))
}

func TestBuildPortForwardExpression(t *testing.T) {
	tests := []struct {
		name         string
		portForwards []string
		expected     string
		expectError  bool
	}{
		{
			name:         "empty port forwards",
			portForwards: []string{},
			expected:     "",
		},
		{
			name:         "single dynamic port forward",
			portForwards: []string{"8080:80"},
			expected:     `.portForwards += [{"guestPort": "80", "hostPort": "8080", "static": false}]`,
		},
		{
			name:         "single static port forward",
			portForwards: []string{"8080:80,static=true"},
			expected:     `.portForwards += [{"guestPort": "80", "hostPort": "8080", "static": true}]`,
		},
		{
			name:         "multiple mixed port forwards",
			portForwards: []string{"8080:80", "2222:22,static=true", "3000:3000"},
			expected:     `.portForwards += [{"guestPort": "80", "hostPort": "8080", "static": false},{"guestPort": "22", "hostPort": "2222", "static": true},{"guestPort": "3000", "hostPort": "3000", "static": false}]`,
		},
		{
			name:         "invalid format - missing colon",
			portForwards: []string{"8080"},
			expectError:  true,
		},
		{
			name:         "invalid format - too many colons",
			portForwards: []string{"8080:80:extra"},
			expectError:  true,
		},
		{
			name:         "invalid static parameter",
			portForwards: []string{"8080:80,invalid=true"},
			expectError:  true,
		},
		{
			name:         "too many parameters",
			portForwards: []string{"8080:80,static=true,extra=value"},
			expectError:  true,
		},
		{
			name:         "whitespace handling",
			portForwards: []string{" 8080 : 80 , static=true "},
			expected:     `.portForwards += [{"guestPort": "80", "hostPort": "8080", "static": true}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildPortForwardExpression(tt.portForwards)
			if tt.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParsePortForward(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		hostPort    string
		guestPort   string
		isStatic    bool
		expectError bool
	}{
		{
			name:      "dynamic port forward",
			spec:      "8080:80",
			hostPort:  "8080",
			guestPort: "80",
			isStatic:  false,
		},
		{
			name:      "static port forward",
			spec:      "8080:80,static=true",
			hostPort:  "8080",
			guestPort: "80",
			isStatic:  true,
		},
		{
			name:      "whitespace handling",
			spec:      " 8080 : 80 , static=true ",
			hostPort:  "8080",
			guestPort: "80",
			isStatic:  true,
		},
		{
			name:        "invalid format - missing colon",
			spec:        "8080",
			expectError: true,
		},
		{
			name:        "invalid format - too many colons",
			spec:        "8080:80:extra",
			expectError: true,
		},
		{
			name:        "invalid parameter",
			spec:        "8080:80,invalid=true",
			expectError: true,
		},
		{
			name:        "too many parameters",
			spec:        "8080:80,static=true,extra=value",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostPort, guestPort, isStatic, err := ParsePortForward(tt.spec)
			if tt.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, tt.hostPort, hostPort)
				assert.Equal(t, tt.guestPort, guestPort)
				assert.Equal(t, tt.isStatic, isStatic)
			}
		})
	}
}

func TestYQExpressions(t *testing.T) {
	expand := func(s string) string {
		s, err := localpathutil.Expand(s)
		assert.NilError(t, err)
		// `D:\foo` -> `D:\\foo` (appears in YAML)
		s = strings.ReplaceAll(s, "\\", "\\\\")
		return s
	}
	tests := []struct {
		name        string
		args        []string
		newInstance bool
		expected    []string
		expectError string
	}{
		{
			name:        "mount",
			args:        []string{"--mount", "/foo", "--mount", "./bar:w"},
			newInstance: false,
			expected:    []string{`.mounts += [{"location": "` + expand("/foo") + `", "writable": false},{"location": "` + expand("./bar") + `", "writable": true}] | .mounts |= unique_by(.location)`},
		},
		{
			name:        "mount-only",
			args:        []string{"--mount-only", "/foo", "--mount-only", "/bar:w"},
			newInstance: false,
			expected:    []string{`.mounts = [{"location": "` + expand("/foo") + `", "writable": false},{"location": "` + expand("/bar") + `", "writable": true}]`},
		},
		{
			name:        "mixture of mount and mount-only",
			args:        []string{"--mount", "/foo", "--mount-only", "/bar:w"},
			newInstance: false,
			expectError: "flag `--mount` conflicts with `--mount-only`",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			RegisterEdit(cmd, "")
			assert.NilError(t, cmd.ParseFlags(tt.args))
			expr, err := YQExpressions(cmd.Flags(), tt.newInstance)
			if tt.expectError != "" {
				assert.ErrorContains(t, err, tt.expectError)
			} else {
				assert.NilError(t, err)
				assert.DeepEqual(t, tt.expected, expr)
			}
		})
	}
}
