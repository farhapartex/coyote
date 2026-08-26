//go:build !unix

package main

import (
	"os"
	"syscall"
)

func ownProcessGroup() *syscall.SysProcAttr {
	return nil
}

func signalProcessTree(process *os.Process, received os.Signal) {
	if process == nil {
		return
	}
	_ = process.Signal(received)
}
