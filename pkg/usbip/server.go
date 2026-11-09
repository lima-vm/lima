// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

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

const (
	maxTransferLength = 16 << 20
	maxISOPackets     = 1024
	// 64 MiB per session; raise it if a legit tool trips it.
	maxBacklog = 64 << 20
)

var (
	// vhci_hcd clears URB_DMA_MAP_SG only on RET_SUBMIT, and any other giveback of a
	// scatter-gather URB oopses the guest. So a session answers every URB before it closes.
	drainQuiet = 200 * time.Millisecond
	drainMax   = 2 * time.Second
)

func deviceDescWire(info DeviceInfo) usbDeviceDesc {
	var d usbDeviceDesc
	copy(d.Busid[:], info.Busid)
	d.Busnum = info.BusNum
	d.Devnum = info.DevNum
	d.Speed = info.Speed
	d.IDVendor = info.Vendor
	d.IDProduct = info.Product
	return d
}

func writeDevlist(conn io.Writer, infos []DeviceInfo) error {
	if err := writeOpHeader(conn, opRepDevlist, 0); err != nil {
		return err
	}
	if err := binary.Write(conn, binary.BigEndian, uint32(len(infos))); err != nil {
		return err
	}
	for _, info := range infos {
		if err := binary.Write(conn, binary.BigEndian, deviceDescWire(info)); err != nil {
			return err
		}
	}
	return nil
}

type inflight struct {
	cancel   context.CancelFunc
	h        urbHeader
	isoDesc  []byte
	started  bool
	unlinked bool
	unlink   urbHeader
}

type urbJob struct {
	ctx     context.Context
	h       urbHeader
	outData []byte
	isoDesc []byte
	entry   *inflight
}

// The fixed part makes header floods reach maxBacklog too.
func urbCost(h urbHeader) int64 {
	return 4<<10 + int64(h.TransferBufferLength) + int64(max(h.NumberOfPackets, 0))*16
}

// push never blocks, so neither the reader nor a worker waits for the guest.
type queue[T any] struct {
	mu     sync.Mutex
	cond   *sync.Cond
	jobs   []T
	closed bool
}

func newQueue[T any]() *queue[T] {
	q := &queue[T]{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue[T]) push(j T) {
	q.mu.Lock()
	q.jobs = append(q.jobs, j)
	q.cond.Signal()
	q.mu.Unlock()
}

func (q *queue[T]) close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *queue[T]) pop() (j T, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.jobs) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.jobs) == 0 {
		return j, false
	}
	j = q.jobs[0]
	clear(q.jobs[:1])
	q.jobs = q.jobs[1:]
	return j, true
}

