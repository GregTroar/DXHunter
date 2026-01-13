package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

var CommandNumber int = 1

type FlexSpot struct {
	ID              int
	CommandNumber   int
	FlexSpotNumber  int
	DX              string
	FrequencyMhz    string
	FrequencyHz     string
	Band            string
	Mode            string
	FlexMode        string
	Source          string
	SpotterCallsign string
	TimeStamp       int64
	UTCTime         string
	LifeTime        string
	Priority        string
	OriginalComment string
	Comment         string
	Color           string
	BackgroundColor string
	NewDXCC         bool
	NewBand         bool
	NewMode         bool
	NewSlot         bool
	Worked          bool
	InWatchlist     bool
	CountryName     string
	DXCC            string
}

type Discovery struct {
	IP       string
	NickName string
	Model    string
	Serial   string
	Version  string
}

type FlexClient struct {
	Address              string
	Port                 string
	Timeout              time.Duration
	LogWriter            *bufio.Writer
	Reader               *bufio.Reader
	Writer               *bufio.Writer
	Conn                 *net.TCPConn
	MsgChan              chan string
	Repo                 FlexDXClusterRepository
	TCPServer            *TCPServer
	HTTPServer           *HTTPServer
	IsConnected          bool
	ctx                  context.Context
	cancel               context.CancelFunc
	reconnectAttempts    int
	maxReconnectAttempts int
	reconnectDelay       time.Duration
	Enabled              bool
	CurrentFrequency     float64 // ✅ NOUVEAU : Fréquence actuelle en MHz
	CurrentBand          string  // ✅ NOUVEAU : Bande actuelle (20M, 40M, etc.)
	PanID                string
}

func NewFlexClient(repo FlexDXClusterRepository, TCPServer *TCPServer, SpotChanToFlex chan TelnetSpot, httpServer *HTTPServer) *FlexClient {
	ctx, cancel := context.WithCancel(context.Background())

	enabled := Cfg.General.FlexRadioSpot && (Cfg.Flex.IP != "" || Cfg.Flex.Discover)

	return &FlexClient{
		Port:                 "4992",
		MsgChan:              TCPServer.MsgChan,
		Repo:                 repo,
		TCPServer:            TCPServer,
		HTTPServer:           httpServer,
		IsConnected:          false,
		Enabled:              enabled,
		ctx:                  ctx,
		cancel:               cancel,
		maxReconnectAttempts: -1,               // -1 = infini
		reconnectDelay:       30 * time.Second, // Délai initial
	}
}

func (fc *FlexClient) resolveAddress() (string, error) {
	if Cfg.Flex.IP == "" && !Cfg.Flex.Discover {
		return "", fmt.Errorf("you must either turn FlexRadio Discovery on or provide an IP address")
	}

	if Cfg.Flex.Discover {
		Log.Info("Attempting FlexRadio discovery...")

		discoveryDone := make(chan struct {
			success   bool
			discovery *Discovery
		}, 1)

		go func() {
			ok, d := DiscoverFlexRadio()
			discoveryDone <- struct {
				success   bool
				discovery *Discovery
			}{ok, d}
		}()

		select {
		case result := <-discoveryDone:
			if result.success {
				Log.Infof("Found: %s with Nick: %s, Version: %s, Serial: %s - using IP: %s",
					result.discovery.Model, result.discovery.NickName, result.discovery.Version,
					result.discovery.Serial, result.discovery.IP)
				return result.discovery.IP, nil
			}
		case <-time.After(10 * time.Second):
			Log.Warn("Discovery timeout after 10 seconds")
		}

		if Cfg.Flex.IP == "" {
			return "", fmt.Errorf("could not discover any FlexRadio on the network and no IP provided")
		}

		Log.Warn("Discovery failed, using configured IP")
	}

	return Cfg.Flex.IP, nil
}

