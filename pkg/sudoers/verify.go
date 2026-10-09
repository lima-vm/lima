// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sudoers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// RunAs is an identity a generated grant is exercised as.
type RunAs struct {
	User  string
	Group string
}

// VerifyPasswordLess flushes the cached sudo credentials and then checks that
// every runAs identity is usable without a password, e.g. because the host has
// a blanket "%admin ALL = (ALL:ALL) NOPASSWD: ALL" rule.
func VerifyPasswordLess(ctx context.Context, runAs ...RunAs) error {
	cmd := exec.CommandContext(ctx, "sudo", "-k")
	logrus.Infof("Running: %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: %w", cmd.Args, err)
	}
	for _, as := range runAs {
		if err := Run(ctx, as.User, as.Group, nil, nil, nil, "", "true"); err != nil {
			return err
		}
	}
	return nil
}

// VerifyFragment checks that file still grants fragment. A missing file is
// accepted as long as every runAs identity works without a password anyway,
// because otherwise the file could never be bootstrapped.
func VerifyFragment(ctx context.Context, file, fragment, hint string, runAs ...RunAs) error {
	content, ok, err := ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := VerifyPasswordLess(ctx, runAs...); err == nil {
				logrus.Debugf("%#q does not exist, but sudo doesn't seem to require a password", file)
				return nil
			}
			logrus.Debugf("%#q does not exist, and sudo requires a password: %s", file, err)
		}
		return fmt.Errorf("can't read %#q: %w: (Hint: %s)", file, err, hint)
	}
	if !ok {
		return nil
	}
	// The file may carry grants from other Lima features, or from other users.
	// Only this fragment has to be present verbatim.
	if !ContainsActiveFragment(content, fragment) {
		return fmt.Errorf("sudoers file %#q is out of sync and must be regenerated (Hint: %s)", file, hint)
	}
	return nil
}

// RegenerateHint is the single wording for re-creating a sudoers file, so that
// every caller keeps advising the same ownership and mode.
func RegenerateHint(exe, file string, blockDevices []string) string {
	args := "sudoers"
	if len(blockDevices) > 0 {
		args += " --block-device=" + strings.Join(blockDevices, ",")
	}
	install := "sudo install -o root -g wheel -m 0444"
	if runtime.GOOS != "darwin" {
		// "wheel" does not exist on every Linux distribution.
		install = "sudo install -o root -m 0440"
	}
	return fmt.Sprintf("run `%s %s >etc_sudoers.d_lima && %s etc_sudoers.d_lima %q`; %s",
		exe, args, install, file, RegenerateGrantWarning)
}
