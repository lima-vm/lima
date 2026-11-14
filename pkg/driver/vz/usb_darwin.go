//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Code-Hex/vz/v4"
	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

// usbAccessoryEntitlement is restricted. A binary that claims it without a
// provisioning profile that grants it is killed at launch.
const usbAccessoryEntitlement = "com.apple.developer.accessory-access.usb"

// usbPassthroughUnavailableReason returns why this process cannot pass host USB
// devices through to a VM, or "" if it can.
var usbPassthroughUnavailableReason = sync.OnceValue(func() string {
	macOSProductVersion, err := osutil.ProductVersion()
	if err != nil {
		return fmt.Sprintf("failed to get the macOS version: %v", err)
	}
	if macOSProductVersion.LessThan(*semver.New("27.0.0")) {
		return fmt.Sprintf("USB passthrough requires macOS 27 or later, got %#q", macOSProductVersion)
	}
	if !builtWithMacOS27SDK() {
		return "USB passthrough requires Lima to be built with the macOS 27 SDK or later"
	}
	if !hasEntitlement(usbAccessoryEntitlement) {
		return fmt.Sprintf("USB passthrough requires the vz driver to be signed with the %#q entitlement", usbAccessoryEntitlement)
	}
	return ""
})

// usbPassthrough is registered before the VM is created, so that devices can be
// attached at boot, and bound to the VM once it runs.
type usbPassthrough struct {
	rules           []limatype.USBDeviceMatch
	accessoryEvents chan vz.USBAccessoryEvent
	stopAccessories func()
	// UUID to registry ID.
	bootDevices map[string]uint64

	// Set by bind.
	controller      *vz.USBController
	cancel          context.CancelFunc
	stopDisconnects func()

	opMu sync.Mutex

	mu sync.Mutex
	// Keyed by registry ID.
	accessories map[uint64]*vz.USBAccessory
	attached    map[uint64]vz.USBDevice
	// Keyed by usbDeviceKey.
	detachedByUser map[string]bool
}

func registerUSBPassthrough(ctx context.Context, rules []limatype.USBDeviceMatch) (*usbPassthrough, error) {
	p := &usbPassthrough{
		rules: rules,
		// vz drops events the channel cannot receive.
		accessoryEvents: make(chan vz.USBAccessoryEvent, 32),
		bootDevices:     make(map[string]uint64),
		accessories:     make(map[uint64]*vz.USBAccessory),
		attached:        make(map[uint64]vz.USBDevice),
		detachedByUser:  make(map[string]bool),
	}
	type result struct {
		accessories []*vz.USBAccessory
		stop        func()
		err         error
	}
	resultCh := make(chan result, 1)
	go func() {
		accessories, stop, err := vz.USBAccessoryChangedNotify(p.accessoryEvents)
		resultCh <- result{accessories, stop, err}
	}()
	var r result
	select {
	case r = <-resultCh:
	case <-ctx.Done():
		go func() {
			if r := <-resultCh; r.err == nil {
				r.stop()
			}
		}()
		return nil, ctx.Err()
	}
	if r.err != nil {
		return nil, fmt.Errorf("failed to register for USB accessories: %w", r.err)
	}
	p.stopAccessories = r.stop
	for _, a := range r.accessories {
		p.accessories[a.RegistryID()] = a
	}
	return p, nil
}

// waitForDevices returns the number of rules still unmatched at timeout. macOS
// reports devices as connection events after registration, not in its result.
// It must be called before bind, as it consumes those events.
func (p *usbPassthrough) waitForDevices(ctx context.Context, timeout time.Duration) int {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		unmatched := p.unmatchedRules()
		if unmatched == 0 {
			return 0
		}
		select {
		case ev := <-p.accessoryEvents:
			id := ev.Accessory.RegistryID()
			p.mu.Lock()
			if ev.Connected {
				p.accessories[id] = ev.Accessory
			} else {
				delete(p.accessories, id)
			}
			p.mu.Unlock()
		case <-timer.C:
			return unmatched
		case <-ctx.Done():
			return unmatched
		}
	}
}

func (p *usbPassthrough) unmatchedRules() int {
	devs := p.list()
	unmatched := 0
	for _, rule := range p.rules {
		if !slices.ContainsFunc(devs, func(dev driver.USBDevice) bool { return usbDeviceMatches(rule, dev) }) {
			unmatched++
		}
	}
	return unmatched
}

