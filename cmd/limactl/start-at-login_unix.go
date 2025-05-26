//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/pkg/autostart"
	"github.com/lima-vm/lima/pkg/store"
)

func startAtLoginAction(cmd *cobra.Command, args []string) error {
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(instName)
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
	if startAtLogin {
		if err := autostart.CreateStartAtLoginEntry(runtime.GOOS, inst.Name, inst.Dir); err != nil {
			logrus.WithError(err).Warnf("Can't create an autostart file for instance %q", inst.Name)
		} else {
			logrus.Infof("The autostart file %q has been created or updated", autostart.GetFilePath(runtime.GOOS, inst.Name))
		}
	} else {
		deleted, err := autostart.DeleteStartAtLoginEntry(runtime.GOOS, instName)
		if err != nil {
			logrus.WithError(err).Warnf("The autostart file %q could not be deleted", instName)
		} else if deleted {
			logrus.Infof("The autostart file %q has been deleted", autostart.GetFilePath(runtime.GOOS, instName))
		}
	}

	return nil
}
