/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/templatestore"
	"github.com/spf13/cobra"
)

func bashCompleteInstanceNames(_ *cobra.Command) ([]string, cobra.ShellCompDirective) {
	instances, err := store.Instances()
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return instances, cobra.ShellCompDirectiveNoFileComp
}

func bashCompleteTemplateNames(_ *cobra.Command) ([]string, cobra.ShellCompDirective) {
	var comp []string
	if templates, err := templatestore.Templates(); err == nil {
		for _, f := range templates {
			comp = append(comp, "template://"+f.Name)
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
