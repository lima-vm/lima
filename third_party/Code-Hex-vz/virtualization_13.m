//
//  virtualization_13.m
//
//  Created by codehex.
//

#import "virtualization_13.h"
#import "virtualization_view.h"

/*!
 @abstract List of console devices. Empty by default.
 @see VZVirtioConsoleDeviceConfiguration
 */
void setConsoleDevicesVZVirtualMachineConfiguration(void *config, void *consoleDevices)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        [(VZVirtualMachineConfiguration *)config
            setConsoleDevices:[(NSMutableArray *)consoleDevices copy]];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Boot loader configuration for booting guest operating systems expecting an EFI ROM.
 @discussion
    You must use a VZGenericPlatformConfiguration in conjunction with the EFI boot loader.
    It is invalid to use it with any other platform configuration.
 @see VZGenericPlatformConfiguration
 @see VZVirtualMachineConfiguration.platform.
*/
void *newVZEFIBootLoader()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZEFIBootLoader alloc] init];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Set the EFI variable store.
 */
void setVariableStoreVZEFIBootLoader(void *bootLoaderPtr, void *variableStore)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        [(VZEFIBootLoader *)bootLoaderPtr setVariableStore:(VZEFIVariableStore *)variableStore];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Initialize the variable store from the path of an existing file.
 @param variableStorePath The path of the variable store on the local file system.
 @discussion To create a new variable store, use -[VZEFIVariableStore initCreatingVariableStoreAtURL:options:error].
 */
void *newVZEFIVariableStorePath(const char *variableStorePath)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        NSString *variableStorePathNSString = [NSString stringWithUTF8String:variableStorePath];
        NSURL *variableStoreURL = [NSURL fileURLWithPath:variableStorePathNSString];
        return [[VZEFIVariableStore alloc] initWithURL:variableStoreURL];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Write an initialized VZEFIVariableStore to path on a file system.
 @param variableStorePath The path to write the variable store to on the local file system.
 @param error If not nil, used to report errors if creation fails.
 @return A newly initialized VZEFIVariableStore on success. If an error was encountered returns @c nil, and @c error contains the error.
 */
void *newCreatingVZEFIVariableStoreAtPath(const char *variableStorePath, void **error)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        NSString *variableStorePathNSString = [NSString stringWithUTF8String:variableStorePath];
        NSURL *variableStoreURL = [NSURL fileURLWithPath:variableStorePathNSString];
        return [[VZEFIVariableStore alloc]
            initCreatingVariableStoreAtURL:variableStoreURL
                                   options:VZEFIVariableStoreInitializationOptionAllowOverwrite
                                     error:(NSError *_Nullable *_Nullable)error];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Get the machine identifier described by the specified data representation.
 @param dataRepresentation The opaque data representation of the machine identifier to be obtained.
 @return A unique identifier identical to the one that generated the dataRepresentation, or nil if the data is invalid.
 @see VZGenericMachineIdentifier.dataRepresentation
 */
void *newVZGenericMachineIdentifierWithBytes(void *machineIdentifierBytes, int len)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        VZGenericMachineIdentifier *machineIdentifier;
        @autoreleasepool {
            NSData *machineIdentifierData = [[NSData alloc] initWithBytes:machineIdentifierBytes length:(NSUInteger)len];
            machineIdentifier = [[VZGenericMachineIdentifier alloc] initWithDataRepresentation:machineIdentifierData];
        }
        return machineIdentifier;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Opaque data representation of the machine identifier.
 @discussion This can be used to recreate the same machine identifier with -[VZGenericMachineIdentifier initWithDataRepresentation:].
 @see -[VZGenericMachineIdentifier initWithDataRepresentation:]
 */
nbyteslice getVZGenericMachineIdentifierDataRepresentation(void *machineIdentifierPtr)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        VZGenericMachineIdentifier *machineIdentifier = (VZGenericMachineIdentifier *)machineIdentifierPtr;
        NSData *data = [machineIdentifier dataRepresentation];
        nbyteslice ret = {
            .ptr = (void *)[data bytes],
            .len = (int)[data length],
        };
        return ret;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Create a new unique machine identifier.
 */
