//
//  virtualization_13_arm64.h
//
//  Created by codehex.
//

#pragma once

#ifdef __arm64__

// FIXME(codehex): this is dirty hack to avoid clang-format error like below
// "Configuration file(s) do(es) not support C++: /github.com/Code-Hex/vz/.clang-format"
#define NSURLComponents NSURLComponents

#import "virtualization_helper.h"
#import <Virtualization/Virtualization.h>

/* exported from cgo */
void linuxInstallRosettaWithCompletionHandler(uintptr_t cgoHandle, void *errPtr);

void *newVZLinuxRosettaDirectoryShare(void **error);
void linuxInstallRosetta(uintptr_t cgoHandle);
int availabilityVZLinuxRosettaDirectoryShare();

void *newVZMacOSVirtualMachineStartOptions(bool startUpFromMacOSRecovery);

void *newVZMacTrackpadConfiguration();

#endif