// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && !no_vz

package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework AppKit -framework IOKit -framework Security
#include <stdlib.h>
#include <AppKit/AppKit.h>
#include <IOKit/IOKitLib.h>
#include <Security/Security.h>

// Virtualization APIs are only usable when built with an SDK that has them.
static int builtWithMacOS27SDK(void) {
    return __MAC_OS_X_VERSION_MAX_ALLOWED >= 270000;
}

static int hasEntitlement(const char *name) {
    SecTaskRef task = SecTaskCreateFromSelf(kCFAllocatorDefault);
    if (task == NULL) {
        return 0;
    }
    CFStringRef key = CFStringCreateWithCString(kCFAllocatorDefault, name, kCFStringEncodingUTF8);
    CFTypeRef value = SecTaskCopyValueForEntitlement(task, key, NULL);
    CFRelease(key);
    CFRelease(task);
    int result = value != NULL && CFGetTypeID(value) == CFBooleanGetTypeID() && CFBooleanGetValue(value);
    if (value != NULL) {
        CFRelease(value);
    }
    return result;
}

// hasGraphicSession reports whether the process belongs to a login session with
// access to the window server, unlike an SSH session.
static int hasGraphicSession(void) {
    SessionAttributeBits attrs = 0;
    if (SessionGetInfo(callerSecuritySession, NULL, &attrs) != errSessionSuccess) {
        return 1;
    }
    return (attrs & sessionHasGraphicAccess) != 0;
}

static void freeUSBString(char *s) { free(s); }

// The caller must free() the result.
static char *copyRegistryString(io_service_t service, const char *key) {
    CFStringRef cfKey = CFStringCreateWithCString(kCFAllocatorDefault, key, kCFStringEncodingUTF8);
    CFTypeRef value = IORegistryEntryCreateCFProperty(service, cfKey, kCFAllocatorDefault, 0);
    CFRelease(cfKey);
    if (value == NULL) {
        return NULL;
    }
    char *result = NULL;
    if (CFGetTypeID(value) == CFStringGetTypeID()) {
        CFIndex size = CFStringGetMaximumSizeForEncoding(CFStringGetLength(value), kCFStringEncodingUTF8) + 1;
        result = malloc(size);
        if (result != NULL && !CFStringGetCString(value, result, size, kCFStringEncodingUTF8)) {
            free(result);
            result = NULL;
        }
    }
    CFRelease(value);
    return result;
}

typedef struct {
    int found;
    char *serial;
    char *vendor;
    char *product;
    uint32_t locationID;
} usbInfo;

// The caller must free() the strings.
static usbInfo usbRegistryInfo(uint64_t registryID) {
    usbInfo result = {0};
    // IOServiceGetMatchingService consumes the matching dictionary.
    io_service_t service = IOServiceGetMatchingService(MACH_PORT_NULL, IORegistryEntryIDMatching(registryID));
    if (service == IO_OBJECT_NULL) {
        return result;
    }
    result.found = 1;
    result.serial = copyRegistryString(service, "USB Serial Number");
    result.vendor = copyRegistryString(service, "USB Vendor Name");
    result.product = copyRegistryString(service, "USB Product Name");
    CFTypeRef location = IORegistryEntryCreateCFProperty(service, CFSTR("locationID"), kCFAllocatorDefault, 0);
    if (location != NULL) {
        if (CFGetTypeID(location) == CFNumberGetTypeID()) {
            CFNumberGetValue(location, kCFNumberSInt32Type, &result.locationID);
        }
        CFRelease(location);
    }
    IOObjectRelease(service);
    return result;
}

@interface LimaUSBApplicationDelegate : NSObject <NSApplicationDelegate>
@end

@implementation LimaUSBApplicationDelegate
// -terminate: would exit() without stopping the VM.
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    return NSTerminateCancel;
}
@end

static dispatch_semaphore_t applicationRunning;
static dispatch_semaphore_t applicationStopped;

static void initApplicationSemaphores(void) {
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        applicationRunning = dispatch_semaphore_create(0);
        applicationStopped = dispatch_semaphore_create(0);
    });
}

// Must be called on the main thread; returns after stopApplication.
static void runRegularApplication(const char *badge) {
    initApplicationSemaphores();
    NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];
    [NSApplication sharedApplication];
    LimaUSBApplicationDelegate *delegate = [[LimaUSBApplicationDelegate alloc] init];
    id<NSApplicationDelegate> previousDelegate = [NSApp delegate];
    [NSApp setDelegate:delegate];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    [[NSApp dockTile] setBadgeLabel:[NSString stringWithUTF8String:badge]];
    [NSApp activateIgnoringOtherApps:YES];
    // Runs once the event loop is running.
    dispatch_async(dispatch_get_main_queue(), ^{
        dispatch_semaphore_signal(applicationRunning);
    });
    [NSApp run];
    [[NSApp dockTile] setBadgeLabel:nil];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
    [NSApp setDelegate:previousDelegate];
    [delegate release];
    [pool drain];
    dispatch_semaphore_signal(applicationStopped);
}

static int waitApplicationRunning(int64_t timeoutNanos) {
    initApplicationSemaphores();
    return dispatch_semaphore_wait(applicationRunning, dispatch_time(DISPATCH_TIME_NOW, timeoutNanos)) == 0;
}

// Must only be called while runRegularApplication runs the event loop.
static int stopApplication(int64_t timeoutNanos) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp stop:nil];
        // stop: takes effect only after the next event.
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                            location:NSZeroPoint
                                       modifierFlags:0
                                           timestamp:0
                                        windowNumber:0
                                             context:nil
                                             subtype:0
                                               data1:0
                                               data2:0];
        [NSApp postEvent:event atStart:YES];
    });
    return dispatch_semaphore_wait(applicationStopped, dispatch_time(DISPATCH_TIME_NOW, timeoutNanos)) == 0;
}
*/
import "C"

import "time"

func builtWithMacOS27SDK() bool { return C.builtWithMacOS27SDK() != 0 }

func hasEntitlement(name string) bool {
	cName := C.CString(name)
	defer C.freeUSBString(cName)
	return C.hasEntitlement(cName) != 0
}

func hasGraphicSession() bool { return C.hasGraphicSession() != 0 }

type usbRegistryInfo struct {
	Serial, Vendor, Product string
	LocationID              uint32
}

// lookupUSBRegistryInfo returns the IORegistry properties of the USB device
// with the given registry ID. ok is false if the device is gone.
func lookupUSBRegistryInfo(registryID uint64) (info usbRegistryInfo, ok bool) {
	c := C.usbRegistryInfo(C.uint64_t(registryID))
	if c.found == 0 {
		return info, false
	}
	for _, s := range []struct {
		c   *C.char
		dst *string
	}{{c.serial, &info.Serial}, {c.vendor, &info.Vendor}, {c.product, &info.Product}} {
		if s.c != nil {
			*s.dst = C.GoString(s.c)
			C.freeUSBString(s.c)
		}
	}
	info.LocationID = uint32(c.locationID)
	return info, true
}

// runRegularApplication shows badge, such as the instance name, on the Dock icon.
func runRegularApplication(badge string) {
	cBadge := C.CString(badge)
	defer C.freeUSBString(cBadge)
	C.runRegularApplication(cBadge)
}

func waitApplicationRunning(timeout time.Duration) bool {
	return C.waitApplicationRunning(C.int64_t(timeout.Nanoseconds())) != 0
}

func stopApplication(timeout time.Duration) bool {
	return C.stopApplication(C.int64_t(timeout.Nanoseconds())) != 0
}
