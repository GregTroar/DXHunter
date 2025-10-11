package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	err error
)

type TCPServer struct {
	Address     string
	Port        string
	Clients     map[net.Conn]bool
	Mutex       *sync.Mutex
	LogWriter   *bufio.Writer
	Reader      *bufio.Reader
	Writer      *bufio.Writer
	Conn        net.Conn
	Listener    net.Listener
	MsgChan     chan string
	CmdChan     chan string
	Log         *log.Logger
	Config      *Config
	MessageSent int
}

func NewTCPServer(address string, port string) *TCPServer {
	return &TCPServer{
		Address:     address,
		Port:        port,
		Clients:     make(map[net.Conn]bool),
		MsgChan:     make(chan string, 100),
		CmdChan:     make(chan string),
		Mutex:       new(sync.Mutex),
		MessageSent: 0,
	}
}

func (s *TCPServer) StartServer() {
	s.LogWriter = bufio.NewWriter(os.Stdout)
	s.Listener, err = net.Listen("tcp", Cfg.TelnetServer.Host+":"+Cfg.TelnetServer.Port)
	if err != nil {
		Log.Info("Could not create telnet server")
	}

	defer s.Listener.Close()

	Log.Infof("Telnet server listening on %s:%s", Cfg.TelnetServer.Host, Cfg.TelnetServer.Port)

	go func() {
		for message := range s.MsgChan {
			s.broadcastMessage(message)
		}
	}()

	for {
		s.Conn, err = s.Listener.Accept()
		Log.Info("Client connected: ", s.Conn.RemoteAddr().String())
		if err != nil {
			Log.Error("Could not accept connections to telnet server")
			continue
		}
		s.Mutex.Lock()
		s.Clients[s.Conn] = true
		s.Mutex.Unlock()

		go s.handleConnection()
	}
}

func (s *TCPServer) handleConnection() {
	s.Conn.Write([]byte("Welcome to the FlexDXCluster telnet server! Type 'bye' to exit.\n"))

	s.Reader = bufio.NewReader(s.Conn)
	s.Writer = bufio.NewWriter(s.Conn)

	for {

		message, err := s.Reader.ReadString('\n')
		if err != nil {
			s.Mutex.Lock()
			delete(s.Clients, s.Conn)
			s.Mutex.Unlock()
			return
		}

		message = strings.TrimSpace(message)

		// if message is by then disconnect
		if message == "bye" {
			s.Mutex.Lock()
			delete(s.Clients, s.Conn)
			s.Mutex.Unlock()
			s.Conn.Close()
			Log.Infof("client %s disconnected", s.Conn.RemoteAddr().String())
		}

		if strings.Contains(message, "DX") || strings.Contains(message, "SH/DX") || strings.Contains(message, "set") || strings.Contains(message, "SET") {
			// send DX spot to the client
			s.CmdChan <- message
		}

	}
}

func (s *TCPServer) Write(message string) (n int, err error) {
	_, err = s.Writer.Write([]byte(message))
	if err == nil {
		err = s.Writer.Flush()
	}
	return
}

func (s *TCPServer) broadcastMessage(message string) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if len(s.Clients) > 0 {
		if s.MessageSent == 0 {
			time.Sleep(3 * time.Second)
			s.MessageSent += 1
		}
		for client := range s.Clients {
			_, err := client.Write([]byte(message + "\r\n"))
			s.MessageSent += 1
			if err != nil {
				fmt.Println("Error while sending message to clients:", client.RemoteAddr())
			}
		}
	}
}
