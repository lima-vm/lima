// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/hostagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newScreenshotCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "screenshot INSTANCE",
		Short:   "Capture a screenshot of the VM display",
		Args:    WrapArgsError(cobra.ExactArgs(1)),
		RunE:    screenshotAction,
		GroupID: advancedCommand,
	}
	cmd.Flags().StringP("output", "o", "", "Output path; extension must be .png or .bmp (default: INSTANCE-screenshot.png)")
	return cmd
}

func screenshotAction(cmd *cobra.Command, args []string) error {
	instName := args[0]

	ctx := cmd.Context()
	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}
	if inst.Status != limatype.StatusRunning {
		return fmt.Errorf("instance %q is not running (status: %s)", instName, inst.Status)
	}

	outputPath, _ := cmd.Flags().GetString("output")
	if outputPath == "" {
		outputPath = instName + "-screenshot.png"
	}

	var format string
	switch strings.ToLower(filepath.Ext(outputPath)) {
	case ".png":
		format = "png"
	case ".bmp":
		format = "bmp"
	default:
		return fmt.Errorf("unsupported output extension %q: must be .png or .bmp", filepath.Ext(outputPath))
	}

	haSock := filepath.Join(inst.Dir, filenames.HostAgentSock)
	haClient, err := client.NewHostAgentClient(haSock)
	if err != nil {
		return fmt.Errorf("failed to connect to hostagent: %w", err)
	}

	data, err := haClient.Screenshot(ctx, format)
	if err != nil {
		return fmt.Errorf("screenshot failed: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %q: %w", outputPath, err)
	}
	logrus.Infof("Screenshot saved to %q (%d bytes)", outputPath, len(data))
	return nil
}
