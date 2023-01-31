package main

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func fieldNames() []string {
	names := []string{}
	var data store.FormatData
	t := reflect.TypeOf(data)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			for j := 0; j < f.Type.NumField(); j++ {
				names = append(names, f.Type.Field(j).Name)
			}
		} else {
			names = append(names, t.Field(i).Name)
		}
	}
	return names
}

func newListCommand() *cobra.Command {
	listCommand := &cobra.Command{
		Use:     "list [flags] [INSTANCE]...",
		Aliases: []string{"ls"},
		Short:   "List instances of Lima.",
		Long: `List instances of Lima.
		The output can be presented in one of several formats, using the --format <format> flag.
		
		  --format json  - output in json format
		  --format yaml  - output in yaml format
		  --format table - output in table format
		  --format '{{ <go template> }}' - if the format begins and ends with '{{ }}', then it is used as a go template.
` + store.FormatHelp +
			`
		The following legacy flags continue to function:
		  --json - equal to '--format json'
		`,
		Args:              cobra.ArbitraryArgs,
		RunE:              listAction,
		ValidArgsFunction: listBashComplete,
	}

	listCommand.Flags().StringP("format", "f", "table", "output format, one of: json, yaml, table, go-template")
	listCommand.Flags().Bool("list-fields", false, "List fields available for format")
	listCommand.Flags().Bool("json", false, "JSONify output")
	listCommand.Flags().BoolP("quiet", "q", false, "Only show names")

	return listCommand
}

func instanceMatches(arg string, instances []string) []string {
	matches := []string{}
	for _, instance := range instances {
		if instance == arg {
			matches = append(matches, instance)
		}
	}
	return matches
}

func listAction(cmd *cobra.Command, args []string) error {
	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	listFields, err := cmd.Flags().GetBool("list-fields")
	if err != nil {
		return err
	}
	jsonFormat, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}

	if jsonFormat {
		format = "json"
	}

	// conflicts
	if jsonFormat && cmd.Flags().Changed("format") {
		return errors.New("option --json conflicts with option --format")
	}
	if listFields && cmd.Flags().Changed("format") {
		return errors.New("option --list-fields conflicts with option --format")
	}

	if quiet && format != "table" {
		return errors.New("option --quiet can only be used with '--format table'")
	}

	if listFields {
		names := fieldNames()
		sort.Strings(names)
		fmt.Println(strings.Join(names, "\n"))
		return nil
	}

	allinstances, err := store.Instances()
	if err != nil {
		return err
	}
	if len(allinstances) == 0 {
		logrus.Warn("No instance found. Run `limactl start` to create an instance.")
		return nil
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

	if quiet {
		for _, instName := range instanceNames {
			fmt.Fprintln(cmd.OutOrStdout(), instName)
		}
		return nil
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

	for _, instance := range instances {
		if len(instance.Errors) > 0 {
			logrus.WithField("errors", instance.Errors).Warnf("instance %q has errors", instance.Name)
		}
	}

	return store.PrintInstances(cmd.OutOrStdout(), instances, format)
}

func listBashComplete(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
