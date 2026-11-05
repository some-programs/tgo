//go:build !unix

package main

import "os/exec"

// setupInterrupt keeps the default behaviour of killing cmd when its context
// is cancelled.
func setupInterrupt(cmd *exec.Cmd) {}
