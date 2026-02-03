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
	vzfileadapter "github.com/Code-Hex/vz/v3/vmnet/fileadapter"
	vzdatagram "github.com/Code-Hex/vz/v3/vmnet/fileadapter/datagram"
	vzdatagramx "github.com/Code-Hex/vz/v3/vmnet/fileadapter/datagramx"
	vzstream "github.com/Code-Hex/vz/v3/vmnet/fileadapter/stream"
	"github.com/sirupsen/logrus"
)

// datagramFileAdapterForInterface returns a file for the given vmnet network.
func datagramFileAdapterForInterface(ctx context.Context, iface *vzvmnet.Interface, opts ...vzfileadapter.Sockopt) (file *os.File, forwarder func(), err error) {
	var errCh <-chan error
	if file, forwarder, errCh, err = vzdatagram.FileAdapterForInterface(ctx, iface, opts...); err != nil {
		return nil, nil, fmt.Errorf("vzdatagram.FileAdapterForInterface failed: %w", err)
	}
	go logErrors(ctx, errCh, "[vmnet] vzdatagram.FileAdapterForInterface")
	return file, forwarder, nil
}

// datagramNextFileAdapterForInterface returns a file for the given vmnet network.
func datagramNextFileAdapterForInterface(ctx context.Context, iface *vzvmnet.Interface, opts ...vzfileadapter.Sockopt) (file *os.File, forwarder func(), err error) {
	var errCh <-chan error
	if file, forwarder, errCh, err = vzdatagramx.FileAdapterForInterface(ctx, iface, opts...); err != nil {
		return nil, nil, fmt.Errorf("vzdatagramx.FileAdapterForInterface failed: %w", err)
	}
	go logErrors(ctx, errCh, "[vmnet] vzdatagramx.FileAdapterForInterface")
	return file, forwarder, nil
}

// streamFileAdapterForInterface returns a file for the given vmnet network.
func streamFileAdapterForInterface(ctx context.Context, iface *vzvmnet.Interface, opts ...vzfileadapter.Sockopt) (file *os.File, forwarder func(), err error) {
	var errCh <-chan error
	if file, forwarder, errCh, err = vzstream.FileAdapterForInterface(ctx, iface, opts...); err != nil {
		return nil, nil, fmt.Errorf("vzstream.FileAdapterForInterface failed: %w", err)
	}
	go logErrors(ctx, errCh, "[vmnet] vzstream.FileAdapterForInterface")
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
				logrus.WithError(e).Info(message + " connection closed")
				continue
			}
			logrus.WithError(e).Error(message + " error")
		}
	}
}