func (p *usbPassthrough) bootDeviceConfigurations() []vz.USBDeviceConfiguration {
	var configs []vz.USBDeviceConfiguration
	for _, dev := range p.list() {
		if !usbDeviceMatchesAny(p.rules, dev) {
			continue
		}
		p.mu.Lock()
		a := p.accessories[dev.RegistryID]
		p.mu.Unlock()
		deviceUUID := uuid.NewString()
		config, err := vz.NewUSBPassthroughDeviceConfiguration(a, vz.WithUSBDeviceUUID(deviceUUID))
		if err != nil {
			logrus.WithError(err).Warnf("Failed to configure USB device %s", usbDeviceString(dev))
			continue
		}
		p.bootDevices[strings.ToUpper(deviceUUID)] = dev.RegistryID
		configs = append(configs, config)
	}
	return configs
}

func (p *usbPassthrough) bind(ctx context.Context, machine *vz.VirtualMachine) error {
	controllers := machine.USBControllers()
	if len(controllers) == 0 {
		return errors.New("the VM has no USB controller")
	}
	p.controller = controllers[0]
	disconnects := make(chan vz.USBDevice, 32)
	stopDisconnects, err := p.controller.USBPassthroughDeviceDidDisconnectNotify(disconnects)
	if err != nil {
		return fmt.Errorf("failed to register for USB device disconnections: %w", err)
	}
	p.stopDisconnects = stopDisconnects

	p.mu.Lock()
	for _, dev := range p.controller.USBDevices() {
		if id, ok := p.bootDevices[strings.ToUpper(dev.UUID())]; ok {
			p.attached[id] = dev
		}
	}
	p.mu.Unlock()
	for _, dev := range p.list() {
		if dev.Attached {
			logrus.Infof("Attached USB device %s", usbDeviceString(dev))
		} else {
			p.attachIfMatching(dev.RegistryID)
		}
	}

	ctx, p.cancel = context.WithCancel(ctx)
	go p.loop(ctx, p.accessoryEvents, disconnects)
	return nil
}

func (p *usbPassthrough) stop() {
	if p.cancel != nil {
		p.cancel()
	}
	if p.stopDisconnects != nil {
		p.stopDisconnects()
	}
	// Unregistering waits for AccessoryAccess, which must not block stopping the VM.
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.stopAccessories()
	}()
	select {
	case <-done:
	case <-time.After(usbUnregisterTimeout):
		logrus.Warn("Timed out unregistering from USB accessories")
	}
}

const usbUnregisterTimeout = 5 * time.Second

func (p *usbPassthrough) loop(ctx context.Context, accessoryEvents <-chan vz.USBAccessoryEvent, disconnects <-chan vz.USBDevice) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-accessoryEvents:
			id := ev.Accessory.RegistryID()
			p.mu.Lock()
			if ev.Connected {
				p.accessories[id] = ev.Accessory
			} else {
				delete(p.accessories, id)
				delete(p.attached, id)
			}
			p.mu.Unlock()
			if ev.Connected {
				p.attachIfMatching(id)
			} else {
				logrus.Infof("USB device %#x disconnected from the host", id)
			}
		case dev := <-disconnects:
			// The framework has already detached the device.
			p.mu.Lock()
			for id, d := range p.attached {
				if d.UUID() == dev.UUID() {
					delete(p.attached, id)
					logrus.Infof("USB device %#x was detached because it disconnected", id)
				}
			}
			p.mu.Unlock()
		}
	}
}

func (p *usbPassthrough) attachIfMatching(id uint64) {
	dev, err := p.device(id)
	if err != nil {
		logrus.WithError(err).Warnf("Failed to inspect USB device %#x", id)
		return
	}
	if dev.Attached || !usbDeviceMatchesAny(p.rules, dev) {
		return
	}
	p.mu.Lock()
	detachedByUser := p.detachedByUser[usbDeviceKey(dev)]
	p.mu.Unlock()
	if detachedByUser {
		logrus.Infof("Not attaching USB device %s, which was detached with `limactl usb detach`", usbDeviceString(dev))
		return
	}
	if err := p.attach(id); err != nil {
		logrus.WithError(err).Warnf("Failed to attach USB device %s", usbDeviceString(dev))
		return
	}
	logrus.Infof("Attached USB device %s", usbDeviceString(dev))
}

