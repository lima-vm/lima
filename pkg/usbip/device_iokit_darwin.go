// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && cgo

//nolint:gocritic // cgo adds its own unsafe import.
package usbip

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <IOKit/usb/IOUSBLib.h>
#include <IOKit/IOCFPlugIn.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

static kern_return_t usbDeviceIterator(io_iterator_t *iter) {
    CFMutableDictionaryRef match = IOServiceMatching("IOUSBHostDevice");
    return match ? IOServiceGetMatchingServices(kIOMainPortDefault, match, iter) : kIOReturnNoMemory;
}

static CFTypeRef regProp(io_service_t svc, const char *key) {
    CFStringRef k = CFStringCreateWithCString(kCFAllocatorDefault, key, kCFStringEncodingUTF8);
    CFTypeRef v = IORegistryEntryCreateCFProperty(svc, k, kCFAllocatorDefault, 0);
    CFRelease(k);
    return v;
}

static int regUint(io_service_t svc, const char *key, uint32_t *out) {
    CFTypeRef v = regProp(svc, key);
    if (!v) return 0;
    SInt64 n = 0;
    int ok = CFGetTypeID(v) == CFNumberGetTypeID() && CFNumberGetValue(v, kCFNumberSInt64Type, &n);
    CFRelease(v);
    *out = (uint32_t)n;
    return ok;
}

static void regString(io_service_t svc, const char *key, char *buf, int size) {
    CFTypeRef v = regProp(svc, key);
    if (!v) return;
    if (CFGetTypeID(v) != CFStringGetTypeID() || !CFStringGetCString(v, buf, size, kCFStringEncodingUTF8)) buf[0] = 0;
    CFRelease(v);
}

// A leaf IOMedia below svc with a client, such as a mounted volume. An error counts as yes.
static int hasOpenMedia(io_service_t svc) {
    io_iterator_t iter;
    if (IORegistryEntryCreateIterator(svc, kIOServicePlane, kIORegistryIterateRecursively, &iter) != KERN_SUCCESS) return 1;
    int found = 0;
    for (io_object_t o; !found && (o = IOIteratorNext(iter)); IOObjectRelease(o)) {
        if (!IOObjectConformsTo(o, "IOMedia")) continue;
        CFTypeRef leaf = regProp(o, "Leaf"), open = regProp(o, "Open");
        found = leaf == kCFBooleanTrue && open == kCFBooleanTrue;
        if (leaf) CFRelease(leaf);
        if (open) CFRelease(open);
    }
    IOObjectRelease(iter);
    return found;
}

static IOUSBDeviceInterface320** openDeviceInterface(io_service_t svc) {
    IOCFPlugInInterface **plugin = NULL;
    SInt32 score;
    kern_return_t kr = IOCreatePlugInInterfaceForService(svc,
        kIOUSBDeviceUserClientTypeID, kIOCFPlugInInterfaceID, &plugin, &score);
    if (kr != KERN_SUCCESS || !plugin) return NULL;

    IOUSBDeviceInterface320 **iface = NULL;
    HRESULT res = (*plugin)->QueryInterface(plugin,
        CFUUIDGetUUIDBytes(kIOUSBDeviceInterfaceID320), (LPVOID *)&iface);
    (*plugin)->Release(plugin);
    return (res == 0 && iface) ? iface : NULL;
}

static int deviceOpen(IOUSBDeviceInterface320 **dev) {
    return (*dev)->USBDeviceOpen(dev);
}

static int deviceClose(IOUSBDeviceInterface320 **dev) {
    return (*dev)->USBDeviceClose(dev);
}

static int deviceGetSpeed(IOUSBDeviceInterface320 **dev, uint8_t *speed) {
    return (*dev)->GetDeviceSpeed(dev, speed);
}

static int deviceInterfaceIterator(IOUSBDeviceInterface320 **dev, io_iterator_t *iter) {
    IOUSBFindInterfaceRequest req;
    req.bInterfaceClass = req.bInterfaceSubClass = req.bInterfaceProtocol = req.bAlternateSetting = kIOUSBFindInterfaceDontCare;
    return (*dev)->CreateInterfaceIterator(dev, &req, iter);
}

static void releaseDeviceInterface(IOUSBDeviceInterface320 **dev) {
    if (dev && *dev) {
        (*dev)->Release(dev);
    }
}

