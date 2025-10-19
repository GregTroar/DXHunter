package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func FreqMhztoHz(freq string) string {
	frequency, err := strconv.ParseFloat(freq, 64)
	if err != nil {
		log.Println("could not convert frequency string to int", err)
	}

	frequency = frequency / 1000

	return strconv.FormatFloat(frequency, 'f', 6, 64)
}

func FreqHztoMhz(freq string) string {
	frequency, err := strconv.ParseFloat(freq, 64)
	if err != nil {
		log.Println("could not convert frequency string to int", err)
	}

	frequency = frequency * 1000

	return strconv.FormatFloat(frequency, 'f', 6, 64)
}

func CheckSignal(tcpClient *TCPClient, tcpServer *TCPServer, flexClient *FlexClient, flexRepo *FlexDXClusterRepository, contactRepo *Log4OMContactsRepository) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)

	<-sigchan

	GracefulShutdown(tcpClient, tcpServer, flexClient, flexRepo, contactRepo)
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
