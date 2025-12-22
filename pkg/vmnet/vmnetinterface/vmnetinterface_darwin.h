#pragma once

#import <Availability.h>
#import <Foundation/Foundation.h>
#import <vmnet/vmnet.h>

// MARK: - Macros

// To avoid including virtualization_helper.h, copied from there.
NSDictionary *vmnetInterfaceDumpProcessinfo();

#define RAISE_REASON_MESSAGE                                                                               \
    "This may possibly be a bug due to library handling errors.\n"                                         \
    "I would appreciate it if you could report it to https://github.com/Code-Hex/vz/issues/new/choose\n\n" \
    "Information: %@\n"

#define RAISE_UNSUPPORTED_MACOS_EXCEPTION()                                 \
    do {                                                                    \
        [NSException                                                        \
             raise:@"UnhandledAvailabilityException"                        \
            format:@RAISE_REASON_MESSAGE, vmnetInterfaceDumpProcessinfo()]; \
        __builtin_unreachable();                                            \
    } while (0)

// for macOS 26 API
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 260000
#define INCLUDE_TARGET_OSX_26 1
#else
#pragma message("macOS 26 API has been disabled")
#endif

// MARK: - helper functions
struct vmpktdesc *allocateVMPktDescArray(int count, uint64_t maxPacketSize);
struct vmpktdesc *resetVMPktDescArray(struct vmpktdesc *pktDescs, int count, uint64_t maxPacketSize);
void deallocateVMPktDescArray(struct vmpktdesc *pktDescs);

// MARK: - interface_ref

uint32_t VmnetInterfaceSetPacketsAvailableEventCallback(void *interface, uintptr_t callback);
uint32_t VmnetStopInterface(void *interface);
uint32_t VmnetRead(void *interface, struct vmpktdesc *packets, int *pktcnt);
uint32_t VmnetWrite(void *interface, struct vmpktdesc *packets, int *pktcnt);

struct vmnetInterfaceStartResult {
    void *iface;
    void *ifaceParam;
    uint64_t maxPacketSize;
    int maxReadPacketCount;
    int maxWritePacketCount;
    uint32_t vmnetReturn;
};

struct vmnetInterfaceStartResult VmnetInterfaceStartWithNetwork(void *network, void *interfaceDesc);

void VmnetReleaseInterface(void *interface);
