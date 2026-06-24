//
//  virtualization_11.h
//
//  Created by codehex.
//

#pragma once

#import "virtualization_helper.h"
#import <Virtualization/Virtualization.h>

/* exported from cgo */
void connectionHandler(void *connection, void *err, uintptr_t cgoHandle);
void changeStateOnObserver(int state, uintptr_t cgoHandle);
bool shouldAcceptNewConnectionHandler(uintptr_t cgoHandle, void *connection, void *socketDevice);
void emitAttachmentWasDisconnected(int index, void *err, uintptr_t cgoHandle);
void closeAttachmentWasDisconnectedChannel(uintptr_t cgoHandle);

@interface Observer : NSObject
- (void)observeValueForKeyPath:(NSString *)keyPath ofObject:(id)object change:(NSDictionary *)change context:(void *)context;
@end

@interface VZVirtualMachineDelegateWrapper : NSObject <VZVirtualMachineDelegate>
@property (nonatomic, strong, readonly) NSHashTable<id<VZVirtualMachineDelegate>> *delegates;

- (instancetype)init;
- (void)addDelegate:(id<VZVirtualMachineDelegate>)delegate;
- (void)guestDidStopVirtualMachine:(VZVirtualMachine *)virtualMachine;
- (void)virtualMachine:(VZVirtualMachine *)virtualMachine didStopWithError:(NSError *)error;
- (void)virtualMachine:(VZVirtualMachine *)virtualMachine
                         networkDevice:(VZNetworkDevice *)networkDevice
    attachmentWasDisconnectedWithError:(NSError *)error API_AVAILABLE(macos(12.0));
@end

@interface ObservableVZVirtualMachine : VZVirtualMachine
- (instancetype)initWithConfiguration:(VZVirtualMachineConfiguration *)configuration
                                queue:(dispatch_queue_t)queue
                   statusUpdateHandle:(uintptr_t)statusUpdateHandle;
- (void)dealloc;
@end

@interface NetworkDeviceDisconnectedHandler : NSObject <VZVirtualMachineDelegate>
- (instancetype)initWithHandle:(uintptr_t)cgoHandle;
- (void)virtualMachine:(VZVirtualMachine *)virtualMachine
                         networkDevice:(VZNetworkDevice *)networkDevice
    attachmentWasDisconnectedWithError:(NSError *)error API_AVAILABLE(macos(12.0));
- (int)networkDevices:(NSArray<VZNetworkDevice *> *)networkDevices
              indexOf:(VZNetworkDevice *)networkDevice API_AVAILABLE(macos(12.0));
- (void)dealloc;
@end

/* VZVirtioSocketListener */
@interface VZVirtioSocketListenerDelegateImpl : NSObject <VZVirtioSocketListenerDelegate>
- (instancetype)initWithHandle:(uintptr_t)cgoHandle;
- (BOOL)listener:(VZVirtioSocketListener *)listener shouldAcceptNewConnection:(VZVirtioSocketConnection *)connection fromSocketDevice:(VZVirtioSocketDevice *)socketDevice;
@end

/* BootLoader */
void *newVZLinuxBootLoader(const char *kernelPath);
void setCommandLineVZLinuxBootLoader(void *bootLoaderPtr, const char *commandLine);
void setInitialRamdiskURLVZLinuxBootLoader(void *bootLoaderPtr, const char *ramdiskPath);

/* VirtualMachineConfiguration */
bool validateVZVirtualMachineConfiguration(void *config, void **error);
unsigned long long minimumAllowedMemorySizeVZVirtualMachineConfiguration();
unsigned long long maximumAllowedMemorySizeVZVirtualMachineConfiguration();
unsigned int minimumAllowedCPUCountVZVirtualMachineConfiguration();
unsigned int maximumAllowedCPUCountVZVirtualMachineConfiguration();
void *newVZVirtualMachineConfiguration(void *bootLoader,
    unsigned int CPUCount,
    unsigned long long memorySize);
void setEntropyDevicesVZVirtualMachineConfiguration(void *config,
    void *entropyDevices);
void setMemoryBalloonDevicesVZVirtualMachineConfiguration(void *config,
    void *memoryBalloonDevices);
