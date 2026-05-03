//go:build !windows

package main

import (
	"syscall"
)

// reuseAddrControl sets SO_REUSEADDR so multiple apps can share the same
// UDP multicast port on Linux and macOS.
func reuseAddrControl(network, address string, c syscall.RawConn) error {
	return c.Control(func(fd uintptr) {
		syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1) //nolint
	})
}
