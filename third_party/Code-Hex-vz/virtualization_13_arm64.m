//
//  virtualization_13_arm64.m
//
//  Created by codehex.
//

#import "virtualization_13_arm64.h"

/*!
 @abstract Initialize a Rosetta directory share if Rosetta support for Linux binaries is installed.
 @param error Error object to store the error, if an error exists.
 @discussion The call returns an error if Rosetta is not available for a directory share. To install Rosetta support, use +[VZLinuxRosettaDirectoryShare installRosettaWithCompletionHandler:].
 */
void *newVZLinuxRosettaDirectoryShare(void **error)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZLinuxRosettaDirectoryShare alloc] initWithError:(NSError *_Nullable *_Nullable)error];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Download and install Rosetta support for Linux binaries if necessary.
 @param completionHandler The completion handler gets called with a valid error on failure and a nil error on success. It will also be invoked on an arbitrary queue.
 @discussion
    The call prompts the user through the download and install flow for Rosetta. This call is successful if the error is nil.
 @see +[VZLinuxRosettaDirectoryShare availability]
 */
void linuxInstallRosetta(uintptr_t cgoHandle)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        [VZLinuxRosettaDirectoryShare installRosettaWithCompletionHandler:^(NSError *error) {
            linuxInstallRosettaWithCompletionHandler(cgoHandle, error);
        }];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Check the availability of Rosetta support for the directory share.
 */
int availabilityVZLinuxRosettaDirectoryShare()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return (int)[VZLinuxRosettaDirectoryShare availability];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Options controlling startup behavior of a virtual machine using VZMacOSBootLoader.
 */
void *newVZMacOSVirtualMachineStartOptions(bool startUpFromMacOSRecovery)
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        VZMacOSVirtualMachineStartOptions *opts = [[VZMacOSVirtualMachineStartOptions alloc] init];
        [opts setStartUpFromMacOSRecovery:(BOOL)startUpFromMacOSRecovery];
        return opts;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract Configuration for a Mac trackpad.
 @discussion
    This device can be used by VZVirtualMachineView to send pointer events and multi-touch trackpad gestures to the virtual machine.
    Note: this device is only recognized by virtual machines running macOS 13.0 and later. In order to support both macOS 13.0 and earlier
    guests, VZVirtualMachineConfiguration.pointingDevices can be set to an array containing both a VZMacTrackpadConfiguration and
    a VZUSBScreenCoordinatePointingDeviceConfiguration object. macOS 13.0 and later guests will use the multi-touch trackpad device,
    while earlier versions of macOS will use the USB pointing device.
 */
void *newVZMacTrackpadConfiguration()
{
#ifdef INCLUDE_TARGET_OSX_13
    if (@available(macOS 13, *)) {
        return [[VZMacTrackpadConfiguration alloc] init];
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}