void *newVZGenericMachineIdentifier()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZGenericMachineIdentifier alloc] init];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Set the machine identifier.
 */
void setMachineIdentifierVZGenericPlatformConfiguration(void *config, void *machineIdentifier)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        [(VZGenericPlatformConfiguration *)config setMachineIdentifier:(VZGenericMachineIdentifier *)machineIdentifier];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Initialize a VZUSBMassStorageDeviceConfiguration with a device attachment.
 @param attachment The storage device attachment. This defines how the virtualized device operates on the host side.
 @see VZDiskImageStorageDeviceAttachment
 */
void *newVZUSBMassStorageDeviceConfiguration(void *attachment)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZUSBMassStorageDeviceConfiguration alloc]
            initWithAttachment:(VZStorageDeviceAttachment *)attachment];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Creates a new Configuration for a Virtio graphics device.
 @discussion
    This device configuration creates a graphics device using paravirtualization.
    The emulated device follows the Virtio GPU Device specification.

    This device can be used to attach a display to be shown in a VZVirtualMachineView.
*/
void *newVZVirtioGraphicsDeviceConfiguration()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZVirtioGraphicsDeviceConfiguration alloc] init];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Set the scanouts to be attached to this graphics device.
 @discussion
    Maximum of one scanout is supported.
*/
void setScanoutsVZVirtioGraphicsDeviceConfiguration(void *graphicsConfiguration, void *scanouts)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        [(VZVirtioGraphicsDeviceConfiguration *)graphicsConfiguration
            setScanouts:[(NSMutableArray *)scanouts copy]];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Create a scanout configuration with the specified pixel dimensions.
 @param widthInPixels The width of the scanout, in pixels.
 @param heightInPixels The height of the scanout, in pixels.
*/
void *newVZVirtioGraphicsScanoutConfiguration(NSInteger widthInPixels, NSInteger heightInPixels)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZVirtioGraphicsScanoutConfiguration alloc]
            initWithWidthInPixels:widthInPixels
                   heightInPixels:heightInPixels];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Create a new Virtio Console Device
 @discussion
    This console device enables communication between the host and the guest using console ports through the Virtio interface.

    The device sets up one or more ports via VZVirtioConsolePortConfiguration on the Virtio console device.
 @see VZVirtioConsolePortConfiguration
 @see VZVirtualMachineConfiguration.consoleDevices
 */
void *newVZVirtioConsoleDeviceConfiguration()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZVirtioConsoleDeviceConfiguration alloc] init];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract The console ports to be configured for this console device.
 */
void *portsVZVirtioConsoleDeviceConfiguration(void *consoleDevice)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [(VZVirtioConsoleDeviceConfiguration *)consoleDevice ports]; // VZVirtioConsolePortConfigurationArray
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract The maximum number of ports allocated by this device. The default is the number of ports attached to this device.
 */
uint32_t maximumPortCountVZVirtioConsolePortConfigurationArray(void *ports)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [(VZVirtioConsolePortConfigurationArray *)ports maximumPortCount];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Get a port configuration at the specified index.
 */
void *getObjectAtIndexedSubscriptVZVirtioConsolePortConfigurationArray(void *portsPtr, int portIndex)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        VZVirtioConsolePortConfigurationArray *ports = (VZVirtioConsolePortConfigurationArray *)portsPtr;
        return ports[portIndex];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Set a port configuration at the specified index.
 */
void setObjectAtIndexedSubscriptVZVirtioConsolePortConfigurationArray(void *portsPtr, void *portConfig, int portIndex)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        VZVirtioConsolePortConfigurationArray *ports = (VZVirtioConsolePortConfigurationArray *)portsPtr;
        ports[portIndex] = (VZVirtioConsolePortConfiguration *)portConfig;
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Virtio Console Port
 @discussion
    A console port is a two-way communication channel between a host VZSerialPortAttachment and a virtual machine console port. One or more console ports are attached to a Virtio console device.

    An optional name may be set for a console port. A console port may also be configured for use as the system console.
 @see VZConsolePortConfiguration
 @see VZVirtualMachineConfiguration.consoleDevices
 */
