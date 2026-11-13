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
#include <pthread.h>
#include <signal.h>
#include <stdlib.h>

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

// A signal on a thread blocked in IOKit aborts the call. Faults stay deliverable.
static void blockAsyncSignals(sigset_t *old) {
    sigset_t set;
    sigfillset(&set);
    sigdelset(&set, SIGSEGV);
    sigdelset(&set, SIGBUS);
    sigdelset(&set, SIGFPE);
    sigdelset(&set, SIGILL);
    sigdelset(&set, SIGABRT);
    sigdelset(&set, SIGTRAP);
    pthread_sigmask(SIG_BLOCK, &set, old);
}

#define MASKED(call) ({ sigset_t old_; blockAsyncSignals(&old_); int ret_ = (call); pthread_sigmask(SIG_SETMASK, &old_, NULL); ret_; })

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

static int deviceReEnumerate(IOUSBDeviceInterface320 **dev) {
    return MASKED((*dev)->USBDeviceReEnumerate(dev, 0));
}

static int deviceSetConfiguration(IOUSBDeviceInterface320 **dev, uint8_t cfg) {
    return MASKED((*dev)->SetConfiguration(dev, cfg));
}

static int deviceGetConfiguration(IOUSBDeviceInterface320 **dev, uint8_t *cfg) {
    return (*dev)->GetConfiguration(dev, cfg);
}

static int deviceControlRequest(IOUSBDeviceInterface320 **dev,
    uint8_t bmReqType, uint8_t bReq, uint16_t wVal, uint16_t wIdx,
    void *data, uint16_t wLen, uint32_t timeoutMs, uint32_t *done) {
    IOUSBDevRequestTO req;
    req.bmRequestType = bmReqType;
    req.bRequest = bReq;
    req.wValue = wVal;
    req.wIndex = wIdx;
    req.wLength = wLen;
    req.pData = data;
    req.wLenDone = 0;
    req.noDataTimeout = 0;
    req.completionTimeout = timeoutMs;
    int ret = MASKED((*dev)->DeviceRequestTO(dev, &req));
    *done = req.wLenDone;
    return ret;
}

