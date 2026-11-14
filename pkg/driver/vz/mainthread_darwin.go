//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import "sync/atomic"

// AppKit must run on the process's initial thread. The external driver serves
// gRPC on other goroutines, so it runs AppKit work through RunMainThreadLoop.
var mainThread atomic.Pointer[mainThreadLoop]

type mainThreadLoop struct {
	funcs chan func()
	done  chan struct{}
}

// RunMainThreadLoop runs serve on a new goroutine and submitted functions on the
// caller until serve returns. The caller must be the locked main goroutine.
func RunMainThreadLoop(serve func()) {
	loop := &mainThreadLoop{
		funcs: make(chan func()),
		done:  make(chan struct{}),
	}
	mainThread.Store(loop)
	defer close(loop.done)

	served := make(chan struct{})
	go func() {
		defer close(served)
		serve()
	}()
	for {
		select {
		case f := <-loop.funcs:
			f()
		case <-served:
			return
		}
	}
}

// goMainThread runs f on the initial thread without waiting. It returns false
// when RunMainThreadLoop is not running.
func goMainThread(f func()) bool {
	loop := mainThread.Load()
	if loop == nil {
		return false
	}
	go func() {
		select {
		case loop.funcs <- f:
		case <-loop.done:
		}
	}()
	return true
}

// onMainThread runs f on the initial thread. After RunMainThreadLoop returns,
// f is dropped.
func onMainThread(f func()) {
	loop := mainThread.Load()
	if loop == nil {
		f()
		return
	}
	finished := make(chan struct{})
	select {
	case loop.funcs <- func() {
		defer close(finished)
		f()
	}:
		<-finished
	case <-loop.done:
	}
}
