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
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

// fakeDev completes transfers at once, or holds them until cancel when stalled.
type fakeDev struct{ stalled bool }

func (fakeDev) Info() DeviceInfo { return DeviceInfo{BusID: "1-1"} }

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

func readToEOF(t *testing.T, c net.Conn) {
	t.Helper()
	r, err := readURBHeader(c)
	assert.Assert(t, errors.Is(err, io.EOF), "frame %+v after the drain, err %v", r, err)
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
	// A session the host ends drains for all of drainMax, so keep it short.
	drainMax = time.Second
}

func send(t *testing.T, c net.Conn, h urbHeader) {
	t.Helper()
	assert.NilError(t, binary.Write(c, binary.BigEndian, h))
}

// assertHeapBelow fails when the live heap reaches limit bytes.
func assertHeapBelow(t *testing.T, limit uint64) {
	t.Helper()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	assert.Assert(t, m.HeapAlloc < limit, "HeapAlloc %d MiB", m.HeapAlloc>>20)
}

// sendOut submits an OUT URB on endpoint 1.
func sendOut(t *testing.T, c net.Conn, seq uint32, payload []byte) {
	t.Helper()
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: seq, Direction: dirOut, Ep: 1, TransferBufferLength: int32(len(payload))})
	_, err := c.Write(payload)
	assert.NilError(t, err)
}

// unlinkReply reads the answer to a CMD_UNLINK that found URB seq: its RET_SUBMIT, then RET_UNLINK 0.
func unlinkReply(t *testing.T, c net.Conn, seq, unlinkSeq uint32, in bool) urbHeader {
	t.Helper()
	r, err := readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Command, uint32(retSubmit), "RET_UNLINK before RET_SUBMIT")
	assert.Equal(t, r.Seqnum, seq)
	if in {
		_, err = io.CopyN(io.Discard, c, int64(r.TransferBufferLength))
		assert.NilError(t, err)
	}
	u, err := readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, u.Command, uint32(retUnlink))
	assert.Equal(t, u.Seqnum, unlinkSeq)
	assert.Equal(t, u.TransferFlags, uint32(0))
	return r
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
			assertHeapBelow(t, 80<<20)
			h.Seqnum++
			send(t, c, h)
			assert.ErrorContains(t, sessionEnd(t, done), "backlog")
		})
	}
}

// Overflowing the backlog ends the session, but the guest still gets every URB back.
func TestBacklogOverflowAnswersEveryURB(t *testing.T) {
	c, done := startSession(t, fakeDev{stalled: true})
	in := urbHeader{Command: cmdSubmit, Direction: dirIn, Ep: 1, TransferBufferLength: maxTransferLength}
	n := uint32(maxBacklog / urbCost(in))
	for seq := range n {
		in.Seqnum = seq + 1
		send(t, c, in)
	}
	// This one overflows. Its payload is discarded, and the frame after it still parses.
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: n + 1, Direction: dirOut, Ep: 2, TransferBufferLength: maxTransferLength})
	_, err := c.Write(make([]byte, maxTransferLength))
	assert.NilError(t, err)
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: n + 2, Direction: dirIn, Ep: 1, TransferBufferLength: 8})

	got := map[uint32]int32{}
	for range n + 2 {
		r, err := readURBHeader(c)
		assert.NilError(t, err)
		assert.Equal(t, r.Command, uint32(retSubmit))
		got[r.Seqnum] = int32(r.TransferFlags)
	}
	assert.Equal(t, len(got), int(n+2))
	for seq, status := range got {
		assert.Equal(t, status, int32(-errnoESHUTDOWN), "seq %d", seq)
	}
	readToEOF(t, c)
	assert.ErrorContains(t, sessionEnd(t, done), "backlog")
}

