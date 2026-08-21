//go:build !windows

package manager

import "syscall"

// processAlive reports whether a process with the given PID exists.
// On Unix, sending signal 0 checks existence without affecting the process.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
