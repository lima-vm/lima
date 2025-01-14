package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/invopop/jsonschema"
	"github.com/lima-vm/lima/pkg/limayaml"
	jsonschema2 "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/sirupsen/logrus"
)

func newGenSchemaCommand() *cobra.Command {
	genschemaCommand := &cobra.Command{
		Use:    "generate-jsonschema",
		Short:  "Generate json-schema document",
		Args:   WrapArgsError(cobra.ArbitraryArgs),
		RunE:   genschemaAction,
		Hidden: true,
	}
	genschemaCommand.Flags().String("schemafile", "", "Output file")
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

func genschemaAction(cmd *cobra.Command, args []string) error {
	file, err := cmd.Flags().GetString("schemafile")
	if err != nil {
		return err
	}

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
	if len(args) == 0 {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(j))
		return err
	}

	if file == "" {
		return errors.New("need --schemafile to validate")
	}
	err = os.WriteFile(file, j, 0o644)
	if err != nil {
		return err
	}
	compiler := jsonschema2.NewCompiler()
	schema2, err := compiler.Compile(file)
	if err != nil {
		return err
	}
	for _, f := range args {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var y any
		err = yaml.Unmarshal(b, &y)
		if err != nil {
			return err
		}
		err = schema2.Validate(y)
		if err != nil {
			return fmt.Errorf("%q: %w", f, err)
		}
		logrus.Infof("%q: OK", f)
	}

	return err
}
