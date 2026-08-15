//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// showNativeError 在 Windows 上弹出 MessageBox 显示错误。
func showNativeError(title, msg string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(msg)
	proc.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10)
}
