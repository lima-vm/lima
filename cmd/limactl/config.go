package main

import (
	"fmt"
	"os"

	"github.com/google/go-cmp/cmp"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	var configCommand = &cobra.Command{
		Use:   "config [FILE.yaml, ...]",
		Short: "Evaluate YAML files",
		RunE:  configAction,
	}
	configCommand.Flags().Bool("diff", false, "Show diff from default")
	configCommand.Flags().String("default", "", "Default template file")
	return configCommand
}

func configAction(cmd *cobra.Command, args []string) error {
	diff, _ := cmd.Flags().GetBool("diff")
	template, err := cmd.Flags().GetString("default")
	if err != nil {
		return err
	}
	var b []byte
	if template != "" {
		var err error
		b, err = os.ReadFile(template)
		if err != nil {
			return err
		}
	} else {
		b = limayaml.DefaultTemplate
	}
	c, err := store.LoadYAML(b)
	if err != nil {
		return fmt.Errorf("failed to load YAML: %w", err)
	}
	d, err := store.SaveYAML(c)
	if err != nil {
		return fmt.Errorf("failed to save YAML: %w", err)
	}
	if len(args) == 0 {
		// show the default
		fmt.Print(string(d))
	}
	for _, f := range args {
		y, err := store.LoadYAMLByFilePath(f)
		if err != nil {
			return fmt.Errorf("failed to load YAML file %q: %w", f, err)
		}
		name, _ := instNameFromYAMLPath(f)
		b, err := store.SaveYAML(y)
		if err != nil {
			return fmt.Errorf("failed to save YAML: %w", err)
		}
		if diff {
			if len(args) > 1 {
				fmt.Printf("# %s\n", name)
			}
			fmt.Print(cmp.Diff(string(d), string(b)))
		} else {
			if len(args) > 1 {
				fmt.Printf("--- # %s.yaml\n", name)
			}
			fmt.Print(string(b))
		}
	}

	return nil
}
