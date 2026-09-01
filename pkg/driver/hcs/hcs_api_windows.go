// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hcs

import (
	"context"
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// ---------------------------------------------------------------------------
// computecore.dll (HCS) bindings
// ---------------------------------------------------------------------------

var (
	modcomputecore = windows.NewLazySystemDLL("computecore.dll")

	procHcsCreateOperation            = modcomputecore.NewProc("HcsCreateOperation")
	procHcsCloseOperation             = modcomputecore.NewProc("HcsCloseOperation")
	procHcsWaitForOperationResult     = modcomputecore.NewProc("HcsWaitForOperationResult")
	procHcsCreateComputeSystem        = modcomputecore.NewProc("HcsCreateComputeSystem")
	procHcsStartComputeSystem         = modcomputecore.NewProc("HcsStartComputeSystem")
	procHcsTerminateComputeSystem     = modcomputecore.NewProc("HcsTerminateComputeSystem")
	procHcsCloseComputeSystem         = modcomputecore.NewProc("HcsCloseComputeSystem")
	procHcsGrantVMAccess              = modcomputecore.NewProc("HcsGrantVmAccess")
	procHcsOpenComputeSystem          = modcomputecore.NewProc("HcsOpenComputeSystem")
	procHcsWaitForComputeSystemExit   = modcomputecore.NewProc("HcsWaitForComputeSystemExit")
	procHcsGetComputeSystemProperties = modcomputecore.NewProc("HcsGetComputeSystemProperties")

	modapiMsWinCoreComL110 = windows.NewLazySystemDLL("api-ms-win-core-com-l1-1-0.dll")
	procCoTaskMemFree      = modapiMsWinCoreComL110.NewProc("CoTaskMemFree")
)

type (
	hcsOperation syscall.Handle
	hcsSystem    syscall.Handle
)

const (
	infiniteTimeout       = 0xFFFFFFFF
	syscallWatcherTimeout = 4 * time.Minute
)

// hresultErr converts an HRESULT return value into a Go error, unwrapping
// FACILITY_WIN32 the same way hcsshim's generated bindings do.
func hresultErr(r0 uintptr) error {
	if int32(r0) >= 0 {
		return nil
	}
	if r0&0x1fff0000 == 0x00070000 {
		r0 &= 0xffff
	}
	return syscall.Errno(r0)
}

// coString copies a CoTaskMem-allocated PWSTR into a Go string and frees it.
func coString(p *uint16) string {
	if p == nil {
		return ""
	}
	s := windows.UTF16PtrToString(p)
	windows.CoTaskMemFree(unsafe.Pointer(p))
	return s
}

func hcsCreateOperation() (hcsOperation, error) {
	r0, _, e1 := syscall.SyscallN(procHcsCreateOperation.Addr(), 0, 0)
	if r0 == 0 {
		return 0, fmt.Errorf("HcsCreateOperation: %w", e1)
	}
	return hcsOperation(r0), nil
}

func hcsCloseOperation(op hcsOperation) {
	_, _, _ = syscall.SyscallN(procHcsCloseOperation.Addr(), uintptr(op))
}

// hcsWait drives one async HCS call to completion and returns the result
// document (which carries structured error info on failure).
func hcsWait(op hcsOperation, what string, callErr error) (string, error) {
	if callErr != nil {
		return "", fmt.Errorf("%s: %w", what, callErr)
	}
	var result *uint16
	r0, _, _ := syscall.SyscallN(procHcsWaitForOperationResult.Addr(),
		uintptr(op), uintptr(infiniteTimeout), uintptr(unsafe.Pointer(&result)))
	doc := coString(result)
	if err := hresultErr(r0); err != nil {
		return doc, fmt.Errorf("%s: %w (result: %s)", what, err, doc)
	}
	return doc, nil
}

func hcsCreateComputeSystem(id, configuration string) (hcsSystem, error) {
	op, err := hcsCreateOperation()
	if err != nil {
		return 0, err
	}
	defer hcsCloseOperation(op)

	idP, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return 0, err
	}
	cfgP, err := syscall.UTF16PtrFromString(configuration)
	if err != nil {
		return 0, err
	}
	var system hcsSystem
	r0, _, _ := syscall.SyscallN(procHcsCreateComputeSystem.Addr(),
		uintptr(unsafe.Pointer(idP)), uintptr(unsafe.Pointer(cfgP)),
		uintptr(op), 0, uintptr(unsafe.Pointer(&system)))
	if _, err := hcsWait(op, "HcsCreateComputeSystem", hresultErr(r0)); err != nil {
		return 0, err
	}
	return system, nil
}

func hcsStartComputeSystem(system hcsSystem) error {
	op, err := hcsCreateOperation()
	if err != nil {
		return err
	}
	defer hcsCloseOperation(op)
	r0, _, _ := syscall.SyscallN(procHcsStartComputeSystem.Addr(),
		uintptr(system), uintptr(op), 0)
	_, err = hcsWait(op, "HcsStartComputeSystem", hresultErr(r0))
	return err
}

