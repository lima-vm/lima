// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sudoers

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

const testFragment = "%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: /bin/mkdir -m 775 -p /private/var/run/lima\n"

func writeSudoersFile(t *testing.T, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "lima.sudoers")
	assert.NilError(t, os.WriteFile(file, []byte(content), 0o600))
	return file
}

func TestVerifyFragmentAllowsAdditionalFragments(t *testing.T) {
	file := writeSudoersFile(t, testFragment+
		"alice ALL=(root:wheel) NOPASSWD:NOSETENV: /opt/lima/libexec/lima/privileged/lima-privileged-block-device /dev/rdisk2\n")
	hint := RegenerateHint("/opt/lima/bin/limactl", file, []string{"/dev/rdisk2"})

	assert.NilError(t, VerifyFragment(t.Context(), file, testFragment, hint))

	err := VerifyFragment(t.Context(), file, strings.Replace(testFragment, "everyone", "admin", 1), hint)
	assert.ErrorContains(t, err, "out of sync")
	assert.ErrorContains(t, err, "--block-device")
	assert.ErrorContains(t, err, "revokes")
}

func TestVerifyFragmentRejectsBroadenedGrant(t *testing.T) {
	file := writeSudoersFile(t, strings.Replace(testFragment, "NOPASSWD:NOSETENV", "NOPASSWD", 1))

	err := VerifyFragment(t.Context(), file, testFragment, "hint")
	assert.ErrorContains(t, err, "out of sync")
}

func TestVerifyFragmentRejectsCommentedFragment(t *testing.T) {
	file := writeSudoersFile(t, "# "+strings.ReplaceAll(testFragment, "\n", "\n# "))

	err := VerifyFragment(t.Context(), file, testFragment, "hint")
	assert.ErrorContains(t, err, "out of sync")
}

func TestVerifyFragmentToleratesUnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("the file mode does not keep the caller out")
	}
	file := writeSudoersFile(t, testFragment)
	assert.NilError(t, os.Chmod(file, 0o000))

	assert.NilError(t, VerifyFragment(t.Context(), file, testFragment, "hint"))
}
