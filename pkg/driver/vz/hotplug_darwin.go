//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"fmt"
	"sync"

	"github.com/Code-Hex/vz/v3"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

var _ driver.FSHotPlugger = (*LimaVzDriver)(nil)

// slotSet tracks which spare hot-mount device slots are free. Callers hold the
// surrounding vzHotPlugState mutex.
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

// vzHotPlugState tracks the runtime hot-mounts mapped onto the spare virtio-fs devices.
type vzHotPlugState struct {
	mu      sync.Mutex
	slots   *slotSet
	devices map[string]int // deviceID -> slot index
}

func (l *LimaVzDriver) hotPlugStateLazy() *vzHotPlugState {
	l.hotPlugOnce.Do(func() {
		l.hotPlug = &vzHotPlugState{
			slots:   newSlotSet(vzHotMountSlots),
			devices: map[string]int{},
		}
	})
	return l.hotPlug
}

// HotPlugFS populates one of the spare virtio-fs devices with the requested host
// directory by setting its share at runtime. VZ has no device hot-plug API, so the
// device itself is reserved at boot; only its backing share changes here.
func (l *LimaVzDriver) HotPlugFS(_ context.Context, req *driver.HotPlugFSRequest) (*driver.HotPlugFSResponse, error) {
	if req.Type != limatype.VIRTIOFS {
		return nil, fmt.Errorf("%w: the VZ driver supports only virtiofs hot-mount, got %#q", driver.ErrFSHotPlugUnsupported, req.Type)
	}
	if l.Instance.Config.OS != nil && *l.Instance.Config.OS == limatype.DARWIN {
		return nil, fmt.Errorf("%w: hot-mount is not supported for macOS guests", driver.ErrFSHotPlugUnsupported)
	}
	if l.machine == nil {
		return nil, fmt.Errorf("instance is not running")
	}

	hp := l.hotPlugStateLazy()
	hp.mu.Lock()
	defer hp.mu.Unlock()

	slot, ok := hp.slots.allocate()
	if !ok {
		return nil, fmt.Errorf("no free hot-mount slot (max %d concurrent hot-mounts reached)", vzHotMountSlots)
	}

	directory, err := vz.NewSharedDirectory(req.HostPath, !req.Writable)
	if err != nil {
		hp.slots.release(slot)
		return nil, err
	}
	share, err := vz.NewSingleDirectoryShare(directory)
	if err != nil {
		hp.slots.release(slot)
		return nil, err
	}
	if err := l.machine.SetVirtioFileSystemDeviceShareAtIndex(slot, share); err != nil {
		hp.slots.release(slot)
		return nil, fmt.Errorf("failed to set share on hot-mount slot %d: %w", slot, err)
	}

	deviceID := fmt.Sprintf("vzfs-%d", slot)
	hp.devices[deviceID] = slot
	return &driver.HotPlugFSResponse{DeviceID: deviceID, Tag: hotMountTag(slot)}, nil
}

// HotUnplugFS clears the share of the spare device back to the empty placeholder.
func (l *LimaVzDriver) HotUnplugFS(_ context.Context, req *driver.HotUnplugFSRequest) error {
	hp := l.hotPlugStateLazy()
	hp.mu.Lock()
	defer hp.mu.Unlock()

	slot, ok := hp.devices[req.DeviceID]
	if !ok {
		return fmt.Errorf("hot-plug device %#q not found", req.DeviceID)
	}
	if l.machine == nil {
		return fmt.Errorf("instance is not running")
	}

	directory, err := vz.NewSharedDirectory(hotMountPlaceholderDir(l.Instance), true)
	if err != nil {
		return err
	}
	share, err := vz.NewSingleDirectoryShare(directory)
	if err != nil {
		return err
	}
	if err := l.machine.SetVirtioFileSystemDeviceShareAtIndex(slot, share); err != nil {
		return fmt.Errorf("failed to clear share on hot-mount slot %d: %w", slot, err)
	}
	hp.slots.release(slot)
	delete(hp.devices, req.DeviceID)
	return nil
}
