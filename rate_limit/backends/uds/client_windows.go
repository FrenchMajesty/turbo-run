//go:build windows

package uds

import "syscall"

// getSysProcAttr returns platform-specific process attributes for daemonizing
func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | syscall.DETACHED_PROCESS,
	}
}
