// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"
)

func newYQRestrictionsHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "yq-restrictions",
		Short: "Restrictions on yq expressions in Lima",
		Long: `Lima uses yq (v4) syntax for the --set flag and provision mode "yq".

Lima embeds yqlib (https://github.com/mikefarah/yq) as a library and
disables several operators to prevent template expressions from reading
the host environment or executing arbitrary commands:

  Disabled by Lima:
    - env               (environment variable access)
    - load, load_str    (arbitrary file reads)

  Disabled by yqlib default:
    - system            (arbitrary command execution)

These restrictions exist because --set expressions and provision.yq
expressions may come from untrusted template files. Allowing them to
access environment variables, read files, or execute commands on the
host would be a security risk.

For full yq v4 expression syntax, see:
  https://mikefarah.gitbook.io/yq/`,
	}
}
