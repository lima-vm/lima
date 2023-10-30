package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cpuguy83/go-md2man/v2/md2man"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newGenDocCommand() *cobra.Command {
	genmanCommand := &cobra.Command{
		Use:    "generate-doc DIR",
		Short:  "Generate cli-reference pages",
		Args:   WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:   gendocAction,
		Hidden: true,
	}
	genmanCommand.Flags().String("type", "man", "Output type  (man, docsy)")
	genmanCommand.Flags().String("output", "", "Output directory")
	genmanCommand.Flags().String("prefix", "", "Install prefix")
	return genmanCommand
}

func gendocAction(cmd *cobra.Command, args []string) error {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	prefix, err := cmd.Flags().GetString("prefix")
	if err != nil {
		return err
	}
	outputType, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := args[0]
	switch outputType {
	case "man":
		if err := genMan(cmd, dir); err != nil {
			return err
		}
	case "docsy":
		if err := genDocsy(cmd, dir); err != nil {
			return err
		}
	}
	if output != "" && prefix != "" {
		replaceAll(dir, output, prefix)
	}
	replaceAll(dir, homeDir, "~")
	return nil
}

func genMan(cmd *cobra.Command, dir string) error {
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
	if err := os.WriteFile(filePath, out, 0o644); err != nil {
		return err
	}
	// limactl(1)
	header := &doc.GenManHeader{
		Title:   "LIMACTL",
		Section: "1",
	}
	return doc.GenManTree(cmd.Root(), header, dir)
}

func genDocsy(cmd *cobra.Command, dir string) error {
	return doc.GenMarkdownTreeCustom(cmd.Root(), dir, func(s string) string {
		// Replace limactl_completion_bash to completion bash for docsy title
		name := filepath.Base(s)
		name = strings.ReplaceAll(name, "limactl_", "")
		name = strings.ReplaceAll(name, "_", " ")
		name = strings.TrimSuffix(name, filepath.Ext(name))
		return fmt.Sprintf(`---
title: %s
weight: 3
---
`, name)
	}, func(s string) string {
		// Use ../ for move one folder up for docsy
		return "../" + strings.TrimSuffix(s, filepath.Ext(s))
	})
}

// replaceAll replaces all occurrences of new with old, for all files in dir
func replaceAll(dir string, old, new string) error {
	logrus.Infof("Replacing %q with %q", old, new)
	return filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		if info.IsDir() {
			return filepath.SkipDir
		}
		in, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out := bytes.Replace(in, []byte(old), []byte(new), -1)
		err = os.WriteFile(path, out, 0o644)
		if err != nil {
			return err
		}
		return nil
	})
}
