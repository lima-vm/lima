//
//  virtualization_15.h
//
//  Created by codehex.
//

#pragma once

// FIXME(codehex): this is dirty hack to avoid clang-format error like below
// "Configuration file(s) do(es) not support C++: /github.com/Code-Hex/vz/.clang-format"
#define NSURLComponents NSURLComponents

#import "virtualization_helper.h"
#import <Virtualization/Virtualization.h>

/* exported from cgo */
void usbAttachDetachCompletionHandler(uintptr_t cgoHandle, void *errPtr);

/* macOS 15 API */
bool isNestedVirtualizationSupported();
void setNestedVirtualizationEnabled(void *config, bool nestedVirtualizationEnabled);
void *newVZXHCIControllerConfiguration();
void setUSBControllersVZVirtualMachineConfiguration(void *config, void *usbControllers);
const char *getUUIDUSBDevice(void *usbDevice);
void *usbDevicesVZUSBController(void *usbController);
void *VZVirtualMachine_usbControllers(void *machine);
void attachDeviceVZUSBController(void *usbController, void *usbDevice, void *queue, uintptr_t cgoHandle);
void detachDeviceVZUSBController(void *usbController, void *usbDevice, void *queue, uintptr_t cgoHandle);
void *newVZUSBMassStorageDeviceWithConfiguration(void *config);