func (fc *FlexClient) connect() error {
	ip, err := fc.resolveAddress()
	if err != nil {
		return err
	}
	fc.Address = ip

	addr, err := net.ResolveTCPAddr("tcp", fc.Address+":"+fc.Port)
	if err != nil {
		return fmt.Errorf("cannot resolve address %s:%s: %w", fc.Address, fc.Port, err)
	}

	Log.Infof("Attempting to connect to FlexRadio at %s:%s (attempt %d)",
		fc.Address, fc.Port, fc.reconnectAttempts+1)

	conn, err := net.DialTimeout("tcp", addr.String(), 10*time.Second)
	if err != nil {
		return fmt.Errorf("could not connect to FlexRadio: %w", err)
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		conn.Close()
		return fmt.Errorf("connection is not a TCP connection")
	}

	fc.Conn = tcpConn
	fc.Reader = bufio.NewReader(fc.Conn)
	fc.Writer = bufio.NewWriter(fc.Conn)
	fc.IsConnected = true
	fc.reconnectAttempts = 0

	if err := fc.Conn.SetKeepAlive(true); err != nil {
		Log.Warn("Could not set keep alive:", err)
	}

	Log.Infof("✅ Connected to FlexRadio at %s:%s", fc.Address, fc.Port)

	return nil
}

func (fc *FlexClient) StartFlexClient() {
	fc.LogWriter = bufio.NewWriter(os.Stdout)
	fc.Timeout = 600 * time.Second

	if !fc.Enabled {
		Log.Info("FlexRadio integration disabled in config - skipping")
		return
	}

	for {
		select {
		case <-fc.ctx.Done():
			Log.Info("Flex Client shutting down...")
			return
		default:
		}

		// Tentative de connexion
		err := fc.connect()
		if err != nil {
			fc.IsConnected = false
			fc.reconnectAttempts++

			// Message moins alarmiste
			if fc.reconnectAttempts == 1 {
				Log.Warnf("FlexRadio not available: %v", err)
				Log.Info("FlexDXCluster will continue without FlexRadio and retry connection periodically")
			} else {
				Log.Infof("FlexRadio still not available. Next retry in %v", fc.reconnectDelay)
			}

			// Attendre avant de réessayer
			select {
			case <-fc.ctx.Done():
				return
			case <-time.After(fc.reconnectDelay):
				continue
			}
		}

		// Connexion réussie, initialiser le Flex
		fc.initializeFlex()

		// Démarrer la lecture (bloquant jusqu'à déconnexion)
		fc.ReadLine()

		// Si ReadLine se termine (déconnexion), réessayer
		fc.IsConnected = false
		Log.Warn("FlexRadio connection lost. Will retry...")

		time.Sleep(5 * time.Second)
	}
}

func (fc *FlexClient) initializeFlex() {
	subSpotAllCmd := fmt.Sprintf("C%v|sub spot all", CommandNumber)
	fc.Write(subSpotAllCmd)
	CommandNumber++
	Log.Info("Subscribed to spots on FlexRadio")

	clrSpotAllCmd := fmt.Sprintf("C%v|spot clear", CommandNumber)
	fc.Write(clrSpotAllCmd)
	CommandNumber++
	Log.Info("Cleared all spots from panadapter")

	subSliceCmd := fmt.Sprintf("C%v|sub slice 0", CommandNumber)
	fc.Write(subSliceCmd)
	CommandNumber++
	Log.Info("Subscribed to slices on FlexRadio")

	subPanCmd := fmt.Sprintf("C%v|sub pan all", CommandNumber)
	fc.Write(subPanCmd)
	CommandNumber++
	Log.Debug("Subscribed to panadapters on FlexRadio")

}

func (fc *FlexClient) Close() {
	fc.cancel()
	if fc.Conn != nil {
		fc.Conn.Close()
	}
}

// ✅ SendSpot - Méthode simplifiée pour envoyer une commande au Flex
func (fc *FlexClient) SendSpot(stringSpot string) {
	if fc.IsConnected {
		fc.Write(stringSpot)
	}
}

