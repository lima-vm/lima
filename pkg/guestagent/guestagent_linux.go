// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package guestagent

import (
	"context"
	"os"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/guestagent/kubernetesservice"
	"github.com/lima-vm/lima/v2/pkg/guestagent/sockets"
	"github.com/lima-vm/lima/v2/pkg/guestagent/ticker"
	"github.com/lima-vm/lima/v2/pkg/guestagent/timesync"
)

func New(ctx context.Context, ticker ticker.Ticker) (Agent, error) {
	a := &agent{
		ticker:                   ticker,
		kubernetesServiceWatcher: kubernetesservice.NewServiceWatcher(),
	}

	go a.kubernetesServiceWatcher.Start(ctx)
	go a.fixSystemTimeSkew()

	return a, nil
}

type agent struct {
	// Ticker is like time.Ticker.
	// We can't use inotify for /proc/net/tcp, so we need this ticker to
	// reload /proc/net/tcp.
	ticker ticker.Ticker

	kubernetesServiceWatcher *kubernetesservice.ServiceWatcher
}

type eventState struct {
	ports []*api.IPPort
}

func comparePorts(old, neww []*api.IPPort) (added, removed []*api.IPPort) {
	mRaw := make(map[string]*api.IPPort, len(old))
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
	return added, removed
}

func (a *agent) collectEvent(ctx context.Context, st eventState) (*api.Event, eventState) {
	var (
		ev  = &api.Event{}
		err error
	)
	newSt := st
	newSt.ports, err = a.LocalPorts(ctx)
	if err != nil {
		ev.Errors = append(ev.Errors, err.Error())
		ev.Time = timestamppb.Now()
		return ev, newSt
	}
	ev.AddedLocalPorts, ev.RemovedLocalPorts = comparePorts(st.ports, newSt.ports)
	ev.Time = timestamppb.Now()
	return ev, newSt
}

func isEventEmpty(ev *api.Event) bool {
	empty := &api.Event{}
	empty.Time = ev.Time
	return reflect.DeepEqual(empty, ev)
}

func (a *agent) Events(ctx context.Context, ch chan *api.Event) {
	defer close(ch)
	tickerCh := a.ticker.Chan()
	defer a.ticker.Stop()
	var st eventState
	for {
		var ev *api.Event
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

func (a *agent) LocalPorts(_ context.Context) ([]*api.IPPort, error) {
	var res []*api.IPPort
	socketsList, err := sockets.List()
	if err != nil {
		return res, err
	}

	for _, f := range socketsList {
		switch f.Kind {
		case sockets.TCP, sockets.TCP6:
			if f.State == sockets.TCPListen {
				res = append(res,
					&api.IPPort{
						Ip:       f.IP.String(),
						Port:     int32(f.Port),
						Protocol: "tcp",
					})
			}
		case sockets.UDP, sockets.UDP6:
			if f.State == sockets.UDPUnconnected {
				res = append(res,
					&api.IPPort{
						Ip:       f.IP.String(),
						Port:     int32(f.Port),
						Protocol: "udp",
					})
			}
		default:
			continue
		}
	}

	kubernetesEntries := a.kubernetesServiceWatcher.GetPorts()
	for _, entry := range kubernetesEntries {
		found := false
		for _, re := range res {
			if re.Port == int32(entry.Port) {
				found = true
			}
		}

		if !found {
			res = append(res,
				&api.IPPort{
					Ip:       entry.IP.String(),
					Port:     int32(entry.Port),
					Protocol: string(entry.Protocol),
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

const deltaLimit = 2 * time.Second

func (a *agent) fixSystemTimeSkew() {
	logrus.Info("fixSystemTimeSkew(): monitoring system time skew")
	for {
		ok, err := timesync.HasRTC()
		if !ok {
			logrus.Warnf("fixSystemTimeSkew: error: %s", err.Error())
			break
		}
		ticker := time.NewTicker(10 * time.Second)
		for now := range ticker.C {
			rtc, err := timesync.GetRTCTime()
			if err != nil {
				logrus.Warnf("fixSystemTimeSkew: lookup error: %s", err.Error())
				continue
			}
			d := rtc.Sub(now)
			logrus.Debugf("fixSystemTimeSkew: rtc=%s systime=%s delta=%s",
				rtc.Format(time.RFC3339), now.Format(time.RFC3339), d)
			if d > deltaLimit || d < -deltaLimit {
				err = timesync.SetSystemTime(rtc)
				if err != nil {
					logrus.Warnf("fixSystemTimeSkew: set system clock error: %s", err.Error())
					continue
				}
				logrus.Infof("fixSystemTimeSkew: system time synchronized with rtc")
				break
			}
		}
		ticker.Stop()
	}
}

func (a *agent) HandleInotify(event *api.Inotify) {
	location := event.MountPath
	if _, err := os.Stat(location); err == nil {
		local := event.Time.AsTime().Local()
		err := os.Chtimes(location, local, local)
		if err != nil {
			logrus.Errorf("error in inotify handle. Event: %s, Error: %s", event, err)
		}
	}
}
