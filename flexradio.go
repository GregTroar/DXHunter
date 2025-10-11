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
	SpotChanToFlex       chan TelnetSpot
	MsgChan              chan string
	Repo                 FlexDXClusterRepository
	TCPServer            *TCPServer
	HTTPServer           *HTTPServer
	IsConnected          bool
	ctx                  context.Context
	cancel               context.CancelFunc
	reconnectAttempts    int
	maxReconnectAttempts int
	baseReconnectDelay   time.Duration
	maxReconnectDelay    time.Duration
	Enabled              bool
}

func NewFlexClient(repo FlexDXClusterRepository, TCPServer *TCPServer, SpotChanToFlex chan TelnetSpot, httpServer *HTTPServer) *FlexClient {
	ctx, cancel := context.WithCancel(context.Background())

	enabled := Cfg.General.FlexRadioSpot && (Cfg.Flex.IP != "" || Cfg.Flex.Discover)

	return &FlexClient{
		Port:                 "4992",
		SpotChanToFlex:       SpotChanToFlex,
		MsgChan:              TCPServer.MsgChan,
		Repo:                 repo,
		TCPServer:            TCPServer,
		IsConnected:          false,
		Enabled:              enabled,
		ctx:                  ctx,
		cancel:               cancel,
		maxReconnectAttempts: -1,              // -1 = infini
		baseReconnectDelay:   5 * time.Second, // Délai initial
		maxReconnectDelay:    5 * time.Minute, // Max 5 minutes
	}
}

func (fc *FlexClient) calculateBackoff() time.Duration {
	delay := time.Duration(float64(fc.baseReconnectDelay) * math.Pow(1.5, float64(fc.reconnectAttempts)))

	if delay > fc.maxReconnectDelay {
		delay = fc.maxReconnectDelay
	}

	return delay
}

