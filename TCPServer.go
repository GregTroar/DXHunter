package main

import (
	"bufio"
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
	Address   string
	Port      string
	Clients   map[net.Conn]*ClientInfo // ✅ Map avec structure ClientInfo
	Mutex     *sync.Mutex
	LogWriter *bufio.Writer
	Reader    *bufio.Reader
	Writer    *bufio.Writer
	Conn      net.Conn
	Listener  net.Listener
	MsgChan   chan string
	CmdChan   chan string
	Log       *log.Logger
	Config    *Config
}

// ✅ Structure pour stocker les infos client
type ClientInfo struct {
	ConnectedAt time.Time
}

func NewTCPServer(address string, port string) *TCPServer {
	return &TCPServer{
		Address: address,
		Port:    port,
		Clients: make(map[net.Conn]*ClientInfo),
		MsgChan: make(chan string, 2000), // Buffer large pour ne jamais bloquer la lecture telnet
		CmdChan: make(chan string, 50),
		Mutex:   new(sync.Mutex),
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
		s.Clients[s.Conn] = &ClientInfo{
			ConnectedAt: time.Now(), // ✅ Enregistre l'heure de connexion
		}
		s.Mutex.Unlock()

		go s.handleConnection()
	}
}

func (s *TCPServer) handleConnection() {
	conn := s.Conn

	conn.Write([]byte("Welcome to the FlexDXCluster telnet server! Type 'bye' to exit.\n"))

	reader := bufio.NewReader(conn)

	defer func() {
		s.Mutex.Lock()
		delete(s.Clients, conn)
		s.Mutex.Unlock()
		conn.Close()
		Log.Infof("Client %s disconnected", conn.RemoteAddr().String())
	}()

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			Log.Debugf("Error reading from client %s: %v", conn.RemoteAddr().String(), err)
			return
		}

		message = strings.TrimSpace(message)

		if message == "bye" {
			Log.Infof("Client %s sent bye command", conn.RemoteAddr().String())
			return
		}

		if strings.Contains(message, "DX") || strings.Contains(message, "SH/DX") ||
			strings.Contains(message, "set") || strings.Contains(message, "SET") {
			select {
			case s.CmdChan <- message:
				Log.Debugf("Command from client %s: %s", conn.RemoteAddr().String(), message)
			default:
				Log.Warn("Command channel is full, dropping command")
			}
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
	clients := make([]net.Conn, 0, len(s.Clients))
	infos := make(map[net.Conn]*ClientInfo)
	for c, info := range s.Clients {
		clients = append(clients, c)
		infos[c] = info
	}
	s.Mutex.Unlock()

	if len(clients) == 0 {
		return
	}

	var failedClients []net.Conn

	for _, client := range clients {
		// Skip les clients trop récents
		if time.Since(infos[client].ConnectedAt) < 5*time.Second {
			continue
		}

		// Deadline courte pour ne pas bloquer si le client est lent
		client.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
		_, err := client.Write([]byte(strings.TrimSpace(message) + "\r\n"))
		client.SetWriteDeadline(time.Time{})
		if err != nil {
			Log.Warnf("Error sending to client %s: %v", client.RemoteAddr(), err)
			failedClients = append(failedClients, client)
		}
	}

	if len(failedClients) > 0 {
		s.Mutex.Lock()
		for _, client := range failedClients {
			delete(s.Clients, client)
			client.Close()
		}
		s.Mutex.Unlock()
	}
}
