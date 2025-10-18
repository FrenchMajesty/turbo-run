//go:build unix

package uds

import "syscall"

// getSysProcAttr returns platform-specific process attributes for daemonizing
func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true, // Create new session to detach from parent
	}
}