func hcsTerminateComputeSystem(system hcsSystem) error {
	op, err := hcsCreateOperation()
	if err != nil {
		return err
	}
	defer hcsCloseOperation(op)
	r0, _, _ := syscall.SyscallN(procHcsTerminateComputeSystem.Addr(),
		uintptr(system), uintptr(op), 0)
	_, err = hcsWait(op, "HcsTerminateComputeSystem", hresultErr(r0))
	return err
}

func execute(ctx context.Context, timeout time.Duration, f func() error) error {
	done := make(chan error, 1)
	go func() { done <- f() }()

	var watcher <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		watcher = t.C
	}

	for {
		select {
		case err := <-done:
			return err
		case <-watcher:
			logrus.WithField("timeout", timeout).
				Warn("HCS syscall exceeded timeout; still waiting to avoid use-after-free in computecore.dll")
			watcher = nil
		case <-ctx.Done():
			logrus.WithError(ctx.Err()).
				Warn("HCS syscall context canceled; still waiting to avoid use-after-free in computecore.dll")
			ctx = context.WithoutCancel(ctx)
		}
	}
}

func hcsGetComputeSystemProperties(ctx context.Context, system hcsSystem, operation hcsOperation, propertyQuery string) (hr error) {
	return execute(ctx, syscallWatcherTimeout, func() error {
		return _hcsGetComputeSystemProperties(system, operation, propertyQuery)
	})
}

func _hcsGetComputeSystemProperties(system hcsSystem, operation hcsOperation, propertyQuery string) (hr error) {
	var _p0 *uint16
	_p0, hr = syscall.UTF16PtrFromString(propertyQuery)
	if hr != nil {
		return hr
	}

	hr = procHcsGetComputeSystemProperties.Find()
	if hr != nil {
		return hr
	}
	r0, _, _ := syscall.SyscallN(procHcsGetComputeSystemProperties.Addr(), uintptr(system), uintptr(operation), uintptr(unsafe.Pointer(_p0)))
	if int32(r0) < 0 {
		if r0&0x1fff0000 == 0x00070000 {
			r0 &= 0xffff
		}
		hr = syscall.Errno(r0)
	}
	return hr
}

func hcsOpenComputeSystem(ctx context.Context, id string, requestedAccess uint32) (computeSystem hcsSystem, hr error) {
	hr = execute(ctx, syscallWatcherTimeout, func() error {
		return _hcsOpenComputeSystem(id, requestedAccess, &computeSystem)
	})
	return computeSystem, hr
}

func _hcsOpenComputeSystem(id string, requestedAccess uint32, computeSystem *hcsSystem) (hr error) {
	var _p0 *uint16
	_p0, hr = syscall.UTF16PtrFromString(id)
	if hr != nil {
		return hr
	}

	hr = procHcsOpenComputeSystem.Find()
	if hr != nil {
		return hr
	}
	r0, _, _ := syscall.SyscallN(procHcsOpenComputeSystem.Addr(), uintptr(unsafe.Pointer(_p0)), uintptr(requestedAccess), uintptr(unsafe.Pointer(computeSystem)))
	if int32(r0) < 0 {
		if r0&0x1fff0000 == 0x00070000 {
			r0 &= 0xffff
		}
		hr = syscall.Errno(r0)
	}

	return hr
}

func coTaskMemFree(buffer unsafe.Pointer) {
	_, _, _ = syscall.SyscallN(procCoTaskMemFree.Addr(), uintptr(buffer))
}

func convertAndFreeCoTaskMemString(buffer *uint16) string {
	str := syscall.UTF16ToString((*[1 << 29]uint16)(unsafe.Pointer(buffer))[:])
	coTaskMemFree(unsafe.Pointer(buffer))
	return str
}

func hcsWaitForComputeSystemExit(ctx context.Context, system hcsSystem, timeoutMs uint32) (result string, hr error) {
	return result, execute(ctx, syscallWatcherTimeout, func() error {
		var resultp *uint16
		err := _hcsWaitForComputeSystemExit(system, timeoutMs, &resultp)
		if resultp != nil {
			result = convertAndFreeCoTaskMemString(resultp)
		}
		return err
	})
}

func _hcsWaitForComputeSystemExit(system hcsSystem, timeoutMs uint32, result **uint16) (hr error) {
	hr = procHcsWaitForComputeSystemExit.Find()
	if hr != nil {
		return hr
	}
	r0, _, _ := syscall.SyscallN(procHcsWaitForComputeSystemExit.Addr(), uintptr(system), uintptr(timeoutMs), uintptr(unsafe.Pointer(result)))
	if int32(r0) < 0 {
		if r0&0x1fff0000 == 0x00070000 {
			r0 &= 0xffff
		}
		hr = syscall.Errno(r0)
	}
	return hr
}

func hcsCloseComputeSystem(system hcsSystem) {
	_, _, _ = syscall.SyscallN(procHcsCloseComputeSystem.Addr(), uintptr(system))
}

// hcsGrantVMAccess ACLs a host file so the VM worker process may read it
// (hcsshim does the equivalent before every kernel-direct boot).
func hcsGrantVMAccess(vmID, path string) error {
	idP, err := syscall.UTF16PtrFromString(vmID)
	if err != nil {
		return err
	}
	pathP, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procHcsGrantVMAccess.Addr(),
		uintptr(unsafe.Pointer(idP)), uintptr(unsafe.Pointer(pathP)))
	return hresultErr(r0)
}
