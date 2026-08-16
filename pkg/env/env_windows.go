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
// broadcasts the change so new terminals pick it up. An empty value deletes
// the variable.
func SetUserEnvVar(name, value string) error {
	return setUserEnvVarTyped(name, value, regSz)
}

// setUserEnvVarTyped is like SetUserEnvVar but writes the given registry type
// (REG_SZ=1 or REG_EXPAND_SZ=2). The type matters for values containing
// %VAR% references, such as the user PATH.
func setUserEnvVarTyped(name, value string, regType uint32) error {
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
		uintptr(regType),
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

// AddPath adds a directory to the user PATH (deduplicated) and broadcasts the
// change so new terminals pick it up. The persistent user PATH is read from
// the registry only: if it is missing, the entry is added to an empty list
// (the merged process PATH is used only for deduplication, so machine-wide
// entries are never copied into the user scope). The original registry value
// type is preserved, and any value containing %VAR% is written as
// REG_EXPAND_SZ so the references keep expanding.
func AddPath(dir string) error {
	userPath, regType, _ := getUserEnvVarTyped("Path")

	entries := splitPath(userPath)
	// dedupe against the persistent user PATH and the merged process PATH
	for _, e := range append(append([]string{}, entries...), splitPath(os.Getenv("PATH"))...) {
		if strings.EqualFold(strings.TrimSpace(e), dir) {
			return nil
		}
	}

	entries = append([]string{dir}, entries...)
	joined := strings.Join(entries, ";")
	if regType != regExpandSz {
		regType = regSz
	}
	if strings.Contains(joined, "%") {
		regType = regExpandSz
	}
	return setUserEnvVarTyped("Path", joined, regType)
}

// RemovePathEntry removes a directory from the user PATH (deduplicated) and
// broadcasts the change if something was actually removed.
func RemovePathEntry(dir string) error {
	userPath, regType, _ := getUserEnvVarTyped("Path")
	entries := splitPath(userPath)
	var kept []string
	removed := false
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e), dir) {
			removed = true
			continue
		}
		kept = append(kept, e)
	}
	if !removed {
		return nil
	}
	joined := strings.Join(kept, ";")
	if regType != regExpandSz && strings.Contains(joined, "%") {
		regType = regExpandSz
	}
	return setUserEnvVarTyped("Path", joined, regType)
}

// splitPath splits a ";"-joined PATH value dropping empty elements.
func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	var out []string
	for _, e := range strings.Split(p, ";") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// GetUserEnvVar reads a user-level environment variable from the registry.
func GetUserEnvVar(name string) (string, error) {
	v, _, err := getUserEnvVarTyped(name)
	return v, err
}

// getUserEnvVarTyped reads a user-level env value together with its registry
// type (REG_SZ / REG_EXPAND_SZ). A missing value returns ("", 0, nil).
func getUserEnvVarTyped(name string) (string, uint32, error) {
	k, err := syscall.UTF16PtrFromString(`Environment`)
	if err != nil {
		return "", 0, err
	}
	var hKey syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		keyCurrentUser, uintptr(unsafe.Pointer(k)), 0, keyQueryValue, uintptr(unsafe.Pointer(&hKey)))
	if r != 0 {
		return "", 0, fmt.Errorf("RegOpenKeyExW failed: %d", r)
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	vn, _ := syscall.UTF16PtrFromString(name)
	buf := make([]uint16, 32768)
	size := uint32(len(buf) * 2)
	var regType uint32
	r, _, _ = procRegQueryValueExW.Call(
		uintptr(hKey), uintptr(unsafe.Pointer(vn)), 0, uintptr(unsafe.Pointer(&regType)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		if r == 2 { // ERROR_FILE_NOT_FOUND
			return "", 0, nil
		}
		return "", 0, fmt.Errorf("RegQueryValueExW failed: %d", r)
	}
	return syscall.UTF16ToString(buf[:size/2]), regType, nil
}
