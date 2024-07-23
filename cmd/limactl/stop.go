package main

import (
	"github.com/lima-vm/lima/pkg/instance"
	networks "github.com/lima-vm/lima/pkg/networks/reconcile"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/spf13/cobra"
)

func newStopCommand() *cobra.Command {
	stopCmd := &cobra.Command{
		Use:               "stop INSTANCE",
		Short:             "Stop an instance",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              stopAction,
		ValidArgsFunction: stopBashComplete,
		GroupID:           basicCommand,
	}

	stopCmd.Flags().BoolP("force", "f", false, "force stop the instance")
	return stopCmd
}

func stopAction(cmd *cobra.Command, args []string) error {
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(instName)
	if err != nil {
		return err
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}
	if force {
		instance.StopForcibly(inst)
	} else {
		err = instance.StopGracefully(inst)
	}
	// TODO: should we also reconcile networks if graceful stop returned an error?
	if err == nil {
		err = networks.Reconcile(cmd.Context(), "")
	}
	return err
}

func stopBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
