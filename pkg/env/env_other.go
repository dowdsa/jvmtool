//go:build !windows

package env

// SetUserEnvVar is a no-op on non-Windows platforms; environment variables are
// configured via the shell rc file (see ApplyBlock) instead.
func SetUserEnvVar(name, value string) error {
	return nil
}

// AddPath is a no-op on non-Windows platforms.
func AddPath(dir string) error {
	return nil
}
