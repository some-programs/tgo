//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// setupInterrupt makes cancelling cmd's context kill go test and everything
// it started. go test ignores a signal sent only to itself and waits for the
// running test binary, so cmd runs in its own process group and the whole
// group is killed.
func setupInterrupt(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
