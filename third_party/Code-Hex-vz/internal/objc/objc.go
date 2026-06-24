package objc

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c
#cgo darwin LDFLAGS: -lobjc -framework Foundation
#import <Foundation/Foundation.h>

void *makeNSMutableArray(unsigned long cap)
{
	return [[NSMutableArray alloc] initWithCapacity:(NSUInteger)cap];
}

void addNSMutableArrayVal(void *ary, void *val)
{
	[(NSMutableArray *)ary addObject:(NSObject *)val];
}

void *makeNSMutableDictionary()
{
	return [[NSMutableDictionary alloc] init];
}

void insertNSMutableDictionary(void *dict, char *key, void *val)
{
	NSString *nskey = [NSString stringWithUTF8String: key];
	[(NSMutableDictionary *)dict setValue:(NSObject *)val forKey:nskey];
}

void releaseNSObject(void* o)
{
	[(NSObject*)o release];
}

void retainNSObject(void* o)
{
	[(NSObject*)o retain];
}

static inline void releaseDispatch(void *queue)
{
	dispatch_release((dispatch_queue_t)queue);
}

int getNSArrayCount(void *ptr)
{
	return (int)[(NSArray*)ptr count];
}

void* getNSArrayItem(void *ptr, int i)
{
	NSArray *arr = (NSArray *)ptr;
	return [arr objectAtIndex:i];
}

const char *getUUID()
{
	NSString *uuid = [[NSUUID UUID] UUIDString];
	return [uuid UTF8String];
}
*/
import "C"
import (
	"runtime"
	"unsafe"
)

// ReleaseDispatch releases allocated dispatch_queue_t
func ReleaseDispatch(p unsafe.Pointer) {
	C.releaseDispatch(p)
}

// Pointer indicates any pointers which are allocated in objective-c world.
type Pointer struct {
	_ptr unsafe.Pointer
}

// NewPointer creates a new Pointer for objc
func NewPointer(p unsafe.Pointer) *Pointer {
	return &Pointer{_ptr: p}
}

// release releases allocated resources in objective-c world.
// decrements reference count.
func (p *Pointer) release() {
	C.releaseNSObject(p._ptr)
	runtime.KeepAlive(p)
}

// retain increments reference count in objective-c world.
func (p *Pointer) retain() {
	C.retainNSObject(p._ptr)
	runtime.KeepAlive(p)
}

// Ptr returns raw pointer.
func (o *Pointer) ptr() unsafe.Pointer {
	if o == nil {
		return nil
	}
	return o._ptr
}

// NSObject indicates NSObject
type NSObject interface {
	ptr() unsafe.Pointer
	release()
	retain()
}

// Release releases allocated resources in objective-c world.
func Release(o NSObject) {
	o.release()
}

// Retain increments reference count in objective-c world.
func Retain(o NSObject) {
	o.retain()
}

// Ptr returns unsafe.Pointer of the NSObject
func Ptr(o NSObject) unsafe.Pointer {
	return o.ptr()
}

// NSArray indicates NSArray
type NSArray struct {
	*Pointer
}

// NewNSArray creates a new NSArray from pointer.
func NewNSArray(p unsafe.Pointer) *NSArray {
	return &NSArray{NewPointer(p)}
}

// ToPointerSlice method returns slice of the obj-c object as unsafe.Pointer.
func (n *NSArray) ToPointerSlice() []unsafe.Pointer {
	count := int(C.getNSArrayCount(n.ptr()))
	ret := make([]unsafe.Pointer, count)
	for i := 0; i < count; i++ {
		ret[i] = C.getNSArrayItem(n.ptr(), C.int(i))
	}
	return ret
}

// ConvertToNSMutableArray converts to NSMutableArray from NSObject slice in Go world.
func ConvertToNSMutableArray(s []NSObject) *Pointer {
	ln := len(s)
	ary := C.makeNSMutableArray(C.ulong(ln))
	for _, v := range s {
		C.addNSMutableArrayVal(ary, v.ptr())
	}
	p := NewPointer(ary)
	runtime.SetFinalizer(p, func(self *Pointer) {
		self.release()
	})
	return p
}

// ConvertToNSMutableDictionary converts to NSMutableDictionary from map[string]NSObject in Go world.
func ConvertToNSMutableDictionary(d map[string]NSObject) *Pointer {
	dict := C.makeNSMutableDictionary()
	for key, value := range d {
		cs := (*C.char)(C.CString(key))
		C.insertNSMutableDictionary(dict, cs, value.ptr())
		C.free(unsafe.Pointer(cs))
	}
	p := NewPointer(dict)
	runtime.SetFinalizer(p, func(self *Pointer) {
		self.release()
	})
	return p
}

func GetUUID() *C.char {
	return C.getUUID()
}
