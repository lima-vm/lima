// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
	"github.com/digitalocean/go-qemu/qmp/raw"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

var _ driver.FSHotPlugger = (*LimaQemuDriver)(nil)

// vhostSockHotPlugFormat is the per-slot virtiofsd socket filename for hot-plugged shares.
const vhostSockHotPlugFormat = "virtiofsd-hp-%d.sock"

// slotSet tracks which spare pcie-root-port slots are free. It is not safe for
// concurrent use; callers must hold the surrounding hotPlugState mutex.
type slotSet struct {
	free []bool
}

func newSlotSet(n int) *slotSet {
	free := make([]bool, n)
	for i := range free {
		free[i] = true
	}
	return &slotSet{free: free}
}

// allocate reserves the lowest free slot, returning its index and true, or 0/false if none free.
func (s *slotSet) allocate() (int, bool) {
	for i, f := range s.free {
		if f {
			s.free[i] = false
			return i, true
		}
	}
	return 0, false
}

func (s *slotSet) release(i int) {
	if i >= 0 && i < len(s.free) {
		s.free[i] = true
	}
}

// hotPlugDevice records a runtime-attached filesystem share device.
type hotPlugDevice struct {
	deviceID    string
	slot        int
	mountType   limatype.MountType
	charOrFsdev string    // chardev id (virtiofs) or fsdev id (9p)
	virtiofsd   *exec.Cmd // virtiofs only
	sockPath    string    // virtiofs only
}

// hotPlugState is the per-instance registry of hot-plugged share devices.
type hotPlugState struct {
	mu      sync.Mutex
	slots   *slotSet
	devices map[string]*hotPlugDevice
}

func newHotPlugState() *hotPlugState {
	return &hotPlugState{
		slots:   newSlotSet(HotPlugRootPorts),
		devices: map[string]*hotPlugDevice{},
	}
}

func (l *LimaQemuDriver) hotPlugStateLazy() *hotPlugState {
	l.hotPlugOnce.Do(func() { l.hotPlug = newHotPlugState() })
	return l.hotPlug
}

func buildExecuteJSON(execute string, arguments map[string]any) ([]byte, error) {
	return json.Marshal(map[string]any{
		"execute":   execute,
		"arguments": arguments,
	})
}

func buildChardevAddJSON(charID, sockPath string) ([]byte, error) {
	return buildExecuteJSON("chardev-add", map[string]any{
		"id": charID,
		"backend": map[string]any{
			"type": "socket",
			"data": map[string]any{
				"addr": map[string]any{
					"type": "unix",
					"data": map[string]any{"path": sockPath},
				},
				"server": false,
			},
		},
	})
}

func build9pFsdevAddCmd(fsdevID, hostPath, securityModel string, writable bool) string {
	cmd := fmt.Sprintf("fsdev_add local,id=%s,path=%s,security_model=%s", fsdevID, hostPath, securityModel)
	if !writable {
		cmd += ",readonly=on"
	}
	return cmd
}

// connectQMP opens a connected QMP monitor for the running instance.
func (l *LimaQemuDriver) connectQMP() (*qmp.SocketMonitor, error) {
	qmpClient, err := newQmpClient(l.qemuConfig())
	if err != nil {
		return nil, err
	}
	if err := qmpClient.Connect(); err != nil {
		return nil, err
	}
	return qmpClient, nil
}

func (l *LimaQemuDriver) qemuConfig() Config {
	return Config{
		Name:         l.Instance.Name,
		InstanceDir:  l.Instance.Dir,
		LimaYAML:     l.Instance.Config,
		SSHLocalPort: l.SSHLocalPort,
		SSHAddress:   l.Instance.SSHAddress,
		VirtioGA:     l.virtioPort != "",
	}
}

// HotPlugFS attaches a 9p or virtiofs share device to the running VM and returns
// an opaque DeviceID for later detachment. The guest-side mount is performed by
// the host agent over SSH after this returns.
func (l *LimaQemuDriver) HotPlugFS(ctx context.Context, req *driver.HotPlugFSRequest) (*driver.HotPlugFSResponse, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("%w: filesystem hot-plug is only supported on Linux hosts", driver.ErrFSHotPlugUnsupported)
	}
	hp := l.hotPlugStateLazy()
	hp.mu.Lock()
	defer hp.mu.Unlock()

	slot, ok := hp.slots.allocate()
	if !ok {
		return nil, fmt.Errorf("no free PCIe hot-plug slot (max %d concurrent hot-mounts reached)", HotPlugRootPorts)
	}
	busID := HotPlugRootPortID(slot)
	deviceID := fmt.Sprintf("lima-fs-%d", slot)

	qmpClient, err := l.connectQMP()
	if err != nil {
		hp.slots.release(slot)
		return nil, err
	}
	defer func() { _ = qmpClient.Disconnect() }()

	var dev *hotPlugDevice
	switch req.Type {
	case limatype.VIRTIOFS:
		dev, err = l.attachVirtiofs(ctx, qmpClient, req, slot, busID, deviceID)
	case limatype.NINEP:
		dev, err = l.attach9p(qmpClient, req, slot, busID, deviceID)
	default:
		err = fmt.Errorf("hot-plug is not supported for mount type %#q", req.Type)
	}
	if err != nil {
		hp.slots.release(slot)
		return nil, err
	}
	hp.devices[deviceID] = dev
	logrus.Infof("Hot-plugged %s device %#q (tag %#q) on %#q", req.Type, deviceID, req.Tag, busID)
	return &driver.HotPlugFSResponse{DeviceID: deviceID}, nil
}

