//go:build windows

package manager

// processAlive reports whether a process with the given PID exists.
// On Windows, Signal(0) is not supported; we rely on the time-based
// stale lock check (lockStaleThreshold) instead. Always returns true
// to defer to the time-based check.
func processAlive(_ int) bool { return true }