// Over the cap, reads wait until a reply frees the backlog, whichever command overflowed it.
func TestBacklogOverflowPausesReads(t *testing.T) {
	for name, last := range map[string]urbHeader{
		"submit": {Command: cmdSubmit, Direction: dirIn, Ep: 2, TransferBufferLength: 8},
		"unlink": {Command: cmdUnlink, TransferFlags: 999},
	} {
		t.Run(name, func(t *testing.T) {
			c, done := startSession(t, fakeDev{stalled: true})
			big := urbHeader{Command: cmdSubmit, Direction: dirIn, Ep: 1, TransferBufferLength: maxTransferLength}
			n := uint32(maxBacklog / urbCost(big))
			for seq := range n {
				big.Seqnum = seq + 1
				send(t, c, big)
			}
			// Leave less room than the last frame costs.
			fill := maxBacklog - int64(n)*urbCost(big) - urbCost(urbHeader{}) - 1000
			send(t, c, urbHeader{Command: cmdSubmit, Seqnum: n + 1, Direction: dirIn, Ep: 1, TransferBufferLength: int32(fill)})
			last.Seqnum = n + 2
			send(t, c, last)

			assert.NilError(t, c.SetWriteDeadline(time.Now().Add(200*time.Millisecond)))
			next := urbHeader{Command: cmdSubmit, Seqnum: n + 3, Direction: dirIn, Ep: 3, TransferBufferLength: 8}
			err := binary.Write(c, binary.BigEndian, next)
			assert.Assert(t, errors.Is(err, os.ErrDeadlineExceeded), "a read went on over the cap: %v", err)
			assert.NilError(t, c.SetWriteDeadline(time.Now().Add(30*time.Second)))

			got := map[uint32]bool{}
			r, err := readURBHeader(c)
			assert.NilError(t, err)
			got[r.Seqnum] = true
			send(t, c, next) // only a resumed read takes this
			for range n + 2 {
				r, err := readURBHeader(c)
				assert.NilError(t, err)
				got[r.Seqnum] = true
			}
			assert.Equal(t, len(got), int(n+3))
			readToEOF(t, c)
			assert.ErrorContains(t, sessionEnd(t, done), "backlog")
		})
	}
}

// Hung calls pin the backlog until the close, and the URB that overflowed it is still answered.
func TestBacklogOverflowAnswersBehindHungCalls(t *testing.T) {
	dev := newGateDev()
	c, done := startSession(t, dev)
	big := urbHeader{Command: cmdSubmit, Direction: dirIn, Ep: 1, TransferBufferLength: maxTransferLength}
	n := uint32(maxBacklog / urbCost(big))
	for seq := range n {
		big.Seqnum = seq + 1
		send(t, c, big)
	}
	<-dev.started
	// Leave less room than one more URB costs even with its payload discarded.
	fill := maxBacklog - int64(n)*urbCost(big) - urbCost(urbHeader{}) - 1000
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: n + 1, Direction: dirIn, Ep: 1, TransferBufferLength: int32(fill)})
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: n + 2, Direction: dirIn, Ep: 2, TransferBufferLength: 8})

	got := map[uint32]bool{}
	for range n + 2 {
		r, err := readURBHeader(c)
		assert.NilError(t, err)
		got[r.Seqnum] = true
	}
	assert.Equal(t, len(got), int(n+2))
	readToEOF(t, c)
	close(dev.release)
	assert.ErrorContains(t, sessionEnd(t, done), "backlog")
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

// gateDev reports each transfer on started and holds it until release closes, ignoring ctx.
type gateDev struct {
	fakeDev
	started chan []byte
	release chan struct{}
}

func newGateDev() gateDev {
	return gateDev{started: make(chan []byte, 64), release: make(chan struct{})}
}

func (d gateDev) Transfer(_ context.Context, _ uint8, _ bool, b []byte) (int, error) {
	d.started <- b
	<-d.release
	return len(b), nil
}

func (d gateDev) Control(ctx context.Context, _ [8]byte, b []byte) (int, error) {
	return d.Transfer(ctx, 0, false, b)
}

