// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vmnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	vzvmnet "github.com/Code-Hex/vz/v3/vmnet"
	"github.com/sirupsen/logrus"
)

// streamFileAdaptorForInterface returns a file for the given vmnet network.
func streamFileAdaptorForInterface(ctx context.Context, iface *vzvmnet.Interface, opts ...vzvmnet.Sockopt) (file *os.File, forwarder func(), err error) {
	var errCh <-chan error
	if file, forwarder, errCh, err = vzvmnet.StreamFileAdaptorForInterface(ctx, iface, opts...); err != nil {
		return nil, nil, fmt.Errorf("StreamFileAdaptorForInterface failed: %w", err)
	}
	go logErrors(ctx, errCh, "[vmnet] StreamFileAdaptorForInterface error")
	return file, forwarder, nil
}

// datagramFileAdaptorForInterface returns a file for the given vmnet network.
func datagramFileAdaptorForInterface(ctx context.Context, iface *vzvmnet.Interface, opts ...vzvmnet.Sockopt) (file *os.File, forwarder func(), err error) {
	var errCh <-chan error
	if file, forwarder, errCh, err = vzvmnet.DatagramFileAdaptorForInterface(ctx, iface, opts...); err != nil {
		return nil, nil, fmt.Errorf("DatagramFileAdaptorForInterface failed: %w", err)
	}
	go logErrors(ctx, errCh, "[vmnet] DatagramFileAdaptorForInterface error")
	return file, forwarder, nil
}

// datagramNextFileAdaptorForInterface returns a file for the given vmnet network.
func datagramNextFileAdaptorForInterface(ctx context.Context, iface *vzvmnet.Interface, opts ...vzvmnet.Sockopt) (file *os.File, forwarder func(), err error) {
	var errCh <-chan error
	if file, forwarder, errCh, err = vzvmnet.DatagramNextFileAdaptorForInterface(ctx, iface, opts...); err != nil {
		return nil, nil, fmt.Errorf("DatagramNextFileAdaptorForInterface failed: %w", err)
	}
	go logErrors(ctx, errCh, "[vmnet] DatagramNextFileAdaptorForInterface error")
	return file, forwarder, nil
}

// logErrors logs errors received from errCh until the context is done.
func logErrors(ctx context.Context, errCh <-chan error, message string) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-errCh:
			if !ok {
				return
			}
			if errors.Is(e, io.EOF) || errors.Is(e, syscall.ECONNRESET) {
				// normal closure
				continue
			}
			logrus.WithError(e).Error(message)
		}
	}
}
