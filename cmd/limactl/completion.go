// Forked from https://github.com/containerd/nerdctl/blob/v0.8.1/completion.go

/*
   Copyright The containerd Authors.

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
	"fmt"

	"github.com/AkihiroSuda/lima/pkg/store"
	"github.com/urfave/cli/v2"
)

var completionCommand = &cli.Command{
	Name:  "completion",
	Usage: "Show shell completion",
	Subcommands: []*cli.Command{
		completionBashCommand,
	},
}

var completionBashCommand = &cli.Command{
	Name:        "bash",
	Usage:       "Show bash completion (use with `source <(limactl completion bash)`)",
	Description: "Usage: add `source <(limactl completion bash)` to ~/.bash_profile",
	Action:      completionBashAction,
}

func completionBashAction(clicontext *cli.Context) error {
	tmpl := `#!/bin/bash
# Autocompletion enabler for limactl.
# Usage: add 'source <(limactl completion bash)' to ~/.bash_profile

# _limactl_bash_autocomplete is forked from https://github.com/urfave/cli/blob/v2.3.0/autocomplete/bash_autocomplete (MIT License)
_limactl_bash_autocomplete() {
  if [[ "${COMP_WORDS[0]}" != "source" ]]; then
    local cur opts base
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    local args="${COMP_WORDS[@]:0:$COMP_CWORD}"
    # make {"limactl", "--foo", "=", "bar"} into {"limactl", "--foo=bar"}
    args="$(echo $args | sed -e 's/ = /=/g')"
    if [[ "$cur" == "-"* ]]; then
      opts=$( ${args} ${cur} --generate-bash-completion )
    else
      opts=$( ${args} --generate-bash-completion )
    fi
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
  fi
}

complete -o bashdefault -o default -o nospace -F _limactl_bash_autocomplete limactl
`
	_, err := fmt.Fprint(clicontext.App.Writer, tmpl)
	return err
}

func bashCompleteInstanceNames(clicontext *cli.Context) {
	w := clicontext.App.Writer
	instances, err := store.Instances()
	if err != nil {
		return
	}
	for _, name := range instances {
		fmt.Fprintln(w, name)
	}
}