void setNetworkDevicesVZVirtualMachineConfiguration(void *config,
    void *networkDevices);
void *networkDevicesVZVirtualMachineConfiguration(void *config);
void setSerialPortsVZVirtualMachineConfiguration(void *config,
    void *serialPorts);
void setSocketDevicesVZVirtualMachineConfiguration(void *config,
    void *socketDevices);
void *socketDevicesVZVirtualMachineConfiguration(void *config);
void setStorageDevicesVZVirtualMachineConfiguration(void *config,
    void *storageDevices);
void *storageDevicesVZVirtualMachineConfiguration(void *config);

/* Configurations */
void *newVZFileHandleSerialPortAttachment(int readFileDescriptor, int writeFileDescriptor);
void *newVZFileSerialPortAttachment(const char *filePath, bool shouldAppend, void **error);
void *newVZVirtioConsoleDeviceSerialPortConfiguration(void *attachment);
void *VZBridgedNetworkInterface_networkInterfaces(void);
const char *VZBridgedNetworkInterface_identifier(void *networkInterface);
const char *VZBridgedNetworkInterface_localizedDisplayName(void *networkInterface);
void *newVZBridgedNetworkDeviceAttachment(void *networkInterface);
void *newVZNATNetworkDeviceAttachment(void);
void *newVZFileHandleNetworkDeviceAttachment(int fileDescriptor);
void *newVZVirtioNetworkDeviceConfiguration(void *attachment);
void setNetworkDevicesVZMACAddress(void *config, void *macAddress);
void *newVZVirtioEntropyDeviceConfiguration(void);
void *newVZVirtioBlockDeviceConfiguration(void *attachment);
void *newVZDiskImageStorageDeviceAttachment(const char *diskPath, bool readOnly, void **error);
void *newVZVirtioTraditionalMemoryBalloonDeviceConfiguration();
void *newVZVirtioSocketDeviceConfiguration();
void *newVZMACAddress(const char *macAddress);
void *newRandomLocallyAdministeredVZMACAddress();
const char *getVZMACAddressString(void *macAddress);
void *newVZVirtioSocketListener(uintptr_t cgoHandle);
void *VZVirtualMachine_socketDevices(void *machine);
void VZVirtioSocketDevice_setSocketListenerForPort(void *socketDevice, void *vmQueue, void *listener, uint32_t port);
void VZVirtioSocketDevice_removeSocketListenerForPort(void *socketDevice, void *vmQueue, uint32_t port);
void VZVirtioSocketDevice_connectToPort(void *socketDevice, void *vmQueue, uint32_t port, uintptr_t cgoHandle);
void *VZVirtualMachine_memoryBalloonDevices(void *machine);

/* VirtualMachine */
void *newVZVirtualMachineWithDispatchQueue(void *config, void *queue, uintptr_t statusUpdateCgoHandle, uintptr_t disconnectedCgoHandle);
bool requestStopVirtualMachine(void *machine, void *queue, void **error);
void startWithCompletionHandler(void *machine, void *queue, uintptr_t cgoHandle);
void pauseWithCompletionHandler(void *machine, void *queue, uintptr_t cgoHandle);
void resumeWithCompletionHandler(void *machine, void *queue, uintptr_t cgoHandle);
bool vmCanStart(void *machine, void *queue);
bool vmCanPause(void *machine, void *queue);
bool vmCanResume(void *machine, void *queue);
bool vmCanRequestStop(void *machine, void *queue);

void *makeDispatchQueue(const char *label);

/* VZVirtioSocketConnection */
typedef struct VZVirtioSocketConnectionFlat {
    uint32_t destinationPort;
    uint32_t sourcePort;
    int fileDescriptor;
} VZVirtioSocketConnectionFlat;

VZVirtioSocketConnectionFlat convertVZVirtioSocketConnection2Flat(void *connection);

/* VZVirtioTraditionalMemoryBalloonDevice */
void VZVirtioTraditionalMemoryBalloonDevice_setTargetVirtualMachineMemorySize(void *balloonDevice, void *queue, unsigned long long targetMemorySize);
unsigned long long VZVirtioTraditionalMemoryBalloonDevice_getTargetVirtualMachineMemorySize(void *balloonDevice, void *queue);