func (fc *FlexClient) ReadLine() {
	defer func() {
		if fc.Conn != nil {
			fc.Conn.Close()
		}
	}()

	for {
		select {
		case <-fc.ctx.Done():
			return
		default:
		}

		// Timeout sur la lecture
		fc.Conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		message, err := fc.Reader.ReadString(byte('\n'))
		if err != nil {
			Log.Debugf("Error reading from FlexRadio: %s", err)
			return
		}
		fc.Conn.SetReadDeadline(time.Time{})

		// Parser les messages slice pour tracker la fréquence
		if err := fc.parseSliceMessage(message); err == nil {
		}

		// When spot is added on Flex Panadapter get the id of the spot to store it in Db
		regRespSpot := *regexp.MustCompile(`R(\d+)\|0\|(\d+)\n`)
		respSpot := regRespSpot.FindStringSubmatch(message)

		if len(respSpot) > 0 {
			spot, _ := fc.Repo.FindSpotByCommandNumber(respSpot[1])
			_, err := fc.Repo.UpdateFlexSpotNumberByID(respSpot[2], *spot)
			if err != nil {
				Log.Errorf("Could not update Flex spot number in database: %s", err)
			}
		}

		// Response when a spot is clicked
		regTriggerSpot := *regexp.MustCompile(`.*spot (\d+) triggered.*\n`)
		respTrigger := regTriggerSpot.FindStringSubmatch(message)

		if len(respTrigger) > 0 {
			spot, err := fc.Repo.FindSpotByFlexSpotNumber(respTrigger[1])
			Log.Debugf("Spot %s has been triggered", message)
			if err != nil {
				Log.Errorf("could not find spot by flex spot number in database: %s", err)
			}

			// Sending the callsign to Log4OM
			SendUDPMessage([]byte("<CALLSIGN>" + spot.DX))
		}

		// Status when a spot is deleted
		regSpotDeleted := *regexp.MustCompile(`S\d+\|spot (\d+) removed`)
		respDelete := regSpotDeleted.FindStringSubmatch(message)

		if len(respDelete) > 0 {
			// D'abord, récupérer le spot avant de le supprimer pour obtenir son ID
			spot, err := fc.Repo.FindSpotByFlexSpotNumber(respDelete[1])
			if err == nil && spot != nil && spot.ID > 0 {
				// Notifier le HTTPServer/Watchlist que ce spot est supprimé
				if fc.HTTPServer != nil && fc.HTTPServer.Watchlist != nil {
					fc.HTTPServer.Watchlist.RemoveActiveSpot(spot.ID)

					// Envoyer une mise à jour WebSocket
					fc.HTTPServer.broadcast <- WSMessage{
						Type: "watchlistUpdate",
						Data: fc.HTTPServer.Watchlist.GetAllWithActiveStatus(),
					}
				}
			}

			// Maintenant supprimer le spot de la base de données
			fc.Repo.DeleteSpotByFlexSpotNumber(respDelete[1])
			Log.Debugf("Spot deleted from Flex Panadapter and database: %s", message)
		}

		if strings.HasPrefix(message, "S") && strings.Contains(message, "display pan") {
			// Format: SCC1D0A5|display pan 0x40000000 client_handle=...
			regPan := regexp.MustCompile(`display pan (0x[0-9A-Fa-f]+)`)
			matchPan := regPan.FindStringSubmatch(message)

			if len(matchPan) > 1 {
				panID := matchPan[1]

				// Stocker le premier panadapter trouvé
				if fc.PanID == "" {
					fc.PanID = panID

					// Parser aussi le bandwidth actuel
					regBW := regexp.MustCompile(`bandwidth=([\d.]+)`)
					if matchBW := regBW.FindStringSubmatch(message); len(matchBW) > 1 {
						Log.Debugf("📡 Panadapter %s detected - Current BW: %s MHz",
							panID, matchBW[1])
					} else {
						Log.Debugf("📡 Panadapter %s detected", panID)
					}
				}
			}
		}

	}
}

func (fc *FlexClient) Write(data string) (n int, err error) {
	if fc.Conn == nil || fc.Writer == nil || !fc.IsConnected {
		return 0, fmt.Errorf("not connected to FlexRadio")
	}

	n, err = fc.Writer.Write([]byte(data + "\n"))
	if err == nil {
		err = fc.Writer.Flush()
	}
	return
}

