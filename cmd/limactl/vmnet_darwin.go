// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/coreos/go-semver/semver"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/vmnet"
)

func newVmnetAction(cmd *cobra.Command, _ []string) error {
	macOSProductVersion, err := osutil.ProductVersion()
	if err != nil {
		return err
	}
	if macOSProductVersion.LessThan(*semver.New("26.0.0")) {
		return errors.New("vmnet requires macOS 26 or higher to run")
	}

	if !cmd.HasLocalFlags() {
		return cmd.Help()
	}

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if machServiceName, _ := cmd.Flags().GetString("mach-service"); machServiceName != "" {
		return vmnet.RunMachService(ctx, machServiceName)
	} else if unregisterMachService, _ := cmd.Flags().GetBool("unregister-mach-service"); unregisterMachService {
		return vmnet.UnregisterMachService(ctx)
	}
	return cmd.Help()
}
