//go:build !windows

package main

import "errors"

func autoStartEnabled() bool { return false }

func setAutoStart(enabled bool) error {
	if enabled {
		return errors.New("开机启动目前仅支持 Windows")
	}
	return nil
}
