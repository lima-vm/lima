//go:build !darwin

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

func autostartEnableAction(cmd *cobra.Command, args []string) error {
	condition, err := cmd.Flags().GetString("condition")
	if err != nil {
		return err
	}
	if condition == "boot" {
		return errors.New("--condition=boot is only supported on macOS")
	}

	ctx := cmd.Context()
	inst, err := store.Inspect(ctx, args[0])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q not found", args[0])
		}
		return err
	}

	if err := autostart.RegisterToStartAtLogin(ctx, inst); err != nil {
		return fmt.Errorf("failed to register instance %#q to start at login: %w", inst.Name, err)
	}
	logrus.Infof("Instance %#q registered to start at login", inst.Name)
	return nil
}

func autostartDisableAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	inst, err := store.Inspect(ctx, args[0])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q not found", args[0])
		}
		return err
	}

	if registered, err := autostart.IsRegistered(ctx, inst); err != nil {
		return err
	} else if !registered {
		logrus.Infof("Instance %#q is not registered for automatic startup", inst.Name)
		return nil
	}

	if err := autostart.UnregisterFromStartAtLogin(ctx, inst); err != nil {
		return fmt.Errorf("failed to unregister instance %#q from start at login: %w", inst.Name, err)
	}
	logrus.Infof("Instance %#q unregistered from start at login", inst.Name)
	return nil
}
