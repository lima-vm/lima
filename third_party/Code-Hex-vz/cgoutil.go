package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c
#cgo darwin LDFLAGS: -lobjc -framework Foundation
#import <Foundation/Foundation.h>


const char *getNSErrorLocalizedDescription(void *err)
{
	NSString *ld = (NSString *)[(NSError *)err localizedDescription];
	return [ld UTF8String];
}

const char *getNSErrorDomain(void *err)
{
	NSString *domain = (NSString *)[(NSError *)err domain];
	return [domain UTF8String];
}

const char *getNSErrorUserInfo(void *err)
{
	NSDictionary<NSErrorUserInfoKey, id> *ui = [(NSError *)err userInfo];
	NSString *uis = [NSString stringWithFormat:@"%@", ui];
	return [uis UTF8String];
}

NSInteger getNSErrorCode(void *err)
{
	return (NSInteger)[(NSError *)err code];
}

typedef struct NSErrorFlat {
	const char *domain;
    const char *localizedDescription;
	const char *userinfo;
    int code;
} NSErrorFlat;

NSErrorFlat convertNSError2Flat(void *err)
{
	NSErrorFlat ret;
	ret.domain = getNSErrorDomain(err);
	ret.localizedDescription = getNSErrorLocalizedDescription(err);
	ret.userinfo = getNSErrorUserInfo(err);
	ret.code = (int)getNSErrorCode(err);

	return ret;
}

void *newNSError()
{
	NSError *err = nil;
	return err;
}

bool hasError(void *err)
{
	return (NSError *)err != nil;
}
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/Code-Hex/vz/v3/internal/objc"
)

// pointer is a type alias which is able to use as embedded type and
// makes as unexported it.
type pointer = objc.Pointer

// NSError indicates NSError.
type NSError struct {
	Domain               string
	Code                 int
	LocalizedDescription string
	UserInfo             string
}

// newNSErrorAsNil makes nil NSError in objective-c world.
func newNSErrorAsNil() unsafe.Pointer {
	return unsafe.Pointer(C.newNSError())
}

// hasNSError checks passed pointer is NSError or not.
func hasNSError(nserrPtr unsafe.Pointer) bool {
	return (bool)(C.hasError(nserrPtr))
}

func (n *NSError) Error() string {
	if n == nil {
		return "<nil>"
	}
	return fmt.Sprintf(
		"Error Domain=%s Code=%d Description=%q UserInfo=%s",
		n.Domain,
		n.Code,
		n.LocalizedDescription,
		n.UserInfo,
	)
}

func newNSError(p unsafe.Pointer) *NSError {
	if !hasNSError(p) {
		return nil
	}
	nsError := C.convertNSError2Flat(p)
	return &NSError{
		Domain:               (*char)(nsError.domain).String(),
		Code:                 int((nsError.code)),
		LocalizedDescription: (*char)(nsError.localizedDescription).String(),
		UserInfo:             (*char)(nsError.userinfo).String(), // NOTE(codehex): maybe we can convert to map[string]interface{}
	}
}

// CharWithGoString makes *Char which is *C.Char wrapper from Go string.
func charWithGoString(s string) *char {
	return (*char)(unsafe.Pointer(C.CString(s)))
}

// Char is a wrapper of C.char
type char C.char

// CString converts *C.char from *Char
func (c *char) CString() *C.char {
	return (*C.char)(c)
}

// String converts Go string from *Char
func (c *char) String() string {
	return C.GoString((*C.char)(c))
}

// Free frees allocated *C.char in Go code
func (c *char) Free() {
	C.free(unsafe.Pointer(c))
}
