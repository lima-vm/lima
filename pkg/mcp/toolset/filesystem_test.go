// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/pkg/sftp"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/mcp/msi"
)

func TestReplaceExact(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		oldString string
		newString string
		expected  int
		want      string
		wantErr   string
	}{
		{
			name:      "single occurrence",
			content:   "foo\nbar\nbaz\n",
			oldString: "bar",
			newString: "BAR",
			expected:  1,
			want:      "foo\nBAR\nbaz\n",
		},
		{
			name:      "multiple occurrences",
			content:   "foo\nbar\nfoo\n",
			oldString: "foo",
			newString: "FOO",
			expected:  2,
			want:      "FOO\nbar\nFOO\n",
		},
		{
			name:      "multi-line old_string",
			content:   "a\nb\nc\nb\n",
			oldString: "b\nc\n",
			newString: "",
			expected:  1,
			want:      "a\nb\n",
		},
		{
			name:      "ambiguous",
			content:   "foo\nbar\nfoo\n",
			oldString: "foo",
			newString: "FOO",
			expected:  1,
			wantErr:   "expected 1 occurrence(s) of old_string, found 2",
		},
		{
			name:      "fewer occurrences than expected",
			content:   "foo\nbar\n",
			oldString: "foo",
			newString: "FOO",
			expected:  2,
			wantErr:   "expected 2 occurrence(s) of old_string, found 1",
		},
		{
			name:      "not found",
			content:   "foo\n",
			oldString: "bar",
			newString: "BAR",
			expected:  1,
			wantErr:   "old_string not found",
		},
		{
			name:      "empty old_string",
			content:   "foo\n",
			oldString: "",
			newString: "bar",
			expected:  1,
			wantErr:   "old_string must not be empty",
		},
		{
			name:      "identical strings",
			content:   "foo\n",
			oldString: "foo",
			newString: "foo",
			expected:  1,
			wantErr:   "old_string and new_string are identical",
		},
		{
			name:      "invalid expected_replacements",
			content:   "foo\n",
			oldString: "foo",
			newString: "bar",
			expected:  0,
			wantErr:   "expected_replacements must be at least 1, got 0",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := replaceExact(tc.content, tc.oldString, tc.newString, tc.expected)
			if tc.wantErr != "" {
				assert.Error(t, err, tc.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestReplaceExactResultSize(t *testing.T) {
	t.Run("many replacements", func(t *testing.T) {
		content := strings.Repeat("a", 1<<20)
		_, err := replaceExact(content, "a", strings.Repeat("b", 1<<20), 1<<20)
		assert.Error(t, err, fmt.Sprintf("the result would be larger than %d bytes", readLimitBytes))
	})
	t.Run("at the limit", func(t *testing.T) {
		got, err := replaceExact("aa", "a", strings.Repeat("b", readLimitBytes/2), 2)
		assert.NilError(t, err)
		assert.Equal(t, len(got), readLimitBytes)
	})
	t.Run("over the limit", func(t *testing.T) {
		_, err := replaceExact("aa", "a", strings.Repeat("b", readLimitBytes/2+1), 2)
		assert.Error(t, err, fmt.Sprintf("the result would be larger than %d bytes", readLimitBytes))
	})
}

// newTestToolSet returns a ToolSet whose SFTP client talks to an in-process
// SFTP server for the local file system.
func newTestToolSet(t *testing.T) *ToolSet {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	server, err := sftp.NewServer(serverConn)
	assert.NilError(t, err)
	go func() { _ = server.Serve() }()
	client, err := sftp.NewClientPipe(clientConn, clientConn)
	assert.NilError(t, err)
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return &ToolSet{inst: &limatype.Instance{}, sftp: client}
}

func TestReplaceConcurrent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the in-process SFTP server is only tested with Unix paths")
	}
	ts := newTestToolSet(t)
	const n = 20
	var lines, want []string
	for i := range n {
		lines = append(lines, fmt.Sprintf("line %d\n", i))
		want = append(want, fmt.Sprintf("LINE %d\n", i))
	}
	path := filepath.Join(t.TempDir(), "file")
	assert.NilError(t, os.WriteFile(path, []byte(strings.Join(lines, "")), 0o644))

	// The MCP server handles calls concurrently, so no Replace may undo another one.
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			_, _, errs[i] = ts.Replace(t.Context(), nil, msi.ReplaceParams{
				Path:      path,
				OldString: lines[i],
				NewString: want[i],
			})
		})
	}
	wg.Wait()
	for _, err := range errs {
		assert.NilError(t, err)
	}
	b, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Equal(t, string(b), strings.Join(want, ""))
}

func TestReplaceRefusesOversizedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the in-process SFTP server is only tested with Unix paths")
	}
	ts := newTestToolSet(t)
	path := filepath.Join(t.TempDir(), "file")
	orig := "foo\n" + strings.Repeat("x", readLimitBytes)
	assert.NilError(t, os.WriteFile(path, []byte(orig), 0o644))

	_, _, err := ts.Replace(t.Context(), nil,
		msi.ReplaceParams{Path: path, OldString: "foo", NewString: "f"})
	b, readErr := os.ReadFile(path)
	assert.NilError(t, readErr)
	assert.Equal(t, len(b), len(orig), "Replace changed the file")
	assert.Error(t, err, fmt.Sprintf("file is larger than %d bytes", readLimitBytes))
}
