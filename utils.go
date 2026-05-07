package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func CheckSignal(tcpClients []*TCPClient, tcpServer *TCPServer, flexClient *FlexClient, flexRepo *FlexDXClusterRepository, contactRepo LogbookProvider) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)

	<-sigchan

	GracefulShutdown(tcpClients, tcpServer, flexClient, flexRepo, contactRepo)
	os.Exit(0)
}

func SendUDPMessage(data []byte) {
	conn, err := net.Dial("udp", "127.0.0.1:2241")
	if err != nil {
		fmt.Printf("Some error %v", err)
		return
	}
	conn.Write(data)
	conn.Close()
}