func TestQueuePerEndpoint(t *testing.T) {
	for name, tc := range map[string]struct {
		ep   uint32
		wait time.Duration
	}{
		"bulk directions independent": {1, 10 * time.Second},
		"ep0 serial":                  {0, 100 * time.Millisecond},
	} {
		t.Run(name, func(t *testing.T) {
			dev := newGateDev()
			c, done := startSession(t, dev)
			send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirIn, Ep: tc.ep})
			<-dev.started
			send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 2, Direction: dirOut, Ep: tc.ep})
			select {
			case <-dev.started:
				assert.Assert(t, tc.ep != 0, "ep0 OUT ran while IN was held")
			case <-time.After(tc.wait):
				assert.Assert(t, tc.ep == 0, "OUT blocked by pending IN")
			}
			close(dev.release)
			assert.NilError(t, c.Close())
			assert.NilError(t, sessionEnd(t, done))
		})
	}
}

type ctxDev struct {
	fakeDev
	started chan []byte
}

func (d ctxDev) Control(ctx context.Context, _ [8]byte, b []byte) (int, error) {
	return d.Transfer(ctx, 0, false, b)
}

func (d ctxDev) Transfer(ctx context.Context, _ uint8, _ bool, b []byte) (int, error) {
	d.started <- b
	<-ctx.Done()
	return 0, ctx.Err()
}

func TestUnlinkQueuedURB(t *testing.T) {
	dev := ctxDev{started: make(chan []byte, 64)}
	ctx, cancel := context.WithCancel(t.Context())
	c, done := startSessionCtx(ctx, t, dev)
	sendOut(t, c, 1, []byte{'a'})
	assert.DeepEqual(t, <-dev.started, []byte{'a'})
	sendOut(t, c, 2, []byte{'b'})

	send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 3, TransferFlags: 2})
	r := unlinkReply(t, c, 2, 3, false)
	assert.Equal(t, int32(r.TransferFlags), int32(-errnoECONNRESET))

	send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 4, TransferFlags: 2})
	u, err := readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, u.Command, uint32(retUnlink), "a repeat unlink answered seq 2 again")
	assert.Equal(t, u.Seqnum, uint32(4))

	send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 5, TransferFlags: 1})
	r = unlinkReply(t, c, 1, 5, false)
	assert.Equal(t, int32(r.TransferFlags), int32(-errnoECONNRESET))

	// A second answer for either URB would tear down the guest's connection.
	cancel()
	readToEOF(t, c)
	assert.NilError(t, sessionEnd(t, done))
	select {
	case b := <-dev.started:
		assert.Assert(t, false, "unlinked OUT reached device: %q", b)
	default:
	}
}

// An unlinked queued URB is gone at once: its seqnum is free, and the worker never sees it.
func TestUnlinkQueuedRetiresURB(t *testing.T) {
	dev := newGateDev()
	c, done := startSession(t, dev)
	sendOut(t, c, 1, []byte{'a'})
	assert.DeepEqual(t, <-dev.started, []byte{'a'})
	sendOut(t, c, 2, []byte{'b'})
	send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 3, TransferFlags: 2})
	r := unlinkReply(t, c, 2, 3, false)
	assert.Equal(t, int32(r.TransferFlags), int32(-errnoECONNRESET))

	sendOut(t, c, 2, []byte{'c'}) // no duplicate: seq 2 is free again
	dev.release <- struct{}{}
	r, err := readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Seqnum, uint32(1))
	assert.DeepEqual(t, <-dev.started, []byte{'c'}) // not the unlinked 'b'

	// Had the worker dropped the new seq 2 along with the old one, this unlink would find nothing.
	send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 4, TransferFlags: 2})
	dev.release <- struct{}{}
	r = unlinkReply(t, c, 2, 4, false)
	assert.Equal(t, r.TransferFlags, uint32(0))

	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
}

// An unlinked queued URB gives its charge and its payload back with the reply,
// not when the hung call ahead returns.
func TestUnlinkQueuedFreesBacklog(t *testing.T) {
	dev := newGateDev()
	c, done := startSession(t, dev)
	payload := make([]byte, 4<<20)
	sendOut(t, c, 1, payload)
	<-dev.started
	for i := range uint32(2 * maxBacklog / len(payload)) {
		seq := 2*i + 2
		sendOut(t, c, seq, payload)
		send(t, c, urbHeader{Command: cmdUnlink, Seqnum: seq + 1, TransferFlags: seq})
		r := unlinkReply(t, c, seq, seq+1, false)
		assert.Equal(t, int32(r.TransferFlags), int32(-errnoECONNRESET), "seq %d met the backlog cap", seq)
	}
	// The charges are free, so nothing else bounds payloads left in the queue.
	assertHeapBelow(t, maxBacklog)
	close(dev.release)
	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
}