func (p *usbPassthrough) device(id uint64) (driver.USBDevice, error) {
	p.mu.Lock()
	a, ok := p.accessories[id]
	_, attached := p.attached[id]
	p.mu.Unlock()
	if !ok {
		return driver.USBDevice{}, fmt.Errorf("USB device %#x is not connected, or Lima was not allowed to access it", id)
	}
	vendorID, productID, err := parseUSBDeviceDescriptor(a.DeviceDescriptorData())
	if err != nil {
		return driver.USBDevice{}, err
	}
	dev := driver.USBDevice{
		RegistryID: id,
		VendorID:   vendorID,
		ProductID:  productID,
		Attached:   attached,
	}
	if info, ok := lookupUSBRegistryInfo(id); ok {
		dev.Serial, dev.Manufacturer, dev.Product, dev.LocationID = info.Serial, info.Vendor, info.Product, info.LocationID
	}
	return dev, nil
}

func (p *usbPassthrough) list() []driver.USBDevice {
	p.mu.Lock()
	ids := slices.Sorted(maps.Keys(p.accessories))
	p.mu.Unlock()
	var devs []driver.USBDevice
	for _, id := range ids {
		if dev, err := p.device(id); err == nil {
			devs = append(devs, dev)
		}
	}
	return devs
}

func (p *usbPassthrough) attach(id uint64) error {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	a, ok := p.accessories[id]
	_, attached := p.attached[id]
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("USB device %#x is not connected, or Lima was not allowed to access it", id)
	}
	if attached {
		return fmt.Errorf("USB device %#x is already attached", id)
	}
	config, err := vz.NewUSBPassthroughDeviceConfiguration(a)
	if err != nil {
		return err
	}
	dev, err := vz.NewUSBPassthroughDevice(config)
	if err != nil {
		return err
	}
	if err := p.controller.Attach(dev); err != nil {
		return err
	}
	p.mu.Lock()
	p.attached[id] = dev
	p.mu.Unlock()
	return nil
}

func (p *usbPassthrough) detach(id uint64) error {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	dev, ok := p.attached[id]
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("USB device %#x is not attached", id)
	}
	if err := p.controller.Detach(dev); err != nil {
		return err
	}
	p.mu.Lock()
	delete(p.attached, id)
	p.mu.Unlock()
	return nil
}

func (p *usbPassthrough) attachByUser(id uint64) error {
	dev, err := p.device(id)
	if err != nil {
		return err
	}
	key := usbDeviceKey(dev)
	p.mu.Lock()
	wasDetached := p.detachedByUser[key]
	delete(p.detachedByUser, key)
	p.mu.Unlock()
	if err := p.attach(id); err != nil {
		if wasDetached {
			p.mu.Lock()
			p.detachedByUser[key] = true
			p.mu.Unlock()
		}
		return err
	}
	return nil
}

// detachByUser keeps the rules from reattaching the device, which reconnects
// to the host under a new registry ID.
func (p *usbPassthrough) detachByUser(id uint64) error {
	dev, err := p.device(id)
	if err != nil {
		return err
	}
	key := usbDeviceKey(dev)
	// Set before detaching, because the device reconnects as soon as it is detached.
	p.mu.Lock()
	wasDetached := p.detachedByUser[key]
	p.detachedByUser[key] = true
	p.mu.Unlock()
	if err := p.detach(id); err != nil {
		if !wasDetached {
			p.mu.Lock()
			delete(p.detachedByUser, key)
			p.mu.Unlock()
		}
		return err
	}
	return nil
}

// usbDeviceKey identifies a device across reconnections by serial number, or
// else by port.
func usbDeviceKey(dev driver.USBDevice) string {
	if dev.Serial != "" {
		return fmt.Sprintf("%04x:%04x:%s", dev.VendorID, dev.ProductID, dev.Serial)
	}
	return fmt.Sprintf("%04x:%04x@%08x", dev.VendorID, dev.ProductID, dev.LocationID)
}

func usbDeviceString(dev driver.USBDevice) string {
	s := fmt.Sprintf("%04x:%04x", dev.VendorID, dev.ProductID)
	if dev.Product != "" {
		s += fmt.Sprintf(" %#q", dev.Product)
	}
	if dev.Serial != "" {
		s += fmt.Sprintf(" (serial %#q)", dev.Serial)
	}
	return s
}
