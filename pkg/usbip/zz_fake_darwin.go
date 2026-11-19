// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && cgo && usbipfake

package usbip

/*
#include <IOKit/IOKitLib.h>
#include <IOKit/usb/IOUSBLib.h>
#include <mach/mach.h>
#include <pthread.h>
#include <signal.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

#pragma clang diagnostic ignored "-Wdeprecated-declarations"

// Interface n, alt a has pipe 1 = IN n*2+1+a*8 and pipe 2 = OUT n*2+2+a*8.
// Interface 0 alt 0 is the FT231X shape: 0x81 and 0x02.
typedef struct {
    IOUSBInterfaceInterface300 *vt;
    UInt8 num, alt;
    int refs, open;
} FakeIntf;

typedef struct {
    IOReturn ioRet, openRet, altRet, ctrlRet;
    UInt32 ctrlDone;
    int block, unblock, blocked, aborted, missAborts, slowAbortUs;
    int aborts, abortZeros, opens, closes, altCalls, clears, requests, writes, reenums;
    int setConfigs, lastAlt, lastIntf, lastPipe, lastBufNull, lastSize;
    int noDataTimeout, completionTimeout, afterRelease, finalReleases;
    int total, numNubs, visible;
    IOReturn speedRet, devOpenRet;
    int speed, devOpens;
    IOReturn getConfigRet;
    int curConfig;
} FakeState;

static pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t cond = PTHREAD_COND_INITIALIZER;
FakeState f;
static sigset_t ioMask;
static FakeIntf *nubs[8];

static IOUSBInterfaceInterface300 intfVt;

static void newNubs(int n) {
    f.numNubs = n;
    for (int i = 0; i < n; i++) {
        // Never freed, so a call on a released object is counted, not a crash.
        nubs[i] = calloc(1, sizeof(FakeIntf));
        nubs[i]->vt = &intfVt;
        nubs[i]->num = i;
    }
}

static void enter(void *self) {
    pthread_mutex_lock(&mu);
    if (((FakeIntf *)self)->refs <= 0) f.afterRelease++;
}

// Called locked. If a block is armed, waits for an abort (1), a timeout (2) or fakeUnblock (0).
// A timeout runs at 1/50 of real time, so tests stay quick.
static int waitIfBlocked(UInt32 timeoutMs) {
    pthread_sigmask(SIG_BLOCK, NULL, &ioMask);
    if (!f.block) return 0;
    f.block = 0;
    f.blocked++;
    pthread_cond_broadcast(&cond);
    struct timespec dl;
    clock_gettime(CLOCK_REALTIME, &dl);
    long ns = dl.tv_nsec + timeoutMs * 20000L;
    dl.tv_sec += ns / 1000000000L;
    dl.tv_nsec = ns % 1000000000L;
    int timedOut = 0;
    while (!f.unblock && !f.aborted && !timedOut) {
        if (timeoutMs) timedOut = pthread_cond_timedwait(&cond, &mu, &dl) != 0;
        else pthread_cond_wait(&cond, &mu);
    }
    int r = f.aborted ? 1 : timedOut ? 2 : 0;
    f.blocked--;
    f.aborted = f.unblock = 0;
    return r;
}

static void abortBlocked(void) {
    if (f.missAborts > 0) {
        f.missAborts--;
    } else if (f.blocked) {
        f.aborted = 1;
        pthread_cond_broadcast(&cond);
    }
}

static ULONG intfAddRef(void *self) {
    pthread_mutex_lock(&mu);
    ULONG r = ++((FakeIntf *)self)->refs;
    pthread_mutex_unlock(&mu);
    return r;
}

static ULONG intfRelease(void *self) {
    pthread_mutex_lock(&mu);
    ULONG r = --((FakeIntf *)self)->refs;
    if (r == 0) f.finalReleases++;
    pthread_mutex_unlock(&mu);
    return r;
}

static IOReturn intfOpen(void *self) {
    enter(self);
    FakeIntf *i = self;
    IOReturn ret = f.openRet ? f.openRet : i->open ? kIOReturnExclusiveAccess : kIOReturnSuccess;
    if (!ret) {
        i->open = 1;
        i->alt = 0;
        f.opens++;
    }
    pthread_mutex_unlock(&mu);
    return ret;
}

static IOReturn intfClose(void *self) {
    enter(self);
    ((FakeIntf *)self)->open = 0;
    f.closes++;
    pthread_mutex_unlock(&mu);
    return kIOReturnSuccess;
}

static IOReturn intfNumber(void *self, UInt8 *num) {
    *num = ((FakeIntf *)self)->num;
    return kIOReturnSuccess;
}

static IOReturn intfNumEndpoints(void *self, UInt8 *n) {
    *n = 2;
    return kIOReturnSuccess;
}

static IOReturn intfPipeProps(void *self, UInt8 ref, UInt8 *dir, UInt8 *num, UInt8 *type, UInt16 *mps, UInt8 *interval) {
    FakeIntf *i = self;
    *dir = ref == 1 ? kUSBIn : kUSBOut;
    *num = i->num * 2 + ref + i->alt * 8;
    *type = kUSBBulk;
    *mps = 1024; // the scaled "full" size, which ZLP logic must not use
    *interval = 0;
    return kIOReturnSuccess;
}

static IOReturn intfAltSetting(void *self, UInt8 *alt) {
    *alt = ((FakeIntf *)self)->alt;
    return kIOReturnSuccess;
}

static IOReturn intfEndpointProps(void *self, UInt8 alt, UInt8 num, UInt8 dir, UInt8 *type, UInt16 *mps, UInt8 *interval) {
    *type = kUSBBulk;
    *mps = 64;
    *interval = 0;
    return kIOReturnSuccess;
}

static IOReturn intfSetAlt(void *self, UInt8 alt) {
    enter(self);
    pthread_sigmask(SIG_BLOCK, NULL, &ioMask);
    f.altCalls++;
    f.lastAlt = alt;
    IOReturn ret = f.altRet;
    if (!ret) ((FakeIntf *)self)->alt = alt;
    pthread_mutex_unlock(&mu);
    return ret;
}

static IOReturn pipeIO(void *self, UInt8 ref, void *buf, UInt32 size, int write) {
    enter(self);
    FakeIntf *i = self;
    f.writes += write;
    f.lastIntf = i->num;
    f.lastPipe = ref;
    f.lastBufNull = buf == NULL;
    f.lastSize = size;
    IOReturn ret = i->open ? f.ioRet : kIOReturnNotOpen;
    if (waitIfBlocked(0)) ret = kIOReturnAborted;
    pthread_mutex_unlock(&mu);
    return ret;
}

static IOReturn intfRead(void *self, UInt8 ref, void *buf, UInt32 *size) {
    return pipeIO(self, ref, buf, *size, 0);
}

static IOReturn intfWrite(void *self, UInt8 ref, void *buf, UInt32 size) {
    return pipeIO(self, ref, buf, size, 1);
}

static IOReturn intfAbort(void *self, UInt8 ref) {
    enter(self);
    f.aborts++;
    abortBlocked();
    int slow = f.slowAbortUs;
    pthread_mutex_unlock(&mu);
    if (slow) {
        usleep(slow);
        enter(self);
        pthread_mutex_unlock(&mu);
    }
    return kIOReturnSuccess;
}

static IOReturn intfClear(void *self, UInt8 ref) {
    enter(self);
    pthread_sigmask(SIG_BLOCK, NULL, &ioMask);
    f.lastIntf = ((FakeIntf *)self)->num;
    f.lastPipe = ref;
    f.clears++;
    pthread_mutex_unlock(&mu);
    return kIOReturnSuccess;
}

static IOUSBInterfaceInterface300 intfVt = {
    .AddRef = intfAddRef,
    .Release = intfRelease,
    .USBInterfaceOpen = intfOpen,
    .USBInterfaceClose = intfClose,
    .GetInterfaceNumber = intfNumber,
    .GetNumEndpoints = intfNumEndpoints,
    .GetPipeProperties = intfPipeProps,
    .GetAlternateSetting = intfAltSetting,
    .GetEndpointProperties = intfEndpointProps,
    .SetAlternateInterface = intfSetAlt,
    .ReadPipe = intfRead,
    .WritePipe = intfWrite,
    .AbortPipe = intfAbort,
    .ClearPipeStallBothEnds = intfClear,
};

static IOReturn devOpen(void *self) {
    pthread_mutex_lock(&mu);
    f.devOpens++;
    IOReturn ret = f.devOpenRet;
    pthread_mutex_unlock(&mu);
    return ret;
}

static IOReturn devSpeed(void *self, UInt8 *speed) {
    pthread_mutex_lock(&mu);
    *speed = f.speed;
    IOReturn ret = f.speedRet;
    pthread_mutex_unlock(&mu);
    return ret;
}

static IOReturn devClose(void *self) { return kIOReturnSuccess; }

static IOReturn devReEnum(void *self, UInt32 options) {
    pthread_mutex_lock(&mu);
    pthread_sigmask(SIG_BLOCK, NULL, &ioMask);
    f.reenums++;
    pthread_mutex_unlock(&mu);
    return kIOReturnSuccess;
}

static ULONG devRelease(void *self) { return 0; }

static IOReturn devSetConfig(void *self, UInt8 cfg) {
    pthread_mutex_lock(&mu);
    pthread_sigmask(SIG_BLOCK, NULL, &ioMask);
    f.setConfigs++;
    f.curConfig = cfg;
    newNubs(cfg ? f.total : 0);
    pthread_mutex_unlock(&mu);
    return kIOReturnSuccess;
}

static IOReturn devGetConfig(void *self, UInt8 *cfg) {
    pthread_mutex_lock(&mu);
    *cfg = f.curConfig;
    IOReturn ret = f.getConfigRet;
    pthread_mutex_unlock(&mu);
    return ret;
}

static IOReturn devRequest(void *self, IOUSBDevRequestTO *req) {
    pthread_mutex_lock(&mu);
    f.requests++;
    f.noDataTimeout = req->noDataTimeout;
    f.completionTimeout = req->completionTimeout;
    IOReturn ret = f.ctrlRet;
    // The plugin sends completionTimeout, else noDataTimeout.
    int w = waitIfBlocked(req->completionTimeout ? req->completionTimeout : req->noDataTimeout);
    if (w == 1) ret = kIOReturnAborted;
    if (w == 2) ret = kIOUSBTransactionTimeout;
    if (!ret) {
        req->wLenDone = f.ctrlDone;
        if (req->pData) memset(req->pData, 0xa5, f.ctrlDone < req->wLength ? f.ctrlDone : req->wLength);
    }
    pthread_mutex_unlock(&mu);
    return ret;
}

// As in the plugin, an unopened device never reaches the kernel.
static IOReturn devAbortZero(void *self) {
    pthread_mutex_lock(&mu);
    f.abortZeros++;
    IOReturn ret = f.devOpenRet ? kIOReturnNotOpen : kIOReturnSuccess;
    if (!ret) abortBlocked();
    pthread_mutex_unlock(&mu);
    return ret;
}

static IOUSBDeviceInterface320 devVt = {
    .Release = devRelease,
    .USBDeviceOpen = devOpen,
    .USBDeviceClose = devClose,
    .USBDeviceReEnumerate = devReEnum,
    .GetDeviceSpeed = devSpeed,
    .SetConfiguration = devSetConfig,
    .GetConfiguration = devGetConfig,
    .DeviceRequestTO = devRequest,
    .USBDeviceAbortPipeZero = devAbortZero,
};
static IOUSBDeviceInterface320 *devPtr = &devVt;

static IOUSBDeviceInterface320 **fakeDev(void) { return &devPtr; }

static void fakeReset(int n) {
    pthread_mutex_lock(&mu);
    memset(&f, 0, sizeof(f));
    newNubs(n);
    f.total = f.visible = n;
    pthread_mutex_unlock(&mu);
}

static int fakeVisible(void) {
    pthread_mutex_lock(&mu);
    int n = f.visible < f.numNubs ? f.visible : f.numNubs;
    pthread_mutex_unlock(&mu);
    return n;
}

// A nub yields a new plugin reference, like IOCreatePlugInInterfaceForService.
static IOUSBInterfaceInterface300 **fakeNub(int i) {
    pthread_mutex_lock(&mu);
    FakeIntf *n = nubs[i];
    n->refs++;
    pthread_mutex_unlock(&mu);
    return (IOUSBInterfaceInterface300 **)n;
}

// A real send right, so IOObjectRelease and IOObjectIsEqualTo work as on a service.
static mach_port_t fakeService(void) {
    mach_port_t p = 0;
    mach_port_allocate(mach_task_self(), MACH_PORT_RIGHT_RECEIVE, &p);
    mach_port_insert_right(mach_task_self(), p, (mach_port_t)p, MACH_MSG_TYPE_MAKE_SEND);
    return p;
}

static void fakeRetainService(mach_port_t p) {
    mach_port_mod_refs(mach_task_self(), p, MACH_PORT_RIGHT_SEND, 1);
}

static int fakeServiceRefs(mach_port_t p) {
    mach_port_urefs_t n = 0;
    mach_port_get_refs(mach_task_self(), p, MACH_PORT_RIGHT_SEND, &n);
    return n;
}

static void fakeLock(void) { pthread_mutex_lock(&mu); }

static void fakeUnlock(void) { pthread_mutex_unlock(&mu); }

static void fakeUnblock(void) {
    pthread_mutex_lock(&mu);
    f.unblock = 1;
    pthread_cond_broadcast(&cond);
    pthread_mutex_unlock(&mu);
}

static int fakeIOMaskHas(int sig) {
    pthread_mutex_lock(&mu);
    int r = sigismember(&ioMask, sig);
    pthread_mutex_unlock(&mu);
    return r;
}

static int fakeThreadMaskHas(int sig) {
    sigset_t cur;
    pthread_sigmask(SIG_BLOCK, NULL, &cur);
    return sigismember(&cur, sig);
}
*/
import "C"