func TestUnlinkInFlightWaitsForDevice(t *testing.T) {
	dev := newGateDev()
	c, done := startSession(t, dev)
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	<-dev.started
	send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 2, TransferFlags: 1})

	assert.NilError(t, c.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
	_, err := readURBHeader(c)
	assert.Assert(t, errors.Is(err, os.ErrDeadlineExceeded), "reply before device call returned: %v", err)

	close(dev.release)
	assert.NilError(t, c.SetReadDeadline(time.Now().Add(10*time.Second)))
	r := unlinkReply(t, c, 1, 2, true)
	assert.Equal(t, r.TransferFlags, uint32(0), "the device finished first")
	assert.Equal(t, r.TransferBufferLength, int32(8))
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 3, Direction: dirIn, Ep: 1})
	r, err = readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Seqnum, uint32(3), "endpoint dead after an unlink")

	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
}

func TestSessionEndStopsQueuedIO(t *testing.T) {
	dev := ctxDev{started: make(chan []byte, 64)}
	c, done := startSession(t, dev)
	for seq := range uint32(8) {
		send(t, c, urbHeader{Command: cmdSubmit, Seqnum: seq + 1, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	}
	<-dev.started
	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
	assert.Equal(t, len(dev.started), 0, "queued URB reached device after session end")
}

func TestReadsContinueWhileGuestNotDraining(t *testing.T) {
	dev := &countDev{}
	c, done := startSession(t, dev)
	for seq := range uint32(4) {
		send(t, c, urbHeader{Command: cmdSubmit, Seqnum: seq + 1, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	}
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if dev.calls.Load() == 4 {
			return poll.Success()
		}
		return poll.Continue("reads wait for the guest")
	}, poll.WithDelay(time.Millisecond))
	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
}

// The length field of CMD_UNLINK is guest data with no meaning.
func TestUnlinkLengthNotCharged(t *testing.T) {
	c, done := startSession(t, fakeDev{})
	send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 1, TransferFlags: 1, TransferBufferLength: maxBacklog * 2})
	r, err := readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Command, uint32(retUnlink))
	assert.Equal(t, r.TransferFlags, uint32(0), "an unknown URB unlinks fine")
	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
}

// partialConn writes half of its second Write, the first reply's payload, then lets writes through.
type partialConn struct {
	net.Conn
	writes atomic.Int32
}

func (c *partialConn) Write(b []byte) (int, error) {
	if c.writes.Add(1) == 2 {
		n, _ := c.Conn.Write(b[:len(b)/2])
		return n, syscall.ENOBUFS
	}
	return c.Conn.Write(b)
}

func TestWriteErrorEndsSession(t *testing.T) {
	c, s := net.Pipe()
	t.Cleanup(func() { _ = c.Close() })
	assert.NilError(t, c.SetDeadline(time.Now().Add(2*time.Second)))
	done := make(chan error, 1)
	go func() { done <- urbLoop(t.Context(), &partialConn{Conn: s}, fakeDev{}) }()

	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirIn, Ep: 1, TransferBufferLength: 96})
	go func() {
		_ = binary.Write(c, binary.BigEndian, urbHeader{Command: cmdSubmit, Seqnum: 2, Direction: dirIn, Ep: 1, TransferBufferLength: 96})
	}()
	got, err := io.ReadAll(c)
	assert.NilError(t, err)
	assert.Equal(t, len(got), urbHeaderSize+48, "bytes followed a partial frame")
	assert.NilError(t, sessionEnd(t, done))
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

// errDev fails each transfer after moving part of it.
type errDev struct {
	fakeDev
	err error
}

