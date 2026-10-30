// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package gendoc generates the cli-reference pages of limactl and its plugins.
package gendoc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// ContentModeCLI is the value of the `--content-mode` flag of a plugin's `generate-doc` command
// that makes it write its cli-reference pages for docsy.
const ContentModeCLI = "cli"

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

// Docsy generates the cli-reference pages for docsy of root and its subcommands in dir.
func Docsy(root *cobra.Command, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, c := range root.Commands() {
		c.Long = escapeMarkdown(c.Long)
	}
	return doc.GenMarkdownTreeCustom(root, dir, func(s string) string {
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
