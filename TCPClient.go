package main

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var spotRe *regexp.Regexp = regexp.MustCompile(`(?i)DX\sde\s([\w\d\/]+?)(?:-[#\d-]+)?\s*:\s*(\d+\.\d+)\s+([\w\d\/]+)\s+(?:(CW|SSB|FT8|FT4|RTTY|USB|LSB|FM)\s+)?(.+?)\s+(\d{4}Z)`)
var spotReShort *regexp.Regexp = regexp.MustCompile(`^(\d+\.\d+)\s+([\w\d\/]+)\s+\d{2}-\w{3}-\d{4}\s+(\d{4}Z)\s+(.+?)\s*<([\w\d\/]+)>\s*$`)
var shortSpotDetectRe *regexp.Regexp = regexp.MustCompile(`^\d+\.\d+\s+[\w\d\/]+\s+\d{2}-\w{3}-\d{4}`)
var defaultLoginRe *regexp.Regexp = regexp.MustCompile("[\\w\\d-_]+ login:")
var defaultPasswordRe *regexp.Regexp = regexp.MustCompile("Password:")

const (
	// Reconnection settings
	MaxReconnectAttempts = 10
	BaseReconnectDelay   = 1 * time.Second
	MaxReconnectDelay    = 60 * time.Second

	// Timeout settings
	ConnectionTimeout = 10 * time.Second
	LoginTimeout      = 30 * time.Second
	ReadTimeout       = 5 * time.Minute

	// Channel buffer sizes
	SpotChannelBuffer = 100
)

type TCPClient struct {
	Login                string
	Password             string
	Address              string
	Port                 string
	LoggedIn             bool
	Timeout              time.Duration
	LogWriter            *bufio.Writer
	Reader               *bufio.Reader
	Writer               *bufio.Writer
	Scanner              *bufio.Scanner
	Mutex                sync.Mutex
	Conn                 net.Conn
	TCPServer            TCPServer
	MsgChan              chan string
	CmdChan              chan string
	SpotChanToFlex       chan TelnetSpot
	SpotChanToHTTPServer chan TelnetSpot
	Log                  *log.Logger
	Config               *Config
	Countries            Countries
	LoginRe              *regexp.Regexp
	PasswordRe           *regexp.Regexp
	ContactRepo          *Log4OMContactsRepository
	ctx                  context.Context
	cancel               context.CancelFunc
	reconnectAttempts    int
	maxReconnectAttempts int
	baseReconnectDelay   time.Duration
	maxReconnectDelay    time.Duration
}

func NewTCPClient(TCPServer *TCPServer, Countries Countries, contactRepo *Log4OMContactsRepository, spotChanToHTTPServer chan TelnetSpot) *TCPClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &TCPClient{
		Address:              Cfg.Cluster.Server,
		Port:                 Cfg.Cluster.Port,
		Login:                Cfg.Cluster.Login,
		Password:             Cfg.Cluster.Password,
		MsgChan:              TCPServer.MsgChan,
		CmdChan:              TCPServer.CmdChan,
		SpotChanToHTTPServer: spotChanToHTTPServer,
		SpotChanToFlex:       make(chan TelnetSpot, SpotChannelBuffer),
		TCPServer:            *TCPServer,
		Countries:            Countries,
		ContactRepo:          contactRepo,
		ctx:                  ctx,
		cancel:               cancel,
		maxReconnectAttempts: MaxReconnectAttempts,
		baseReconnectDelay:   BaseReconnectDelay,
		maxReconnectDelay:    MaxReconnectDelay,
	}
}

func (c *TCPClient) setDefaultParams() {
	if c.Timeout == 0 {
		c.Timeout = 600 * time.Second
	}
	if c.LogWriter == nil {
		c.LogWriter = bufio.NewWriter(os.Stdout)
	}
	c.LoggedIn = false

	if c.LoginRe == nil {
		c.LoginRe = defaultLoginRe
	}

	if c.PasswordRe == nil {
		c.PasswordRe = defaultPasswordRe
	}
}

func (c *TCPClient) ReloadFilters() {
	if c.LoggedIn {
		Log.Info("Reloading cluster filters...")
		c.SetFilters()
	}
}

func (c *TCPClient) calculateBackoff() time.Duration {
	// Formule: min(baseDelay * 2^attempts, maxDelay)
	delay := time.Duration(float64(c.baseReconnectDelay) * math.Pow(2, float64(c.reconnectAttempts)))

	if delay > c.maxReconnectDelay {
		delay = c.maxReconnectDelay
	}

	return delay
}