func (d errDev) Transfer(_ context.Context, _ uint8, _ bool, b []byte) (int, error) {
	return copy(b, "part"), d.err
}

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
			status, actual, data := doSubmit(t.Context(), errDev{err: tc.err}, h, nil)
			assert.Equal(t, status, tc.want)
			assert.Equal(t, string(data[:actual]), "part", "a failed IN URB keeps the bytes that moved")
		})
	}

	t.Run("cancelled mid call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		dev := cancelDev{cancel: cancel}
		status, actual, data := doSubmit(ctx, dev, h, nil)
		assert.Equal(t, status, int32(-errnoECONNRESET))
		assert.Equal(t, string(data[:actual]), "part")
	})
}

type cancelDev struct {
	fakeDev
	cancel context.CancelFunc
}

func (d cancelDev) Transfer(_ context.Context, _ uint8, _ bool, b []byte) (int, error) {
	d.cancel()
	return copy(b, "part"), urbErrno(errnoEPROTO)
}

func TestRetUnlinkNeverPrecedesRetSubmit(t *testing.T) {
	const pairs = 5000
	c, s := net.Pipe()
	t.Cleanup(func() { _ = c.Close() })
	assert.NilError(t, c.SetDeadline(time.Now().Add(2*time.Minute)))
	done := make(chan error, 1)
	go func() { done <- urbLoop(t.Context(), s, fakeDev{}) }()

	go func() {
		for i := range uint32(pairs) {
			n := 2*i + 1
			if binary.Write(c, binary.BigEndian, urbHeader{Command: cmdSubmit, Seqnum: n, Direction: dirIn, Ep: 1, TransferBufferLength: 64}) != nil ||
				binary.Write(c, binary.BigEndian, urbHeader{Command: cmdUnlink, Seqnum: n + 1, TransferFlags: n}) != nil {
				return
			}
		}
	}()

	submitted, unlinked := map[uint32]bool{}, map[uint32]bool{}
	for unlinks := 0; unlinks < pairs; {
		r, err := readURBHeader(c)
		assert.NilError(t, err)
		switch r.Command {
		case retSubmit:
			assert.Assert(t, !submitted[r.Seqnum], "second RET_SUBMIT for %d", r.Seqnum)
			submitted[r.Seqnum] = true
			_, err = io.CopyN(io.Discard, c, int64(r.TransferBufferLength))
			assert.NilError(t, err)
		case retUnlink:
			unlinks++
			assert.Assert(t, !unlinked[r.Seqnum], "second RET_UNLINK for %d", r.Seqnum)
			unlinked[r.Seqnum] = true
			assert.Assert(t, submitted[r.Seqnum-1], "RET_UNLINK %d before RET_SUBMIT", r.Seqnum)
			assert.Equal(t, r.TransferFlags, uint32(0), "seq %d", r.Seqnum)
		}
	}
	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
}

type goneDev struct {
	fakeDev
	gone func() bool
}

func (d goneDev) Gone() bool { return d.gone() }

func unplugWarnings(hook *logrustest.Hook) int {
	n := 0
	for _, e := range hook.AllEntries() {
		if e.Level == logrus.WarnLevel && strings.Contains(e.Message, "unplugged") {
			n++
		}
	}
	return n
}

func TestGoneAfterSessionEndIsIgnored(t *testing.T) {
	hook := logrustest.NewGlobal()
	called, release := make(chan struct{}), make(chan struct{})
	notify := sync.OnceFunc(func() { close(called) })
	c, done := startSession(t, goneDev{gone: func() bool {
		notify()
		<-release
		return true
	}})
	<-called
	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, unplugWarnings(hook), 0)
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
		status, actual, _ := doSubmit(t.Context(), dev, h, make([]byte, 64))
		assert.Equal(t, status, int32(-errnoEPROTO))
		assert.Equal(t, actual, int32(64), "the data stage still reached the device")
	})

	t.Run("cancel releases a blocked terminator", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		h := urbHeader{Direction: dirOut, Ep: 2, TransferFlags: urbZeroPacket, TransferBufferLength: 64}
		status, _, _ := doSubmit(ctx, &zlpDev{mps: 64, block: true}, h, make([]byte, 64))
		assert.Equal(t, status, int32(-errnoECONNRESET))
	})
}

