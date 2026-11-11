// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
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

// vhci_hcd clears URB_DMA_MAP_SG only on RET_SUBMIT, and any other giveback of a
// scatter-gather URB oopses the guest. So a session answers every URB before it closes.
var drainMax = 2 * time.Second

type inflight struct {
	cancel   context.CancelFunc
	h        urbHeader
	isoDesc  []byte
	started  bool
	unlinked bool
	unlink   urbHeader
	answered bool // by the drain, before the device call returned
}

type urbJob struct {
	ctx     context.Context
	h       urbHeader
	outData []byte
	isoDesc []byte
	entry   *inflight
}

// Endpoint 0 is one bidirectional pipe.
func epKey(h urbHeader) uint32 {
	if h.Ep == 0 {
		return 0
	}
	return h.Ep<<1 | h.Direction&1
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

// ponytail: linear scan, bounded by maxBacklog; index the jobs if unlink floods show up in profiles.
func (q *queue[T]) drop(match func(T) bool) {
	q.mu.Lock()
	q.jobs = slices.DeleteFunc(q.jobs, match)
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
			logrus.Warnf("usbip: host device %s unplugged, re-enumerated or detached; ending session", dev.Info().BusID)
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

	// A socket buffers writes, so a written reply and a silent conn do not show that the guest read it.
	// A reply it reads late can still make it resubmit. So only the end of reading ends the drain early.
	var readDone atomic.Bool

	// One writer: a guest slow to read must not delay the next device read.
	replies := newQueue[func()]()
	reply := replies.push
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for {
			send, ok := replies.pop()
			if !ok {
				return
			}
			send()
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

	// After the session ends, answer what is pending and what the guest still sends.
	// Close once the guest has, or after drainMax.
	closed := make(chan struct{})
	context.AfterFunc(sessionCtx, func() {
		defer close(closed)
		deadline := time.Now().Add(drainMax)
		for time.Now().Before(deadline) {
			stateMu.Lock()
			// Unlike pending, the backlog also holds a URB whose payload is still arriving,
			// and each reply until it is written.
			busy := backlog > 0
			stateMu.Unlock()
			if !busy && readDone.Load() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		// A device call can outlast the drain, so answer its URB and those queued behind it now.
		stateMu.Lock()
		for _, p := range pending {
			p.answered = true
			e := *p
			reply(func() {
				err := retSubmit(e.h, e.isoDesc, drainStatus(e.h), 0, nil)
				if err == nil && e.unlinked {
					err = writeRetUnlink(conn, e.unlink, 0)
				}
				wrote(err, 0) // the worker frees the charge once the call returns
			})
		}
		stateMu.Unlock()
		flushed := make(chan struct{})
		reply(func() { close(flushed) })
		select {
		case <-flushed:
		case <-time.After(drainMax): // a guest that stops reading
		}
		_ = conn.Close()
	})

	// Over the cap the session ends, but its URBs are answered as in any drain.
	// Reads pause until replies free the backlog, so a guest that stops reading cannot grow it.
	var backlogErr error
	defer func() {
		if backlogErr != nil {
			err = backlogErr
		}
	}()
	overflow := func() {
		if sessionCtx.Err() == nil {
			backlogErr = fmt.Errorf("urb backlog exceeds %d bytes", maxBacklog)
			endSession()
		}
	}
	throttle := func() {
		for {
			stateMu.Lock()
			over := backlog > maxBacklog
			stateMu.Unlock()
			if !over {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	defer wg.Wait()
	defer func() {
		readDone.Store(true)
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
			retired := job.entry.unlinked // popped as the unlink answered it and freed its charge
			job.entry.started = !retired
			stateMu.Unlock()
			if retired {
				continue
			}

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
				if !entry.answered {
					// RET_SUBMIT first, so the guest gives the URB back on that path.
					err = retSubmit(job.h, job.isoDesc, status, actual, inData)
					if err == nil && entry.unlinked {
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
			cost := urbCost(h)
			// The URB that would overflow is the first one the drain answers.
			draining := sessionCtx.Err() != nil || backlog+cost > maxBacklog
			if draining {
				cost = urbCost(urbHeader{NumberOfPackets: h.NumberOfPackets}) // the payload is discarded
			}
			backlog += cost
			stateMu.Unlock()
			if dup {
				wrote(nil, cost)
				return fmt.Errorf("duplicate seqnum %d", h.Seqnum)
			}
			if draining {
				overflow()
			}

			var outData []byte
			if h.Direction == dirOut && h.TransferBufferLength > 0 {
				if draining {
					_, err = io.CopyN(io.Discard, conn, int64(h.TransferBufferLength))
				} else {
					outData = make([]byte, h.TransferBufferLength)
					_, err = io.ReadFull(conn, outData)
				}
				if err != nil {
					wrote(nil, cost)
					return fmt.Errorf("reading urb out data: %w", err)
				}
			}
			var isoDesc []byte
			if h.NumberOfPackets > 0 {
				isoDesc = make([]byte, int(h.NumberOfPackets)*16)
				if _, err := io.ReadFull(conn, isoDesc); err != nil {
					wrote(nil, cost)
					return fmt.Errorf("reading iso descriptors: %w", err)
				}
			}

			if draining || sessionCtx.Err() != nil {
				reply(func() { wrote(retSubmit(h, isoDesc, drainStatus(h), 0, nil), cost) })
				// Queued before the pause, so the close cannot overtake this reply.
				throttle()
				continue
			}

			urbCtx, cancel := context.WithCancel(sessionCtx)
			entry := &inflight{cancel: cancel, h: h, isoDesc: isoDesc}

			stateMu.Lock()
			pending[h.Seqnum] = entry
			q, ok := workers[epKey(h)]
			if !ok {
				q = newQueue[*urbJob]()
				workers[epKey(h)] = q
				wg.Add(1)
				go worker(q)
			}
			stateMu.Unlock()

			q.push(&urbJob{ctx: urbCtx, h: h, outData: outData, isoDesc: isoDesc, entry: entry})

		case cmdUnlink:
			stateMu.Lock()
			entry, found := pending[h.TransferFlags] // unlink_seqnum shares this offset
			// The drain answered the URB, or a first unlink left that to the worker.
			if found && (entry.unlinked || entry.answered) {
				found = false
			}
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
				if found {
					// Retire the URB now: its seqnum, payload and charge must not wait for the worker.
					delete(pending, entry.h.Seqnum)
					workers[epKey(entry.h)].drop(func(j *urbJob) bool { return j.entry == entry })
					cost += urbCost(entry.h)
				}
				reply(func() {
					var err error
					if found {
						err = retSubmit(entry.h, entry.isoDesc, -errnoECONNRESET, 0, nil)
					}
					if err == nil {
						err = writeRetUnlink(conn, h, 0)
					}
					wrote(err, cost)
				})
			}
			stateMu.Unlock()
			if over {
				overflow()
				throttle()
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

	// actual_length counts what moved even when the URB fails.
	actual = int32(n)
	if err != nil {
		if ctx.Err() != nil {
			return -errnoECONNRESET, actual, buf
		}
		if errors.Is(err, ErrDeviceGone) {
			logrus.WithError(err).Debugf("usbip: device gone on ep %d", h.Ep)
			return -errnoENODEV, actual, buf
		}
		logrus.WithError(err).Debugf("usbip: transfer failed on ep %d", h.Ep)
		if e, ok := errors.AsType[urbErrno](err); ok {
			return -int32(e), actual, buf
		}
		return -errnoEPIPE, actual, buf
	}
	return 0, actual, buf
}
