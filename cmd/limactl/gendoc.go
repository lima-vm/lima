// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cpuguy83/go-md2man/v2/md2man"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/lima-vm/lima/v2/pkg/plugins"
	"github.com/lima-vm/lima/v2/pkg/usrlocal"
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
		if err := genPluginsDocsy(cmd.Context(), bundledPlugins(), dir); err != nil {
			return err
		}
	}
	if output != "" && prefix != "" {
		if err := replaceAll(dir, output, prefix); err != nil {
			return err
		}
	}
	return replaceAll(dir, homeDir, "~")
}

func genMan(cmd *cobra.Command, dir string) error {
	logrus.Infof("Generating man %#q", dir)
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

func escapeMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		// Need to escape backticks first, before adding more
		for c := range strings.SplitSeq("\\`*_[]()#+-.|", "") {
			lines[i] = strings.ReplaceAll(lines[i], c, "\\"+c)
		}
		if i < len(lines)-1 {
			if lines[i] != "" && lines[i+1] != "" {
				lines[i] += "  " // line break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func genDocsy(cmd *cobra.Command, dir string) error {
	for _, c := range cmd.Root().Commands() {
		c.Long = escapeMarkdown(c.Long)
	}
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

// bundledPlugins returns the plugins installed next to limactl.
// Plugins found only in $PATH are skipped, as they are not part of Lima.
func bundledPlugins() []plugins.Plugin {
	allPlugins, err := plugins.Discover()
	if err != nil {
		logrus.WithError(err).Warn("Failed to discover plugins")
		return nil
	}
	dirs := usrlocal.SelfDirs()
	if libexecDirs, err := usrlocal.LibexecLima(); err == nil {
		dirs = append(dirs, libexecDirs...)
	}
	var res []plugins.Plugin
	for _, plugin := range allPlugins {
		if slices.Contains(dirs, filepath.Dir(plugin.Path)) {
			res = append(res, plugin)
		}
	}
	return res
}

// genPluginsDocsy generates the cli-reference pages of the plugins that support `generate-doc --cli-dir DIR`.
func genPluginsDocsy(ctx context.Context, allPlugins []plugins.Plugin, dir string) error {
	for _, plugin := range allPlugins {
		if !supportsGenDocsy(ctx, plugin.Path) {
			continue
		}
		logrus.Infof("Generating docsy for plugin %#q", plugin.Name)
		cmd := exec.CommandContext(ctx, plugin.Path, "generate-doc", "--cli-dir", dir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to generate docsy for plugin %#q: %w", plugin.Name, err)
		}
	}
	return nil
}

// supportsGenDocsy checks whether the plugin has the `generate-doc --cli-dir` flag.
func supportsGenDocsy(ctx context.Context, path string) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "generate-doc", "--help").Output()
	if err != nil {
		return false
	}
	return bytes.Contains(out, []byte("--cli-dir"))
}

// replaceAll replaces all occurrences of text with replacement, for all files in dir.
func replaceAll(dir, text, replacement string) error {
	logrus.Infof("Replacing %#q with %#q", text, replacement)
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
		out := bytes.ReplaceAll(in, []byte(text), []byte(replacement))
		err = os.WriteFile(path, out, 0o644)
		if err != nil {
			return err
		}
		return nil
	})
}
