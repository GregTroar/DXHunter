//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// runTray on non-Windows is a simple signal waiter — no system tray available
// without platform-native CGO dependencies.
func runTray(
	tcpClients []*TCPClient,
	tcpServer *TCPServer,
	flexClient *FlexClient,
	flexRepo *FlexDXClusterRepository,
	contactRepo LogbookProvider,
) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	<-sigchan
	GracefulShutdown(tcpClients, tcpServer, flexClient, flexRepo, contactRepo)
}