func urbLoop(ctx context.Context, conn net.Conn, dev Device) (err error) {
	var gone atomic.Bool
	defer func() {
		if gone.Load() {
			err = ErrDeviceGone
		}
	}()

	var stateMu sync.Mutex
	pending := map[uint32]*inflight{}
	workers := map[uint32]*queue[*urbJob]{}
	var backlog int64
	var wg sync.WaitGroup

	sessionCtx, endSession := context.WithCancel(ctx)
	deviceGone := func() {
		if gone.CompareAndSwap(false, true) {
			logrus.Warnf("usbip: host device %s unplugged, re-enumerated or detached; ending session", dev.Info().Busid)
		}
		endSession()
	}

	// An idle device reports no errors, so poll.
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-t.C:
				// Close may have run during Gone, which then proves nothing.
				if dev.Gone() && sessionCtx.Err() == nil {
					deviceGone()
					return
				}
			}
		}
	}()

	// The drain waits until the guest has read every reply and then stayed quiet, on the monotonic clock.
	start := time.Now()
	var lastIO, unsent atomic.Int64
	stamp := func() { lastIO.Store(int64(time.Since(start))) }

	// One writer: a guest slow to read must not delay the next device read.
	replies := newQueue[func()]()
	reply := func(send func()) {
		unsent.Add(1)
		replies.push(send)
	}
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for {
			send, ok := replies.pop()
			if !ok {
				return
			}
			send()
			unsent.Add(-1)
			stamp()
		}
	}()
	defer func() {
		replies.close()
		<-writerDone
	}()
	wrote := func(err error, cost int64) {
		if err != nil {
			logrus.WithError(err).Debug("usbip: writing urb reply failed")
			// Close here, not later: nothing may follow a partial frame.
			endSession()
			_ = conn.Close()
		}
		stateMu.Lock()
		backlog -= cost
		stateMu.Unlock()
	}
	retSubmit := func(h urbHeader, isoDesc []byte, status, actual int32, data []byte) error {
		err := writeRetSubmit(conn, h, status, actual, data)
		if err == nil && len(isoDesc) > 0 {
			_, err = conn.Write(isoDesc)
		}
		return err
	}

	// After the session ends, answer what is pending and what the guest still sends, then close.
	closed := make(chan struct{})
	context.AfterFunc(sessionCtx, func() {
		defer close(closed)
		deadline := time.Now().Add(drainMax)
		for time.Now().Before(deadline) {
			stateMu.Lock()
			busy := len(pending) > 0
			stateMu.Unlock()
			if !busy && unsent.Load() == 0 && time.Since(start)-time.Duration(lastIO.Load()) >= drainQuiet {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		flushed := make(chan struct{})
		reply(func() { close(flushed) })
		select {
		case <-flushed:
		case <-time.After(drainMax): // a guest that stops reading
		}
		_ = conn.Close()
	})

	defer wg.Wait()
	defer func() {
		endSession()
		<-closed
		stateMu.Lock()
		for _, q := range workers {
			q.close()
		}
		stateMu.Unlock()
	}()

	worker := func(q *queue[*urbJob]) {
		defer wg.Done()
		for {
			job, ok := q.pop()
			if !ok {
				return
			}
			// Children cancel after their parent, so job.ctx may still look live.
			if sessionCtx.Err() != nil {
				job.entry.cancel()
			}
			stateMu.Lock()
			job.entry.started = !job.entry.unlinked
			stateMu.Unlock()

			status, actual, inData := doSubmit(job.ctx, dev, job.h, job.outData)

			// Queued with the delete: a CMD_UNLINK that misses pending must reply after this RET_SUBMIT.
			stateMu.Lock()
			entry := *job.entry
			delete(pending, job.h.Seqnum)
			gone := status == -errnoENODEV
			if !entry.unlinked && status == -errnoECONNRESET && sessionCtx.Err() != nil {
				status = drainStatus(job.h) // the session ended, not the guest
			}
			reply(func() {
				var err error
				switch {
				case !entry.unlinked:
					err = retSubmit(job.h, job.isoDesc, status, actual, inData)
				case entry.started:
					// RET_SUBMIT first, so the guest gives the URB back on that path.
					err = retSubmit(job.h, job.isoDesc, status, actual, inData)
					if err == nil {
						err = writeRetUnlink(conn, entry.unlink, 0)
					}
				}
				wrote(err, urbCost(job.h))
				// After the write: the guest takes this reply before EOF.
				if gone {
					deviceGone()
				}
			})
			stateMu.Unlock()
			entry.cancel()
		}
	}

	for {
		h, err := readURBHeader(conn)
		if err != nil {
			if errors.Is(err, io.EOF) || sessionCtx.Err() != nil {
				return nil
			}
			return fmt.Errorf("reading urb header: %w", err)
		}
		stamp()

		switch h.Command {
		case cmdSubmit:
			if h.Direction > dirIn || h.Ep > 15 ||
				h.TransferBufferLength < 0 || h.TransferBufferLength > maxTransferLength ||
				h.NumberOfPackets > maxISOPackets {
				return fmt.Errorf("invalid urb: dir=%d ep=%d len=%d np=%d",
					h.Direction, h.Ep, h.TransferBufferLength, h.NumberOfPackets)
			}

			stateMu.Lock()
			_, dup := pending[h.Seqnum]
			backlog += urbCost(h)
			over := backlog > maxBacklog
			stateMu.Unlock()
			if dup {
				return fmt.Errorf("duplicate seqnum %d", h.Seqnum)
			}
			if over {
				return fmt.Errorf("urb backlog exceeds %d bytes", maxBacklog)
			}

			var outData []byte
			if h.Direction == dirOut && h.TransferBufferLength > 0 {
				outData = make([]byte, h.TransferBufferLength)
				if _, err := io.ReadFull(conn, outData); err != nil {
					return fmt.Errorf("reading urb out data: %w", err)
				}
			}
			var isoDesc []byte
			if h.NumberOfPackets > 0 {
				isoDesc = make([]byte, int(h.NumberOfPackets)*16)
				if _, err := io.ReadFull(conn, isoDesc); err != nil {
					return fmt.Errorf("reading iso descriptors: %w", err)
				}
			}

			if sessionCtx.Err() != nil {
				reply(func() { wrote(retSubmit(h, isoDesc, drainStatus(h), 0, nil), urbCost(h)) })
				continue
			}

			urbCtx, cancel := context.WithCancel(sessionCtx)
			entry := &inflight{cancel: cancel, h: h, isoDesc: isoDesc}

			// Endpoint 0 is one bidirectional pipe.
			key := h.Ep << 1
			if h.Ep != 0 {
				key |= h.Direction & 1
			}

			stateMu.Lock()
			pending[h.Seqnum] = entry
			q, ok := workers[key]
			if !ok {
				q = newQueue[*urbJob]()
				workers[key] = q
				wg.Add(1)
				go worker(q)
			}
			stateMu.Unlock()

			q.push(&urbJob{ctx: urbCtx, h: h, outData: outData, isoDesc: isoDesc, entry: entry})

		case cmdUnlink:
			stateMu.Lock()
			entry, found := pending[h.TransferFlags] // unlink_seqnum shares this offset
			started := false
			if found {
				started = entry.started
				entry.unlinked = true
				entry.unlink = h
				entry.cancel()
			}
			over := false
			// Otherwise the worker replies once the device call returns.
			if !started {
				// h.TransferBufferLength is unchecked here, so charge the fixed part only.
				cost := urbCost(urbHeader{})
				backlog += cost
				over = backlog > maxBacklog
				var sub *inflight
				if found {
					sub = entry
				}
				reply(func() {
					var err error
					if sub != nil {
						err = retSubmit(sub.h, sub.isoDesc, -errnoECONNRESET, 0, nil)
					}
					if err == nil {
						err = writeRetUnlink(conn, h, 0)
					}
					wrote(err, cost)
				})
			}
			stateMu.Unlock()
			if over {
				return fmt.Errorf("urb backlog exceeds %d bytes", maxBacklog)
			}

		default:
			return fmt.Errorf("unsupported urb command %#08x", h.Command)
		}
	}
}

// drainStatus fails a URB as an unplug does. mt76 takes -ENODEV on a control transfer as removal,
// but -ESHUTDOWN as a hung bus that it resets. -ESHUTDOWN stops completion handlers resubmitting.
func drainStatus(h urbHeader) int32 {
	if h.Ep == 0 {
		return -errnoENODEV
	}
	return -errnoESHUTDOWN
}

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