func (c *TCPClient) connect() error {
	addr := c.Address + ":" + c.Port

	Log.Debugf("Attempting to connect to %s (attempt %d/%d)", addr, c.reconnectAttempts+1, c.maxReconnectAttempts)

	conn, err := net.DialTimeout("tcp", addr, ConnectionTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.Conn = conn
	c.Reader = bufio.NewReader(c.Conn)
	c.Writer = bufio.NewWriter(c.Conn)
	c.LoggedIn = false
	c.reconnectAttempts = 0 // Reset sur connexion réussie

	return nil
}

func (c *TCPClient) StartClient() {
	c.setDefaultParams()

	// Goroutine pour gérer les commandes
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case message := <-c.TCPServer.CmdChan:
				Log.Infof("Received Command: %s", message)
				c.Write([]byte(message + "\r\n"))
			}
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			Log.Info("TCP Client shutting down...")
			return
		default:
		}

		// Tentative de connexion
		err := c.connect()
		if err != nil {
			c.reconnectAttempts++

			if c.reconnectAttempts >= c.maxReconnectAttempts {
				Log.Errorf("Max reconnection attempts (%d) reached. Giving up.", c.maxReconnectAttempts)
				return
			}

			backoff := c.calculateBackoff()
			Log.Warnf("Connection failed: %v. Retrying in %v...", err, backoff)

			// Attente avec possibilité d'annulation
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(backoff):
				continue
			}
		}

		// Connexion réussie, démarrer la lecture
		c.ReadLine()

		// Si ReadLine se termine (déconnexion), on va tenter de se reconnecter
		Log.Warn("Connection lost. Attempting to reconnect...")

		// Petit délai avant reconnexion
		time.Sleep(2 * time.Second)
	}
}

func (c *TCPClient) Close() {
	c.cancel() // Annule le contexte pour arrêter toutes les goroutines
	if c.Conn != nil {
		c.Writer.Write([]byte("bye\r\n"))
		c.Writer.Flush()
		c.Conn.Close()
	}
}

func (c *TCPClient) SetFilters() {
	if Cfg.Cluster.FT8 {
		c.Write([]byte("set/ft8\r\n"))
		Log.Info("FT8: On")
	}

	if Cfg.Cluster.Skimmer {
		c.Write([]byte("set/skimmer\r\n"))
		Log.Info("Skimmer: On")
	}

	if Cfg.Cluster.FT4 {
		c.Write([]byte("set/ft4\r\n"))
		Log.Info("FT4: On")
	}

	if Cfg.Cluster.Beacon {
		c.Write([]byte("set/beacon\r\n"))
		Log.Info("Beacon: On")
	}

	if !Cfg.Cluster.FT8 {
		c.Write([]byte("set/noft8\r\n"))
		Log.Info("FT8: Off")
	}

	if !Cfg.Cluster.FT4 {
		c.Write([]byte("set/noft4\r\n"))
		Log.Info("FT4: Off")
	}

	if !Cfg.Cluster.Skimmer {
		c.Write([]byte("set/noskimmer\r\n"))
		Log.Info("Skimmer: Off")
	}

	if !Cfg.Cluster.Beacon {
		c.Write([]byte("set/nobeacon\r\n"))
		Log.Info("Beacon: Off")
	}
}

func (c *TCPClient) ReadLine() {
	defer func() {
		if c.Conn != nil {
			c.Conn.Close()
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if !c.LoggedIn {
			// Lecture avec timeout
			c.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			message, err := c.Reader.ReadBytes(':')
			if err != nil {
				Log.Errorf("Error reading login prompt: %s", err)
				return
			}
			c.Conn.SetReadDeadline(time.Time{}) // Reset deadline

			if strings.Contains(string(message), Cfg.Cluster.LoginPrompt) || strings.Contains(string(message), "login:") {
				time.Sleep(time.Second * 1)
				Log.Debug("Found login prompt...sending callsign")
				c.Write([]byte(c.Login + "\n\r"))
				c.LoggedIn = true
				Log.Infof("Connected to DX cluster %s:%s", Cfg.Cluster.Server, Cfg.Cluster.Port)
				continue
			}
		}

		if c.LoggedIn {
			// Check for cancellation before reading
			select {
			case <-c.ctx.Done():
				return
			default:
			}

			// Lecture avec timeout pour détecter les connexions mortes
			c.Conn.SetReadDeadline(time.Now().Add(ReadTimeout))
			message, err := c.Reader.ReadBytes('\n')
			if err != nil {
				Log.Errorf("Error reading message: %s", err)
				return
			}
			c.Conn.SetReadDeadline(time.Time{}) // Reset deadline

			messageString := string(message)

			if messageString != "" {
				if strings.Contains(messageString, "password") {
					Log.Debug("Found password prompt...sending password...")
					c.Write([]byte(c.Password + "\r\n"))
				}

				if strings.Contains(messageString, "Hello") || strings.Contains(messageString, "Welcome") {
					go c.SetFilters()
					if Cfg.Cluster.Command != "" {
						c.Write([]byte(Cfg.Cluster.Command + "\n\r"))
						Log.Debugf("Sending Command: %s", Cfg.Cluster.Command)
					}
				}
				if strings.Contains(messageString, "DX") || shortSpotDetectRe.MatchString(messageString) {
					IncrementSpotsReceived()
					ProcessTelnetSpot(spotRe, spotReShort, messageString, c.SpotChanToFlex, c.SpotChanToHTTPServer, c.Countries, c.ContactRepo)
				}
				// Send the spot message to TCP server
				select {
				case c.MsgChan <- messageString:
				case <-c.ctx.Done():
					return
				}
			}
		}
	}
}

// Write sends raw data to remove telnet server
func (c *TCPClient) Write(data []byte) (n int, err error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	if c.Conn == nil || c.Writer == nil {
		return 0, fmt.Errorf("not connected")
	}

	n, err = c.Writer.Write(data)
	if err != nil {
		Log.Errorf("Error while sending command to telnet client: %s", err)
		return n, err
	}

	err = c.Writer.Flush()
	return n, err
}
