//
//  virtualization_debug.m
//
//  Created by codehex.
//

#import "virtualization_debug.h"
#import "virtualization_helper.h"

/*!
 @abstract Create a VZGDBDebugStubConfiguration with debug port for GDB server.
*/
void *newVZGDBDebugStubConfiguration(uint32_t port)
{
    if (@available(macOS 12, *)) {
        return [[_VZGDBDebugStubConfiguration alloc] initWithPort:(NSInteger)port];
    }

    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}

/*!
 @abstract _VZDebugStubConfiguration. Empty by default.
*/
void setDebugStubVZVirtualMachineConfiguration(void *config, void *debugStub)
{
    if (@available(macOS 12, *)) {
        [(VZVirtualMachineConfiguration *)config _setDebugStub:(_VZDebugStubConfiguration *)debugStub];
        return;
    }

    RAISE_UNSUPPORTED_MACOS_EXCEPTION();
}