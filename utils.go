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

func CheckSignal(TCPClient *TCPClient, TCPServer *TCPServer, FlexClient *FlexClient, fRepo *FlexDXClusterRepository, cRepo *Log4OMContactsRepository) {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	// Gracely closing all connextions if signal is received
	for sig := range sigCh {
		Log.Infof("received signal: %v, shutting down all connections.", sig)

		TCPClient.Close()
		TCPServer.Conn.Close()
		FlexClient.Conn.Close()

		if err := fRepo.db.Close(); err != nil {
			Log.Error("failed to close the database connection properly")
			os.Exit(1)
		}

		if err := cRepo.db.Close(); err != nil {
			Log.Error("failed to close Log4OM database connection properly")
			os.Exit(1)
		}

		os.Exit(0)
	}
}

func SendUDPMessage(message string) {
	conn, err := net.Dial("udp", "127.0.0.1:2241")
	if err != nil {
		fmt.Printf("Some error %v", err)
		return
	}
	conn.Write([]byte(message))
	conn.Close()
}
