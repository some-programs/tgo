//go:build unix

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// killDelay is how long go test gets to report after an interrupt before its
// process group is killed.
const killDelay = 5 * time.Second

// setupInterrupt makes cancelling cmd's context behave like Ctrl-C in a
// terminal. go test ignores an interrupt sent only to itself and waits for
// the running test binary, so cmd runs in its own process group and the
// interrupt is sent to the whole group. That lets go test report the
// interrupted tests. The group is killed if it is still running after
// killDelay or when a second interrupt arrives.
//
// The returned function must be called after cmd.Wait returns.
func setupInterrupt(cmd *exec.Cmd) (done func()) {
	exited := make(chan struct{})
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid
		again := make(chan os.Signal, 1)
		signal.Notify(again, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		go func() {
			defer signal.Stop(again)
			select {
			case <-exited:
				return
			case <-again:
			case <-time.After(killDelay):
			}
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}()
		return syscall.Kill(-pgid, syscall.SIGINT)
	}
	// Backstop in case something still holds stdout after the group is killed.
	cmd.WaitDelay = killDelay + time.Second
	return func() { close(exited) }
}
