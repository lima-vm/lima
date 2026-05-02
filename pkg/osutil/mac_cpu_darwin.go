//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"os/exec"
	"strings"
	"sync"
)

var isAppleSiliconM4OrNewer = sync.OnceValue(func() bool {
	cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	b, err := cmd.Output()
	if err != nil {
		return false
	}
	brand := strings.TrimSpace(string(b))
	if strings.Contains(brand, "Apple M4") || strings.Contains(brand, "Apple M5") || strings.Contains(brand, "Apple M6") || strings.Contains(brand, "Apple M7") || strings.Contains(brand, "Apple M8") || strings.Contains(brand, "Apple M9") {
		return true
	}
	return false
})

// IsAppleSiliconM4OrNewer returns true if the host CPU is Apple M4 or newer.
func IsAppleSiliconM4OrNewer() bool {
	return isAppleSiliconM4OrNewer()
}
