package main

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/spf13/cobra"
	orderedmap "github.com/wk8/go-ordered-map/v2"
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

func toAny(args []string) []any {
	result := []any{nil}
	for _, arg := range args {
		result = append(result, arg)
	}
	return result
}

func getProp(props *orderedmap.OrderedMap[string, *jsonschema.Schema], key string) *jsonschema.Schema {
	value, ok := props.Get(key)
	if !ok {
		return nil
	}
	return value
}

func genschemaAction(cmd *cobra.Command, _ []string) error {
	schema := jsonschema.Reflect(&limayaml.LimaYAML{})
	// allow Disk to be either string (name) or object (struct)
	schema.Definitions["Disk"].Type = "" // was: "object"
	schema.Definitions["Disk"].OneOf = []*jsonschema.Schema{
		{Type: "string"},
		{Type: "object"},
	}
	properties := schema.Definitions["LimaYAML"].Properties
	getProp(properties, "os").Enum = toAny(limayaml.OSTypes)
	getProp(properties, "arch").Enum = toAny(limayaml.ArchTypes)
	getProp(properties, "mountType").Enum = toAny(limayaml.MountTypes)
	getProp(properties, "vmType").Enum = toAny(limayaml.VMTypes)
	j, err := json.MarshalIndent(schema, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(j))
	return err
}
