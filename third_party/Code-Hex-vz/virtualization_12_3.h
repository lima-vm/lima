//
//  virtualization_12_3.h
//
//  Created by codehex.
//

#pragma once

#import "virtualization_helper.h"
#import <Virtualization/Virtualization.h>

// FIXME(codehex): this is dirty hack to avoid clang-format error like below
// "Configuration file(s) do(es) not support C++: /github.com/Code-Hex/vz/.clang-format"
#define NSURLComponents NSURLComponents

void setBlockDeviceIdentifierVZVirtioBlockDeviceConfiguration(void *blockDeviceConfig, const char *identifier, void **error);