static int deviceAbortPipeZero(IOUSBDeviceInterface320 **dev) {
    return (*dev)->USBDeviceAbortPipeZero(dev);
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

static int interfaceOpen(IOUSBInterfaceInterface300 **intf) {
    return (*intf)->USBInterfaceOpen(intf);
}

static int interfaceClose(IOUSBInterfaceInterface300 **intf) {
    return (*intf)->USBInterfaceClose(intf);
}

static int interfaceSetAltSetting(IOUSBInterfaceInterface300 **intf, uint8_t alt) {
    return MASKED((*intf)->SetAlternateInterface(intf, alt));
}

static int interfaceGetNumEndpoints(IOUSBInterfaceInterface300 **intf, uint8_t *num) {
    return (*intf)->GetNumEndpoints(intf, num);
}

static int interfaceGetPipeProperties(IOUSBInterfaceInterface300 **intf,
    uint8_t pipeRef, uint8_t *direction, uint8_t *number,
    uint8_t *transferType, uint16_t *maxPacketSize, uint8_t *interval) {
    return (*intf)->GetPipeProperties(intf, pipeRef, direction, number,
        transferType, maxPacketSize, interval);
}

// GetPipeProperties scales its size with burst and SetPipePolicy; this is the raw wMaxPacketSize.
static void interfaceRawMaxPacket(IOUSBInterfaceInterface300 **intf, uint8_t dir, uint8_t num, uint16_t *mps) {
    UInt8 alt, type, interval;
    if ((*intf)->GetAlternateSetting(intf, &alt) == kIOReturnSuccess) {
        (*intf)->GetEndpointProperties(intf, alt, num, dir, &type, mps, &interval);
    }
}

static int interfaceReadPipe(IOUSBInterfaceInterface300 **intf,
    uint8_t pipeRef, void *buf, uint32_t *size) {
    return MASKED((*intf)->ReadPipe(intf, pipeRef, buf, size));
}

static int interfaceWritePipe(IOUSBInterfaceInterface300 **intf,
    uint8_t pipeRef, void *buf, uint32_t size) {
    return MASKED((*intf)->WritePipe(intf, pipeRef, buf, size));
}

static int interfaceAbortPipe(IOUSBInterfaceInterface300 **intf, uint8_t pipeRef) {
    return (*intf)->AbortPipe(intf, pipeRef);
}

static int interfaceClearStall(IOUSBInterfaceInterface300 **intf, uint8_t pipeRef) {
    return MASKED((*intf)->ClearPipeStallBothEnds(intf, pipeRef));
}

static int interfaceNumber(IOUSBInterfaceInterface300 **intf, uint8_t *num) {
    return (*intf)->GetInterfaceNumber(intf, num);
}

static void retainInterfaceInterface(IOUSBInterfaceInterface300 **intf) {
    (*intf)->AddRef(intf);
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
	"context"
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/strutil"
)

type pipe struct {
	ref       C.uint8_t
	intf      **C.IOUSBInterfaceInterface300
	maxPacket C.uint16_t
}

const (
	reqClearFeature     = 0x01
	reqSetConfiguration = 0x09
	reqSetInterface     = 0x0b
)

const abortRetry = 50 * time.Millisecond

var reenumerateWait = 2 * time.Second

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
		Busid:       busidLocationID(loc),
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

type allowlistProvider struct {
	instDir string
}

func NewProvider(instDir string) Provider {
	return &allowlistProvider{instDir: instDir}
}

func (p *allowlistProvider) Devices() ([]DeviceInfo, error) {
	allow, err := ReadAllowlist(p.instDir)
	if err != nil {
		return nil, err
	}
	if len(allow) == 0 {
		return nil, nil
	}
	hosts, err := List()
	if err != nil {
		return nil, err
	}
	var out []DeviceInfo
	for _, info := range hosts {
		if Allowed(allow, EntryFor(info)) {
			out = append(out, info)
		}
	}
	return out, nil
}

func (p *allowlistProvider) Open(busid string) (Device, error) {
	svcs, err := usbServices()
	if err != nil {
		return nil, err
	}
	defer releaseServices(svcs)
	for _, svc := range svcs {
		info, ok := readInfo(svc)
		if !ok || info.Busid != busid {
			continue
		}
		// Before the lock too, so a VM with no grant cannot touch another VM's lock.
		if err := checkGrant(p.instDir, info); err != nil {
			return nil, err
		}
		// Only now is busid safe in a path: the guest sent it, but it matched the host's.
		holder := RealName(p.instDir)
		lock, err := lockGranted(p.instDir, holder, info)
		if err != nil {
			return nil, err
		}
		g, err := openService(svc, info)
		if err != nil {
			_ = lock.Close()
			return nil, err
		}
		return &lockedDevice{Device: g, lock: lock, holder: holder, reenumerate: g.reenumerate}, nil
	}
	return nil, fmt.Errorf("usb device %s not found on host", busid)
}

func (p *allowlistProvider) Unplugged(busid string) {
	err := UpdateAllowlist(p.instDir, func(list []AllowEntry) []AllowEntry {
		return slices.DeleteFunc(list, func(e AllowEntry) bool { return e.Busid == busid })
	})
	if err != nil {
		logrus.WithError(err).Warnf("usbip: dropping grant for %s failed", busid)
	}
}

func openService(svc C.io_service_t, info DeviceInfo) (*iokitDevice, error) {
	devIf := C.openDeviceInterface(svc)
	if devIf == nil {
		return nil, fmt.Errorf("failed to get device interface for %s", info.Busid)
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
				info.Busid, uint32(ret))
		} else {
			return nil, fmt.Errorf("failed to open device %s: 0x%08x", info.Busid, uint32(ret))
		}
	} else {
		logrus.Debugf("usbip: device %s opened normally", info.Busid)
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

// reenumerate clears device state, as the kernel stub's reset does at session end. ResetDevice does nothing on macOS.
// It waits so that an attach right after detach finds the new registry entry.
func (g *iokitDevice) reenumerate() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.opened {
		return
	}
	g.releaseLocked()
	if ret := C.deviceReEnumerate(g.ptr); ret != C.kIOReturnSuccess {
		logrus.Debugf("usbip: %s: re-enumerate: IOReturn 0x%08x", g.info.Busid, uint32(ret))
		return
	}
	for deadline := time.Now().Add(reenumerateWait); time.Now().Before(deadline); time.Sleep(abortRetry) {
		hosts, err := List()
		if err == nil && slices.ContainsFunc(hosts, func(h DeviceInfo) bool {
			return h.Busid == g.info.Busid && h.RegistryID != g.info.RegistryID
		}) {
			return
		}
	}
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

// kIOReturnNotResponding is not gone: a hang must not end the session, and the Gone poll catches an unplug.
func ioReturnErr(ret C.int) error {
	var e urbErrno
	switch ret {
	case C.kIOReturnNoDevice:
		return fmt.Errorf("%w: IOKit 0x%08x", ErrDeviceGone, uint32(ret))
	case C.kIOUSBPipeStalled, C.kUSBHostReturnPipeStalled:
		e = errnoEPIPE
	case C.kIOUSBTransactionTimeout, C.kIOReturnTimeout:
		e = errnoETIME
	case C.kIOReturnOverrun:
		e = errnoEOVERFLOW
	default:
		e = errnoEPROTO
	}
	return fmt.Errorf("IOKit 0x%08x: %w", uint32(ret), e)
}

// IOKit calls have no timeout, and an abort landing before the call is queued is lost, so repeat it.
func abortOnCancel(ctx context.Context, abort func()) (join func()) {
	done, exited := make(chan struct{}), make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(exited)
		for {
			abort()
			select {
			case <-done:
				return
			case <-time.After(abortRetry):
			}
		}
	})
	return func() {
		close(done)
		if !stop() {
			<-exited
		}
	}
}

