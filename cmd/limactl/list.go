// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/cheggaaa/pb/v3/termutil"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/uiutil"
	"github.com/lima-vm/lima/v2/pkg/yqutil"
)

func fieldNames() []string {
	names := []string{}
	t := reflect.TypeFor[store.FormatData]()
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous {
			for j := range f.Type.NumField() {
				if tag := f.Tag.Get("lima"); tag != "deprecated" {
					names = append(names, f.Type.Field(j).Name)
				}
			}
		} else {
			if tag := f.Tag.Get("lima"); tag != "deprecated" {
				names = append(names, t.Field(i).Name)
			}
		}
	}
	return names
}

func newListCommand() *cobra.Command {
	listCommand := &cobra.Command{
		Use:     "list [flags] [INSTANCE]...",
		Aliases: []string{"ls"},
		Short:   "List instances of Lima",
		Long: `List instances of Lima.

The output can be presented in one of several formats, using the --format <format> flag.

  --format json  - Output in JSON format
  --format yaml  - Output in YAML format
  --format table - Output in table format
  --format '{{ <go template> }}' - If the format begins and ends with '{{ }}', then it is used as a go template.

Filtering instances:
  --filter EXPR  - Filter instances using yq expression (this is equivalent to --yq 'select(EXPR)')
                   Can be specified multiple times (combined with AND logic) and it works with all output formats.
                   Examples:
                     --filter '.status == "Running"'
                     --filter '.vmType == "vz"'
                     --filter '.status == "Running"' --filter '.vmType == "vz"' (Same as AND)
` + store.FormatHelp + `
The following legacy flags continue to function:
  --json - equal to '--format json'`,
		Args:              WrapArgsError(cobra.ArbitraryArgs),
		RunE:              listAction,
		ValidArgsFunction: listBashComplete,
		GroupID:           basicCommand,
	}

	listCommand.Flags().StringP("format", "f", "table", "Output format, one of: json, yaml, table, go-template")
	listCommand.Flags().Bool("list-fields", false, "List fields available for format")
	listCommand.Flags().Bool("json", false, "Same as --format=json")
	listCommand.Flags().BoolP("quiet", "q", false, "Only show names")
	listCommand.Flags().Bool("all-fields", false, "Show all fields")
	listCommand.Flags().StringArray("yq", nil, "Apply yq expression to each instance")
	listCommand.Flags().StringArrayP("filter", "l", nil, "Filter instances using yq expression (equivalent to --yq 'select(EXPR)')")

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

// unmatchedInstancesError is created when unmatched instance names found.
type unmatchedInstancesError struct{}

// Error implements error.
func (unmatchedInstancesError) Error() string {
	return "unmatched instances"
}

// ExitCode implements ExitCoder.
func (unmatchedInstancesError) ExitCode() int {
	return 1
}

func listAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
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
	yq, err := cmd.Flags().GetStringArray("yq")
	if err != nil {
		return err
	}
	filter, err := cmd.Flags().GetStringArray("filter")
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
	if len(yq) != 0 {
		if cmd.Flags().Changed("format") && format != "json" && format != "yaml" {
			return errors.New("option --yq only works with --format json or yaml")
		}
		if listFields {
			return errors.New("option --list-fields conflicts with option --yq")
		}
	}
	if len(filter) != 0 {
		if listFields {
			return errors.New("option --list-fields conflicts with option --filter")
		}
	}

	if quiet && format != "table" {
		return errors.New("option --quiet can only be used with '--format table'")
	}

	if listFields {
		names := fieldNames()
		slices.Sort(names)
		fmt.Fprintln(cmd.OutOrStdout(), strings.Join(names, "\n"))
		return nil
	}

	if err := store.Validate(); err != nil {
		logrus.Warnf("The directory %q does not look like a valid Lima directory: %v", store.Directory(), err)
	}

	allInstances, err := store.Instances()
	if err != nil {
		return err
	}
	if len(args) == 0 && len(allInstances) == 0 {
		logrus.Warn("No instance found. Run `limactl create` to create an instance.")
		return nil
	}

	instanceNames := []string{}
	unmatchedInstances := false
	if len(args) > 0 {
		for _, arg := range args {
			matches := instanceMatches(arg, allInstances)
			if len(matches) > 0 {
				instanceNames = append(instanceNames, matches...)
			} else {
				logrus.Warnf("No instance matching %v found.", arg)
				unmatchedInstances = true
			}
		}
	} else {
		instanceNames = allInstances
	}

	if quiet && len(yq) == 0 && len(filter) == 0 {
		for _, instName := range instanceNames {
			fmt.Fprintln(cmd.OutOrStdout(), instName)
		}
		if unmatchedInstances {
			return unmatchedInstancesError{}
		}
		return nil
	}

	// get the state and config for all the requested instances
	var instances []*limatype.Instance
	for _, instanceName := range instanceNames {
		instance, err := store.Inspect(ctx, instanceName)
		if err != nil {
			return fmt.Errorf("unable to load instance %s: %w", instanceName, err)
		}
		instances = append(instances, instance)
	}

	if len(filter) > 0 {
		var filterExprs []string
		for _, f := range filter {
			filterExprs = append(filterExprs, "select("+f+")")
		}
		instances, err = filterInstances(instances, filterExprs)
		if err != nil {
			return err
		}
	}

	if quiet && len(yq) == 0 {
		for _, instance := range instances {
			fmt.Fprintln(cmd.OutOrStdout(), instance.Name)
		}
		if unmatchedInstances {
			return unmatchedInstancesError{}
		}
		return nil
	}

	for _, instance := range instances {
		if len(instance.Errors) > 0 {
			logrus.WithField("errors", instance.Errors).Warnf("instance %q has errors", instance.Name)
		}
	}

	allFields, err := cmd.Flags().GetBool("all-fields")
	if err != nil {
		return err
	}

	options := store.PrintOptions{AllFields: allFields}
	isTTY := uiutil.OutputIsTTY(cmd.OutOrStdout())
	if isTTY {
		if w, err := termutil.TerminalWidth(); err == nil {
			options.TerminalWidth = w
		}
	}

	// --yq implies --format json unless --format has been explicitly specified
	if len(yq) != 0 && !cmd.Flags().Changed("format") {
		format = "json"
	}

	// Always pipe JSON and YAML through yq to colorize it if isTTY
	if len(yq) == 0 && (format == "json" || format == "yaml") {
		yq = append(yq, ".")
	}

	if len(yq) == 0 {
		err = store.PrintInstances(cmd.OutOrStdout(), instances, format, &options)
		if err == nil && unmatchedInstances {
			return unmatchedInstancesError{}
		}
		return err
	}

	if quiet {
		yq = append(yq, ".name")
	}
	yqExpr := strings.Join(yq, " | ")

	buf := new(bytes.Buffer)
	err = store.PrintInstances(buf, instances, format, &options)
	if err != nil {
		return err
	}

	if format == "json" {
		// The JSON encoder will create empty objects (YAML maps), even when they have the ",omitempty" tag.
		deleteEmptyObjects := `del(.. | select(tag == "!!map" and length == 0))`
		yqExpr += " | " + deleteEmptyObjects

		encoderPrefs := yqlib.ConfiguredJSONPreferences.Copy()
		encoderPrefs.ColorsEnabled = false
		encoderPrefs.Indent = 0
		plainEncoder := yqlib.NewJSONEncoder(encoderPrefs)
		// Using non-0 indent means the instance will be printed over multiple lines,
		// so is no longer in JSON Lines format. This is a compromise for readability.
		encoderPrefs.Indent = 4
		encoderPrefs.ColorsEnabled = true
		colorEncoder := yqlib.NewJSONEncoder(encoderPrefs)

		// Each line contains the JSON object for one Lima instance.
		scanner := bufio.NewScanner(buf)
		for scanner.Scan() {
			var str string
			if str, err = yqutil.EvaluateExpressionWithEncoder(yqExpr, scanner.Text(), plainEncoder); err != nil {
				return err
			}
			// Repeatedly delete empty objects until there are none left.
			for {
				length := len(str)
				if str, err = yqutil.EvaluateExpressionWithEncoder(deleteEmptyObjects, str, plainEncoder); err != nil {
					return err
				}
				if len(str) >= length {
					break
				}
			}
			if isTTY {
				// pretty-print and colorize the output
				if str, err = yqutil.EvaluateExpressionWithEncoder(".", str, colorEncoder); err != nil {
					return err
				}
			}
			if _, err = fmt.Fprint(cmd.OutOrStdout(), str); err != nil {
				return err
			}
		}
		err = scanner.Err()
		if err == nil && unmatchedInstances {
			return unmatchedInstancesError{}
		}
		return err
	}

	var str string
	if isTTY {
		// This branch is trading the better formatting from yamlfmt for colorizing from yqlib.
		if str, err = yqutil.EvaluateExpressionPlain(yqExpr, buf.String(), true); err != nil {
			return err
		}
	} else {
		var res []byte
		if res, err = yqutil.EvaluateExpression(yqExpr, buf.Bytes()); err != nil {
			return err
		}
		str = string(res)
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), str)
	if err == nil && unmatchedInstances {
		return unmatchedInstancesError{}
	}
	return err
}

func listBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}

// filterInstances applies yq expressions to instances and returns the filtered results.
func filterInstances(instances []*limatype.Instance, yqExprs []string) ([]*limatype.Instance, error) {
	if len(yqExprs) == 0 {
		return instances, nil
	}

	// the yq expression is evaluated with yqutil.EvaluateExpression, which disables environment variable access
	// and file operations, mitigating injection attacks like ".name=strenv(SOME_SECRET_ENV)" which could
	// trick Lima into exposing environment variables.
	yqExpr := strings.Join(yqExprs, " | ")

	var filteredInstances []*limatype.Instance
	for _, instance := range instances {
		jsonBytes, err := json.Marshal(instance)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal instance %q: %w", instance.Name, err)
		}

		result, err := yqutil.EvaluateExpression(yqExpr, jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to apply filter %q: %w", yqExpr, err)
		}

		if len(bytes.TrimSpace(result)) > 0 {
			filteredInstances = append(filteredInstances, instance)
		}
	}

	return filteredInstances, nil
}