func (l *LimaQemuDriver) attachVirtiofs(ctx context.Context, qmpClient *qmp.SocketMonitor, req *driver.HotPlugFSRequest, slot int, busID, deviceID string) (*hotPlugDevice, error) {
	qExe, _, err := Exe(*l.Instance.Config.Arch)
	if err != nil {
		return nil, err
	}
	vhostExe, err := FindVirtiofsd(ctx, qExe)
	if err != nil {
		return nil, err
	}
	sockPath := filepath.Join(l.Instance.Dir, fmt.Sprintf(vhostSockHotPlugFormat, slot))
	if err := os.Remove(sockPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logrus.Warnf("Failed to remove old vhost socket: %v", err)
	}
	// virtiofsd must outlive the request, so it is not bound to ctx.
	vhostCmd := exec.Command(vhostExe, "--socket-path", sockPath, "--shared-dir", req.HostPath)
	if err := vhostCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start virtiofsd: %w", err)
	}
	if err := waitFileExists(sockPath, 30*time.Second); err != nil {
		killProcess(vhostCmd)
		return nil, fmt.Errorf("virtiofsd socket did not appear: %w", err)
	}

	charID := fmt.Sprintf("char-fs-hp-%d", slot)
	chardevAdd, err := buildChardevAddJSON(charID, sockPath)
	if err != nil {
		killProcess(vhostCmd)
		return nil, err
	}
	if _, err := qmpClient.Run(chardevAdd); err != nil {
		killProcess(vhostCmd)
		return nil, fmt.Errorf("chardev-add failed: %w", err)
	}

	queueSize := 1024
	if req.QueueSize != nil {
		queueSize = *req.QueueSize
	}
	deviceAdd, err := buildExecuteJSON("device_add", map[string]any{
		"driver":     "vhost-user-fs-pci",
		"id":         deviceID,
		"chardev":    charID,
		"tag":        req.Tag,
		"bus":        busID,
		"queue-size": queueSize,
	})
	if err != nil {
		killProcess(vhostCmd)
		return nil, err
	}
	if _, err := qmpClient.Run(deviceAdd); err != nil {
		if _, rmErr := qmpClient.Run(mustJSON("chardev-remove", map[string]any{"id": charID})); rmErr != nil {
			logrus.Warnf("failed to roll back chardev %#q: %v", charID, rmErr)
		}
		killProcess(vhostCmd)
		return nil, fmt.Errorf("device_add (virtiofs) failed: %w", err)
	}
	return &hotPlugDevice{
		deviceID:    deviceID,
		slot:        slot,
		mountType:   limatype.VIRTIOFS,
		charOrFsdev: charID,
		virtiofsd:   vhostCmd,
		sockPath:    sockPath,
	}, nil
}

func (l *LimaQemuDriver) attach9p(qmpClient *qmp.SocketMonitor, req *driver.HotPlugFSRequest, slot int, busID, deviceID string) (*hotPlugDevice, error) {
	// fsdev_add is issued as an HMP command whose options are comma-separated and
	// space-delimited, so a host path containing a comma or space would corrupt it.
	if strings.ContainsAny(req.HostPath, ", ") {
		return nil, fmt.Errorf("9p hot-mount does not support host paths containing spaces or commas: %q", req.HostPath)
	}
	securityModel := "none"
	if req.NineP != nil && req.NineP.SecurityModel != nil {
		securityModel = *req.NineP.SecurityModel
	}
	fsdevID := fmt.Sprintf("fsdev-hp-%d", slot)
	rawClient := raw.NewMonitor(qmpClient)
	if _, err := rawClient.HumanMonitorCommand(build9pFsdevAddCmd(fsdevID, req.HostPath, securityModel, req.Writable), nil); err != nil {
		return nil, fmt.Errorf("fsdev_add failed: %w", err)
	}
	deviceAdd, err := buildExecuteJSON("device_add", map[string]any{
		"driver":    "virtio-9p-pci",
		"id":        deviceID,
		"fsdev":     fsdevID,
		"mount_tag": req.Tag,
		"bus":       busID,
	})
	if err != nil {
		_, _ = rawClient.HumanMonitorCommand("fsdev_del "+fsdevID, nil)
		return nil, err
	}
	if _, err := qmpClient.Run(deviceAdd); err != nil {
		_, _ = rawClient.HumanMonitorCommand("fsdev_del "+fsdevID, nil)
		return nil, fmt.Errorf("device_add (9p) failed: %w", err)
	}
	return &hotPlugDevice{
		deviceID:    deviceID,
		slot:        slot,
		mountType:   limatype.NINEP,
		charOrFsdev: fsdevID,
	}, nil
}

