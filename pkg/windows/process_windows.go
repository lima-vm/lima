// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type CommandLineJSON []struct {
	CommandLine string
}

// GetProcessCommandLine returns a slice of string containing all commandlines for a given process name.
func GetProcessCommandLine(name string) ([]string, error) {
	ctx := context.TODO()
	out, err := exec.CommandContext(ctx,
		"powershell.exe",
		"-nologo",
		"-noprofile",
		fmt.Sprintf(
			`Get-CimInstance Win32_Process -Filter "name = '%s'" | Select CommandLine | ConvertTo-Json`,
			name,
		),
	).CombinedOutput()
	if err != nil {
		return nil, err
	}

	var outJSON CommandLineJSON
	if err = json.Unmarshal(out, &outJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %q as %T: %w", out, outJSON, err)
	}

	var ret []string
	for _, s := range outJSON {
		ret = append(ret, s.CommandLine)
	}

	return ret, nil
}
