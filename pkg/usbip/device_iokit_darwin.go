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

#pragma clang diagnostic pop
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/lima-vm/lima/v2/pkg/strutil"
)

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
