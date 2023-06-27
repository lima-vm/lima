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
