//go:build !external_hcs

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

// Import hcs driver to register it in the registry on windows.
import _ "github.com/lima-vm/lima/v2/pkg/driver/hcs"
