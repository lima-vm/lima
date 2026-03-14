// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"maps"
	"net"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/templatestore"
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
			name := "template:" + f.Name
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

func bashCompleteNetworkNames(_ *cobra.Command) ([]string, cobra.ShellCompDirective) {
	config, err := networks.LoadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	networks := slices.Sorted(maps.Keys(config.Networks))
	return networks, cobra.ShellCompDirectiveNoFileComp
}

func bashFlagCompleteNetworkInterfaceNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	intf, err := net.Interfaces()
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	var intfNames []string
	for _, f := range intf {
		intfNames = append(intfNames, f.Name)
	}
	return intfNames, cobra.ShellCompDirectiveNoFileComp
}
