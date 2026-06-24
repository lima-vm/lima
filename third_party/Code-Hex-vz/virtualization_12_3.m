//
//  virtualization_12_3.m
//
//  Created by codehex.
//

#import "virtualization_12_3.h"

void setBlockDeviceIdentifierVZVirtioBlockDeviceConfiguration(void *blockDeviceConfig, const char *identifier, void **error)
{
#ifdef INCLUDE_TARGET_OSX_12_3
    if (@available(macOS 12.3, *)) {
        NSString *identifierNSString = [NSString stringWithUTF8String:identifier];
        BOOL valid = [VZVirtioBlockDeviceConfiguration
            validateBlockDeviceIdentifier:identifierNSString
                                    error:(NSError *_Nullable *_Nullable)error];
        if (!valid) {
            return;
        }
        [(VZVirtioBlockDeviceConfiguration *)blockDeviceConfig setBlockDeviceIdentifier:identifierNSString];
        return;
    }
#endif
    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}