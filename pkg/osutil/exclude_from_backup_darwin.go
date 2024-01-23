package osutil

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c
#cgo darwin LDFLAGS: -lobjc -framework Foundation
#import <Foundation/Foundation.h>

void setExcludeFromBackup(const char *path, int exclude) {
	@autoreleasepool {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		NSError *error = nil;
		if (![url setResourceValue: exclude == 1 ? @YES : @NO forKey:NSURLIsExcludedFromBackupKey error:&error]) {
			NSLog(@"setResourceValue failed: %@", error);
		}
	}
}
*/
import "C" //nolint:gocritic
/*
The reason for separating the import statement is that CGO only recognizes the standalone import of "C".
However, the linter tool `gocritic` incorrectly identifies this separation as a duplicate import and raises
a `dupImport` warning. To avoid this warning, the `gocritic` check has been disabled.
Ref: https://github.com/go-critic/go-critic/issues/845
*/
import (
	"unsafe" //nolint:gocritic
)

// SetExcludeFromBackup sets the `NSURLIsExcludedFromBackupKey` attribute of the specified file or directory.
func SetExcludeFromBackup(path string, exclude bool) {
	cs := C.CString(path)
	defer C.free(unsafe.Pointer(cs))
	if exclude {
		C.setExcludeFromBackup(cs, C.int(1))
	} else {
		C.setExcludeFromBackup(cs, C.int(0))
	}
}
