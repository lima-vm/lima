package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"text/tabwriter"
	"text/template"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
	listCommand := &cobra.Command{
		Use:               "list",
		Aliases:           []string{"ls"},
		Short:             "List instances of Lima.",
		Args:              cobra.NoArgs,
		RunE:              listAction,
		ValidArgsFunction: cobra.NoFileCompletions,
	}

	listCommand.Flags().StringP("format", "f", "", "Format the output using the given Go template")
	listCommand.Flags().Bool("json", false, "JSONify output")
	listCommand.Flags().BoolP("quiet", "q", false, "Only show names")

	return listCommand
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
	jsonFormat, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}

	if quiet && jsonFormat {
		return errors.New("option --quiet conflicts with --json")
	}
	if goFormat != "" && jsonFormat {
		return errors.New("option --format conflicts with --json")
	}

	instances, err := store.Instances()
	if err != nil {
		return err
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

	if len(instances) == 0 {
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
