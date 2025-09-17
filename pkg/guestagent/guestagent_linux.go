// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package guestagent

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"syscall"
	"time"

	"github.com/elastic/go-libaudit/v2"
	"github.com/elastic/go-libaudit/v2/auparse"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/guestagent/events"
	"github.com/lima-vm/lima/v2/pkg/guestagent/iptables"
	"github.com/lima-vm/lima/v2/pkg/guestagent/procnettcp"
	"github.com/lima-vm/lima/v2/pkg/guestagent/timesync"
)

type Config struct {
	Ticker            func() (<-chan time.Time, func())
	IptablesIdle      time.Duration
	DockerSockets     []string
	ContainerdSockets []string
	KubernetesConfigs []string
}

func New(cfg *Config) (Agent, error) {
	dockerEventMonitor := events.NewDockerEventMonitor(cfg.DockerSockets)
	containerdEventMonitor := events.NewContainerdEventMonitor(cfg.ContainerdSockets)
	kubeServiceWatcher := events.NewKubeServiceWatcher(cfg.KubernetesConfigs)

	a := &agent{
		newTicker:              cfg.Ticker,
		IptablesIdle:           cfg.IptablesIdle,
		dockerEventMonitor:     dockerEventMonitor,
		containerdEventMonitor: containerdEventMonitor,
		kubeServiceWatcher:     kubeServiceWatcher,
	}

	auditClient, err := libaudit.NewMulticastAuditClient(nil)
	if err != nil {
		// syscall.EPROTONOSUPPORT or syscall.EAFNOSUPPORT is returned when calling attempting to connect to NETLINK_AUDIT
		// on a kernel built without auditing support.
		// https://github.com/elastic/go-libaudit/blob/ec298e53a6841a1f7715abbc7122635622f349bd/audit.go#L112-L115
		if !errors.Is(err, syscall.EPROTONOSUPPORT) && !errors.Is(err, syscall.EAFNOSUPPORT) {
			return nil, err
		}
		logrus.Infof("Auditing is not available: %s", err)
		return startGuestAgentRoutines(a, false), nil
	}

	auditStatus, err := auditClient.GetStatus()
	if err != nil {
		// syscall.EPERM is returned when using audit from a non-initial namespace
		// https://github.com/torvalds/linux/blob/633b47cb009d09dc8f4ba9cdb3a0ca138809c7c7/kernel/audit.c#L1054-L1057
		if !errors.Is(err, syscall.EPERM) {
			return nil, err
		}
		logrus.Infof("Auditing is not permitted: %s", err)
		return startGuestAgentRoutines(a, false), nil
	}

	if auditStatus.Enabled == 0 {
		logrus.Info("Enabling auditing")
		if err = auditClient.SetEnabled(true, libaudit.WaitForReply); err != nil {
			return nil, err
		}
		auditStatus, err := auditClient.GetStatus()
		if err != nil {
			return nil, err
		}
		if auditStatus.Enabled == 0 {
			if err = auditClient.SetEnabled(true, libaudit.WaitForReply); err != nil {
				return nil, err
			}
		}
		go a.setWorthCheckingIPTablesRoutine(auditClient)
	} else {
		a.worthCheckingIPTables = true
	}
	logrus.Infof("Auditing enabled (%d)", auditStatus.Enabled)
	return startGuestAgentRoutines(a, true), nil
}

// startGuestAgentRoutines sets worthCheckingIPTables to true if auditing is not supported,
// instead of using setWorthCheckingIPTablesRoutine to dynamically set the value.
//
// Auditing is not supported in a kernels and is not currently supported outside of the initial namespace, so does not work
// from inside a container or WSL2 instance, for example.
func startGuestAgentRoutines(a *agent, supportsAuditing bool) *agent {
	if !supportsAuditing {
		a.worthCheckingIPTables = true
	}
	go a.fixSystemTimeSkew()

	return a
}

type agent struct {
	// Ticker is like time.Ticker.
	// We can't use inotify for /proc/net/tcp, so we need this ticker to
	// reload /proc/net/tcp.
	newTicker func() (<-chan time.Time, func())

	worthCheckingIPTables   bool
	worthCheckingIPTablesMu sync.RWMutex
	IptablesIdle            time.Duration
	latestIPTables          []iptables.Entry
	latestIPTablesMu        sync.RWMutex
	dockerEventMonitor      *events.DockerEventMonitor
	containerdEventMonitor  *events.ContainerdEventMonitor
	kubeServiceWatcher      *events.KubeServiceWatcher
}