static IOUSBInterfaceInterface300** openInterfaceInterface(io_service_t svc) {
    IOCFPlugInInterface **plugin = NULL;
    SInt32 score;
    kern_return_t kr = IOCreatePlugInInterfaceForService(svc,
        kIOUSBInterfaceUserClientTypeID, kIOCFPlugInInterfaceID, &plugin, &score);
    if (kr != KERN_SUCCESS || !plugin) return NULL;

    IOUSBInterfaceInterface300 **iface = NULL;
    HRESULT res = (*plugin)->QueryInterface(plugin,
        CFUUIDGetUUIDBytes(kIOUSBInterfaceInterfaceID300), (LPVOID *)&iface);
    (*plugin)->Release(plugin);
    return (res == 0 && iface) ? iface : NULL;
}

static int interfaceClose(IOUSBInterfaceInterface300 **intf) {
    return (*intf)->USBInterfaceClose(intf);
}

static void releaseInterfaceInterface(IOUSBInterfaceInterface300 **intf) {
    if (intf && *intf) {
        (*intf)->Release(intf);
    }
}

#pragma clang diagnostic pop
*/
import "C"

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"unsafe"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/strutil"
)

type pipe struct {
	ref       C.uint8_t
	intf      **C.IOUSBInterfaceInterface300
	maxPacket C.uint16_t
}

func linkSpeed(dev **C.IOUSBDeviceInterface320) (uint32, error) {
	var s C.uint8_t
	if ret := C.deviceGetSpeed(dev, &s); ret != C.kIOReturnSuccess {
		return 0, fmt.Errorf("get device speed: IOReturn 0x%08x", uint32(ret))
	}
	switch s {
	case C.kUSBDeviceSpeedLow:
		return usbipSpeedLow, nil
	case C.kUSBDeviceSpeedFull:
		return usbipSpeedFull, nil
	case C.kUSBDeviceSpeedHigh:
		return usbipSpeedHigh, nil
	}
	// vhci treats every SuperSpeed variant alike.
	return usbipSpeedSuper, nil
}

func busidLocationID(loc uint32) string {
	return fmt.Sprintf("%d-0x%08x", loc>>24, loc)
}

// Retained registry entries. Nothing is opened, so no bus traffic.
func usbServices() ([]C.io_service_t, error) {
	var iter C.io_iterator_t
	if kr := C.usbDeviceIterator(&iter); kr != C.KERN_SUCCESS {
		return nil, fmt.Errorf("enumerating USB devices: 0x%08x", uint32(kr))
	}
	if iter == 0 {
		return nil, nil
	}
	defer C.IOObjectRelease(iter)
	var svcs []C.io_service_t
	for svc := C.IOIteratorNext(iter); svc != 0; svc = C.IOIteratorNext(iter) {
		svcs = append(svcs, svc)
	}
	return svcs, nil
}

func releaseServices(svcs []C.io_service_t) {
	for _, svc := range svcs {
		C.IOObjectRelease(svc)
	}
}

// False for hubs, which the kernel stub refuses too. A missing class reads 0, which is no hub.
func deviceInfo(num func(key string) (uint32, bool), str func(key string) string) (DeviceInfo, bool) {
	loc, hasLoc := num("locationID")
	vendor, hasVendor := num("idVendor")
	product, hasProduct := num("idProduct")
	class, _ := num("bDeviceClass")
	if !hasLoc || !hasVendor || !hasProduct || class == C.kUSBHubClass {
		return DeviceInfo{}, false
	}
	addr, _ := num("USB Address")
	return DeviceInfo{
		BusNum:      loc >> 24,
		DevNum:      addr,
		BusID:       busidLocationID(loc),
		Vendor:      uint16(vendor),
		Product:     uint16(product),
		VendorName:  strutil.RemoveControlChars(str("USB Vendor Name")),
		ProductName: strutil.RemoveControlChars(str("USB Product Name")),
	}, true
}

func readInfo(svc C.io_service_t) (DeviceInfo, bool) {
	var id C.uint64_t
	if C.IORegistryEntryGetRegistryEntryID(svc, &id) != C.KERN_SUCCESS {
		return DeviceInfo{}, false
	}
	info, ok := deviceInfo(func(key string) (uint32, bool) {
		k := C.CString(key)
		defer C.free(unsafe.Pointer(k))
		var v C.uint32_t
		ok := C.regUint(svc, k, &v) != 0
		return uint32(v), ok
	}, func(key string) string {
		k := C.CString(key)
		defer C.free(unsafe.Pointer(k))
		// A USB string has at most 126 UTF-16 units.
		var buf [512]C.char
		C.regString(svc, k, &buf[0], C.int(len(buf)))
		return C.GoString(&buf[0])
	})
	info.RegistryID = uint64(id)
	return info, ok
}

