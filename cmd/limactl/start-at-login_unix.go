//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/autostart"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func startAtLoginAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logrus.Infof("Instance %q not found", instName)
			return nil
		}
		return err
	}

	flags := cmd.Flags()
	startAtLogin, err := flags.GetBool("enabled")
	if err != nil {
		return err
	}
	if registered, err := autostart.IsRegistered(ctx, inst); err != nil {
		return fmt.Errorf("failed to check if the autostart entry for instance %q is registered: %w", inst.Name, err)
	} else if startAtLogin {
		verb := "create"
		if registered {
			verb = "update"
		}
		if err := autostart.RegisterToStartAtLogin(ctx, inst); err != nil {
			return fmt.Errorf("failed to %s the autostart entry for instance %q: %w", verb, inst.Name, err)
		}
		logrus.Infof("The autostart entry for instance %q has been %sd", inst.Name, verb)
	} else {
		if !registered {
			logrus.Infof("The autostart entry for instance %q is not registered", inst.Name)
		} else if err := autostart.UnregisterFromStartAtLogin(ctx, inst); err != nil {
			return fmt.Errorf("failed to unregister the autostart entry for instance %q: %w", inst.Name, err)
		} else {
			logrus.Infof("The autostart entry for instance %q has been unregistered", inst.Name)
		}
	}

	return nil
}
