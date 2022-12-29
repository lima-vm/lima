package main

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newInspectCommand() *cobra.Command {
	inspectCommand := &cobra.Command{
		Use:   "inspect [flags] [INSTANCE]...",
		Short: "Inspect instances of Lima.",
		Long: `Inspect instances of Lima.
		Includes both the current state, as well as the combined configuration used to create the instance.
		The output can be presented in one of several formats, using the --format <format> flag.
		
		--format json  - output in json format
		--format yaml  - output in yaml format
		--format table - output in table format
		--format '{{ <go template> }}' - if the format begins and ends with '{{ }}', then it is used as a go template.
	  `,
		Args:              cobra.ArbitraryArgs,
		RunE:              inspectAction,
		ValidArgsFunction: inspectBashComplete,
	}

	inspectCommand.Flags().StringP("format", "f", "json", "output format, one of: 'json', 'yaml', a go-template")

	return inspectCommand
}

func inspectAction(cmd *cobra.Command, args []string) error {
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	allinstances, err := store.Instances()
	if err != nil {
		return err
	}

	instanceNames := []string{}
	if len(args) > 0 {
		for _, arg := range args {
			matches := instanceMatches(arg, allinstances)
			if len(matches) > 0 {
				instanceNames = append(instanceNames, matches...)
			} else {
				logrus.Warnf("No instance matching %v found.", arg)
			}
		}
	} else {
		instanceNames = allinstances
	}

	// get the state and config for all the requested instances
	var instances []*store.Instance
	for _, instanceName := range instanceNames {
		instance, err := store.Inspect(instanceName)
		if err != nil {
			return fmt.Errorf("unable to load instance %s: %w", instanceName, err)
		}
		instances = append(instances, instance)
	}

	return store.PrintInstances(cmd.OutOrStdout(), instances, format)
}

func inspectBashComplete(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
