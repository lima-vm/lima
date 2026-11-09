// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
)

// Linux numbers: the guest reads them, and macOS ones differ.
const (
	errnoENODEV     = 19
	errnoEPIPE      = 32
	errnoETIME      = 62
	errnoEPROTO     = 71
	errnoECONNRESET = 104
	errnoESHUTDOWN  = 108
)

type urbErrno int32

func (e urbErrno) Error() string { return fmt.Sprintf("errno %d", int32(e)) }

func doSubmit(ctx context.Context, dev Device, h urbHeader, outData []byte) (status, actual int32, data []byte) {
	if ctx.Err() != nil {
		return -errnoECONNRESET, 0, nil
	}

	// No iso support. -ESHUTDOWN stops snd-usb resubmitting, unlike -EOPNOTSUPP.
	if h.NumberOfPackets > 0 {
		return -errnoESHUTDOWN, 0, nil
	}

	in := h.Direction == dirIn
	buf := outData
	if in {
		buf = make([]byte, h.TransferBufferLength)
	}

	var (
		n   int
		err error
	)
	if h.Ep == 0 {
		n, err = dev.Control(ctx, h.Setup, buf)
	} else {
		ep := uint8(h.Ep)
		n, err = dev.Transfer(ctx, ep, in, buf)
		if !in && err == nil && h.TransferFlags&urbZeroPacket != 0 && len(buf) > 0 {
			if mps := dev.OutMaxPacket(ep); mps > 0 && len(buf)%mps == 0 {
				_, err = dev.Transfer(ctx, ep, false, nil)
			}
		}
	}

	if err != nil {
		if ctx.Err() != nil {
			return -errnoECONNRESET, 0, nil
		}
		if errors.Is(err, ErrDeviceGone) {
			logrus.WithError(err).Debugf("usbip: device gone on ep %d", h.Ep)
			return -errnoENODEV, 0, nil
		}
		logrus.WithError(err).Debugf("usbip: transfer failed on ep %d", h.Ep)
		if e, ok := errors.AsType[urbErrno](err); ok {
			return -int32(e), 0, nil
		}
		return -errnoEPIPE, 0, nil
	}
	return 0, int32(n), buf
}
