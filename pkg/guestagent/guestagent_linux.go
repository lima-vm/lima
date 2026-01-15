// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package guestagent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"

	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/guestagent/kubernetesservice"
	"github.com/lima-vm/lima/v2/pkg/guestagent/sockets"
	"github.com/lima-vm/lima/v2/pkg/guestagent/ticker"
)

func New(ctx context.Context, ticker ticker.Ticker, runtimeDir string) (Agent, error) {
	socketsLister, err := sockets.NewLister()
	if err != nil {
		return nil, err
	}
	a := &agent{
		ticker:                   ticker,
		socketLister:             socketsLister,
		kubernetesServiceWatcher: kubernetesservice.NewServiceWatcher(),
		runtimeDir:               runtimeDir,
	}

	go a.kubernetesServiceWatcher.Start(ctx)

	go func() {
		<-ctx.Done()
		logrus.Debug("Closing the agent")
		if err := a.Close(); err != nil {
			logrus.Errorf("error on agent.Close(): %v", err)
		}
	}()

	return a, nil
}

var _ Agent = (*agent)(nil)

type agent struct {
	// Ticker is like time.Ticker.
	// We can't use inotify for /proc/net/tcp, so we need this ticker to
	// reload /proc/net/tcp.
	ticker                   ticker.Ticker
	socketLister             *sockets.Lister
	kubernetesServiceWatcher *kubernetesservice.ServiceWatcher
	runtimeDir               string
}

type eventState struct {
	Ports []*api.IPPort `json:"ports,omitempty"`
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
	newSt.Ports, err = a.LocalPorts(ctx)
	if err != nil {
		ev.Errors = append(ev.Errors, err.Error())
		ev.Time = timestamppb.Now()
		return ev, newSt
	}
	ev.AddedLocalPorts, ev.RemovedLocalPorts = comparePorts(st.Ports, newSt.Ports)
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

	st, err := a.LoadEventState()
	if err != nil {
		logrus.Errorf("failed to load state: %v", err)
	}
	defer func() {
		if err := a.SaveEventState(st); err != nil {
			logrus.Errorf("failed to save state: %v", err)
		}
	}()
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
				logrus.Debug("ticker channel closed")
				return
			}
			logrus.Debug("tick!")
		}
	}
}

func (a *agent) LocalPorts(_ context.Context) ([]*api.IPPort, error) {
	var res []*api.IPPort
	socketsList, err := a.socketLister.List()
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

func (a *agent) Close() error {
	if a.socketLister != nil {
		if err := a.socketLister.Close(); err != nil {
			return err
		}
	}
	a.ticker.Stop()
	return nil
}

const eventStateFileName = "event-state.json"

// LoadEventState loads the event state from a file in JSON format.
// If the file does not exist, it returns an empty eventState with no error.
// The saved eventState is expected to be removed on OS restart.
func (a *agent) LoadEventState() (eventState, error) {
	logrus.Debug("Loading event state")
	path := filepath.Join(a.runtimeDir, eventStateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return eventState{}, nil
		}
		return eventState{}, err
	}
	var st eventState
	if err := json.Unmarshal(data, &st); err != nil {
		return eventState{}, err
	}
	// We don't remove the file after loading for debugging purposes.
	return st, nil
}

// SaveEventState saves the event state to a file in JSON format.
// It overwrites the file if it already exists.
// The saved eventState is expected to be removed on OS restart.
func (a *agent) SaveEventState(st eventState) error {
	logrus.Debug("Saving event state")
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	path := filepath.Join(a.runtimeDir, eventStateFileName)
	return os.WriteFile(path, data, 0o644)
}
