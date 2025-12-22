#import "vmnetinterface_darwin.h"

// MARK: - Macros

// To avoid including virtualization_helper.h, copied from there.
NSDictionary *vmnetInterfaceDumpProcessinfo()
{
    NSString *osVersionString = [[NSProcessInfo processInfo] operatingSystemVersionString];
    return @{
        @"LLVM (Clang) Version" : @__VERSION__,
#ifdef __arm64__
        @"Target for arm64" : @1,
#else
        @"Target for arm64" : @0,
#endif
        // The version of the macOS on which the process is executing.
        @"Running OS Version" : osVersionString,
#ifdef __MAC_OS_X_VERSION_MAX_ALLOWED
        @"Max Allowed OS Version" : @__MAC_OS_X_VERSION_MAX_ALLOWED,
#endif
#ifdef __MAC_OS_X_VERSION_MIN_REQUIRED
        @"Min Required OS Version" : @__MAC_OS_X_VERSION_MIN_REQUIRED,
#endif
    };
}

// MARK: - Helper functions defined in Go

struct vmpktdesc *allocateVMPktDescArray(int count, uint64_t maxPacketSize)
{
    // Calculate total size needed for pktdesc array and iovec array
    size_t totalSize = (sizeof(struct vmpktdesc) + sizeof(struct iovec)) * count;
    struct vmpktdesc *pktDescs = (struct vmpktdesc *)malloc(totalSize);
    return resetVMPktDescArray(pktDescs, count, maxPacketSize);
}

struct vmpktdesc *resetVMPktDescArray(struct vmpktdesc *pktDescs, int count, uint64_t maxPacketSize)
{
    struct iovec *iovecArray = (struct iovec *)(pktDescs + count);
    for (int i = 0; i < count; i++) {
        pktDescs[i].vm_pkt_size = maxPacketSize;
        pktDescs[i].vm_pkt_iov = &iovecArray[i];
        pktDescs[i].vm_pkt_iovcnt = 1;
        pktDescs[i].vm_flags = 0;
        iovecArray[i].iov_len = maxPacketSize;
    }
    return pktDescs;
}

void deallocateVMPktDescArray(struct vmpktdesc *pktDescs)
{
    if (pktDescs != NULL) {
        free(pktDescs);
    }
}

// MARK: - interface_ref

extern void callPacketsAvailableEventCallback(uintptr_t cgoHandle, int estimatedCount);

uint32_t VmnetInterfaceSetPacketsAvailableEventCallback(void *iface, uintptr_t callback)
{
#ifdef INCLUDE_TARGET_OSX_26
    if (@available(macOS 26, *)) {
        dispatch_queue_t queue = dispatch_queue_create("vmnet.interface.eventcallback", DISPATCH_QUEUE_SERIAL);
        vmnet_return_t result = vmnet_interface_set_event_callback((interface_ref)iface, VMNET_INTERFACE_PACKETS_AVAILABLE, queue, ^(interface_event_t eventMask, xpc_object_t event) {
            if ((eventMask & VMNET_INTERFACE_PACKETS_AVAILABLE) != 0) {
                int estimated = (int)xpc_dictionary_get_uint64(event, vmnet_estimated_packets_available_key);
                callPacketsAvailableEventCallback(callback, estimated);
            }
        });
        dispatch_release(queue);
        return result;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

uint32_t VmnetStopInterface(void *interface)
{
#ifdef INCLUDE_TARGET_OSX_26
    if (@available(macOS 26, *)) {
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        dispatch_queue_t queue = dispatch_queue_create("vmnet.interface.stop", DISPATCH_QUEUE_SERIAL);
        __block vmnet_return_t status;
        vmnet_return_t scheduleStatus = vmnet_stop_interface((interface_ref)interface, queue, ^(vmnet_return_t stopStatus) {
            status = stopStatus;
            dispatch_semaphore_signal(sem);
        });
        dispatch_release(queue);
        if (scheduleStatus != VMNET_SUCCESS) {
            dispatch_release(sem);
            return scheduleStatus;
        }
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
        dispatch_release(sem);
        return status;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

uint32_t VmnetRead(void *interface, struct vmpktdesc *packets, int *pktcnt)
{
#ifdef INCLUDE_TARGET_OSX_26
    if (@available(macOS 26, *)) {
        return vmnet_read((interface_ref)interface, packets, pktcnt);
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

uint32_t VmnetWrite(void *interface, struct vmpktdesc *packets, int *pktcnt)
{
#ifdef INCLUDE_TARGET_OSX_26
    if (@available(macOS 26, *)) {
        return vmnet_write((interface_ref)interface, packets, pktcnt);
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

extern void callStartInterfaceCompletionHandler(uintptr_t handlerPtr, uint32_t vmnetReturn, void *interfaceParam);

struct vmnetInterfaceStartResult VmnetInterfaceStartWithNetwork(void *network, void *interfaceDesc)
{
#ifdef INCLUDE_TARGET_OSX_26
    if (@available(macOS 26, *)) {
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        dispatch_queue_t queue = dispatch_queue_create("vmnet.interface.start", DISPATCH_QUEUE_SERIAL);
        __block struct vmnetInterfaceStartResult result;
        vmnet_start_interface_completion_handler_t handler = ^(vmnet_return_t vmnetReturn, xpc_object_t ifaceParam) {
            result.ifaceParam = xpc_retain(ifaceParam);
            result.maxPacketSize = xpc_dictionary_get_uint64(ifaceParam, vmnet_max_packet_size_key);
            result.maxReadPacketCount = xpc_dictionary_get_uint64(ifaceParam, vmnet_read_max_packets_key);
            result.maxWritePacketCount = xpc_dictionary_get_uint64(ifaceParam, vmnet_write_max_packets_key);
            result.vmnetReturn = vmnetReturn;
            dispatch_semaphore_signal(sem);
        };
        interface_ref iface = vmnet_interface_start_with_network((vmnet_network_ref)network, (xpc_object_t)interfaceDesc, queue, handler);
        result.iface = iface;
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
        dispatch_release(queue);
        dispatch_release(sem);
        return result;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

void VmnetReleaseInterface(void *interface)
{
    CFRelease(interface);
}
