// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ticker

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/sirupsen/logrus"
)

func NewEbpfTicker(tracepoints []string) (Ticker, error) {
	var (
		ticker ebpfTicker
		err    error
	)
	defer func() {
		if err != nil {
			ticker.Stop()
		}
	}()
	ticker.events, err = ebpf.NewMap(&ebpf.MapSpec{
		Name:       "lima_ticker_events",
		Type:       ebpf.RingBuf,
		MaxEntries: 1 << 20,
	})
	if err != nil {
		return nil, err
	}

	ticker.prog, err = buildEbpfProg(ticker.events)
	if err != nil {
		return nil, err
	}

	for _, tp := range tracepoints {
		tpPair := strings.SplitN(tp, ":", 2)
		tpLink, err := link.Tracepoint(tpPair[0], tpPair[1], ticker.prog, nil)
		if err != nil {
			return nil, err
		}
		ticker.links = append(ticker.links, tpLink)
	}

	ticker.reader, err = ringbuf.NewReader(ticker.events)
	if err != nil {
		return nil, err
	}

	ticker.ch = make(chan time.Time)
	go func() {
		defer close(ticker.ch)
		for {
			_, rdErr := ticker.reader.Read()
			if rdErr != nil {
				if !errors.Is(rdErr, ringbuf.ErrClosed) {
					logrus.WithError(rdErr).Warn("ebpfTicker: failed to read ringbuf")
				}
				logrus.Debug("ebpfTicker: exiting")
				return
			}
			ticker.ch <- time.Now()
		}
	}()

	return &ticker, nil
}

var _ Ticker = (*ebpfTicker)(nil)

type ebpfTicker struct {
	events *ebpf.Map
	prog   *ebpf.Program
	links  []link.Link
	reader *ringbuf.Reader
	ch     chan time.Time
}

func (ticker *ebpfTicker) Chan() <-chan time.Time {
	return ticker.ch
}

func (ticker *ebpfTicker) Stop() {
	if ticker.events != nil {
		_ = ticker.events.Close()
	}
	if ticker.prog != nil {
		_ = ticker.prog.Close()
	}
	for _, l := range ticker.links {
		_ = l.Close()
	}
	if ticker.reader != nil {
		_ = ticker.reader.Close()
	}
	// ticker.ch will be closed in go routine in NewEbpfTicker() to avoid sending on closed channel
}

func buildEbpfProg(events *ebpf.Map) (*ebpf.Program, error) {
	inst := asm.Instructions{
		// ignore events from the guestagent process itself
		asm.FnGetCurrentPidTgid.Call(),
		asm.RSh.Imm(asm.R0, 32),
		asm.JEq.Imm(asm.R0, int32(os.Getpid()), "ret"),

		// ringbuf = &map
		asm.LoadMapPtr(asm.R1, events.FD()),

		// data = FP - 8
		asm.Mov.Reg(asm.R2, asm.R10),
		asm.Add.Imm(asm.R2, -8),

		// *data = 1
		asm.StoreImm(asm.R2, 0, 1, asm.Word),

		// size = 1
		asm.Mov.Imm(asm.R3, 1),

		// flags = 0
		asm.Mov.Imm(asm.R4, 0),

		// long bpf_ringbuf_output(void *ringbuf, void *data, u64 size, u64 flags)
		// https://man7.org/linux/man-pages/man7/bpf-helpers.7.html
		asm.FnRingbufOutput.Call(),

		// return 0
		asm.Mov.Imm(asm.R0, 0).WithSymbol("ret"),
		asm.Return(),
	}

	spec := &ebpf.ProgramSpec{
		Name:         "lima_ticker",
		Type:         ebpf.TracePoint,
		License:      "Apache-2.0", // No need to be GPL?
		Instructions: inst,
	}

	return ebpf.NewProgram(spec)
}