import (
	"errors"
	"runtime"
	"syscall"
	"time"
)

func newFakeDevice(n int) *iokitDevice {
	C.fakeReset(C.int(n))
	return &iokitDevice{
		ptr:    C.fakeDev(),
		opened: true,
		nubs: func() []**C.IOUSBInterfaceInterface300 {
			var intfs []**C.IOUSBInterfaceInterface300
			for i := range C.fakeVisible() {
				intfs = append(intfs, C.fakeNub(i))
			}
			return intfs
		},
		services:    func() ([]C.io_service_t, error) { return nil, errors.New("no fake registry") },
		mounted:     func() bool { return false },
		reconfigure: func(uint8) bool { return false },
		ifaces:      map[uint8]**C.IOUSBInterfaceInterface300{},
		pipes:       map[uint8]pipe{},
		refused:     map[uint8]bool{},
	}
}

func fakeService() C.io_service_t { return C.fakeService() }

func fakeServiceRefs(svc C.io_service_t) int { return int(C.fakeServiceRefs(svc)) }

// fakeRegistry makes enumeration return err, or svcs retained as IOIteratorNext does.
func (g *iokitDevice) fakeRegistry(err error, svcs ...C.io_service_t) {
	g.services = func() ([]C.io_service_t, error) {
		for _, svc := range svcs {
			C.fakeRetainService(svc)
		}
		return svcs, err
	}
}

