package guestagent

import (
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"time"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/guestagent/procnettcp"
	"github.com/sirupsen/logrus"
	"github.com/yalue/native_endian"
)

func New(newTicker func() (<-chan time.Time, func())) Agent {
	return &agent{
		newTicker: newTicker,
	}
}

type agent struct {
	// Ticker is like time.Ticker.
	// We can't use inotify for /proc/net/tcp, so we need this ticker to
	// reload /proc/net/tcp.
	newTicker func() (<-chan time.Time, func())
}

type eventState struct {
	ports []api.IPPort
}

func comparePorts(old, neww []api.IPPort) (added, removed []api.IPPort) {
	mRaw := make(map[string]api.IPPort, len(old))
	mStillExist := make(map[string]bool, len(old))

	for _, f := range old {
		k := f.String()
		mRaw[k] = f
		mStillExist[k] = false
	}
	for _, f := range neww {
		k := f.String()
		if _, ok := mRaw[k]; !ok {
			added = append(added, f)
		}
		mStillExist[k] = true
	}

	for k, stillExist := range mStillExist {
		if !stillExist {
			if x, ok := mRaw[k]; ok {
				removed = append(removed, x)
			}
		}
	}
	return
}

func (a *agent) collectEvent(ctx context.Context, st eventState) (api.Event, eventState) {
	var (
		ev  api.Event
		err error
	)
	newSt := st
	newSt.ports, err = a.LocalPorts(ctx)
	if err != nil {
		ev.Errors = append(ev.Errors, err.Error())
		ev.Time = time.Now()
		return ev, newSt
	}
	ev.LocalPortsAdded, ev.LocalPortsRemoved = comparePorts(st.ports, newSt.ports)
	ev.Time = time.Now()
	return ev, newSt
}

func isEventEmpty(ev api.Event) bool {
	var empty api.Event
	// ignore ev.Time
	copied := ev
	copied.Time = time.Time{}
	return reflect.DeepEqual(empty, copied)
}

func (a *agent) Events(ctx context.Context, ch chan api.Event) {
	defer close(ch)
	tickerCh, tickerClose := a.newTicker()
	defer tickerClose()
	var st eventState
	for {
		var ev api.Event
		ev, st = a.collectEvent(ctx, st)
		if !isEventEmpty(ev) {
			ch <- ev
		}
		select {
		case <-ctx.Done():
			return
		case _, ok := <-tickerCh:
			if !ok {
				return
			}
			logrus.Debug("tick!")
		}
	}
}

func (a *agent) LocalPorts(ctx context.Context) ([]api.IPPort, error) {
	if native_endian.NativeEndian() == binary.BigEndian {
		return nil, errors.New("big endian architecture is unsupported, because I don't know how /proc/net/tcp looks like on big endian hosts")
	}
	var res []api.IPPort
	tcpParsed, err := procnettcp.ParseFiles()
	if err != nil {
		return res, err
	}

	for _, f := range tcpParsed {
		switch f.Kind {
		case procnettcp.TCP, procnettcp.TCP6:
		default:
			continue
		}
		if f.State == procnettcp.TCPListen {
			res = append(res,
				api.IPPort{
					IP:   f.IP,
					Port: int(f.Port),
				})
		}
	}
	return res, nil
}

func (a *agent) Info(ctx context.Context) (*api.Info, error) {
	var (
		info api.Info
		err  error
	)
	info.LocalPorts, err = a.LocalPorts(ctx)
	if err != nil {
		return nil, err
	}
	return &info, nil
}
