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

package entitlementutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lima-vm/lima/pkg/uiutil"

	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
)

// IsSigned returns an error if the binary is not signed, or the sign is invalid,
// or not associated with the "com.apple.security.hypervisor" entitlement.
func IsSigned(qExe string) error {
	cmd := exec.Command("codesign", "--verify", qExe)
	out, err := cmd.CombinedOutput()
	logrus.WithError(err).Debugf("Executed %v: out=%q", cmd.Args, string(out))
	if err != nil {
		return fmt.Errorf("failed to run %v: %w (out=%q)", cmd.Args, err, string(out))
	}

	cmd = exec.Command("codesign", "--display", "--entitlements", "-", "--xml", qExe)
	out, err = cmd.CombinedOutput()
	logrus.WithError(err).Debugf("Executed %v: out=%q", cmd.Args, string(out))
	if err != nil {
		return fmt.Errorf("failed to run %v: %w (out=%q)", cmd.Args, err, string(out))
	}
	if !strings.Contains(string(out), "com.apple.security.hypervisor") {
		return fmt.Errorf("binary %q seems signed but lacking the \"com.apple.security.hypervisor\" entitlement", qExe)
	}
	return nil
}

func Sign(qExe string) error {
	ent, err := os.CreateTemp("", "lima-qemu-entitlements-*.xml")
	if err != nil {
		return fmt.Errorf("failed to create a temporary file for signing QEMU binary: %w", err)
	}
	entName := ent.Name()
	defer os.RemoveAll(entName)
	const entXML = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>com.apple.security.hypervisor</key>
    <true/>
  </dict>
</plist>`
	if _, err = ent.WriteString(entXML); err != nil {
		ent.Close()
		return fmt.Errorf("failed to write to a temporary file %q for signing QEMU binary: %w", entName, err)
	}
	ent.Close()
	signCmd := exec.Command("codesign", "--sign", "-", "--entitlements", entName, "--force", qExe)
	out, err := signCmd.CombinedOutput()
	logrus.WithError(err).Debugf("Executed %v: out=%q", signCmd.Args, string(out))
	if err != nil {
		return fmt.Errorf("failed to run %v: %w (out=%q)", signCmd.Args, err, string(out))
	}
	return nil
}

// isColimaWrapper__useThisFunctionOnlyForPrintingHints returns true
// if qExe is like "/Users/<USER>/.colima/_wrapper/4e1b408f843d1c63afbbdcf80c40e4c88d33509f/bin/qemu-system-x86_64".
//
// The result can be used *ONLY* for controlling hint messages.
// DO NOT change the behavior of Lima depending on this result.
//
//nolint:revive // underscores in this function name intentionally added
func isColimaWrapper__useThisFunctionOnlyForPrintingHints__(qExe string) bool {
	return strings.Contains(qExe, "/.colima/_wrapper/")
}

// AskToSignIfNotSignedProperly asks to sign the QEMU binary with the "com.apple.security.hypervisor" entitlement.
//
// On Homebrew, QEMU binaries are usually already signed, but Homebrew's signing infrastructure is broken for Intel as of August 2023.
// https://github.com/lima-vm/lima/issues/1742
func AskToSignIfNotSignedProperly(qExe string) {
	if isSignedErr := IsSigned(qExe); isSignedErr != nil {
		logrus.WithError(isSignedErr).Warnf("QEMU binary %q does not seem properly signed with the \"com.apple.security.hypervisor\" entitlement", qExe)
		if isColimaWrapper__useThisFunctionOnlyForPrintingHints__(qExe) {
			logrus.Info("Hint: the warning above is usually negligible for colima ( Printed due to https://github.com/abiosoft/colima/issues/796 )")
		}
		var ans bool
		if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			message := fmt.Sprintf("Try to sign %q with the \"com.apple.security.hypervisor\" entitlement?", qExe)
			var askErr error
			ans, askErr = uiutil.Confirm(message, true)
			if askErr != nil {
				logrus.WithError(askErr).Warn("No answer was given")
			}
		}
		if ans {
			if signErr := Sign(qExe); signErr != nil {
				logrus.WithError(signErr).Warnf("Failed to sign %q", qExe)
			} else {
				logrus.Infof("Successfully signed %q with the \"com.apple.security.hypervisor\" entitlement", qExe)
			}
		} else {
			logrus.Warn("If QEMU does not start up, you may have to sign the QEMU binary with the \"com.apple.security.hypervisor\" entitlement manually. See https://github.com/lima-vm/lima/issues/1742 .")
		}
	} else {
		logrus.Infof("QEMU binary %q seems properly signed with the \"com.apple.security.hypervisor\" entitlement", qExe)
	}
}
