//go:build windows

package main

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// reuseAddrControl sets SO_REUSEADDR so multiple apps (MSHV, WSJT-X, us)
// can share the same UDP multicast port on Windows.
func reuseAddrControl(network, address string, c syscall.RawConn) error {
	return c.Control(func(fd uintptr) {
		windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1) //nolint
	})
}
