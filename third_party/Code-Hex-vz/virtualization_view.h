//
//  virtualization_view.h
//
//  Created by codehex.
//

#pragma once

#import "virtualization_helper.h"
#import <Availability.h>
#import <Cocoa/Cocoa.h>
#import <Virtualization/Virtualization.h>

@interface VZApplication : NSApplication {
    bool shouldKeepRunning;
}
@end

@interface AboutViewController : NSViewController
- (instancetype)init;
@end

@interface AboutPanel : NSPanel
- (instancetype)init;
@end

API_AVAILABLE(macos(12.0))
@interface AppDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate, VZVirtualMachineDelegate, NSToolbarDelegate>
- (instancetype)initWithVirtualMachine:(VZVirtualMachine *)virtualMachine
                                 queue:(dispatch_queue_t)queue
                           windowWidth:(CGFloat)windowWidth
                          windowHeight:(CGFloat)windowHeight
                           windowTitle:(NSString *)windowTitle
                      enableController:(BOOL)enableController;
@end