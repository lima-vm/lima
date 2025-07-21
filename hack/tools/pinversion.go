//go:build tools

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package tools is used to explicitly pin tool versions.
// It's needed to work around @dependabot's lack of upgrading indirect dependencies.
package tools

import (
	_ "github.com/containerd/ltag"
	_ "github.com/golangci/golangci-lint/v2/pkg/exitcodes"
	_ "github.com/yoheimuta/protolint/lib"
	_ "google.golang.org/grpc"
	_ "google.golang.org/protobuf/proto"
	_ "mvdan.cc/sh/v3/pattern"
)
