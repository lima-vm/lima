// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// fakeDev completes transfers at once, or holds them until cancel when stalled.
type fakeDev struct{ stalled bool }

func (fakeDev) Info() DeviceInfo { return DeviceInfo{Busid: "1-1"} }

func (fakeDev) Control(_ context.Context, _ [8]byte, b []byte) (int, error) { return len(b), nil }

func (d fakeDev) Transfer(ctx context.Context, _ uint8, _ bool, b []byte) (int, error) {
	if d.stalled {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	return len(b), nil
}

func (fakeDev) OutMaxPacket(uint8) int { return 0 }

func (fakeDev) Gone() bool { return false }

func (fakeDev) Close() error { return nil }

func TestCancelledSubmitSkipsDevice(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var dev countDev
	status, _, _ := doSubmit(ctx, &dev, urbHeader{Direction: dirIn, Ep: 1, TransferBufferLength: 8}, nil)
	assert.Equal(t, status, int32(-errnoECONNRESET))
	status, _, _ = doSubmit(ctx, &dev, urbHeader{Direction: dirIn, TransferBufferLength: 8}, nil)
	assert.Equal(t, status, int32(-errnoECONNRESET))
	assert.Equal(t, dev.calls.Load(), int32(0))
}

type countDev struct {
	fakeDev
	calls atomic.Int32
}

func (d *countDev) Control(context.Context, [8]byte, []byte) (int, error) {
	d.calls.Add(1)
	return 0, nil
}

func (d *countDev) Transfer(context.Context, uint8, bool, []byte) (int, error) {
	d.calls.Add(1)
	return 0, nil
}

type errDev struct {
	fakeDev
	err error
}

func (d errDev) Transfer(context.Context, uint8, bool, []byte) (int, error) { return 0, d.err }

func TestSubmitErrorStatus(t *testing.T) {
	h := urbHeader{Direction: dirIn, Ep: 1, TransferBufferLength: 8}
	for name, tc := range map[string]struct {
		err  error
		want int32
	}{
		"typed": {fmt.Errorf("wrapped: %w", urbErrno(errnoETIME)), -errnoETIME},
		"local": {errors.New("no pipe"), -errnoEPIPE},
		"gone":  {fmt.Errorf("x: %w", ErrDeviceGone), -errnoENODEV},
	} {
		t.Run(name, func(t *testing.T) {
			status, _, _ := doSubmit(t.Context(), errDev{err: tc.err}, h, nil)
			assert.Equal(t, status, tc.want)
		})
	}

	t.Run("cancelled mid call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		dev := cancelDev{cancel: cancel}
		status, _, _ := doSubmit(ctx, dev, h, nil)
		assert.Equal(t, status, int32(-errnoECONNRESET))
	})
}

type cancelDev struct {
	fakeDev
	cancel context.CancelFunc
}

func (d cancelDev) Transfer(context.Context, uint8, bool, []byte) (int, error) {
	d.cancel()
	return 0, urbErrno(errnoEPROTO)
}

type zlpDev struct {
	fakeDev
	mps    int
	zlpErr error
	block  bool
	lens   []int
}

func (d *zlpDev) OutMaxPacket(uint8) int { return d.mps }

func (d *zlpDev) Transfer(ctx context.Context, _ uint8, _ bool, b []byte) (int, error) {
	d.lens = append(d.lens, len(b))
	if len(b) > 0 {
		return len(b), nil
	}
	if d.block {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return 0, d.zlpErr
}

func TestZeroPacket(t *testing.T) {
	for name, tc := range map[string]struct {
		dir   uint32
		flags uint32
		size  int
		mps   int
		want  []int
	}{
		"no flag":        {dirOut, 0, 512, 512, []int{512}},
		"one packet":     {dirOut, urbZeroPacket, 512, 512, []int{512, 0}},
		"two packets":    {dirOut, urbZeroPacket, 1024, 512, []int{1024, 0}},
		"short packet":   {dirOut, urbZeroPacket, 100, 512, []int{100}},
		"empty":          {dirOut, urbZeroPacket, 0, 512, []int{0}},
		"unknown packet": {dirOut, urbZeroPacket, 512, 0, []int{512}},
		"in":             {dirIn, urbZeroPacket, 512, 512, []int{512}},
	} {
		t.Run(name, func(t *testing.T) {
			dev := &zlpDev{mps: tc.mps}
			h := urbHeader{Direction: tc.dir, Ep: 2, TransferFlags: tc.flags, TransferBufferLength: int32(tc.size)}
			status, actual, _ := doSubmit(t.Context(), dev, h, make([]byte, tc.size))
			assert.Equal(t, status, int32(0))
			assert.Equal(t, actual, int32(tc.size), "the terminator must not change actual_length")
			assert.DeepEqual(t, dev.lens, tc.want)
		})
	}

	t.Run("terminator failure is reported", func(t *testing.T) {
		dev := &zlpDev{mps: 64, zlpErr: urbErrno(errnoEPROTO)}
		h := urbHeader{Direction: dirOut, Ep: 2, TransferFlags: urbZeroPacket, TransferBufferLength: 64}
		status, _, _ := doSubmit(t.Context(), dev, h, make([]byte, 64))
		assert.Equal(t, status, int32(-errnoEPROTO))
	})

	t.Run("cancel releases a blocked terminator", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		h := urbHeader{Direction: dirOut, Ep: 2, TransferFlags: urbZeroPacket, TransferBufferLength: 64}
		status, _, _ := doSubmit(ctx, &zlpDev{mps: 64, block: true}, h, make([]byte, 64))
		assert.Equal(t, status, int32(-errnoECONNRESET))
	})
}
