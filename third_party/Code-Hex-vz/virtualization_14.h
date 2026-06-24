//
//  virtualization_14.h
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
void attachmentDidEncounterErrorHandler(uintptr_t cgoHandle, void *err);
void attachmentWasConnectedHandler(uintptr_t cgoHandle);

/* macOS 14 API */
void *newVZNVMExpressControllerDeviceConfiguration(void *attachment);
void *newVZDiskBlockDeviceStorageDeviceAttachment(int fileDescriptor, bool readOnly, int syncMode, void **error);
void *newVZNetworkBlockDeviceStorageDeviceAttachment(const char *url, double timeout, bool forcedReadOnly, int syncMode, void **error, uintptr_t cgoHandle);

#ifdef INCLUDE_TARGET_OSX_14
@interface VZNetworkBlockDeviceStorageDeviceAttachmentDelegateImpl : NSObject <VZNetworkBlockDeviceStorageDeviceAttachmentDelegate>
- (instancetype)initWithHandle:(uintptr_t)cgoHandle;
- (void)attachment:(VZNetworkBlockDeviceStorageDeviceAttachment *)attachment didEncounterError:(NSError *)error API_AVAILABLE(macos(14.0));
- (void)attachmentWasConnected:(VZNetworkBlockDeviceStorageDeviceAttachment *)attachment API_AVAILABLE(macos(14.0));
@end
#endif
