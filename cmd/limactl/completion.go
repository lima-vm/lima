package main

import (
	"github.com/lima-vm/lima/pkg/store"
	"github.com/spf13/cobra"
)

func bashCompleteInstanceNames(cmd *cobra.Command) ([]string, cobra.ShellCompDirective) {
	instances, err := store.Instances()
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return instances, cobra.ShellCompDirectiveNoFileComp
}
