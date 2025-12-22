//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vmnetinterface

import (
	"context"
	"errors"
	"os"
)

func FileDescriptorForNetwork(_ context.Context, _ string) (*os.File, error) {
	return nil, errors.New("FileDescriptorForNetwork is only supported on darwin")
}