// ✅ NOUVEAU : Parser les messages slice pour extraire la fréquence
func (fc *FlexClient) parseSliceMessage(message string) error {
	// Format: S<handle>|slice 0 RF_frequency=14.195000 mode=USB ...
	regSlice := regexp.MustCompile(`slice 0.*RF_frequency=([\d.]+)`)
	match := regSlice.FindStringSubmatch(message)

	if len(match) < 2 {
		return fmt.Errorf("not a slice message")
	}

	var frequency float64
	_, err := fmt.Sscanf(match[1], "%f", &frequency)
	if err != nil {
		return err
	}

	if frequency != fc.CurrentFrequency {
		fc.CurrentFrequency = frequency
		fc.CurrentBand = FrequencyToBand(frequency)
		if fc.CurrentBand == "N/A" {
			fc.CurrentBand = "ALL"
		}

		Log.Debugf("Flex frequency changed: %.6f MHz → Band: %s", frequency, fc.CurrentBand)

		// ✅ Broadcaster la nouvelle bande via WebSocket
		if fc.HTTPServer != nil {
			fc.HTTPServer.broadcast <- WSMessage{
				Type: "flexBandChange",
				Data: map[string]interface{}{
					"frequency": frequency,
					"band":      fc.CurrentBand,
				},
			}
		}
	}

	return nil
}

func DiscoverFlexRadio() (bool, *Discovery) {
	if Cfg.Flex.Discover {
		Log.Debugln("FlexRadio Discovery is turned on...searching for radio on the network")

		pc, err := net.ListenPacket("udp", ":4992")
		if err != nil {
			Log.Errorf("Could not receive UDP packets to discover FlexRadio: %v", err)
			return false, nil
		}
		defer pc.Close()

		pc.SetReadDeadline(time.Now().Add(10 * time.Second))

		buf := make([]byte, 1024)

		for {
			n, _, err := pc.ReadFrom(buf)
			if err != nil {
				// Timeout atteint
				return false, nil
			}

			discoverRe := regexp.MustCompile(`discovery_protocol_version=.*\smodel=(.*)\sserial=(.*)\sversion=(.*)\snickname=(.*)\scallsign=.*\sip=(.*)\sport=.*`)
			match := discoverRe.FindStringSubmatch(string(buf[:n]))

			if len(match) > 0 {
				d := &Discovery{
					NickName: match[4],
					Model:    match[1],
					Serial:   match[2],
					Version:  match[3],
					IP:       match[5],
				}
				return true, d
			}
		}
	} else {
		Log.Infoln("FlexRadio Discovery is turned off...using IP provided in the config file")
	}

	return false, nil
}

func (fc *FlexClient) ZoomPanadapter(mode string, freq string) error {
	if fc.PanID == "" {
		return nil
	}

	var bandwidth float64

	switch mode {
	case "USB", "LSB", "SSB", "AM", "FM", "DRM":
		bandwidth = 0.2
	case "CW", "CWR":
		bandwidth = 0.025
	case "RTTY", "DIGL", "DIGU":
		bandwidth = 0.0125
	case "FT8", "FT4", "PSK", "FSK":
		bandwidth = 0.05
	default:
		bandwidth = 0.2
	}

	cmd := fmt.Sprintf("C%v|display pan s %s bandwidth=%.6f", CommandNumber, fc.PanID, bandwidth)
	fc.Write(cmd)
	CommandNumber++
	cmdCenter := fmt.Sprintf("C%v|display pan s %s center=%v", CommandNumber, fc.PanID, freq)
	fc.Write(cmdCenter)
	CommandNumber++

	Log.Debugf("🔍 Auto-zoom: %s mode → %.4f MHz", mode, bandwidth)
	return nil
}

func (fc *FlexClient) AdjustAGC(mode string) {

	var AGCMode string

	switch mode {
	case "USB", "LSB", "SSB", "AM", "FM", "DRM":
		AGCMode = "slow"
	case "CW", "CWR":
		AGCMode = "fast"
	case "RTTY", "DIGL", "DIGU":
		AGCMode = "med"
	case "FT8", "FT4", "PSK", "FSK":
		AGCMode = "med"
	default:
		AGCMode = "slow"
	}

	cmd := fmt.Sprintf("C%v|slice s 0 agc_mode=%s", CommandNumber, AGCMode)
	fc.Write(cmd)
	CommandNumber++

	Log.Debugf("🔍 AGC Adjustment: %s mode → %s", mode, AGCMode)

}
