// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networkchange

/*
#cgo darwin CFLAGS: -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc
#import "networkchange_darwin.h"
*/
import "C"

// Notifier represents a network change notifier.
type Notifier struct {
	token         int
	notifyHandler *cgoHandler
}

type NotifyHandler func(*Notifier)

// NewNotifier creates a new Notifier instance.
// It registers for network change notifications and sets up the provided handler to be called upon notifications.
// The caller is responsible for calling Cancel() to clean up resources.
//
// It uses the Darwin notify API:
//   - https://developer.apple.com/documentation/darwinnotify/notify_register_dispatch
//   - https://developer.apple.com/documentation/darwinnotify/knotifyscnetworkchange/
func NewNotifier(handler NotifyHandler) *Notifier {
	if handler == nil {
		return nil
	}
	var token C.int
	cgoHandler, handle := newCgoHandler(handler)
	res := C.notifyRegisterDispatch(&token, handle)
	if res != 0 {
		cgoHandler.releaseOnCleanup()
		return nil
	}
	return &Notifier{
		token:         int(token),
		notifyHandler: cgoHandler,
	}
}

//export callNotifyHandler
func callNotifyHandler(handlerPtr uintptr, token int) {
	handler := unwrapHandler[NotifyHandler](handlerPtr)
	handler(&Notifier{token: token})
}

// Suspend suspends the notifier.
//   - https://developer.apple.com/documentation/darwinnotify/notify_suspend/
func (n *Notifier) Suspend() {
	C.notify_suspend(C.int(n.token))
}

// Resume resumes the notifier.
//   - https://developer.apple.com/documentation/darwinnotify/notify_resume/
func (n *Notifier) Resume() {
	C.notify_resume(C.int(n.token))
}

// Cancel cancels the notifier.
//   - https://developer.apple.com/documentation/darwinnotify/notify_cancel/
func (n *Notifier) Cancel() {
	C.notify_cancel(C.int(n.token))
}
