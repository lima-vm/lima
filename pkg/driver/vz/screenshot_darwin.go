// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && !no_vz

package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework AppKit -framework CoreGraphics -framework ImageIO
#include <stdlib.h>
#include <string.h>
#include <dispatch/dispatch.h>
#include <AppKit/AppKit.h>
#include <CoreGraphics/CoreGraphics.h>
#include <ImageIO/ImageIO.h>

static void freeCString(char *s) { free(s); }

// captureWindowImageBytes captures the frontmost non-panel on-screen window of
// the current process, encodes it using the given ImageIO UTI, and returns a
// malloc'd buffer. The caller must free() the returned pointer.
static void *captureWindowImageBytes(int *outLen, const char *uti) {
    __block void *result = NULL;
    __block int resultLen = 0;

    NSString *utiStr = [NSString stringWithUTF8String:uti];

    // dispatch_sync to the main queue is required for window server access.
    // CaptureScreenshot is always called from a Go goroutine, which is never
    // the main thread, so no deadlock is possible in normal Lima operation.
    // The isMainThread guard is a safety net for unexpected call paths.
    void (^captureBlock)(void) = ^{
        NSWindow *target = nil;
        for (NSWindow *w in [NSApplication sharedApplication].windows) {
            if ([w isKindOfClass:[NSPanel class]]) continue;
            if (!w.isVisible) continue;
            target = w;
            break;
        }
        if (!target) return;

        CGWindowID wid = (CGWindowID)[target windowNumber];
        CGImageRef img = CGWindowListCreateImage(
            CGRectNull,
            kCGWindowListOptionIncludingWindow,
            wid,
            kCGWindowImageBoundsIgnoreFraming | kCGWindowImageShouldBeOpaque
        );
        if (!img) return;

        NSMutableData *data = [NSMutableData data];
        CGImageDestinationRef dst = CGImageDestinationCreateWithData(
            (__bridge CFMutableDataRef)data,
            (__bridge CFStringRef)utiStr,
            1, NULL
        );
        if (!dst) {
            CGImageRelease(img);
            return;
        }
        CGImageDestinationAddImage(dst, img, NULL);
        bool ok = CGImageDestinationFinalize(dst);
        CFRelease(dst);
        CGImageRelease(img);
        if (!ok) return;

        resultLen = (int)data.length;
        result = malloc(resultLen);
        memcpy(result, data.bytes, resultLen);
    };

    if ([NSThread isMainThread]) {
        captureBlock();
    } else {
        dispatch_sync(dispatch_get_main_queue(), captureBlock);
    }

    *outLen = resultLen;
    return result;
}
*/
import "C"

import (
	"errors"
	"fmt"

	"github.com/lima-vm/lima/v2/pkg/driver"
)

// CaptureScreenshot captures the VM display window.
// format is "png" or "bmp"; anything else defaults to PNG.
// Implements driver.Screenshotter. Requires the GUI app bundle to be running.
func (l *LimaVzDriver) CaptureScreenshot(format string) ([]byte, error) {
	if !l.canRunGUI() {
		return nil, fmt.Errorf("%w (set video.display to \"default\" or \"vz\" to enable screenshots)", driver.ErrNoDisplay)
	}
	uti := "public.png"
	if format == "bmp" {
		uti = "com.microsoft.bmp"
	}
	cuti := C.CString(uti)
	defer C.freeCString(cuti)
	var outLen C.int
	ptr := C.captureWindowImageBytes(&outLen, cuti)
	if ptr == nil || outLen == 0 {
		return nil, errors.New("screenshot capture returned no data; GUI window may not be visible")
	}
	defer C.free(ptr)
	return C.GoBytes(ptr, outLen), nil
}