func controlSetup(setup [8]byte) (rType, request uint8, value, index uint16) {
	le := binary.LittleEndian
	return setup[0], setup[1], le.Uint16(setup[2:]), le.Uint16(setup[4:])
}

func (g *iokitDevice) Control(ctx context.Context, setup [8]byte, data []byte) (int, error) {
	rType, request, value, index := controlSetup(setup)

	switch {
	case rType == 0x00 && request == reqSetConfiguration:
		return 0, g.setConfiguration(uint8(value))
	case rType == 0x01 && request == reqSetInterface:
		return 0, g.setInterface(uint8(index), uint8(value))
	case rType == 0x02 && request == reqClearFeature && value == 0:
		// No pipe means ep0 or an unclaimed interface: forward the request as is.
		if p, err := g.findPipe(uint8(index), index&0x80 != 0); err == nil {
			defer C.releaseInterfaceInterface(p.intf)
			if ret := C.interfaceClearStall(p.intf, p.ref); ret != C.kIOReturnSuccess {
				return 0, fmt.Errorf("clear halt %#02x: %w", index, ioReturnErr(ret))
			}
			return 0, nil
		}
	}

	var ptr unsafe.Pointer
	if len(data) > 0 {
		ptr = unsafe.Pointer(&data[0])
	}

	// USBDeviceAbortPipeZero refuses an unopened device, so only a timeout can free the call.
	// ponytail: cuts a request over 5s on such a device, no abort route exists there.
	var timeout C.uint32_t
	if !g.opened {
		timeout = C.kUSBDefaultControlNoDataTimeoutMS
	}
	defer abortOnCancel(ctx, func() { C.deviceAbortPipeZero(g.ptr) })()
	var done C.uint32_t
	ret := C.deviceControlRequest(g.ptr,
		C.uint8_t(rType), C.uint8_t(request),
		C.uint16_t(value), C.uint16_t(index),
		ptr, C.uint16_t(len(data)), timeout, &done)
	if ret != C.kIOReturnSuccess {
		return 0, fmt.Errorf("control transfer: %w", ioReturnErr(ret))
	}
	return min(int(done), len(data)), nil
}

func (g *iokitDevice) setConfiguration(num uint8) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// SetConfiguration terminates every macOS driver, and that loses a mounted volume's dirty cache.
	if g.opened && !g.mounted() {
		g.releaseLocked()
		if ret := C.deviceSetConfiguration(g.ptr, C.uint8_t(num)); ret != C.kIOReturnSuccess {
			return fmt.Errorf("set configuration %d: %w", num, ioReturnErr(ret))
		}
	} else {
		// Without SetConfiguration only the active configuration can succeed.
		var cur C.uint8_t
		if ret := C.deviceGetConfiguration(g.ptr, &cur); ret != C.kIOReturnSuccess {
			return fmt.Errorf("get configuration: %w", ioReturnErr(ret))
		}
		if uint8(cur) != num {
			return fmt.Errorf("set configuration %d: device stays at %d: %w", num, uint8(cur), urbErrno(errnoEPIPE))
		}
	}
	g.claimLocked()
	return nil
}

func (g *iokitDevice) setInterface(num, alt uint8) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	intf, ok := g.ifaces[num]
	if !ok {
		g.claimLocked()
		if intf, ok = g.ifaces[num]; !ok {
			return fmt.Errorf("set interface %d alt %d: interface not claimed", num, alt)
		}
	}
	if ret := C.interfaceSetAltSetting(intf, C.uint8_t(alt)); ret != C.kIOReturnSuccess {
		// Pipes may be gone after a failure. The next miss reopens the interface.
		g.dropInterfaceLocked(num)
		return fmt.Errorf("set interface %d alt %d: %w", num, alt, ioReturnErr(ret))
	}
	g.mapPipesLocked(intf)
	return nil
}

