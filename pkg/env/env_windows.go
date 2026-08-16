//go:build windows

package env

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	advapi32                = syscall.NewLazyDLL("advapi32.dll")
	user32                  = syscall.NewLazyDLL("user32.dll")
	procRegSetValueExW      = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW     = advapi32.NewProc("RegDeleteValueW")
	procRegOpenKeyExW       = advapi32.NewProc("RegOpenKeyExW")
	procRegCloseKey         = advapi32.NewProc("RegCloseKey")
	procRegQueryValueExW    = advapi32.NewProc("RegQueryValueExW")
	procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
)

const (
	hwndBroadcast   = uintptr(0xffff)
	wmSettingChange = 0x001a
	smtoAbortIfHung = 0x0002
	keyCurrentUser  = 0x80000001 // HKEY_CURRENT_USER
	keySetValue     = 0x0002
	keyQueryValue   = 0x0001
	regSz           = 1
	regExpandSz     = 2
)

// SetUserEnvVar persists an environment variable for the current user and
// broadcasts the change so new terminals pick it up.
func SetUserEnvVar(name, value string) error {
	k, err := syscall.UTF16PtrFromString(`Environment`)
	if err != nil {
		return err
	}
	var hKey syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		keyCurrentUser, uintptr(unsafe.Pointer(k)), 0, keySetValue, uintptr(unsafe.Pointer(&hKey)))
	if r != 0 {
		return fmt.Errorf("RegOpenKeyExW failed: %d", r)
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	if value == "" {
		// delete the value
		vn, _ := syscall.UTF16PtrFromString(name)
		r, _, _ = procRegDeleteValueW.Call(uintptr(hKey), uintptr(unsafe.Pointer(vn)))
		if r != 0 && r != 2 { // 2 = ERROR_FILE_NOT_FOUND
			return fmt.Errorf("RegDeleteValueW failed: %d", r)
		}
		broadcastSettingChange()
		return nil
	}

	vn, _ := syscall.UTF16PtrFromString(name)
	vv, _ := syscall.UTF16FromString(value)
	r, _, _ = procRegSetValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(vn)),
		0,
		regSz,
		uintptr(unsafe.Pointer(&vv[0])),
		uintptr(len(vv)*2),
	)
	if r != 0 {
		return fmt.Errorf("RegSetValueExW failed: %d", r)
	}
	broadcastSettingChange()
	return nil
}

// broadcastSettingChange notifies the system that environment variables changed.
func broadcastSettingChange() {
	env, _ := syscall.UTF16PtrFromString("Environment")
	procSendMessageTimeoutW.Call(
		hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(env)),
		smtoAbortIfHung, 5000, 0)
}

// AddPath adds a directory to the user PATH (deduplicated), and broadcasts the
// change so new terminals pick it up.
func AddPath(dir string) error {
	current := os.Getenv("PATH")
	// read the persistent user PATH from the registry, not the process env
	userPath, err := GetUserEnvVar("Path")
	if err != nil || userPath == "" {
		userPath = current
	}
	entries := strings.Split(userPath, ";")
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e), dir) {
			return nil // already present
		}
	}
	entries = append([]string{dir}, entries...)
	return SetUserEnvVar("Path", strings.Join(entries, ";"))
}

// GetUserEnvVar reads a user-level environment variable from the registry.
func GetUserEnvVar(name string) (string, error) {
	k, err := syscall.UTF16PtrFromString(`Environment`)
	if err != nil {
		return "", err
	}
	var hKey syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		keyCurrentUser, uintptr(unsafe.Pointer(k)), 0, keyQueryValue, uintptr(unsafe.Pointer(&hKey)))
	if r != 0 {
		return "", fmt.Errorf("RegOpenKeyExW failed: %d", r)
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	vn, _ := syscall.UTF16PtrFromString(name)
	buf := make([]uint16, 32768)
	size := uint32(len(buf) * 2)
	r, _, _ = procRegQueryValueExW.Call(
		uintptr(hKey), uintptr(unsafe.Pointer(vn)), 0, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		if r == 2 { // ERROR_FILE_NOT_FOUND
			return "", nil
		}
		return "", fmt.Errorf("RegQueryValueExW failed: %d", r)
	}
	return syscall.UTF16ToString(buf[:size/2]), nil
}
