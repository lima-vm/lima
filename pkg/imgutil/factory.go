// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package imgutil

import (
	"github.com/lima-vm/lima/pkg/imgutil/proxyimgutil"
)

// NewImageUtil returns an appropriate implementation of the Interface
// based on system capabilities. If qemu-img is available, it returns
// a QEMU-based implementation, otherwise it falls back to the native
// implementation.
func NewImageUtil(format string) (proxyimgutil.Interface, proxyimgutil.InfoProvider, error) {
	return proxyimgutil.NewProxyImageUtil(format)
}
