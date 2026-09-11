// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// scrollbar_fix_darwin.m - Runtime patch to disable scrollbars on VZ display window
//
// The Code-Hex/vz library wraps VZVirtualMachineView in an NSScrollView with
// scrollbars hardcoded to YES (for their zoom feature). This causes ~16px of
// the VM display to be cut off by scrollbar gutters.
//
// This file uses Objective-C method swizzling to intercept NSScrollView's
// setDocumentView: method and disable scrollbars when the document view is
// a VZVirtualMachineView.
//
// This is a temporary workaround until Code-Hex/vz adds a configuration option
// to disable scrollbars. See: https://github.com/Code-Hex/vz/issues/XXX

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

// Store original implementation pointer
static IMP original_setDocumentView = NULL;

// Patched implementation that disables scrollbars for VZ views
static void patched_setDocumentView(id self, SEL _cmd, NSView *view) {
    // Call original implementation first
    ((void (*)(id, SEL, NSView *))original_setDocumentView)(self, _cmd, view);
    
    // If the document view is a VZVirtualMachineView, disable scrollbars
    // The scrollbars cause the VM display to be cut off by ~16px
    if (view != nil && [view isKindOfClass:NSClassFromString(@"VZVirtualMachineView")]) {
        [(NSScrollView *)self setHasVerticalScroller:NO];
        [(NSScrollView *)self setHasHorizontalScroller:NO];
    }
}

// Constructor attribute ensures this runs before main()
// This patches NSScrollView before Code-Hex/vz creates any windows
__attribute__((constructor))
static void patchNSScrollViewForVZ(void) {
    Method m = class_getInstanceMethod([NSScrollView class], @selector(setDocumentView:));
    if (m != NULL) {
        original_setDocumentView = method_setImplementation(m, (IMP)patched_setDocumentView);
    }
}
