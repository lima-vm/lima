//
//  virtualization_15.m
//
//  Created by codehex.
//
#import "virtualization_15.h"

/*!
 @abstract Check if nested virtualization is supported.
 @return true if supported.
 */
bool isNestedVirtualizationSupported()
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        return (bool)VZGenericPlatformConfiguration.isNestedVirtualizationSupported;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Set nestedVirtualizationEnabled. The default is false.
 */
void setNestedVirtualizationEnabled(void *config, bool nestedVirtualizationEnabled)
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        VZGenericPlatformConfiguration *platformConfig = (VZGenericPlatformConfiguration *)config;
        platformConfig.nestedVirtualizationEnabled = (BOOL)nestedVirtualizationEnabled;
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Configuration for the USB XHCI controller.
 @discussion This configuration creates a USB XHCI controller device for the guest.
 */
void *newVZXHCIControllerConfiguration()
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        return [[VZXHCIControllerConfiguration alloc] init];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

void setUSBControllersVZVirtualMachineConfiguration(void *config, void *usbControllers)
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        [(VZVirtualMachineConfiguration *)config
            setUsbControllers:[(NSMutableArray *)usbControllers copy]];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Device UUID.
 @discussion
    Device UUID from device configuration objects that conform to `VZUSBDeviceConfiguration`.
 @see VZUSBDeviceConfiguration
 */
const char *getUUIDUSBDevice(void *usbDevice)
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        NSString *uuid = [[(id<VZUSBDevice>)usbDevice uuid] UUIDString];
        return [uuid UTF8String];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Return a list of USB devices attached to controller.
 @discussion
    If corresponding USB controller configuration included in VZVirtualMachineConfiguration contained any USB devices,
    those devices will appear here when virtual machine is started.
 @see VZUSBDevice
 @see VZUSBDeviceConfiguration
 @see VZUSBControllerConfiguration
 @see VZVirtualMachineConfiguration
 */
void *usbDevicesVZUSBController(void *usbController)
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        return [(VZUSBController *)usbController usbDevices];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Return the list of USB controllers configured on this virtual machine. Return an empty array if no USB controller is configured.
 @see VZUSBControllerConfiguration
 @see VZVirtualMachineConfiguration
 */
void *VZVirtualMachine_usbControllers(void *machine)
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        return [(VZVirtualMachine *)machine usbControllers]; // NSArray<VZUSBController *>
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Attach a USB device.
 @discussion
    If the device is successfully attached to the controller, it will appear in the usbDevices property,
    its usbController property will be set to point to the USB controller that it is attached to
    and completion handler will return nil.
    If the device was previously attached to this or another USB controller, attach function will fail
    with the `VZErrorDeviceAlreadyAttached`. If the device cannot be initialized correctly, attach
    function will fail with `VZErrorDeviceInitializationFailure`.
    This method must be called on the virtual machine's queue.
 @param device USB device to attach.
 @param completionHandler Block called after the device has been attached or on error.
    The error parameter passed to the block is nil if the attach was successful.
    It will be also invoked on an virtual machine's queue.
 @see VZUSBDevice
 */
void attachDeviceVZUSBController(void *usbController, void *usbDevice, void *queue, uintptr_t cgoHandle)
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        dispatch_sync((dispatch_queue_t)queue, ^{
            [(VZUSBController *)usbController attachDevice:(id<VZUSBDevice>)usbDevice
                                         completionHandler:^(NSError *error) {
                                             usbAttachDetachCompletionHandler(cgoHandle, error);
                                         }];
        });
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Detach a USB device.
 @discussion
    If the device is successfully detached from the controller, it will disappear from the usbDevices property,
    its usbController property will be set to nil and completion handler will return nil.
    If the device wasn't attached to the controller at the time of calling detach method, it will fail
    with the `VZErrorDeviceNotFound` error.
    This method must be called on the virtual machine's queue.
 @param device USB device to detach.
 @param completionHandler Block called after the device has been detached or on error.
    The error parameter passed to the block is nil if the detach was successful.
    It will be also invoked on an virtual machine's queue.
 @see VZUSBDevice
 */
void detachDeviceVZUSBController(void *usbController, void *usbDevice, void *queue, uintptr_t cgoHandle)
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        dispatch_sync((dispatch_queue_t)queue, ^{
            [(VZUSBController *)usbController detachDevice:(id<VZUSBDevice>)usbDevice
                                         completionHandler:^(NSError *error) {
                                             usbAttachDetachCompletionHandler(cgoHandle, error);
                                         }];
        });
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Initialize the runtime USB Mass Storage device object.
 @param configuration The configuration of the USB Mass Storage device.
 @see VZUSBMassStorageDeviceConfiguration
 */
void *newVZUSBMassStorageDeviceWithConfiguration(void *config)
{
#ifdef INCLUDE_TARGET_OSX_15
    if (@available(macOS 15, *)) {
        return [[VZUSBMassStorageDevice alloc] initWithConfiguration:(VZUSBMassStorageDeviceConfiguration *)config];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}