// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package hclstub is a stub replacement for github.com/hashicorp/hcl/v2.
// It exists only to satisfy module resolution when building with tags
// that exclude HCL-dependent code paths.
package hclstub
