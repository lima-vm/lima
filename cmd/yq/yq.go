// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// SPDX-FileCopyrightText: Copyright (c) 2017 Mike Farah

// This file has been adapted from https://github.com/mikefarah/yq/blob/v4.47.1/yq.go

package yq

import (
	"os"
	"path/filepath"
	"strings"

	command "github.com/mikefarah/yq/v4/cmd"
)

func main() {
	cmd := command.New()
	args := os.Args[1:]
	_, _, err := cmd.Find(args)
	if err != nil && args[0] != "__complete" {
		// default command when nothing matches...
		newArgs := []string{"eval"}
		cmd.SetArgs(append(newArgs, os.Args[1:]...))
	}
	code := 0
	if err := cmd.Execute(); err != nil {
		code = 1
	}
	os.Exit(code)
}

// MaybeRunYQ runs as `yq` if the program name or first argument is `yq`.
// Only returns to caller if os.Args doesn't contain a `yq` command.
func MaybeRunYQ() {
	progName := filepath.Base(os.Args[0])
	// remove all extensions, so we match "yq.lima.exe"
	progName, _, _ = strings.Cut(progName, ".")
	if progName == "yq" {
		main()
	}
	if len(os.Args) > 1 && os.Args[1] == "yq" {
		os.Args = os.Args[1:]
		main()
	}
}
