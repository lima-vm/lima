package main

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/spf13/cobra"
)

func newGenSchemaCommand() *cobra.Command {
	genschemaCommand := &cobra.Command{
		Use:    "generate-jsonschema",
		Short:  "Generate json-schema document",
		Args:   WrapArgsError(cobra.NoArgs),
		RunE:   genschemaAction,
		Hidden: true,
	}
	return genschemaCommand
}

func genschemaAction(cmd *cobra.Command, _ []string) error {
	schema := jsonschema.Reflect(&limayaml.LimaYAML{})
	// allow Disk to be either string (name) or object (struct)
	schema.Definitions["Disk"].Type = "" // was: "object"
	schema.Definitions["Disk"].OneOf = []*jsonschema.Schema{
		{Type: "string"},
		{Type: "object"},
	}
	j, err := json.MarshalIndent(schema, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(j))
	return err
}
