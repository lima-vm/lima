//go:build go1.18
// +build go1.18

package objc

import "runtime"

func SetFinalizer[T any](obj T, finalizer func(T)) {
	runtime.SetFinalizer(obj, finalizer)
}