// vhci_hcd oopses giving back a scatter-gather URB at disconnect, so none may be pending at close.
func TestSessionEndAnswersEveryURB(t *testing.T) {
	dev := ctxDev{started: make(chan []byte, 64)}
	ctx, cancel := context.WithCancel(t.Context())
	c, done := startSessionCtx(ctx, t, dev)

	for seq := range uint32(3) {
		send(t, c, urbHeader{Command: cmdSubmit, Seqnum: seq + 1, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	}
	<-dev.started
	cancel()
	// Submitted before the guest learns of the session end.
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 4, Direction: dirIn, Ep: 2, TransferBufferLength: 8})

	got := map[uint32]int32{}
	for range 4 {
		r, err := readURBHeader(c)
		assert.NilError(t, err)
		assert.Equal(t, r.Command, uint32(retSubmit))
		assert.Equal(t, r.TransferBufferLength, int32(0))
		got[r.Seqnum] = int32(r.TransferFlags)
	}
	e := int32(-errnoESHUTDOWN)
	assert.DeepEqual(t, got, map[uint32]int32{1: e, 2: e, 3: e, 4: e})
	readToEOF(t, c)
	assert.NilError(t, sessionEnd(t, done))
	assert.Equal(t, len(dev.started), 0, "URB reached the device after session end")
}

// mt76 resets the device on a control -ESHUTDOWN, so control fails with -ENODEV. A drain is no unplug.
func TestDrainFailsControlAsRemoved(t *testing.T) {
	dev := ctxDev{started: make(chan []byte, 64)}
	ctx, cancel := context.WithCancel(t.Context())
	c, done := startSessionCtx(ctx, t, dev)

	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirIn, TransferBufferLength: 4})
	<-dev.started
	cancel()
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 2, Direction: dirIn, TransferBufferLength: 4})
	for range 2 {
		r, err := readURBHeader(c)
		assert.NilError(t, err)
		assert.Equal(t, int32(r.TransferFlags), int32(-errnoENODEV), "seq %d", r.Seqnum)
	}
	readToEOF(t, c)
	assert.NilError(t, sessionEnd(t, done), "the drain reported the device gone")
}

// net.Pipe has no buffer, so the reply waits in the writer until the guest reads it.
func TestDrainWaitsForSlowReader(t *testing.T) {
	dev := ctxDev{started: make(chan []byte, 64)}
	ctx, cancel := context.WithCancel(t.Context())
	c, done := startSessionCtx(ctx, t, dev)

	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	<-dev.started
	cancel()
	time.Sleep(drainMax / 2)
	r, err := readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Seqnum, uint32(1))
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 2, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	r, err = readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Seqnum, uint32(2))
	assert.Equal(t, int32(r.TransferFlags), int32(-errnoESHUTDOWN))
	readToEOF(t, c)
	assert.NilError(t, sessionEnd(t, done))
}

// A socket takes the reply at once, so the drain cannot tell a stalled guest from one that is done.
// It answers until the guest closes.
func TestDrainWaitsForStalledGuest(t *testing.T) {
	old := drainMax
	drainMax = 10 * time.Second
	t.Cleanup(func() { drainMax = old })
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	defer l.Close()
	c, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", l.Addr().String())
	assert.NilError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	assert.NilError(t, c.SetDeadline(time.Now().Add(30*time.Second)))
	s, err := l.Accept()
	assert.NilError(t, err)
	dev := ctxDev{started: make(chan []byte, 64)}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- urbLoop(ctx, s, dev) }()

	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	<-dev.started
	cancel()
	time.Sleep(time.Second) // the reply is written, and nobody has read it
	r, err := readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Seqnum, uint32(1))
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 2, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	r, err = readURBHeader(c)
	assert.NilError(t, err, "the drain closed on a stalled guest")
	assert.Equal(t, r.Seqnum, uint32(2))
	assert.Equal(t, int32(r.TransferFlags), int32(-errnoESHUTDOWN))

	closed := time.Now()
	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
	assert.Assert(t, time.Since(closed) < drainMax/2, "the drain outlasted the guest")
}