func (fc *FlexClient) resolveAddress() (string, error) {
	if Cfg.Flex.IP == "" && !Cfg.Flex.Discover {
		return "", fmt.Errorf("you must either turn FlexRadio Discovery on or provide an IP address")
	}

	if Cfg.Flex.Discover {
		Log.Debug("Attempting FlexRadio discovery...")

		// Timeout sur la découverte (10 secondes max)
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

	Log.Debugf("Attempting to connect to FlexRadio at %s:%s (attempt %d)",
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

		// Consommer les spots pour éviter les blocages
		go func() {
			for {
				select {
				case <-fc.ctx.Done():
					return
				case <-fc.SpotChanToFlex:
					// Ignorer les spots
				}
			}
		}()
		return
	}

	// Goroutine pour envoyer les spots au Flex
	go func() {
		for {
			select {
			case <-fc.ctx.Done():
				return
			case spot := <-fc.SpotChanToFlex:
				fc.SendSpottoFlex(spot)
			}
		}
	}()

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

			backoff := fc.calculateBackoff()

			// Message moins alarmiste
			if fc.reconnectAttempts == 1 {
				Log.Warnf("FlexRadio not available: %v", err)
				Log.Info("FlexDXCluster will continue without FlexRadio and retry connection periodically")
			} else {
				Log.Debugf("FlexRadio still not available. Next retry in %v", backoff)
			}

			// Attendre avant de réessayer
			select {
			case <-fc.ctx.Done():
				return
			case <-time.After(backoff):
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

	clrSpotAllCmd := fmt.Sprintf("C%v|spot clear", CommandNumber)
	fc.Write(clrSpotAllCmd)
	CommandNumber++

	Log.Debug("Subscribed to spots on FlexRadio and cleared all spots from panadapter")
}

func (fc *FlexClient) Close() {
	fc.cancel()
	if fc.Conn != nil {
		fc.Conn.Close()
	}
}

func (fc *FlexClient) SendSpottoFlex(spot TelnetSpot) {

	freq := FreqMhztoHz(spot.Frequency)

	flexSpot := FlexSpot{
		CommandNumber:   CommandNumber,
		DX:              spot.DX,
		FrequencyMhz:    freq,
		FrequencyHz:     spot.Frequency,
		Band:            spot.Band,
		Mode:            spot.Mode,
		Source:          "FlexDXCluster",
		SpotterCallsign: spot.Spotter,
		TimeStamp:       time.Now().Unix(),
		UTCTime:         spot.Time,
		LifeTime:        Cfg.Flex.SpotLife,
		OriginalComment: spot.Comment,
		Comment:         spot.Comment,
		Color:           "#ffeaeaea",
		BackgroundColor: "#ff000000",
		Priority:        "5",
		NewDXCC:         spot.NewDXCC,
		NewBand:         spot.NewBand,
		NewMode:         spot.NewMode,
		NewSlot:         spot.NewSlot,
		Worked:          spot.CallsignWorked,
		InWatchlist:     false,
		CountryName:     spot.CountryName,
		DXCC:            spot.DXCC,
	}

	flexSpot.Comment = flexSpot.Comment + " [" + flexSpot.Mode + "] [" + flexSpot.SpotterCallsign + "] [" + flexSpot.UTCTime + "]"

	if fc.HTTPServer != nil && fc.HTTPServer.Watchlist != nil {
		if fc.HTTPServer.Watchlist.Matches(flexSpot.DX) {
			flexSpot.InWatchlist = true
			flexSpot.Comment = flexSpot.Comment + " [Watchlist]"
			Log.Infof("🎯 Watchlist match: %s", flexSpot.DX)
		}
	}

	// If new DXCC
	if spot.NewDXCC {
		flexSpot.Priority = "1"
		flexSpot.Comment = flexSpot.Comment + " [New DXCC]"
		if Cfg.General.SpotColorNewEntity != "" && Cfg.General.BackgroundColorNewEntity != "" {
			flexSpot.Color = Cfg.General.SpotColorNewEntity
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorNewEntity
		} else {
			flexSpot.Color = "#ff3bf908"
			flexSpot.BackgroundColor = "#ff000000"
		}
	} else if spot.DX == Cfg.General.Callsign {
		flexSpot.Priority = "1"
		if Cfg.General.SpotColorMyCallsign != "" && Cfg.General.BackgroundColorMyCallsign != "" {
			flexSpot.Color = Cfg.General.SpotColorMyCallsign
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorMyCallsign
		} else {
			flexSpot.Color = "#ffff0000"
			flexSpot.BackgroundColor = "#ff000000"
		}
	} else if spot.CallsignWorked {
		flexSpot.Priority = "5"
		flexSpot.Comment = flexSpot.Comment + " [Worked]"
		if Cfg.General.SpotColorWorked != "" && Cfg.General.BackgroundColorWorked != "" {
			flexSpot.Color = Cfg.General.SpotColorWorked
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorWorked
		} else {
			flexSpot.Color = "#ff000000"
			flexSpot.BackgroundColor = "#ff00c0c0"
		}
	} else if spot.NewMode && spot.NewBand {
		flexSpot.Priority = "1"
		flexSpot.Comment = flexSpot.Comment + " [New Band & Mode]"
		if Cfg.General.SpotColorNewBandMode != "" && Cfg.General.BackgroundColorNewBandMode != "" {
			flexSpot.Color = Cfg.General.SpotColorNewBandMode
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorNewBandMode
		} else {
			flexSpot.Color = "#ffc603fc"
			flexSpot.BackgroundColor = "#ff000000"
		}
	} else if spot.NewMode && !spot.NewBand {
		flexSpot.Priority = "2"
		flexSpot.Comment = flexSpot.Comment + " [New Mode]"
		if Cfg.General.SpotColorNewMode != "" && Cfg.General.BackgroundColorNewMode != "" {
			flexSpot.Color = Cfg.General.SpotColorNewMode
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorNewMode
		} else {
			flexSpot.Color = "#fff9a908"
			flexSpot.BackgroundColor = "#ff000000"
		}
	} else if spot.NewBand && !spot.NewMode {
		flexSpot.Color = "#fff9f508"
		flexSpot.Priority = "3"
		flexSpot.BackgroundColor = "#ff000000"
		flexSpot.Comment = flexSpot.Comment + " [New Band]"
	} else if !spot.NewBand && !spot.NewMode && !spot.NewDXCC && !spot.CallsignWorked && spot.NewSlot {
		flexSpot.Color = "#ff91d2ff"
		flexSpot.Priority = "5"
		flexSpot.BackgroundColor = "#ff000000"
		flexSpot.Comment = flexSpot.Comment + " [New Slot]"
	} else if !spot.NewBand && !spot.NewMode && !spot.NewDXCC && !spot.CallsignWorked {
		flexSpot.Color = "#ffeaeaea"
		flexSpot.Priority = "5"
		flexSpot.BackgroundColor = "#ff000000"
	} else {
		flexSpot.Color = "#ffeaeaea"
		flexSpot.Priority = "5"
		flexSpot.BackgroundColor = "#ff000000"
	}

	// Send notification to Gotify
	Gotify(flexSpot)

	flexSpot.Comment = strings.ReplaceAll(flexSpot.Comment, " ", "\u00A0")

	srcFlexSpot, err := fc.Repo.FindDXSameBand(flexSpot)
	if err != nil {
		Log.Debugf("Could not find the DX in the database: %v", err)
	}

	var stringSpot string

	if srcFlexSpot.DX == "" {
		fc.Repo.CreateSpot(flexSpot)
		CommandNumber++

		if fc.HTTPServer != nil {
			fc.HTTPServer.broadcast <- WSMessage{
				Type: "spots",
				Data: fc.Repo.GetAllSpots("1000"),
			}
		}

		if fc.IsConnected {
			stringSpot = fmt.Sprintf("C%v|spot add rx_freq=%v callsign=%s mode=%s source=%s spotter_callsign=%s timestamp=%v lifetime_seconds=%s comment=%s color=%s background_color=%s priority=%s",
				flexSpot.CommandNumber, flexSpot.FrequencyMhz,
				flexSpot.DX, flexSpot.Mode, flexSpot.Source, flexSpot.SpotterCallsign,
				flexSpot.TimeStamp, flexSpot.LifeTime, flexSpot.Comment, flexSpot.Color,
				flexSpot.BackgroundColor, flexSpot.Priority)
			CommandNumber++
			fc.SendSpot(stringSpot)
		}

	} else if srcFlexSpot.DX != "" && srcFlexSpot.Band == flexSpot.Band {
		fc.Repo.DeleteSpotByFlexSpotNumber(string(flexSpot.FlexSpotNumber))

		if fc.IsConnected {
			stringSpot = fmt.Sprintf("C%v|spot remove %v", flexSpot.CommandNumber, srcFlexSpot.FlexSpotNumber)
			fc.SendSpot(stringSpot)
			CommandNumber++
		}

		fc.Repo.CreateSpot(flexSpot)
		CommandNumber++

		if fc.IsConnected {
			stringSpot = fmt.Sprintf("C%v|spot add rx_freq=%v callsign=%s mode=%s source=%s spotter_callsign=%s timestamp=%v lifetime_seconds=%s comment=%s color=%s background_color=%s priority=%s",
				flexSpot.CommandNumber, flexSpot.FrequencyMhz,
				flexSpot.DX, flexSpot.Mode, flexSpot.Source, flexSpot.SpotterCallsign,
				flexSpot.TimeStamp, flexSpot.LifeTime, flexSpot.Comment, flexSpot.Color,
				flexSpot.BackgroundColor, flexSpot.Priority)
			CommandNumber++
			fc.SendSpot(stringSpot)
		}

	} else if srcFlexSpot.DX != "" && srcFlexSpot.Band != flexSpot.Band {
		fc.Repo.CreateSpot(flexSpot)
		CommandNumber++

		if fc.IsConnected {
			stringSpot = fmt.Sprintf("C%v|spot add rx_freq=%v callsign=%s mode=%s source=%s spotter_callsign=%s timestamp=%v lifetime_seconds=%s comment=%s color=%s background_color=%s priority=%s",
				flexSpot.CommandNumber, flexSpot.FrequencyMhz,
				flexSpot.DX, flexSpot.Mode, flexSpot.Source, flexSpot.SpotterCallsign,
				flexSpot.TimeStamp, flexSpot.LifeTime, flexSpot.Comment, flexSpot.Color,
				flexSpot.BackgroundColor, flexSpot.Priority)
			CommandNumber++
			fc.SendSpot(stringSpot)
		}
	}
}

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
			if err != nil {
				Log.Errorf("could not find spot by flex spot number in database: %s", err)
			}

			// Sending the callsign to Log4OM
			SendUDPMessage("<CALLSIGN>" + spot.DX)
		}

		// Status when a spot is deleted
		regSpotDeleted := *regexp.MustCompile(`S\d+\|spot (\d+) removed`)
		respDelete := regSpotDeleted.FindStringSubmatch(message)

		if len(respDelete) > 0 {
			fc.Repo.DeleteSpotByFlexSpotNumber(respDelete[1])
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
