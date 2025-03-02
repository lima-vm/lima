/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"

	"github.com/lima-vm/lima/pkg/infoutil"
	"github.com/spf13/cobra"
)

func newInfoCommand() *cobra.Command {
	infoCommand := &cobra.Command{
		Use:     "info",
		Short:   "Show diagnostic information",
		Args:    WrapArgsError(cobra.NoArgs),
		RunE:    infoAction,
		GroupID: advancedCommand,
	}
	return infoCommand
}

func infoAction(cmd *cobra.Command, _ []string) error {
	info, err := infoutil.GetInfo()
	if err != nil {
		return err
	}
	j, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(j))
	return err
}
