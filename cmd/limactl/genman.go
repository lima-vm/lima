package main

import (
	"os"
	"path/filepath"

	"github.com/cpuguy83/go-md2man/v2/md2man"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newGenManCommand() *cobra.Command {
	genmanCommand := &cobra.Command{
		Use:    "generate-man DIR",
		Short:  "Generate manual pages",
		Args:   WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:   genmanAction,
		Hidden: true,
	}
	return genmanCommand
}

func genmanAction(cmd *cobra.Command, args []string) error {
	dir := args[0]
	logrus.Infof("Generating man %q", dir)
	// lima(1)
	filePath := filepath.Join(dir, "lima.1")
	md := "LIMA 1\n======" + `
# NAME
lima - ` + cmd.Root().Short + `
# SYNOPSIS
**lima** [_COMMAND_...]
# DESCRIPTION
lima is an alias for "limactl shell default".
The instance name ("default") can be changed by specifying $LIMA_INSTANCE.

The shell and initial workdir inside the instance can be specified via $LIMA_SHELL
and $LIMA_WORKDIR.
# SEE ALSO
**limactl**(1)
`
	out := md2man.Render([]byte(md))
	if err := os.WriteFile(filePath, out, 0644); err != nil {
		return err
	}
	// limactl(1)
	header := &doc.GenManHeader{
		Title:   "LIMACTL",
		Section: "1",
	}
	return doc.GenManTree(cmd.Root(), header, dir)
}