// setWorthCheckingIPTablesRoutine sets worthCheckingIPTables to be true
// when received NETFILTER_CFG audit message.
//
// setWorthCheckingIPTablesRoutine sets worthCheckingIPTables to be false
// when no NETFILTER_CFG audit message was received for the iptablesIdle time.
func (a *agent) setWorthCheckingIPTablesRoutine(auditClient *libaudit.AuditClient) {
	logrus.Info("setWorthCheckingIPTablesRoutine(): monitoring netfilter audit events")
	var latestTrue time.Time
	go func() {
		for {
			time.Sleep(a.IptablesIdle)
			a.worthCheckingIPTablesMu.Lock()
			// time is monotonic, see https://pkg.go.dev/time#hdr-Monotonic_Clocks
			elapsedSinceLastTrue := time.Since(latestTrue)
			if elapsedSinceLastTrue >= a.IptablesIdle {
				logrus.Debug("setWorthCheckingIPTablesRoutine(): setting to false")
				a.worthCheckingIPTables = false
			}
			a.worthCheckingIPTablesMu.Unlock()
		}
	}()
	for {
		msg, err := auditClient.Receive(false)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if msg.Type == auparse.AUDIT_NETFILTER_CFG {
			a.worthCheckingIPTablesMu.Lock()
			logrus.Debug("setWorthCheckingIPTablesRoutine(): setting to true")
			a.worthCheckingIPTables = true
			latestTrue = time.Now()
			a.worthCheckingIPTablesMu.Unlock()
		}
	}
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

	errorCh := make(chan error)
	if a.kubeServiceWatcher != nil {
		go func() {
			if err := a.kubeServiceWatcher.MonitorServices(ctx, ch); err != nil {
				errorCh <- err
			}
		}()
	}
	if a.containerdEventMonitor != nil {
		go a.containerdEventMonitor.MonitorPorts(ctx, ch)
	}
	if a.dockerEventMonitor != nil {
		go a.dockerEventMonitor.MonitorPorts(ctx, ch)
	}

	tickerCh, tickerClose := a.newTicker()
	defer tickerClose()
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
		case err := <-errorCh:
			logrus.Errorf("event monitoring failed: %s", err)
		case _, ok := <-tickerCh:
			if !ok {
				return
			}
			logrus.Debug("tick!")
		}
	}
}

func (a *agent) LocalPorts(ctx context.Context) ([]*api.IPPort, error) {
	var res []*api.IPPort
	tcpParsed, err := procnettcp.ParseFiles()
	if err != nil {
		return res, err
	}

	for _, f := range tcpParsed {
		switch f.Kind {
		case procnettcp.TCP, procnettcp.TCP6:
			if f.State == procnettcp.TCPListen {
				res = append(res,
					&api.IPPort{
						Ip:       f.IP.String(),
						Port:     int32(f.Port),
						Protocol: "tcp",
					})
			}
		case procnettcp.UDP, procnettcp.UDP6:
			if f.State == procnettcp.UDPUnconnected {
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

	a.worthCheckingIPTablesMu.RLock()
	worthCheckingIPTables := a.worthCheckingIPTables
	a.worthCheckingIPTablesMu.RUnlock()
	logrus.Debugf("LocalPorts(): worthCheckingIPTables=%v", worthCheckingIPTables)

	var ipts []iptables.Entry
	if a.worthCheckingIPTables {
		ipts, err = iptables.GetPorts(ctx)
		if err != nil {
			return res, err
		}
		a.latestIPTablesMu.Lock()
		a.latestIPTables = ipts
		a.latestIPTablesMu.Unlock()
	} else {
		a.latestIPTablesMu.RLock()
		ipts = a.latestIPTables
		a.latestIPTablesMu.RUnlock()
	}

	for _, ipt := range ipts {
		port := int32(ipt.AddrPort.Port())
		// Make sure the port isn't already listed from procnettcp
		found := false
		for _, re := range res {
			if re.Port == port {
				found = true
			}
		}
		if !found {
			if ipt.TCP {
				res = append(res,
					&api.IPPort{
						Ip:       ipt.AddrPort.Addr().String(),
						Port:     port,
						Protocol: "tcp",
					})
			}
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
