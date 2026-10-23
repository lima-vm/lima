// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package editflags

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/limatmpl"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/templatestore"
)

// defaultInstanceName matches DefaultInstanceName in package main, which editflags cannot import.
const defaultInstanceName = "default"

// paramCompletionTimeout limits how long completion waits for the declared params. Completion downloads remote
// templates, bases and scripts again on every request, so a server that never answers would otherwise hang the shell.
var paramCompletionTimeout = 3 * time.Second

// paramFlagCompletion completes `--param` with the names of the params declared
// by the instance or template that the command operates on.
func paramFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	arg := defaultInstanceName
	if len(args) > 0 {
		arg = args[0]
	} else if name, _ := cmd.Flags().GetString("name"); name != "" { // Only create and start have a --name flag.
		arg = name
	}
	ctx := cmd.Context()
	if ctx == nil {
		// Cobra sets the context only when it runs the command; limactl-menu calls completers without running it.
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, paramCompletionTimeout)
	defer cancel()
	var comp []string
	for _, name := range slices.Sorted(maps.Keys(declaredParams(ctx, arg))) {
		if strings.HasPrefix(name, toComplete) {
			comp = append(comp, name+"=")
		}
	}
	return comp, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}

// declaredParams returns the params declared by the instance or template arg, or nil on any error.
func declaredParams(ctx context.Context, arg string) map[string]string {
	locator := arg
	if dirnames.ValidateInstName(arg) == nil {
		inst, err := store.Inspect(ctx, arg)
		switch {
		case err == nil && inst.Config != nil:
			return inst.Config.Param
		case err == nil:
			// Inspect leaves Config nil when lima.yaml fails to load. `limactl edit` still reads the params
			// from the file itself, so completion reads the file too.
			locator = filepath.Join(inst.Dir, filenames.LimaYAML)
		case errors.Is(err, os.ErrNotExist):
			// A new instance is created from the default template.
			locator = "template:" + templatestore.Default
		default:
			return nil
		}
	} else if arg == "-" {
		// Reading the template from stdin would block on the terminal during completion.
		return nil
	}
	tmpl, err := limatmpl.Read(ctx, "", locator)
	if err != nil {
		return nil
	}
	if err := tmpl.Embed(ctx, true, true); err != nil {
		return nil
	}
	if err := tmpl.Unmarshal(); err != nil {
		return nil
	}
	return tmpl.Config.Param
}