// A pthread mutex must be unlocked by the thread that locked it.
func fakeLocked(fn func(f *C.FakeState)) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	C.fakeLock()
	defer C.fakeUnlock()
	fn(&C.f)
}

func fakeSetDevice(speed int, speedRet, openRet uint32) {
	fakeLocked(func(f *C.FakeState) {
		f.speed, f.speedRet, f.devOpenRet = C.int(speed), C.IOReturn(int32(speedRet)), C.IOReturn(int32(openRet))
	})
}

func fakeStats() (s C.FakeState) {
	fakeLocked(func(f *C.FakeState) { s = *f })
	return s
}

func fakeSetCurConfig(cfg int, ret uint32) {
	fakeLocked(func(f *C.FakeState) { f.curConfig, f.getConfigRet = C.int(cfg), C.IOReturn(int32(ret)) })
}

func fakeSetRets(io, open, alt, ctrl uint32) {
	fakeLocked(func(f *C.FakeState) {
		f.ioRet, f.openRet, f.altRet, f.ctrlRet = C.IOReturn(int32(io)), C.IOReturn(int32(open)), C.IOReturn(int32(alt)), C.IOReturn(int32(ctrl))
	})
}

func fakeSetCtrlDone(n uint32) { fakeLocked(func(f *C.FakeState) { f.ctrlDone = C.UInt32(n) }) }

func fakeSetVisible(n int) { fakeLocked(func(f *C.FakeState) { f.visible = C.int(n) }) }

// fakeArmBlock makes the next pipe or control call block, ignoring the first missAborts aborts.
func fakeArmBlock(missAborts int) {
	fakeLocked(func(f *C.FakeState) { f.block, f.missAborts = 1, C.int(missAborts) })
}

func fakeSetMissAborts(n int) { fakeLocked(func(f *C.FakeState) { f.missAborts = C.int(n) }) }

// Long enough to outlast the call it aborts.
func fakeSetSlowAbort(d time.Duration) {
	fakeLocked(func(f *C.FakeState) { f.slowAbortUs = C.int(d.Microseconds()) })
}

func fakeUnblock() { C.fakeUnblock() }

// fakeIOBlocked reports whether the last blocking call ran with sig blocked.
func fakeIOBlocked(sig syscall.Signal) bool { return C.fakeIOMaskHas(C.int(sig)) == 1 }

func fakeThreadBlocks(sig syscall.Signal) bool { return C.fakeThreadMaskHas(C.int(sig)) == 1 }
