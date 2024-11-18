package main

import (
	"github.com/lima-vm/lima/pkg/instance"
	networks "github.com/lima-vm/lima/pkg/networks/reconcile"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newSaveCommand() *cobra.Command {
	saveCmd := &cobra.Command{
		Use:   "save INSTANCE",
		Short: "Save an instance",
		PersistentPreRun: func(*cobra.Command, []string) {
			logrus.Warn("`limactl save` is experimental")
		},
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              saveAction,
		ValidArgsFunction: saveBashComplete,
		GroupID:           basicCommand,
	}

	return saveCmd
}

func saveAction(cmd *cobra.Command, args []string) error {
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(instName)
	if err != nil {
		return err
	}

	err = instance.StopGracefully(inst, true)
	// TODO: should we also reconcile networks if graceful save returned an error?
	if err == nil {
		err = networks.Reconcile(cmd.Context(), "")
	}
	return err
}

func saveBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
