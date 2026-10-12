//go:build !windows

package service

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
