// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/templatestore"
)

func bashCompleteInstanceNames(_ *cobra.Command) ([]string, cobra.ShellCompDirective) {
	instances, err := store.Instances()
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return instances, cobra.ShellCompDirectiveNoFileComp
}

func bashCompleteTemplateNames(_ *cobra.Command, toComplete string) ([]string, cobra.ShellCompDirective) {
	var comp []string
	if templates, err := templatestore.Templates(); err == nil {
		for _, f := range templates {
			name := "template://" + f.Name
			if !strings.HasPrefix(name, toComplete) {
				continue
			}
			if len(toComplete) == len(name) {
				comp = append(comp, name)
				continue
			}

			// Skip private snippets (beginning with '_') from completion.
			if (name[len(toComplete)-1] == '/') && (name[len(toComplete)] == '_') {
				continue
			}

			comp = append(comp, name)
		}
	}
	return comp, cobra.ShellCompDirectiveDefault
}

func bashCompleteDiskNames(_ *cobra.Command) ([]string, cobra.ShellCompDirective) {
	disks, err := store.Disks()
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return disks, cobra.ShellCompDirectiveNoFileComp
}
