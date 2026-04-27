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
	ConsoleChan          chan string
	Log                  *log.Logger
	Config               *Config
	ClusterCfg           ClusterConfig
	LoginRe              *regexp.Regexp
	PasswordRe           *regexp.Regexp
	ClusterType          string
	ContactRepo          LogbookProvider
	ctx                  context.Context
	cancel               context.CancelFunc
	reconnectAttempts    int
	maxReconnectAttempts int
	baseReconnectDelay   time.Duration
	maxReconnectDelay    time.Duration
}

func NewTCPClient(TCPServer *TCPServer, clusterCfg ClusterConfig, contactRepo LogbookProvider, spotChanToHTTPServer chan TelnetSpot, consoleChan chan string) *TCPClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &TCPClient{
		Address:              clusterCfg.Server,
		Port:                 clusterCfg.Port,
		Login:                clusterCfg.Login,
		Password:             clusterCfg.Password,
		MsgChan:              TCPServer.MsgChan,
		CmdChan:              make(chan string, 100), // CmdChan propre à ce client
		SpotChanToHTTPServer: spotChanToHTTPServer,
		ConsoleChan:          consoleChan,
		ClusterType:          clusterCfg.Type, // Depuis config si défini, sinon auto-détection
		SpotChanToFlex:       make(chan TelnetSpot, SpotChannelBuffer),
		TCPServer:            *TCPServer,
		ClusterCfg:           clusterCfg,
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
			case message := <-c.CmdChan:
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
	Log.Infof("[%s] Applying filters: FT8=%v FT4=%v Skimmer=%v Beacon=%v (type: %s)",
		c.ClusterCfg.Name, c.ClusterCfg.FT8, c.ClusterCfg.FT4, c.ClusterCfg.Skimmer, c.ClusterCfg.Beacon, c.ClusterType)

	switch c.ClusterType {
	case "dxspider":
		c.setFiltersDXSpider()
	case "ar_cluster":
		c.setFiltersARCluster()
	default: // cc_cluster et unknown
		c.setFiltersCCCluster()
	}
}

func (c *TCPClient) setFiltersCCCluster() {
	if c.ClusterCfg.FT8 {
		c.Write([]byte("set/ft8\r\n"))
		Log.Info("[CC] FT8: On")
	} else {
		c.Write([]byte("set/noft8\r\n"))
		Log.Info("[CC] FT8: Off")
	}
	if c.ClusterCfg.FT4 {
		c.Write([]byte("set/ft4\r\n"))
		Log.Info("[CC] FT4: On")
	} else {
		c.Write([]byte("set/noft4\r\n"))
		Log.Info("[CC] FT4: Off")
	}
	if c.ClusterCfg.Skimmer {
		c.Write([]byte("set/skimmer\r\n"))
		Log.Info("[CC] Skimmer: On")
	} else {
		c.Write([]byte("set/noskimmer\r\n"))
		Log.Info("[CC] Skimmer: Off")
	}
	if c.ClusterCfg.Beacon {
		c.Write([]byte("set/beacon\r\n"))
		Log.Info("[CC] Beacon: On")
	} else {
		c.Write([]byte("set/nobeacon\r\n"))
		Log.Info("[CC] Beacon: Off")
	}
}

func (c *TCPClient) setFiltersDXSpider() {
	// DX Spider utilise SET/SKIMMER avec des arguments et SET/NOSKIMMER
	if c.ClusterCfg.Skimmer && c.ClusterCfg.FT8 && c.ClusterCfg.FT4 {
		c.Write([]byte("SET/SKIMMER CW FT8 FT4\r\n"))
		Log.Info("[DXSpider] Skimmer+FT8+FT4: On")
	} else if c.ClusterCfg.Skimmer && c.ClusterCfg.FT8 {
		c.Write([]byte("SET/SKIMMER CW FT8\r\n"))
		Log.Info("[DXSpider] Skimmer+FT8: On")
	} else if c.ClusterCfg.Skimmer {
		c.Write([]byte("SET/SKIMMER CW\r\n"))
		Log.Info("[DXSpider] Skimmer CW: On")
	} else {
		c.Write([]byte("UNSET/SKIMMER\r\n"))
		Log.Info("[DXSpider] Skimmer: Off")
	}
	// DX Spider n'a pas de commandes set/noft8 séparées — géré via SET/SKIMMER
}