func (g *iokitDevice) openNubs() []**C.IOUSBInterfaceInterface300 {
	var iter C.io_iterator_t
	if ret := C.deviceInterfaceIterator(g.ptr, &iter); ret != C.kIOReturnSuccess {
		logrus.Debugf("usbip: %s: interface iterator: IOReturn 0x%08x", g.info.Busid, uint32(ret))
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

// Runs at SET_CONFIGURATION to beat host apps, and on a pipe miss for late nubs.
// ponytail: each miss opens a plugin per nub, cache refused nubs if a guest hot loop shows up.
func (g *iokitDevice) claimLocked() {
	for _, intf := range g.nubs() {
		var cnum C.uint8_t
		ret := C.interfaceNumber(intf, &cnum)
		num := uint8(cnum)
		if _, held := g.ifaces[num]; ret != C.kIOReturnSuccess || held {
			C.releaseInterfaceInterface(intf)
			continue
		}
		if ret := C.interfaceOpen(intf); ret != C.kIOReturnSuccess {
			C.releaseInterfaceInterface(intf)
			if !g.refused[num] {
				g.refused[num] = true
				logrus.Warnf("usbip: %s: claim interface %d: IOReturn 0x%08x; macOS or another process may hold it",
					g.info.Busid, num, uint32(ret))
			}
			continue
		}
		g.ifaces[num] = intf
		g.mapPipesLocked(intf)
	}
}

func (g *iokitDevice) mapPipesLocked(intf **C.IOUSBInterfaceInterface300) {
	maps.DeleteFunc(g.pipes, func(_ uint8, p pipe) bool { return p.intf == intf })

	var n C.uint8_t
	if C.interfaceGetNumEndpoints(intf, &n) != C.kIOReturnSuccess {
		return
	}
	for ref := 1; ref <= int(n); ref++ {
		var dir, num, xferType, interval C.uint8_t
		var maxPkt C.uint16_t
		if C.interfaceGetPipeProperties(intf, C.uint8_t(ref), &dir, &num, &xferType, &maxPkt, &interval) != C.kIOReturnSuccess {
			continue
		}
		C.interfaceRawMaxPacket(intf, dir, num, &maxPkt)
		addr := uint8(num)
		if dir == C.kUSBIn {
			addr |= 0x80
		}
		g.pipes[addr] = pipe{ref: C.uint8_t(ref), intf: intf, maxPacket: maxPkt}
	}
}

// The caller releases the pin: SET_CONFIGURATION can drop the interface mid-call.
func (g *iokitDevice) findPipe(ep uint8, in bool) (pipe, error) {
	addr := ep & 0x0f
	if in {
		addr |= 0x80
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	p, ok := g.pipes[addr]
	if !ok {
		g.claimLocked()
		if p, ok = g.pipes[addr]; !ok {
			return pipe{}, fmt.Errorf("no interface provides endpoint %#02x", addr)
		}
	}
	C.retainInterfaceInterface(p.intf)
	return p, nil
}

func (g *iokitDevice) OutMaxPacket(ep uint8) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return int(g.pipes[ep&0x0f].maxPacket)
}

func (g *iokitDevice) Transfer(ctx context.Context, ep uint8, in bool, buf []byte) (int, error) {
	p, err := g.findPipe(ep, in)
	if err != nil {
		return 0, err
	}
	defer C.releaseInterfaceInterface(p.intf)
	defer abortOnCancel(ctx, func() { C.interfaceAbortPipe(p.intf, p.ref) })()

	var ptr unsafe.Pointer
	if len(buf) > 0 {
		ptr = unsafe.Pointer(&buf[0])
	}
	size := C.uint32_t(len(buf))
	var ret C.int
	if in {
		ret = C.interfaceReadPipe(p.intf, p.ref, ptr, &size)
	} else {
		ret = C.interfaceWritePipe(p.intf, p.ref, ptr, size)
	}
	if ret != C.kIOReturnSuccess {
		switch ret {
		case C.kIOUSBPipeStalled, C.kUSBHostReturnPipeStalled, C.kIOReturnAborted, C.kIOReturnNoDevice:
		default:
			// An overrun leaves the macOS pipe halted, and only a device stall is the guest's to clear.
			C.interfaceClearStall(p.intf, p.ref)
		}
		return 0, fmt.Errorf("transfer on ep %#02x: %w", ep, ioReturnErr(ret))
	}
	return int(size), nil
}
