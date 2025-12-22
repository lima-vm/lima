//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vmnet

import (
	"context"
	"errors"
	"os"
)

func RegisterMachService(context.Context) error {
	return errors.New("RegisterMachService is only supported on darwin")
}

func RequestQEMUDatagramFileDescriptorForNetwork(context.Context, string) (*os.File, error) {
	return nil, errors.New("RequestQEMUDatagramFileDescriptorForNetwork is only supported on darwin")
}

func RequestQEMUDatagramNextFileDescriptorForNetwork(context.Context, string) (*os.File, error) {
	return nil, errors.New("RequestQEMUDatagramNextFileDescriptorForNetwork is only supported on darwin")
}

func RequestQEMUStreamFileDescriptorForNetwork(context.Context, string) (*os.File, error) {
	return nil, errors.New("RequestQEMUStreamFileDescriptorForNetwork is only supported on darwin")
}
