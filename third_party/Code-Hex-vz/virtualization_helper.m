//
//  virtualization_helper.m
//
//  Created by codehex.
//

#import "virtualization_helper.h"

#ifdef __arm64__
#define TARGET_ARM64 1
#else
#define TARGET_ARM64 0
#endif

NSDictionary *dumpProcessinfo()
{
    NSString *osVersionString = [[NSProcessInfo processInfo] operatingSystemVersionString];
    return @{
        @"LLVM (Clang) Version" : @__VERSION__,
        @"Target for arm64" : @TARGET_ARM64,
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