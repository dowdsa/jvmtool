//go:build !windows

package main

// showNativeError 非 Windows 平台无操作。
func showNativeError(title, msg string) {}