// HotUnplugFS detaches a previously hot-plugged share device.
func (l *LimaQemuDriver) HotUnplugFS(ctx context.Context, req *driver.HotUnplugFSRequest) error {
	hp := l.hotPlugStateLazy()
	hp.mu.Lock()
	defer hp.mu.Unlock()

	dev, ok := hp.devices[req.DeviceID]
	if !ok {
		return fmt.Errorf("hot-plug device %#q not found", req.DeviceID)
	}
	qmpClient, err := l.connectQMP()
	if err != nil {
		return err
	}
	defer func() { _ = qmpClient.Disconnect() }()

	if err := l.detachDevice(ctx, qmpClient, dev); err != nil {
		return err
	}
	hp.slots.release(dev.slot)
	delete(hp.devices, req.DeviceID)
	logrus.Infof("Hot-unplugged device %#q", req.DeviceID)
	return nil
}

func (l *LimaQemuDriver) detachDevice(ctx context.Context, qmpClient *qmp.SocketMonitor, dev *hotPlugDevice) error {
	// Subscribe to events before issuing device_del so we don't miss DEVICE_DELETED.
	eventCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	events, err := qmpClient.Events(eventCtx)
	if err != nil {
		return err
	}
	deviceDel, err := buildExecuteJSON("device_del", map[string]any{"id": dev.deviceID})
	if err != nil {
		return err
	}
	if _, err := qmpClient.Run(deviceDel); err != nil {
		return fmt.Errorf("device_del failed: %w", err)
	}
	if err := waitDeviceDeleted(eventCtx, events, dev.deviceID); err != nil {
		return err
	}
	// Keep draining events so the monitor's listen goroutine never blocks on the
	// (now subscribed) events channel while we issue the cleanup commands below.
	// The channel is closed when qmpClient is disconnected by the caller.
	go func() {
		for range events {
		}
	}()
	switch dev.mountType {
	case limatype.VIRTIOFS:
		if _, err := qmpClient.Run(mustJSON("chardev-remove", map[string]any{"id": dev.charOrFsdev})); err != nil {
			logrus.Warnf("chardev-remove failed: %v", err)
		}
		killVirtiofsd(dev)
	case limatype.NINEP:
		rawClient := raw.NewMonitor(qmpClient)
		if _, err := rawClient.HumanMonitorCommand("fsdev_del "+dev.charOrFsdev, nil); err != nil {
			logrus.Warnf("fsdev_del failed: %v", err)
		}
	}
	return nil
}

// waitDeviceDeleted blocks until a DEVICE_DELETED event for deviceID arrives or ctx expires.
func waitDeviceDeleted(ctx context.Context, events <-chan qmp.Event, deviceID string) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for guest to release device %#q: %w", deviceID, ctx.Err())
		case ev, ok := <-events:
			if !ok {
				return fmt.Errorf("event stream closed before device %#q was released", deviceID)
			}
			if ev.Event != "DEVICE_DELETED" {
				continue
			}
			if id, _ := ev.Data["device"].(string); id == deviceID {
				return nil
			}
		}
	}
}

// closeHotPlug kills any leaked virtiofsd processes. Best-effort; called on Stop.
func (l *LimaQemuDriver) closeHotPlug() {
	if l.hotPlug == nil {
		return
	}
	l.hotPlug.mu.Lock()
	defer l.hotPlug.mu.Unlock()
	for _, dev := range l.hotPlug.devices {
		killVirtiofsd(dev)
	}
	l.hotPlug.devices = map[string]*hotPlugDevice{}
}

// killProcess terminates and reaps a process, avoiding a zombie.
func killProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// killVirtiofsd terminates a device's virtiofsd (if any) and removes its socket.
func killVirtiofsd(dev *hotPlugDevice) {
	killProcess(dev.virtiofsd)
	if dev.sockPath != "" {
		_ = os.Remove(dev.sockPath)
	}
}

func mustJSON(execute string, arguments map[string]any) []byte {
	b, err := buildExecuteJSON(execute, arguments)
	if err != nil {
		// arguments are static maps of strings/ints; marshalling cannot fail.
		panic(err)
	}
	return b
}
