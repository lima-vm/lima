//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRunMainThreadLoop(t *testing.T) {
	t.Cleanup(func() { mainThread.Store(nil) })

	var loopGoroutine, ranOn, wentOn bool
	RunMainThreadLoop(func() {
		onMainThread(func() { ranOn = true })
		ran := make(chan struct{})
		assert.Assert(t, goMainThread(func() {
			wentOn = true
			close(ran)
		}))
		<-ran
		loopGoroutine = true
	})
	assert.Assert(t, loopGoroutine)
	assert.Assert(t, ranOn)
	assert.Assert(t, wentOn)

	// After the loop has returned, functions are dropped instead of blocking.
	called := false
	onMainThread(func() { called = true })
	assert.Assert(t, !called)
	assert.Assert(t, goMainThread(func() { called = true }))
}

func TestOnMainThreadWithoutLoop(t *testing.T) {
	mainThread.Store(nil)
	called := false
	onMainThread(func() { called = true })
	assert.Assert(t, called)
	assert.Assert(t, !goMainThread(func() {}))
}