func (c *TCPClient) setFiltersARCluster() {
	// AR Cluster — commandes similaires à CC Cluster pour les filtres de base
	if c.ClusterCfg.FT8 {
		c.Write([]byte("set/ft8\r\n"))
	} else {
		c.Write([]byte("set/noft8\r\n"))
	}
	if c.ClusterCfg.FT4 {
		c.Write([]byte("set/ft4\r\n"))
	} else {
		c.Write([]byte("set/noft4\r\n"))
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
			c.Conn.SetReadDeadline(time.Time{})

			messageString := string(message)

			// ═══════════════════════════════════════════════════════════
			// Détecter le type de cluster AVANT le login aussi
			// ═══════════════════════════════════════════════════════════
			if c.ClusterType == "" || c.ClusterType == "unknown" {
				c.detectClusterType(messageString)
			}

			// Envoyer à la console (même avant login)
			select {
			case c.ConsoleChan <- messageString:
			default:
			}

			if strings.Contains(messageString, c.ClusterCfg.LoginPrompt) || strings.Contains(messageString, "login:") {
				time.Sleep(time.Second * 1)
				Log.Debug("Found login prompt...sending callsign")
				c.Write([]byte(c.Login + "\n\r"))
				c.LoggedIn = true
				Log.Infof("Connected to DX cluster %s:%s", c.ClusterCfg.Server, c.ClusterCfg.Port)
				// Envoyer les filtres 3 secondes après le login, peu importe le message de bienvenue
				go func() {
					time.Sleep(3 * time.Second)
					c.SetFilters()
				}()
				continue
			}
		}

		if c.LoggedIn {
			select {
			case <-c.ctx.Done():
				return
			default:
			}

			c.Conn.SetReadDeadline(time.Now().Add(ReadTimeout))
			message, err := c.Reader.ReadBytes('\n')
			trimmed := strings.TrimSpace(string(message))
			if trimmed != "" {
				Log.Debugf(trimmed)
			}
			if err != nil {
				Log.Errorf("Error reading message: %s", err)
				return
			}
			c.Conn.SetReadDeadline(time.Time{})

			messageString := string(message)

			if messageString != "" {
				// Handle password prompt
				if strings.Contains(messageString, "password") {
					Log.Debug("Found password prompt...sending password...")
					c.Write([]byte(c.Password + "\r\n"))
				}

				// Handle welcome message — envoyer la commande cluster si configurée
				if strings.Contains(messageString, "Hello") || strings.Contains(messageString, "Welcome") {
					if c.ClusterCfg.Command != "" {
						c.Write([]byte(c.ClusterCfg.Command + "\n\r"))
						Log.Debugf("Sending Command: %s", c.ClusterCfg.Command)
					}
				}

				// ═══════════════════════════════════════════════════════════
				// Détecter le type de cluster sur TOUTES les lignes
				// (seulement si pas encore détecté)
				// ═══════════════════════════════════════════════════════════
				if c.ClusterType == "" || c.ClusterType == "unknown" {
					c.detectClusterType(messageString)
				}

				// Check if it's a DX spot
				isDXSpot := strings.Contains(messageString, "DX de ") || shortSpotDetectRe.MatchString(messageString)

				if isDXSpot {
					// Filtre applicatif basé sur la config du cluster
					// (pour les clusters qui ne supportent pas set/noft8 etc.)
					msgUpper := strings.ToUpper(messageString)
					isFT8 := strings.Contains(msgUpper, "FT8")
					isFT4 := strings.Contains(msgUpper, "FT4")
					isSkimmer := strings.Contains(msgUpper, "CW SKIMMER") || strings.Contains(msgUpper, "SKIMMER")
					isBeacon := strings.Contains(msgUpper, "BEACON")

					skip := false
					if isFT8 && !c.ClusterCfg.FT8 {
						skip = true
					}
					if isFT4 && !c.ClusterCfg.FT4 {
						skip = true
					}
					if isSkimmer && !c.ClusterCfg.Skimmer {
						skip = true
					}
					if isBeacon && !c.ClusterCfg.Beacon {
						skip = true
					}

					if !skip {
						IncrementSpotsReceived()
						raw := messageString
						go ProcessTelnetSpot(spotRe, spotReShort, raw, c.SpotChanToFlex, c.SpotChanToHTTPServer, c.ContactRepo, c.ClusterCfg.Name)
					}
				}

				// Envoyer à la console uniquement les messages non-spot.
				// Les spots remplissaient le buffer (100 items) et faisaient dropper
				// les réponses aux commandes tapées par l'utilisateur.
				if !isDXSpot {
					consoleMsg := messageString
					if strings.HasPrefix(strings.TrimSpace(messageString), "To ALL de ") {
						consoleMsg = "TO_ALL:" + strings.TrimSpace(messageString)
					}
					select {
					case c.ConsoleChan <- consoleMsg:
					default:
					}
				}

				// Send to TCP server (non-blocking: never stall the TCP reader)
				select {
				case c.MsgChan <- messageString:
				case <-c.ctx.Done():
					return
				default:
				}
			}
		}
	}
}

func (c *TCPClient) detectClusterType(line string) {
	// Si le type est forcé dans la config, ne pas auto-détecter
	if c.ClusterCfg.Type != "" {
		c.ClusterType = c.ClusterCfg.Type
		return
	}

	lineLower := strings.ToLower(line)

	switch {
	case strings.Contains(lineLower, "dxspider"):
		c.ClusterType = "dxspider"
		Log.Infof("[%s] ✅ Detected cluster type: DX Spider", c.ClusterCfg.Name)
	case strings.Contains(lineLower, "cc cluster") || strings.Contains(lineLower, "cc-cluster") || strings.Contains(lineLower, "cccmds"):
		c.ClusterType = "cc_cluster"
		Log.Infof("[%s] ✅ Detected cluster type: CC Cluster", c.ClusterCfg.Name)
	case strings.Contains(lineLower, "ar-cluster") || strings.Contains(lineLower, "ar cluster") || strings.Contains(lineLower, "arcluster"):
		c.ClusterType = "ar_cluster"
		Log.Infof("[%s] ✅ Detected cluster type: AR Cluster", c.ClusterCfg.Name)
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