void *newVZVirtioConsolePortConfiguration()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZVirtioConsolePortConfiguration alloc] init];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Set the console port's name. The default behavior is to not use a name unless set.
 */
void setNameVZVirtioConsolePortConfiguration(void *consolePortConfig, const char *name)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        NSString *nameNSString = [NSString stringWithUTF8String:name];
        [(VZVirtioConsolePortConfiguration *)consolePortConfig setName:nameNSString];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Set the console port may be marked for use as the system console. The default is false.
 */
void setIsConsoleVZVirtioConsolePortConfiguration(void *consolePortConfig, bool isConsole)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        [(VZVirtioConsolePortConfiguration *)consolePortConfig setIsConsole:(BOOL)isConsole];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Set the console port attachment. Defines how the virtual machine's console port interfaces with the host system. Default is nil.
 @see VZFileHandleSerialPortAttachment
 @see VZFileSerialPortAttachment
 @see VZSpiceAgentPortAttachment
 */
void setAttachmentVZVirtioConsolePortConfiguration(void *consolePortConfig, void *serialPortAttachment)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        [(VZVirtioConsolePortConfiguration *)consolePortConfig
            setAttachment:(VZSerialPortAttachment *)serialPortAttachment];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

void *newVZSpiceAgentPortAttachment()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZSpiceAgentPortAttachment alloc] init];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Enable the Spice agent clipboard sharing capability.
 @discussion
    If enabled, the clipboard capability will be advertised to the Spice guest agent. Copy and paste events
    will be shared between the host and the virtual machine.

    This property is enabled by default.
 */
void setSharesClipboardVZSpiceAgentPortAttachment(void *attachment, bool sharesClipboard)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [(VZSpiceAgentPortAttachment *)attachment setSharesClipboard:(BOOL)sharesClipboard];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract The Spice agent port name.
 @discussion
     A console port configured with this name will spawn a Spice guest agent if supported by the guest.

     VZConsolePortConfiguration.attachment must be set to VZSpiceAgentPortAttachment.
     VZVirtioConsolePortConfiguration.isConsole must remain false on a Spice agent port.
 */
const char *getSpiceAgentPortName()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZSpiceAgentPortAttachment spiceAgentPortName] UTF8String];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Start a virtual machine with options.
 @discussion
    Start a virtual machine that is in either Stopped or Error state.
 @param options Options used to control how the virtual machine is started.
 @param completionHandler Block called after the virtual machine has been successfully started or on error.
    The error parameter passed to the block is nil if the start was successful.
 @seealso VZMacOSVirtualMachineStartOptions
 */
void startWithOptionsCompletionHandler(void *machine, void *queue, void *options, uintptr_t cgoHandle)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        vm_completion_handler_t handler = makeVMCompletionHandler(cgoHandle);
        dispatch_sync((dispatch_queue_t)queue, ^{
            [(VZVirtualMachine *)machine startWithOptions:options completionHandler:handler];
        });
        Block_release(handler);
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract The macOS automount tag.
 @discussion A device configured with this tag will be automatically mounted in a macOS guest.
 */
const char *getMacOSGuestAutomountTag()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZVirtioFileSystemDeviceConfiguration macOSGuestAutomountTag] UTF8String];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract The maximum transmission unit (MTU) associated with this attachment.
 @discussion
    The client side of the associated datagram socket must be properly configured with the appropriate values for
    `SO_SNDBUF`, and `SO_RCVBUF`, which can be set using the `setsockopt` system call. The value of `SO_RCVBUF` is
    expected to be at least double the value of `SO_SNDBUF`, and for optimal performance, the value of `SO_RCVBUF`
    is recommended to be four times the value of `SO_SNDBUF`.

    The default MTU is 1500.
    The maximum MTU allowed is 65535, and the minimum MTU allowed is 1500. An invalid MTU value will result in an invalid
    virtual machine configuration.
 */
void setMaximumTransmissionUnitVZFileHandleNetworkDeviceAttachment(void *attachment, NSInteger mtu)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        [(VZFileHandleNetworkDeviceAttachment *)attachment setMaximumTransmissionUnit:mtu];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}