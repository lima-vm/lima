// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package editflags

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

func completeParams(t *testing.T, toComplete string, args ...string) []string {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	comp, directive := paramFlagCompletion(cmd, args, toComplete)
	assert.Equal(t, directive, cobra.ShellCompDirectiveNoSpace|cobra.ShellCompDirectiveNoFileComp)
	return comp
}

func TestParamFlagCompletionTemplate(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "base.yaml"), []byte("param:\n  BASE: b\n"), 0o644))
	path := filepath.Join(dir, "tmpl.yaml")
	assert.NilError(t, os.WriteFile(path, []byte("base: base.yaml\nparam:\n  ZETA: z\n  ALPHA: a\n"), 0o644))

	assert.DeepEqual(t, completeParams(t, "", path), []string{"ALPHA=", "BASE=", "ZETA="})
	assert.DeepEqual(t, completeParams(t, "B", path), []string{"BASE="})
}

func TestParamFlagCompletionInstance(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)
	templatesDir := t.TempDir()
	t.Setenv("LIMA_TEMPLATES_PATH", templatesDir)
	assert.NilError(t, os.WriteFile(filepath.Join(templatesDir, "default.yaml"), []byte("param:\n  DEFAULT: d\n"), 0o644))
	writeInstance := func(name, limaYAML string) {
		instDir := filepath.Join(limaHome, name)
		assert.NilError(t, os.Mkdir(instDir, 0o755))
		assert.NilError(t, os.WriteFile(filepath.Join(instDir, "lima.yaml"), []byte(limaYAML), 0o644))
	}
	// store.Inspect() validates the config, which requires an image and every param to be used.
	writeInstance("valid", "images:\n- location: /dev/null\nparam:\n  VALID: v\nprovision:\n- script: echo {{.Param.VALID}}\n")
	writeInstance("invalid", "param:\n  INVALID: i\n")

	assert.DeepEqual(t, completeParams(t, "", "valid"), []string{"VALID="})
	assert.DeepEqual(t, completeParams(t, "", "invalid"), []string{"INVALID="})
	// No instance named "default" exists, so the default template declares the params.
	assert.DeepEqual(t, completeParams(t, ""), []string{"DEFAULT="})

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.Flags().String("name", "", "")
	assert.NilError(t, cmd.Flags().Set("name", "valid"))
	comp, _ := paramFlagCompletion(cmd, nil, "")
	assert.DeepEqual(t, comp, []string{"VALID="})
}

func TestParamFlagCompletionStdin(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "stdin.yaml")
	assert.NilError(t, os.WriteFile(path, []byte("param:\n  STDIN: s\n"), 0o644))
	stdin, err := os.Open(path)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = stdin.Close() })
	origStdin := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() { os.Stdin = origStdin })

	assert.Assert(t, completeParams(t, "", "-") == nil)
}

func TestParamFlagCompletionStalledServer(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	// The kernel completes the TCP handshake, but nothing accepts the connection, so the HTTP request never
	// gets a response.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	origTimeout := paramCompletionTimeout
	paramCompletionTimeout = 100 * time.Millisecond
	t.Cleanup(func() { paramCompletionTimeout = origTimeout })

	done := make(chan []string, 1)
	go func() {
		cmd := &cobra.Command{}
		cmd.SetContext(t.Context())
		comp, _ := paramFlagCompletion(cmd, []string{"http://" + ln.Addr().String() + "/lima.yaml"}, "")
		done <- comp
	}()
	select {
	case comp := <-done:
		assert.Assert(t, comp == nil)
	case <-time.After(10 * time.Second):
		assert.Assert(t, false, "completion still waits for a server that never answers")
	}
}
