// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"runtime"
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

func startSession(t *testing.T, dev Device) (c net.Conn, done <-chan error) {
	t.Helper()
	return startSessionCtx(t.Context(), t, dev)
}

// startSessionCtx ends the session from the host side when ctx ends.
func startSessionCtx(ctx context.Context, t *testing.T, dev Device) (c net.Conn, done <-chan error) {
	t.Helper()
	c, s := net.Pipe()
	t.Cleanup(func() { _ = c.Close() })
	assert.NilError(t, c.SetDeadline(time.Now().Add(30*time.Second)))
	ch := make(chan error, 1)
	go func() { ch <- urbLoop(ctx, s, dev) }()
	return c, ch
}

func sessionEnd(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		assert.Assert(t, false, "urbLoop stuck")
		return nil
	}
}

func init() {
	// Every session end drains, so keep it short.
	drainQuiet, drainMax = 50*time.Millisecond, time.Second
}

func send(t *testing.T, c net.Conn, h urbHeader) {
	t.Helper()
	assert.NilError(t, binary.Write(c, binary.BigEndian, h))
}

func TestSubmitRejectsInvalid(t *testing.T) {
	for name, h := range map[string]urbHeader{
		"direction":     {Direction: 2, Ep: 1, TransferBufferLength: 64},
		"ep":            {Direction: dirIn, Ep: 16, TransferBufferLength: 4},
		"negative in":   {Direction: dirIn, Ep: 1, TransferBufferLength: -1},
		"negative out":  {Direction: dirOut, Ep: 1, TransferBufferLength: -1},
		"negative ep0":  {Direction: dirIn, TransferBufferLength: math.MinInt32},
		"oversize":      {Direction: dirIn, Ep: 1, TransferBufferLength: maxTransferLength + 1},
		"iso oversized": {Direction: dirIn, Ep: 1, NumberOfPackets: maxISOPackets + 1},
	} {
		t.Run(name, func(t *testing.T) {
			c, done := startSession(t, fakeDev{})
			h.Command = cmdSubmit
			send(t, c, h)
			assert.ErrorContains(t, sessionEnd(t, done), "invalid urb")
		})
	}
}

func TestSubmitAccepts(t *testing.T) {
	for name, h := range map[string]urbHeader{
		"max ep":       {Direction: dirIn, Ep: 15, TransferBufferLength: 4096},
		"ep0 empty":    {Direction: dirIn},
		"zlp out":      {Direction: dirOut, Ep: 15},
		"zlp in":       {Direction: dirIn, Ep: 1},
		"max length":   {Direction: dirIn, Ep: 1, TransferBufferLength: maxTransferLength},
		"spec non-iso": {Direction: dirIn, Ep: 1, TransferBufferLength: 8, NumberOfPackets: -1},
	} {
		t.Run(name, func(t *testing.T) {
			c, done := startSession(t, fakeDev{})
			h.Command, h.Seqnum = cmdSubmit, 7
			send(t, c, h)
			r, err := readURBHeader(c)
			assert.NilError(t, err)
			assert.Equal(t, r.Command, uint32(retSubmit))
			assert.Equal(t, r.Seqnum, uint32(7))
			assert.Equal(t, r.TransferFlags, uint32(0))
			assert.Equal(t, r.TransferBufferLength, h.TransferBufferLength)
			_, err = io.CopyN(io.Discard, c, int64(r.TransferBufferLength))
			assert.NilError(t, err)
			assert.NilError(t, c.Close())
			assert.NilError(t, sessionEnd(t, done))
		})
	}
}

func TestFloodEndsSession(t *testing.T) {
	for name, h := range map[string]urbHeader{
		"headers":     {Command: cmdSubmit, Direction: dirIn, Ep: 1},
		"out payload": {Command: cmdSubmit, Direction: dirOut, Ep: 1, TransferBufferLength: maxTransferLength},
		"unlinks":     {Command: cmdUnlink, TransferFlags: 1},
	} {
		t.Run(name, func(t *testing.T) {
			c, done := startSession(t, fakeDev{stalled: true})
			for range maxBacklog / urbCost(h) {
				h.Seqnum++
				send(t, c, h)
				if h.Command == cmdSubmit && h.Direction == dirOut {
					_, err := c.Write(make([]byte, h.TransferBufferLength))
					assert.NilError(t, err)
				}
			}
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			assert.Assert(t, m.HeapAlloc < 80<<20, "HeapAlloc %d MiB", m.HeapAlloc>>20)
			h.Seqnum++
			send(t, c, h)
			assert.ErrorContains(t, sessionEnd(t, done), "backlog")
		})
	}
}

// One seqnum throughout: a reply frees it too.
func TestBacklogReleasedAfterReply(t *testing.T) {
	c, done := startSession(t, fakeDev{})
	h := urbHeader{Command: cmdSubmit, Seqnum: 7, Direction: dirIn, Ep: 1, TransferBufferLength: 4 << 20}
	for range 2 * maxBacklog / int(h.TransferBufferLength) {
		send(t, c, h)
		r, err := readURBHeader(c)
		assert.NilError(t, err)
		assert.Equal(t, r.Command, uint32(retSubmit))
		_, err = io.CopyN(io.Discard, c, int64(r.TransferBufferLength))
		assert.NilError(t, err)
	}
	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
}

func TestDuplicateSeqnumEndsSession(t *testing.T) {
	c, done := startSession(t, fakeDev{stalled: true})
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 7, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 7, Direction: dirIn, Ep: 2, TransferBufferLength: 8})
	assert.ErrorContains(t, sessionEnd(t, done), "duplicate seqnum 7")
}

func TestBadCommandWithSilentGuest(t *testing.T) {
	c, done := startSession(t, fakeDev{})
	send(t, c, urbHeader{Command: cmdSubmit, Direction: dirIn, Ep: 1, TransferBufferLength: 64})
	send(t, c, urbHeader{Command: 0xdeadbeef})
	assert.ErrorContains(t, sessionEnd(t, done), "unsupported urb command")
}

func TestIsoURBKeepsSession(t *testing.T) {
	for name, dir := range map[string]uint32{"in": dirIn, "out": dirOut} {
		t.Run(name, func(t *testing.T) {
			c, done := startSession(t, fakeDev{})
			const np = 3
			desc := bytes.Repeat([]byte{0, 0, 0, 1, 0, 0, 0, 192, 0, 0, 0, 0, 0, 0, 0, 0}, np)
			send(t, c, urbHeader{
				Command: cmdSubmit, Seqnum: 1, Direction: dir, Ep: 1,
				TransferBufferLength: np * 192, NumberOfPackets: np,
			})
			if dir == dirOut {
				_, err := c.Write(make([]byte, np*192))
				assert.NilError(t, err)
			}
			_, err := c.Write(desc)
			assert.NilError(t, err)
			send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 2, Direction: dirIn})

			for range 2 {
				r, err := readURBHeader(c)
				assert.NilError(t, err)
				if r.Seqnum == 2 {
					assert.Equal(t, r.TransferFlags, uint32(0))
					continue
				}
				assert.Equal(t, int32(r.TransferFlags), int32(-errnoESHUTDOWN))
				assert.Equal(t, r.TransferBufferLength, int32(0))
				assert.Equal(t, r.NumberOfPackets, int32(np))
				got := make([]byte, len(desc))
				_, err = io.ReadFull(c, got)
				assert.NilError(t, err)
				assert.DeepEqual(t, got, desc)
			}
			assert.NilError(t, c.Close())
			assert.NilError(t, sessionEnd(t, done))
		})
	}
}

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
