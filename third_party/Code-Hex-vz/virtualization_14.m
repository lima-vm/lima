//
//  virtualization_14.m
//
//  Created by codehex.
//

#import "virtualization_14.h"

/*!
 @abstract Initialize a VZNVMExpressControllerDeviceConfiguration with a device attachment.
 @param attachment The storage device attachment. This defines how the virtualized device operates on the host side.
 @see VZDiskImageStorageDeviceAttachment
 @see https://nvmexpress.org/wp-content/uploads/NVM-Express-1_1b-1.pdf
 */
void *newVZNVMExpressControllerDeviceConfiguration(void *attachment)
{
#ifdef INCLUDE_TARGET_OSX_14
    if (@available(macOS 14, *)) {
        return [[VZNVMExpressControllerDeviceConfiguration alloc] initWithAttachment:(VZStorageDeviceAttachment *)attachment];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Initialize the disk attachment from a file handle.
 @param fileHandle File handle to a block device.
 @param readOnly If YES, the disk attachment is read only, otherwise, if the file handle allows writes, the device can write data into it.
 @param synchronizationMode Defines how the disk synchronizes with the underlying storage when the guest operating system flushes data.
 @param error If not nil, assigned with the error if the initialization failed.
 @return An initialized `VZDiskBlockDeviceStorageDeviceAttachment` or nil if there was an error.
 @discussion
    The file handle is retained by the disk attachment.
    The handle must be open when the virtual machine starts.

    The `readOnly` parameter affects how the disk is exposed to the guest operating system
    by the storage controller. If the disk is intended to be used read-only, it is also recommended
    to open the file handle as read-only.
 */
void *newVZDiskBlockDeviceStorageDeviceAttachment(int fileDescriptor, bool readOnly, int syncMode, void **error)
{
#ifdef INCLUDE_TARGET_OSX_14
    if (@available(macOS 14, *)) {
        NSFileHandle *fileHandle = [[NSFileHandle alloc] initWithFileDescriptor:fileDescriptor];
        return [[VZDiskBlockDeviceStorageDeviceAttachment alloc]
             initWithFileHandle:fileHandle
                       readOnly:(BOOL)readOnly
            synchronizationMode:(VZDiskSynchronizationMode)syncMode
                          error:(NSError *_Nullable *_Nullable)error];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Initialize a network block device storage attachment from an NBD URI.
 @param uri The NBD’s URI represented as a URL.
 @param timeout The timeout value in seconds for the connection between the client and server. When the timeout expires, an attempt to reconnect with the server takes place.
 @param forcedReadOnly If YES, the framework forces the disk attachment to be read-only, regardless of whether or not the NBD server supports write requests.
 @param synchronizationMode Defines how the disk synchronizes with the underlying storage when the guest operating system flushes data.
 @param error If not nil, assigned with the error if the initialization failed.
 @return An initialized `VZDiskBlockDeviceStorageDeviceAttachment` or nil if there was an error.
 @discussion
    The forcedReadOnly parameter affects how framework exposes the NBD client to the guest operating
    system by the storage controller. As part of the NBD protocol, the NBD server advertises whether
    or not the disk exposed by the NBD client is read-only during the handshake phase of the protocol.

    Setting forcedReadOnly to YES forces the NBD client to show up as read-only to the guest
    regardless of whether or not the NBD server advertises itself as read-only.
 */
void *newVZNetworkBlockDeviceStorageDeviceAttachment(const char *uri, double timeout, bool forcedReadOnly, int syncMode, void **error, uintptr_t cgoHandle)
{
#ifdef INCLUDE_TARGET_OSX_14
    if (@available(macOS 14, *)) {
        NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:uri]];

        VZNetworkBlockDeviceStorageDeviceAttachment *attachment = [[VZNetworkBlockDeviceStorageDeviceAttachment alloc]
                    initWithURL:url
                        timeout:(NSTimeInterval)timeout
                 forcedReadOnly:(BOOL)forcedReadOnly
            synchronizationMode:(VZDiskSynchronizationMode)syncMode
                          error:(NSError *_Nullable *_Nullable)error];

        if (attachment) {
            [attachment setDelegate:[[[VZNetworkBlockDeviceStorageDeviceAttachmentDelegateImpl alloc] initWithHandle:cgoHandle] autorelease]];
        }

        return attachment;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

#ifdef INCLUDE_TARGET_OSX_14
@implementation VZNetworkBlockDeviceStorageDeviceAttachmentDelegateImpl {
    uintptr_t _cgoHandle;
}

- (instancetype)initWithHandle:(uintptr_t)cgoHandle
{
    self = [super init];
    _cgoHandle = cgoHandle;
    return self;
}

- (void)attachment:(VZNetworkBlockDeviceStorageDeviceAttachment *)attachment didEncounterError:(NSError *)error
{
    attachmentDidEncounterErrorHandler(_cgoHandle, error);
}

- (void)attachmentWasConnected:(VZNetworkBlockDeviceStorageDeviceAttachment *)attachment
{
    attachmentWasConnectedHandler(_cgoHandle);
}
@end
#endif
