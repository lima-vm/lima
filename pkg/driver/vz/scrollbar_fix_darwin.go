// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

// This file includes scrollbar_fix_darwin.m via cgo.
// See that file for documentation on why this patch is needed.

package vz

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -lobjc
#include "scrollbar_fix_darwin.m"
*/
import "C"
