//
//  virtualization_debug.h
//
//  Created by codehex.
//

#pragma once

#import <Foundation/Foundation.h>
#import <Virtualization/Virtualization.h>

@interface _VZDebugStubConfiguration : NSObject <NSCopying>
@end

@interface _VZGDBDebugStubConfiguration : NSObject <NSCopying>
@property NSInteger port;
- (instancetype)initWithPort:(NSInteger)port;
@end

@interface VZVirtualMachineConfiguration ()
- (void)_setDebugStub:(_VZDebugStubConfiguration *)config;
@end

void *newVZGDBDebugStubConfiguration(uint32_t port);
void setDebugStubVZVirtualMachineConfiguration(void *config, void *debugStub);