// A submit during the drain must not queue behind a device call that ignores cancel.
func TestDrainAnswersSubmitBehindHungCall(t *testing.T) {
	dev := newGateDev()
	ctx, cancel := context.WithCancel(t.Context())
	c, done := startSessionCtx(ctx, t, dev)

	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	<-dev.started
	cancel()
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 2, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
	assert.NilError(t, c.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
	r, err := readURBHeader(c)
	assert.NilError(t, err, "late submit waited behind the hung call")
	assert.Equal(t, r.Seqnum, uint32(2))
	assert.Equal(t, int32(r.TransferFlags), int32(-errnoESHUTDOWN))

	close(dev.release)
	assert.NilError(t, c.SetReadDeadline(time.Now().Add(10*time.Second)))
	r, err = readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Seqnum, uint32(1))
	_, err = io.CopyN(io.Discard, c, int64(r.TransferBufferLength))
	assert.NilError(t, err)
	readToEOF(t, c)
	assert.NilError(t, sessionEnd(t, done))
}

// sentConn counts each RET_SUBMIT urbLoop writes, or tries to after the close, per seqnum.
type sentConn struct {
	net.Conn
	mu  sync.Mutex
	ret map[uint32]int
}

func (c *sentConn) Write(b []byte) (int, error) {
	if len(b) == urbHeaderSize && binary.BigEndian.Uint32(b) == retSubmit {
		c.mu.Lock()
		c.ret[binary.BigEndian.Uint32(b[4:])]++
		c.mu.Unlock()
	}
	return c.Conn.Write(b)
}

// A device call that outlasts drainMax still gets its URB, and the one queued behind it, answered once.
func TestDrainDeadlineAnswersHungCall(t *testing.T) {
	for name, unlinked := range map[string]bool{"plain": false, "unlinked": true} {
		t.Run(name, func(t *testing.T) {
			dev := newGateDev()
			c, s := net.Pipe()
			t.Cleanup(func() { _ = c.Close() })
			assert.NilError(t, c.SetDeadline(time.Now().Add(30*time.Second)))
			sc := &sentConn{Conn: s, ret: map[uint32]int{}}
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)
			go func() { done <- urbLoop(ctx, sc, dev) }()

			for seq := range uint32(2) {
				send(t, c, urbHeader{Command: cmdSubmit, Seqnum: seq + 1, Direction: dirIn, Ep: 1, TransferBufferLength: 8})
			}
			<-dev.started
			if unlinked {
				// The unlink answers queued seq 2. The drain answers hung seq 1, then its unlink.
				send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 3, TransferFlags: 2})
				unlinkReply(t, c, 2, 3, false)
				send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 4, TransferFlags: 1})
				cancel()
				r := unlinkReply(t, c, 1, 4, false)
				assert.Equal(t, int32(r.TransferFlags), int32(-errnoESHUTDOWN))
			} else {
				cancel()
				got := map[uint32]int32{}
				for range 2 {
					r, err := readURBHeader(c)
					assert.NilError(t, err)
					assert.Equal(t, r.Command, uint32(retSubmit))
					got[r.Seqnum] = int32(r.TransferFlags)
				}
				e := int32(-errnoESHUTDOWN)
				assert.DeepEqual(t, got, map[uint32]int32{1: e, 2: e})
			}
			readToEOF(t, c)
			close(dev.release) // the late completions must not even try to write
			assert.NilError(t, sessionEnd(t, done))
			sc.mu.Lock()
			defer sc.mu.Unlock()
			assert.DeepEqual(t, sc.ret, map[uint32]int{1: 1, 2: 1})
		})
	}
}

// A submit whose payload is still arriving when the session ends gets an answer too.
func TestDrainWaitsForPayload(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	c, done := startSessionCtx(ctx, t, fakeDev{})

	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirOut, Ep: 1, TransferBufferLength: 2})
	_, err := c.Write([]byte{0})
	assert.NilError(t, err)
	cancel()
	time.Sleep(150 * time.Millisecond)
	_, err = c.Write([]byte{0})
	assert.NilError(t, err, "the drain closed mid-frame")
	r, err := readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Seqnum, uint32(1))
	assert.Equal(t, int32(r.TransferFlags), int32(-errnoESHUTDOWN))
	readToEOF(t, c)
	assert.NilError(t, sessionEnd(t, done))
}
