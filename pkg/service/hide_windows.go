//go:build windows

package service

import (
	"os/exec"
	"syscall"
)

func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = 0x08000000 | 0x00000200 // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
	cmd.SysProcAttr.HideWindow = true
}
