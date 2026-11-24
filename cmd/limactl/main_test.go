// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func TestIsAllowedUnderRosetta(t *testing.T) {
	app := newApp()
	app.InitDefaultCompletionCmd()

	// Root command itself has Parent() == nil and must not panic, returning false.
	assert.Equal(t, isAllowedUnderRosetta(app), false, "root command should not be allowed under rosetta")

	// Subcommands for packaging and docs must be allowed.
	genDocCmd, _, err := app.Find([]string{"generate-doc"})
	assert.NilError(t, err)
	assert.Equal(t, isAllowedUnderRosetta(genDocCmd), true, "generate-doc should be allowed under rosetta")

	validateCmd, _, err := app.Find([]string{"validate"})
	assert.NilError(t, err)
	assert.Equal(t, isAllowedUnderRosetta(validateCmd), true, "validate should be allowed under rosetta")

	completionCmd, _, err := app.Find([]string{"completion"})
	assert.NilError(t, err)
	assert.Equal(t, isAllowedUnderRosetta(completionCmd), true, "completion should be allowed under rosetta")

	completionBashCmd, _, err := app.Find([]string{"completion", "bash"})
	assert.NilError(t, err)
	assert.Equal(t, isAllowedUnderRosetta(completionBashCmd), true, "completion bash should be allowed under rosetta")

	// Standard commands must not be allowed under rosetta.
	startCmd, _, err := app.Find([]string{"start"})
	assert.NilError(t, err)
	assert.Equal(t, isAllowedUnderRosetta(startCmd), false, "start should not be allowed under rosetta")

	// Standalone dummy command with nil parent must not panic.
	dummy := &cobra.Command{Use: "custom"}
	assert.Equal(t, isAllowedUnderRosetta(dummy), false)
}

func TestRootPersistentPreRunNoPanic(t *testing.T) {
	app := newApp()
	app.InitDefaultCompletionCmd()
	t.Setenv("LIMA_HOME", t.TempDir())
	assert.NilError(t, app.ParseFlags([]string{}))

	// Live environment check: must not panic on rootCmd whether running under Rosetta or natively.
	err := app.PersistentPreRunE(app, nil)
	if osutil.IsBeingRosettaTranslated() {
		assert.ErrorContains(t, err, "limactl is running under rosetta")
	} else {
		assert.NilError(t, err)
	}

	orig := isBeingRosettaTranslated
	t.Cleanup(func() {
		isBeingRosettaTranslated = orig
	})

	isBeingRosettaTranslated = func() bool { return true }
	// Root command has Parent() == nil and must return the Rosetta error without panicking.
	assert.ErrorContains(t, app.PersistentPreRunE(app, nil), "limactl is running under rosetta")

	startCmd, _, err := app.Find([]string{"start"})
	assert.NilError(t, err)
	assert.ErrorContains(t, app.PersistentPreRunE(startCmd, nil), "limactl is running under rosetta")

	for _, args := range [][]string{{"generate-doc"}, {"validate"}, {"completion"}, {"completion", "bash"}} {
		subCmd, _, err := app.Find(args)
		assert.NilError(t, err)
		assert.NilError(t, app.PersistentPreRunE(subCmd, nil))
	}

	isBeingRosettaTranslated = func() bool { return false }
	assert.NilError(t, app.PersistentPreRunE(app, nil))
}
