// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package proxyimgutil

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/imgutil/nativeimgutil"
)

// NewProxyImageUtil returns a proxy implementation for raw and qcow2 image formats.
func NewProxyImageUtil(format string) (Interface, InfoProvider, error) {
	switch format {
	case "raw":
		return nativeimgutil.NewNativeImageUtil(), nativeimgutil.NewNativeInfoProvider(), nil
	case "qcow2":
		return nativeimgutil.NewNativeImageUtil(), nativeimgutil.NewNativeInfoProvider(), nil
	default:
		return nil, nil, fmt.Errorf("unsupported image format: %s", format)
	}
}
