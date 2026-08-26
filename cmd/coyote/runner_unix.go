//go:build unix

package main

import (
	"os"
	"syscall"
)

func ownProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func signalProcessTree(process *os.Process, received os.Signal) {
	if process == nil {
		return
	}
	if signal, ok := received.(syscall.Signal); ok {
		if err := syscall.Kill(-process.Pid, signal); err == nil {
			return
		}
	}
	_ = process.Signal(received)
}
