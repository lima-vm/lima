package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func instanceFields() []string {
	fields := []string{}
	var instance store.Instance
	t := reflect.TypeOf(instance)
	for i := 0; i < t.NumField(); i++ {
		fields = append(fields, t.Field(i).Name)
	}
	return fields
}

func newListCommand() *cobra.Command {
	listCommand := &cobra.Command{
		Use:               "list [flags] [INSTANCE]...",
		Aliases:           []string{"ls"},
		Short:             "List instances of Lima.",
		Args:              cobra.ArbitraryArgs,
		RunE:              listAction,
		ValidArgsFunction: listBashComplete,
	}

	listCommand.Flags().StringP("format", "f", "", "Format the output using the given Go template")
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
	goFormat, err := cmd.Flags().GetString("format")
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

	if goFormat != "" && listFields {
		return errors.New("option --format conflicts with --list-fields")
	}
	if jsonFormat && listFields {
		return errors.New("option --json conflicts with --list-fields")
	}
	if listFields {
		fmt.Println(strings.Join(instanceFields(), "\n"))
		return nil
	}
	if quiet && jsonFormat {
		return errors.New("option --quiet conflicts with --json")
	}
	if goFormat != "" && jsonFormat {
		return errors.New("option --format conflicts with --json")
	}

	allinstances, err := store.Instances()
	if err != nil {
		return err
	}

	instances := []string{}
	if len(args) > 0 {
		for _, arg := range args {
			matches := instanceMatches(arg, allinstances)
			if len(matches) > 0 {
				instances = append(instances, matches...)
			} else {
				logrus.Warnf("No instance matching %v found.", arg)
			}
		}
	} else {
		instances = allinstances
	}

	if quiet {
		for _, instName := range instances {
			fmt.Fprintln(cmd.OutOrStdout(), instName)
		}
		return nil
	}

	if goFormat != "" {
		tmpl, err := template.New("format").Parse(goFormat)
		if err != nil {
			return err
		}
		for _, instName := range instances {
			inst, err := store.Inspect(instName)
			if err != nil {
				logrus.WithError(err).Errorf("instance %q does not exist?", instName)
				continue
			}
			err = tmpl.Execute(cmd.OutOrStdout(), inst)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout())
		}
		return nil
	}
	if jsonFormat {
		for _, instName := range instances {
			inst, err := store.Inspect(instName)
			if err != nil {
				logrus.WithError(err).Errorf("instance %q does not exist?", instName)
				continue
			}
			b, err := json.Marshal(inst)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
		}
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 4, 8, 4, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tSSH\tARCH\tCPUS\tMEMORY\tDISK\tDIR")

	if len(allinstances) == 0 {
		logrus.Warn("No instance found. Run `limactl start` to create an instance.")
	}

	for _, instName := range instances {
		inst, err := store.Inspect(instName)
		if err != nil {
			logrus.WithError(err).Errorf("instance %q does not exist?", instName)
			continue
		}
		if len(inst.Errors) > 0 {
			logrus.WithField("errors", inst.Errors).Warnf("instance %q has errors", instName)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			inst.Name,
			inst.Status,
			fmt.Sprintf("127.0.0.1:%d", inst.SSHLocalPort),
			inst.Arch,
			inst.CPUs,
			units.BytesSize(float64(inst.Memory)),
			units.BytesSize(float64(inst.Disk)),
			inst.Dir,
		)
	}

	return w.Flush()
}

func listBashComplete(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