// List leaves Speed unset: GetDeviceSpeed needs a user client, and no list consumer reads it.
func List() ([]DeviceInfo, error) {
	svcs, err := usbServices()
	if err != nil {
		return nil, err
	}
	defer releaseServices(svcs)
	var out []DeviceInfo
	for _, svc := range svcs {
		if info, ok := readInfo(svc); ok {
			out = append(out, info)
		}
	}
	return out, nil
}

func openService(svc C.io_service_t, info DeviceInfo) (*iokitDevice, error) {
	devIf := C.openDeviceInterface(svc)
	if devIf == nil {
		return nil, fmt.Errorf("failed to get device interface for %s", info.BusID)
	}
	g, err := newDevice(svc, devIf, info)
	if err != nil {
		C.releaseDeviceInterface(devIf)
		return nil, err
	}
	return g, nil
}

type iokitDevice struct {
	info    DeviceInfo
	ptr     **C.IOUSBDeviceInterface320
	service C.io_service_t
	opened  bool

	// Replaced by the fake backend.
	nubs     func() []**C.IOUSBInterfaceInterface300
	services func() ([]C.io_service_t, error)
	mounted  func() bool

	mu      sync.Mutex
	ifaces  map[uint8]**C.IOUSBInterfaceInterface300
	pipes   map[uint8]pipe // by endpoint address, active alt settings only
	refused map[uint8]bool
}

func newDevice(svc C.io_service_t, devIf **C.IOUSBDeviceInterface320, info DeviceInfo) (*iokitDevice, error) {
	speed, err := linkSpeed(devIf)
	if err != nil {
		return nil, err
	}
	info.Speed = speed

	opened := false
	if ret := C.deviceOpen(devIf); ret != C.kIOReturnSuccess {
		if ret == C.kIOReturnExclusiveAccess {
			logrus.Debugf("usbip: device %s held by kernel driver (open=0x%08x), proceeding without USBDeviceOpen",
				info.BusID, uint32(ret))
		} else {
			return nil, fmt.Errorf("failed to open device %s: 0x%08x", info.BusID, uint32(ret))
		}
	} else {
		logrus.Debugf("usbip: device %s opened normally", info.BusID)
		opened = true
	}

	C.IOObjectRetain(svc)
	g := &iokitDevice{
		info:     info,
		ptr:      devIf,
		service:  svc,
		opened:   opened,
		services: usbServices,
		ifaces:   map[uint8]**C.IOUSBInterfaceInterface300{},
		pipes:    map[uint8]pipe{},
		refused:  map[uint8]bool{},
	}
	g.nubs = g.openNubs
	g.mounted = func() bool { return C.hasOpenMedia(g.service) != 0 }
	return g, nil
}

func (g *iokitDevice) Info() DeviceInfo {
	return g.info
}

// A replug creates a new registry entry, and a terminated entry stops matching.
func (g *iokitDevice) Gone() bool {
	svcs, err := g.services()
	if err != nil {
		logrus.WithError(err).Debug("usbip: presence check failed")
		return false
	}
	defer releaseServices(svcs)
	return !slices.ContainsFunc(svcs, func(svc C.io_service_t) bool {
		return C.IOObjectIsEqualTo(svc, g.service) != 0
	})
}

func (g *iokitDevice) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.releaseLocked()
	if g.opened {
		C.deviceClose(g.ptr)
	}
	C.releaseDeviceInterface(g.ptr)
	C.IOObjectRelease(g.service)
	return nil
}

func (g *iokitDevice) releaseLocked() {
	for num := range g.ifaces {
		g.dropInterfaceLocked(num)
	}
}

func (g *iokitDevice) dropInterfaceLocked(num uint8) {
	intf := g.ifaces[num]
	maps.DeleteFunc(g.pipes, func(_ uint8, p pipe) bool { return p.intf == intf })
	C.interfaceClose(intf)
	C.releaseInterfaceInterface(intf)
	delete(g.ifaces, num)
}

func (g *iokitDevice) openNubs() []**C.IOUSBInterfaceInterface300 {
	var iter C.io_iterator_t
	if ret := C.deviceInterfaceIterator(g.ptr, &iter); ret != C.kIOReturnSuccess {
		logrus.Debugf("usbip: %s: interface iterator: IOReturn 0x%08x", g.info.BusID, uint32(ret))
		return nil
	}
	defer C.IOObjectRelease(iter)

	var intfs []**C.IOUSBInterfaceInterface300
	for svc := C.IOIteratorNext(iter); svc != 0; svc = C.IOIteratorNext(iter) {
		if intf := C.openInterfaceInterface(svc); intf != nil {
			intfs = append(intfs, intf)
		}
		C.IOObjectRelease(svc)
	}
	return intfs
}
