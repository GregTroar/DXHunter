package main

import (
	"embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

//go:embed frontend/dist/*
var frontendFiles embed.FS
var httpServerInstance *HTTPServer
var writeMutex sync.Mutex

// ============================================================================
// TYPES
// ============================================================================

type HTTPServer struct {
	Router          *mux.Router
	FlexRepo        *FlexDXClusterRepository
	ContactRepo     LogbookProvider
	TCPServer       *TCPServer
	TCPClients      []*TCPClient
	clientsMu       sync.RWMutex
	FlexClient      *FlexClient
	Port            string
	Log             *log.Logger
	lastQSOCount    int
	lastBandOpening map[string]time.Time
	statsCache      Stats
	statsMutex      sync.RWMutex
	lastUpdate      time.Time
	wsClients       map[*websocket.Conn]bool
	wsMutex         sync.RWMutex
	broadcast       chan WSMessage
	Watchlist       *Watchlist
	ConfigPath      string
	ConsoleChan     chan string
	SpotChan        chan TelnetSpot
	FTx             *FTxService
}

type Stats struct {
	TotalSpots       int           `json:"totalSpots"`
	NewDXCC          int           `json:"newDXCC"`
	ConnectedClients int           `json:"connectedClients"`
	TotalContacts    int           `json:"totalContacts"`
	ClusterStatus    string        `json:"clusterStatus"`
	FlexStatus       string        `json:"flexStatus"`
	MyCallsign       string        `json:"myCallsign"`
	MyGrid           string        `json:"myGrid"`
	Mode             string        `json:"mode"`
	Filters          Filters       `json:"filters"`
	SpotsReceived    int64         `json:"spotsReceived"`
	SpotsProcessed   int64         `json:"spotsProcessed"`
	SpotsRejected    int64         `json:"spotsRejected"`
	SpotSuccessRate  float64       `json:"spotSuccessRate"`
	ContestMode      bool          `json:"contestMode"`
	ContestPrefix    string        `json:"contestPrefix"`
	ContestCallsigns []string      `json:"contestCallsigns"`
	ClusterType      string        `json:"clusterType"`
	Clusters         []ClusterInfo `json:"clusters"`
	LoTWReady        bool          `json:"lotwReady"`
	LoTWCount        int           `json:"lotwCount"`
	FTxEnabled       bool          `json:"ftxEnabled"`
}

type ClusterInfo struct {
	Name   string `json:"name"`
	Master bool   `json:"master"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

type Filters struct {
	Skimmer bool `json:"skimmer"`
	FT8     bool `json:"ft8"`
	FT4     bool `json:"ft4"`
	Beacon  bool `json:"beacon"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ── Config API DTOs ──────────────────────────────────────────────────────────

type ConfigDTO struct {
	General  ConfigGeneralDTO   `json:"general"`
	FTx      ConfigFTxDTO       `json:"ftx"`
	QRZ      ConfigQRZDTO       `json:"qrz"`
	Gotify   ConfigGotifyDTO    `json:"gotify"`
	Flex     ConfigFlexDTO      `json:"flex"`
	Clusters []ConfigClusterDTO `json:"clusters"`
}

type ConfigGeneralDTO struct {
	Callsign          string `json:"callsign"`
	Grid              string `json:"grid"`
	LogLevel          string `json:"logLevel"`
	SendFreqModeToLog bool   `json:"sendFreqModeToLog"`
	FlexRadioSpot     bool   `json:"flexRadioSpot"`
	TelnetServer      bool   `json:"telnetServer"`
}

type ConfigFTxDTO struct {
	Enabled     bool   `json:"enabled"`
	Multicast   bool   `json:"multicast"`
	MulticastIP string `json:"multicastIp"`
	Port        int    `json:"port"`
}

type ConfigQRZDTO struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ConfigGotifyDTO struct {
	Enable         bool   `json:"enable"`
	URL            string `json:"url"`
	Token          string `json:"token"`
	NewDXCC        bool   `json:"newDXCC"`
	NewBand        bool   `json:"newBand"`
	NewMode        bool   `json:"newMode"`
	NewBandAndMode bool   `json:"newBandAndMode"`
	WatchList      bool   `json:"watchlist"`
	WindowsNotify  bool   `json:"windowsNotify"`
}

type ConfigFlexDTO struct {
	IP       string `json:"ip"`
	Discover bool   `json:"discovery"`
	SpotLife string `json:"spotLife"`
}

type ConfigClusterDTO struct {
	Name        string `json:"name"`
	Server      string `json:"server"`
	Port        string `json:"port"`
	Login       string `json:"login"`
	Password    string `json:"password"`
	Enabled     bool   `json:"enabled"`
	Master      bool   `json:"master"`
	Skimmer     bool   `json:"skimmer"`
	FT8         bool   `json:"ft8"`
	FT4         bool   `json:"ft4"`
	Beacon      bool   `json:"beacon"`
	Command     string `json:"command"`
	LoginPrompt string `json:"loginPrompt"`
	Type        string `json:"clusterType"`
}

func configToDTO() ConfigDTO {
	clusters := make([]ConfigClusterDTO, len(Cfg.Clusters))
	for i, c := range Cfg.Clusters {
		clusters[i] = ConfigClusterDTO{
			Name: c.Name, Server: c.Server, Port: c.Port, Login: c.Login,
			Password: c.Password, Enabled: c.Enabled, Master: c.Master,
			Skimmer: c.Skimmer, FT8: c.FT8, FT4: c.FT4, Beacon: c.Beacon,
			Command: c.Command, LoginPrompt: c.LoginPrompt, Type: c.Type,
		}
	}
	return ConfigDTO{
		General: ConfigGeneralDTO{
			Callsign: Cfg.General.Callsign, Grid: Cfg.General.Grid,
			LogLevel: Cfg.General.LogLevel, SendFreqModeToLog: Cfg.General.SendFreqModeToLog,
			FlexRadioSpot: Cfg.General.FlexRadioSpot, TelnetServer: Cfg.General.TelnetServer,
		},
		FTx: ConfigFTxDTO{
			Enabled: Cfg.FTx.Enabled, Multicast: Cfg.FTx.Multicast,
			MulticastIP: Cfg.FTx.MulticastIP, Port: Cfg.FTx.Port,
		},
		QRZ:  ConfigQRZDTO{Username: Cfg.QRZ.Username, Password: Cfg.QRZ.Password},
		Flex: ConfigFlexDTO{IP: Cfg.Flex.IP, Discover: Cfg.Flex.Discover, SpotLife: Cfg.Flex.SpotLife},
		Gotify: ConfigGotifyDTO{
			Enable: Cfg.Gotify.Enable, URL: Cfg.Gotify.URL, Token: Cfg.Gotify.Token,
			NewDXCC: Cfg.Gotify.NewDXCC, NewBand: Cfg.Gotify.NewBand,
			NewMode: Cfg.Gotify.NewMode, NewBandAndMode: Cfg.Gotify.NewBandAndMode,
			WatchList: Cfg.Gotify.WatchList, WindowsNotify: Cfg.Gotify.WindowsNotify,
		},
		Clusters: clusters,
	}
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type WatchlistSpot struct {
	DX              string `json:"dx"`
	FrequencyMhz    string `json:"frequencyMhz"`
	Band            string `json:"band"`
	Mode            string `json:"mode"`
	SpotterCallsign string `json:"spotterCallsign"`
	UTCTime         string `json:"utcTime"`
	CountryName     string `json:"countryName"`
	NewDXCC         bool   `json:"newDXCC"`
	NewBand         bool   `json:"newBand"`
	NewMode         bool   `json:"newMode"`
	NewSlot         bool   `json:"newSlot"`
	Worked          bool   `json:"worked"`
	WorkedBandMode  bool   `json:"workedBandMode"`
}

type RemoteControlRequest struct {
	XMLName              xml.Name `xml:"RemoteControlRequest"`
	MessageId            string   `xml:"MessageId"`
	RemoteControlMessage string   `xml:"RemoteControlMessage"`
	Frequency            string   `xml:"Frequency,omitempty"`
	Mode                 string   `xml:"Mode,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ============================================================================
// CONSTRUCTOR & SETUP
// ============================================================================

func NewHTTPServer(flexRepo *FlexDXClusterRepository, contactRepo LogbookProvider,
	tcpServer *TCPServer, tcpClients []*TCPClient, flexClient *FlexClient, port string, configPath string, consoleChan chan string, spotChan chan TelnetSpot) *HTTPServer {

	server := &HTTPServer{
		Router:          mux.NewRouter(),
		FlexRepo:        flexRepo,
		ContactRepo:     contactRepo,
		TCPServer:       tcpServer,
		TCPClients:      tcpClients,
		FlexClient:      flexClient,
		Port:            port,
		Log:             Log,
		wsClients:       make(map[*websocket.Conn]bool),
		broadcast:       make(chan WSMessage, 256),
		ConsoleChan:     consoleChan,
		SpotChan:        spotChan,
		Watchlist:       NewWatchlist("watchlist.json"),
		ConfigPath:      configPath,
		lastQSOCount:    0,
		lastBandOpening: make(map[string]time.Time),
	}

	httpServerInstance = server
	server.setupRoutes()
	go server.handleBroadcasts()
	go server.broadcastUpdates()
	go server.handleConsoleMessages()

	// FTx multicast listener — shares the broadcast channel
	if Cfg.FTx.Enabled {
		server.FTx = NewFTxService(contactRepo, server.broadcast)
		go server.FTx.Start()
	}

	return server
}

func (s *HTTPServer) handleFTxToggle(w http.ResponseWriter, r *http.Request) {
	Cfg.FTx.Enabled = !Cfg.FTx.Enabled
	s.Log.Infof("FTx toggled to: %v", Cfg.FTx.Enabled)

	if Cfg.FTx.Enabled && s.FTx == nil {
		s.FTx = NewFTxService(s.ContactRepo, s.broadcast)
		go s.FTx.Start()
	}

	s.broadcast <- WSMessage{Type: "stats", Data: s.calculateStats()}
	s.sendSuccess(w, map[string]bool{"enabled": Cfg.FTx.Enabled}, fmt.Sprintf("FTx %s", map[bool]string{true: "enabled", false: "disabled"}[Cfg.FTx.Enabled]))
}

func (s *HTTPServer) handleFTxReply(w http.ResponseWriter, r *http.Request) {
	if s.FTx == nil {
		s.sendError(w, "FTx not enabled")
		return
	}
	var req struct {
		Decode   FTxDecode `json:"decode"`
		ClientID string    `json:"clientId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid request: "+err.Error())
		return
	}
	if err := s.FTx.SendReply(req.Decode, req.ClientID); err != nil {
		s.sendError(w, "reply failed: "+err.Error())
		return
	}
	s.sendSuccess(w, nil, "Reply sent")
}

func (s *HTTPServer) handleFTxHaltTX(w http.ResponseWriter, r *http.Request) {
	if s.FTx == nil {
		s.sendError(w, "FTx not enabled")
		return
	}
	var req struct {
		ClientID string `json:"clientId"`
		AutoOnly bool   `json:"autoOnly"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ClientID = "MSHV"
	}
	if err := s.FTx.HaltTX(req.ClientID, req.AutoOnly); err != nil {
		s.sendError(w, "halt TX failed: "+err.Error())
		return
	}
	s.sendSuccess(w, nil, "TX halted")
}

func (s *HTTPServer) handleFTxConfigure(w http.ResponseWriter, r *http.Request) {
	if s.FTx == nil {
		s.sendError(w, "FTx not enabled")
		return
	}
	var req struct {
		Mode        string `json:"mode"`
		ClearDXCall bool   `json:"clearDXCall"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid request: "+err.Error())
		return
	}
	if err := s.FTx.SendConfigure(req.Mode, req.ClearDXCall); err != nil {
		s.sendError(w, "configure failed: "+err.Error())
		return
	}
	s.sendSuccess(w, nil, "Configure sent")
}

func (s *HTTPServer) handleFTxHighlight(w http.ResponseWriter, r *http.Request) {
	if s.FTx == nil {
		s.sendError(w, "FTx not enabled")
		return
	}
	var req struct {
		ClientID  string `json:"clientId"`
		Callsign  string `json:"callsign"`
		BgColor   [4]uint8 `json:"bgColor"`  // RGBA
		FgColor   [4]uint8 `json:"fgColor"`
		Highlight bool     `json:"highlight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid request: "+err.Error())
		return
	}
	if err := s.FTx.HighlightCallsign(req.ClientID, req.Callsign, req.BgColor, req.FgColor, req.Highlight); err != nil {
		s.sendError(w, "highlight failed: "+err.Error())
		return
	}
	s.sendSuccess(w, nil, "Highlight sent")
}

func (s *HTTPServer) setupRoutes() {
	s.Router.Use(s.corsMiddleware)

	api := s.Router.PathPrefix("/api").Subrouter()

	// Setup
	api.HandleFunc("/setup-required", s.handleSetupRequired).Methods("GET", "OPTIONS")

	// Config
	api.HandleFunc("/config", s.getConfigAPI).Methods("GET", "OPTIONS")
	api.HandleFunc("/config", s.saveConfigAPI).Methods("POST", "OPTIONS")
	api.HandleFunc("/config/test-qrz", s.testQRZConfig).Methods("POST", "OPTIONS")
	api.HandleFunc("/config/test-cluster", s.testClusterConnection).Methods("POST", "OPTIONS")

	// Stats & Data
	api.HandleFunc("/stats", s.getStats).Methods("GET", "OPTIONS")
	api.HandleFunc("/stats/spots", s.getSpotProcessingStats).Methods("GET", "OPTIONS")
	api.HandleFunc("/spots", s.getSpots).Methods("GET", "OPTIONS")
	api.HandleFunc("/spots/{id}", s.getSpotByID).Methods("GET", "OPTIONS")
	api.HandleFunc("/contacts", s.getContacts).Methods("GET", "OPTIONS")

	// Log data
	api.HandleFunc("/log/recent", s.getRecentQSOs).Methods("GET", "OPTIONS")
	api.HandleFunc("/log/stats", s.getLogStats).Methods("GET", "OPTIONS")
	api.HandleFunc("/log/dxcc-progress", s.getDXCCProgress).Methods("GET", "OPTIONS")
	api.HandleFunc("/logs", s.getLogs).Methods("GET", "OPTIONS")

	// Watchlist
	api.HandleFunc("/watchlist", s.getWatchlist).Methods("GET", "OPTIONS")
	api.HandleFunc("/watchlist/spots", s.getWatchlistSpotsWithStatus).Methods("GET", "OPTIONS")
	api.HandleFunc("/watchlist/add", s.addToWatchlist).Methods("POST", "OPTIONS")
	api.HandleFunc("/watchlist/remove", s.removeFromWatchlist).Methods("DELETE", "OPTIONS")
	api.HandleFunc("/watchlist/notify", s.setWatchlistNotify).Methods("POST", "OPTIONS")

	// Actions
	api.HandleFunc("/filters", s.updateFilters).Methods("POST", "OPTIONS")
	api.HandleFunc("/send-callsign", s.handleSendCallsign).Methods("POST", "OPTIONS")
	api.HandleFunc("/tune", s.handleTuneFromToast).Methods("GET", "OPTIONS")
	api.HandleFunc("/contest/toggle", s.toggleContestMode).Methods("POST", "OPTIONS")
	api.HandleFunc("/shutdown", s.shutdownApp).Methods("POST", "OPTIONS")

	// Callsign DX info (query param to handle callsigns with slash e.g. V4/SP9FIH)
	api.HandleFunc("/callsign/band-modes", s.getCallsignBandModes).Methods("GET", "OPTIONS")
	api.HandleFunc("/callsign/spots", s.getCallsignSpots).Methods("GET", "OPTIONS")

	// FTx (WSJT-X/JTDX/MSHV) — reply to a decoded station
	api.HandleFunc("/ftx/reply", s.handleFTxReply).Methods("POST", "OPTIONS")
	api.HandleFunc("/ftx/toggle", s.handleFTxToggle).Methods("POST", "OPTIONS")
	api.HandleFunc("/ftx/halttx", s.handleFTxHaltTX).Methods("POST", "OPTIONS")
	api.HandleFunc("/ftx/highlight", s.handleFTxHighlight).Methods("POST", "OPTIONS")
	api.HandleFunc("/ftx/configure", s.handleFTxConfigure).Methods("POST", "OPTIONS")

	// External data
	api.HandleFunc("/solar", s.HandleSolarData).Methods("GET", "OPTIONS")
	api.HandleFunc("/adxo", s.HandleADXO).Methods("GET", "OPTIONS")
	api.HandleFunc("/dxworld", s.HandleDXWorld).Methods("GET", "OPTIONS")
	api.HandleFunc("/qrz/{call}", s.HandleQRZ).Methods("GET", "OPTIONS")
	api.HandleFunc("/cty/update", s.updateCtyPlist).Methods("POST", "OPTIONS")

	// WebSocket (seul point d'entrée pour les commandes Telnet maintenant)
	api.HandleFunc("/ws", s.handleWebSocket).Methods("GET")

	s.setupStaticFiles()
}

// ============================================================================
// HELPERS - Réutilisables pour réduire la duplication
// ============================================================================

// decodeJSONBody décode le body JSON et retourne une erreur si échec
func (s *HTTPServer) decodeJSONBody(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// sendError envoie une réponse d'erreur JSON
func (s *HTTPServer) sendError(w http.ResponseWriter, message string) {
	s.sendJSON(w, APIResponse{Success: false, Error: message})
}

// sendSuccess envoie une réponse de succès JSON
func (s *HTTPServer) sendSuccess(w http.ResponseWriter, data interface{}, message string) {
	s.sendJSON(w, APIResponse{Success: true, Data: data, Message: message})
}

func (s *HTTPServer) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *HTTPServer) handleSetupRequired(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"required": false})
}

// isClusterConnected vérifie si le client TCP est connecté
// MasterClient retourne le TCPClient maître (master:true ou le premier de la liste)
func (s *HTTPServer) MasterClient() *TCPClient {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	for _, c := range s.TCPClients {
		if c.ClusterCfg.Master {
			return c
		}
	}
	if len(s.TCPClients) > 0 {
		return s.TCPClients[0]
	}
	return nil
}

func (s *HTTPServer) isClusterConnected() bool {
	master := s.MasterClient()
	return master != nil && master.LoggedIn
}

// isFlexConnected vérifie si le FlexRadio est connecté
func (s *HTTPServer) isFlexConnected() bool {
	return s.FlexClient != nil && s.FlexClient.IsConnected
}

// sendTelnetCommand envoie une commande au cluster (utilisé par WebSocket uniquement)
func (s *HTTPServer) sendTelnetCommand(command string) error {
	if !s.isClusterConnected() {
		return fmt.Errorf("not connected to cluster")
	}
	s.MasterClient().CmdChan <- command
	s.Log.Infof("Telnet command sent to master cluster: %s", command)
	return nil
}

// filterWatchlistEntries filtre les entrées watchlist selon le mode contest
func (s *HTTPServer) filterWatchlistEntries(entries []WatchlistEntry, contestMode bool) []WatchlistEntry {
	filtered := make([]WatchlistEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsContest == contestMode {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// ============================================================================
// WEBSOCKET HANDLERS
// ============================================================================

func (s *HTTPServer) handleConsoleMessages() {
	// Regex pour détecter les spots DX
	spotDetectRe := regexp.MustCompile(`(?i)^DX\sde\s`)
	shortSpotDetectRe := regexp.MustCompile(`^\d+\.\d+\s+[\w\d\/]+\s+\d{2}-\w{3}-\d{4}`)

	// Déduplication To ALL: hash -> timestamp dernier envoi
	toAllSeen := make(map[string]int64)

	for msg := range s.ConsoleChan {
		cleanMsg := strings.TrimSpace(msg)
		if cleanMsg == "" {
			continue
		}

		// Filtrer les spots DX
		if spotDetectRe.MatchString(cleanMsg) || shortSpotDetectRe.MatchString(cleanMsg) {
			continue
		}

		// To ALL -- forward as dedicated toast (deduplicated: TTL 30s)
		if strings.HasPrefix(cleanMsg, "TO_ALL:") {
			toAllMsg := strings.TrimSpace(strings.TrimPrefix(cleanMsg, "TO_ALL:"))
			now := time.Now().Unix()
			// Nettoyer les entrees expirées
			for k, t := range toAllSeen {
				if now-t > 30 {
					delete(toAllSeen, k)
				}
			}
			// Ignorer si déjà vu dans les 30 dernières secondes
			if _, seen := toAllSeen[toAllMsg]; seen {
				continue
			}
			toAllSeen[toAllMsg] = now
			s.broadcast <- WSMessage{
				Type: "toAll",
				Data: map[string]interface{}{
					"message":   toAllMsg,
					"timestamp": time.Now().Format("15:04:05"),
				},
			}
			continue
		}

		// Broadcaster les réponses non-spot
		s.broadcast <- WSMessage{
			Type: "telnetResponse",
			Data: map[string]interface{}{
				"message":   cleanMsg,
				"timestamp": time.Now().Format("15:04:05"),
			},
		}
	}
}

func (s *HTTPServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Log.Errorf("WebSocket upgrade failed: %v", err)
		return
	}

	s.wsMutex.Lock()
	s.wsClients[conn] = true
	clientCount := len(s.wsClients)
	s.wsMutex.Unlock()

	s.Log.Infof("New WebSocket client connected (total: %d)", clientCount)

	s.sendInitialData(conn)
	go s.handleWebSocketClient(conn)
}

func (s *HTTPServer) handleWebSocketClient(conn *websocket.Conn) {
	defer func() {
		s.wsMutex.Lock()
		delete(s.wsClients, conn)
		clientCount := len(s.wsClients)
		s.wsMutex.Unlock()

		conn.Close()
		s.Log.Infof("WebSocket client disconnected (remaining: %d)", clientCount)
	}()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.Log.Errorf("WebSocket error: %v", err)
			}
			break
		}

		if messageType == websocket.TextMessage {
			s.handleWebSocketMessage(conn, message)
		}
	}
}

func (s *HTTPServer) handleWebSocketMessage(conn *websocket.Conn, message []byte) {
	var msg struct {
		Type string                 `json:"type"`
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		s.Log.Errorf("Failed to parse WebSocket message: %v", err)
		return
	}

	switch msg.Type {
	case "telnetCommand":
		s.handleWSTelnetCommand(conn, msg.Data)
	default:
		s.Log.Debugf("Unknown WebSocket message type: %s", msg.Type)
	}
}

func (s *HTTPServer) handleWSTelnetCommand(conn *websocket.Conn, data map[string]interface{}) {
	command, ok := data["command"].(string)
	if !ok || command == "" {
		s.safeWrite(conn, WSMessage{
			Type: "telnetCommandResponse",
			Data: map[string]interface{}{
				"success": false,
				"message": "Command is required",
			},
		})
		return
	}

	// Choisir le cluster cible par nom, sinon master
	clusterName, _ := data["clusterName"].(string)
	target := s.MasterClient()
	if clusterName != "" {
		for _, c := range s.TCPClients {
			if c.ClusterCfg.Name == clusterName {
				target = c
				break
			}
		}
	}

	var err error
	if target == nil || !target.LoggedIn {
		err = fmt.Errorf("cluster not connected")
	} else {
		target.CmdChan <- command
		s.Log.Infof("Telnet command sent to [%s]: %s", target.ClusterCfg.Name, command)
	}

	response := WSMessage{
		Type: "telnetCommandResponse",
		Data: map[string]interface{}{
			"success": err == nil,
			"command": command,
			"message": map[bool]string{true: "Command sent to cluster", false: "Not connected to cluster"}[err == nil],
		},
	}
	s.safeWrite(conn, response)

	if err == nil {
		s.broadcast <- WSMessage{
			Type: "telnetResponse",
			Data: map[string]interface{}{
				"message":   "> " + command,
				"timestamp": time.Now().Format("15:04:05"),
				"isCommand": true,
			},
		}
	}
}

func (s *HTTPServer) safeWrite(conn *websocket.Conn, msg WSMessage) error {
	writeMutex.Lock()
	defer writeMutex.Unlock()
	return conn.WriteJSON(msg)
}

func (s *HTTPServer) sendInitialData(conn *websocket.Conn) {
	// Stats
	s.safeWrite(conn, WSMessage{Type: "stats", Data: s.calculateStats()})

	// Logbook type (so the UI can show the correct badge)
	s.safeWrite(conn, WSMessage{Type: "logbookType", Data: Cfg.Database.LogbookType})

	// Spots
	s.safeWrite(conn, WSMessage{Type: "spots", Data: s.FlexRepo.GetAllSpots("0")})

	// Watchlist
	s.safeWrite(conn, WSMessage{Type: "watchlist", Data: s.Watchlist.GetAll()})

	// Log data (guarded — ContactRepo may be nil if no logbook is configured)
	if s.ContactRepo != nil {
		s.safeWrite(conn, WSMessage{Type: "log", Data: s.ContactRepo.GetRecentQSOs("19")})
		s.safeWrite(conn, WSMessage{Type: "logStats", Data: s.ContactRepo.GetQSOStats()})

		dxccCount := s.ContactRepo.GetDXCCCount()
		s.safeWrite(conn, WSMessage{Type: "dxccProgress", Data: map[string]interface{}{
			"worked":     dxccCount,
			"total":      340,
			"percentage": float64(dxccCount) / 340.0 * 100.0,
		}})

	}

	// App logs
	if logBuffer != nil {
		s.safeWrite(conn, WSMessage{Type: "appLogs", Data: logBuffer.GetAll()})
	}

	// ADXO activations
	s.safeWrite(conn, WSMessage{Type: "adxo", Data: adxoCache.Get()})

	// DX-World news
	s.safeWrite(conn, WSMessage{Type: "dxworld", Data: dxwCache.Get()})
}

// ============================================================================
// BROADCAST LOOP
// ============================================================================

func (s *HTTPServer) handleBroadcasts() {
	for msg := range s.broadcast {
		s.wsMutex.RLock()
		clientsToRemove := make([]*websocket.Conn, 0)

		for client := range s.wsClients {
			if err := s.safeWrite(client, msg); err != nil {
				s.Log.Errorf("WebSocket write error: %v", err)
				client.Close()
				clientsToRemove = append(clientsToRemove, client)
			}
		}
		s.wsMutex.RUnlock()

		// Supprimer les clients défaillants
		if len(clientsToRemove) > 0 {
			s.wsMutex.Lock()
			for _, client := range clientsToRemove {
				delete(s.wsClients, client)
			}
			s.wsMutex.Unlock()
		}
	}
}

// enrichedSpots returns all spots with LoTWUser populated.
func (s *HTTPServer) enrichedSpots() []FlexSpot {
	spots := s.FlexRepo.GetAllSpots("0")
	for i := range spots {
		spots[i].LoTWUser = IsLoTWUser(spots[i].DX)
	}
	return spots
}

func (s *HTTPServer) broadcastUpdates() {
	defer func() {
		if r := recover(); r != nil {
			s.Log.Errorf("PANIC in broadcastUpdates (restarting): %v", r)
			go s.broadcastUpdates()
		}
	}()

	statsTicker := time.NewTicker(1 * time.Second)
	logTicker := time.NewTicker(5 * time.Second)
	watchlistSaveTicker := time.NewTicker(20 * time.Second)
	cleanupTicker := time.NewTicker(30 * time.Second)
	watchlistCleanupTicker := time.NewTicker(24 * time.Hour)

	defer func() {
		statsTicker.Stop()
		logTicker.Stop()
		watchlistSaveTicker.Stop()
		cleanupTicker.Stop()
		watchlistCleanupTicker.Stop()
		Log.Info("Broadcast updates stopped")
	}()

	// Run watchlist stale cleanup once at startup
	if s.Watchlist != nil {
		removed := s.Watchlist.CleanupStale(28 * 24 * time.Hour)
		if len(removed) > 0 {
			Log.Infof("Watchlist startup cleanup: removed %d stale entries: %v", len(removed), removed)
		}
	}

	for {
		select {
		case <-watchlistCleanupTicker.C:
			if s.Watchlist != nil {
				removed := s.Watchlist.CleanupStale(28 * 24 * time.Hour)
				if len(removed) > 0 {
					Log.Infof("Watchlist daily cleanup: removed %d stale entries: %v", len(removed), removed)
				}
			}

		case <-statsTicker.C:
			if s.clientCount() == 0 {
				continue
			}

			// ✅ Stats avec timeout
			statsMsg := WSMessage{Type: "stats", Data: s.calculateStats()}
			select {
			case s.broadcast <- statsMsg:
				// Envoyé avec succès
			case <-time.After(50 * time.Millisecond):
				s.Log.Debug("Broadcast channel busy, dropping stats update")
			}

			// ✅ Spots avec timeout
			spots := s.enrichedSpots()
			if len(spots) > 0 {
				s.checkBandOpening(spots)
				spotsMsg := WSMessage{Type: "spots", Data: spots}
				select {
				case s.broadcast <- spotsMsg:
					// Envoyé avec succès
				case <-time.After(50 * time.Millisecond):
					s.Log.Debug("Broadcast channel busy, dropping spots update")
				}
			}

		case <-logTicker.C:
			if s.clientCount() == 0 || s.ContactRepo == nil {
				continue
			}

			qsos := s.ContactRepo.GetRecentQSOs("19")
			if len(qsos) > 0 {
				qsosMsg := WSMessage{Type: "log", Data: qsos}
				select {
				case s.broadcast <- qsosMsg:
				case <-time.After(50 * time.Millisecond):
					s.Log.Debug("Broadcast channel busy, dropping QSOs update")
				}
			}

			stats := s.ContactRepo.GetQSOStats()
			if stats.Today > 0 {
				s.checkQSOMilestones(stats.Today)
				logStatsMsg := WSMessage{Type: "logStats", Data: stats}
				select {
				case s.broadcast <- logStatsMsg:
				case <-time.After(50 * time.Millisecond):
					s.Log.Debug("Broadcast channel busy, dropping log stats update")
				}
			}

			dxccCount := s.ContactRepo.GetDXCCCount()
			dxccMsg := WSMessage{Type: "dxccProgress", Data: map[string]interface{}{
				"worked":     dxccCount,
				"total":      340,
				"percentage": float64(dxccCount) / 340.0 * 100.0,
			}}
			select {
			case s.broadcast <- dxccMsg:
			case <-time.After(50 * time.Millisecond):
				s.Log.Debug("Broadcast channel busy, dropping DXCC update")
			}

		case <-watchlistSaveTicker.C:
			// ✅ Sauvegarde watchlist (pas de broadcast, donc pas de timeout)
			if s.Watchlist != nil {
				if err := s.Watchlist.save(); err != nil {
					s.Log.Errorf("Failed to save watchlist: %v", err)
				}
			}

		case <-cleanupTicker.C:
			// ✅ Nettoyage périodique pour éviter l'accumulation
			s.cleanupBroadcastChannel()

			// ✅ Log de statut pour débogage
			s.wsMutex.RLock()
			clientCount := len(s.wsClients)
			s.wsMutex.RUnlock()

			s.Log.Debugf("Broadcast status: %d clients, channel %d/%d",
				clientCount, len(s.broadcast), cap(s.broadcast))
		}
	}
}

func (s *HTTPServer) cleanupBroadcastChannel() {
	// Si le canal est presque plein, vider les anciens messages
	if len(s.broadcast) > cap(s.broadcast)*3/4 { // > 75% plein
		s.Log.Warnf("Broadcast channel almost full (%d/%d), cleaning up",
			len(s.broadcast), cap(s.broadcast))

		// Garder seulement les 50 derniers messages
		keptMessages := make([]WSMessage, 0, 50)
		for i := 0; i < 50 && len(s.broadcast) > 0; i++ {
			select {
			case msg := <-s.broadcast:
				keptMessages = append(keptMessages, msg)
			default:
				break
			}
		}

		// Vider complètement le canal
		for len(s.broadcast) > 0 {
			<-s.broadcast
		}

		// Remettre les messages gardés
		for _, msg := range keptMessages {
			select {
			case s.broadcast <- msg:
			default:
				break
			}
		}

		s.Log.Infof("Broadcast channel cleaned: kept %d messages", len(keptMessages))
	}
}

func (s *HTTPServer) clientCount() int {
	s.wsMutex.RLock()
	defer s.wsMutex.RUnlock()
	return len(s.wsClients)
}

// ============================================================================
// API HANDLERS - Stats & Data
// ============================================================================

func (s *HTTPServer) getStats(w http.ResponseWriter, r *http.Request) {
	s.sendSuccess(w, s.calculateStats(), "")
}

func (s *HTTPServer) updateCtyPlist(w http.ResponseWriter, r *http.Request) {
	s.Log.Info("Manual cty.plist update requested")
	result, err := UpdateCtyPlist("cty.plist")
	if err != nil {
		s.sendError(w, result.Message)
		return
	}

	s.sendSuccess(w, result, result.Message)
}

func (s *HTTPServer) getSpotProcessingStats(w http.ResponseWriter, r *http.Request) {
	received, processed, rejected := GetSpotStats()
	s.sendSuccess(w, map[string]interface{}{
		"received":    received,
		"processed":   processed,
		"rejected":    rejected,
		"successRate": GetSpotSuccessRate(),
	}, "")
}

func (s *HTTPServer) getSpots(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "50"
	}
	s.sendSuccess(w, s.FlexRepo.GetAllSpots(limit), "")
}

func (s *HTTPServer) getSpotByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	spot, err := s.FlexRepo.FindSpotByFlexSpotNumber(vars["id"])
	if err != nil {
		s.sendError(w, "Spot not found")
		return
	}
	s.sendSuccess(w, spot, "")
}

func (s *HTTPServer) getContacts(w http.ResponseWriter, r *http.Request) {
	s.sendSuccess(w, map[string]interface{}{"totalContacts": s.ContactRepo.CountEntries()}, "")
}

func (s *HTTPServer) getLogs(w http.ResponseWriter, r *http.Request) {
	if logBuffer == nil {
		s.sendSuccess(w, []LogEntry{}, "")
		return
	}
	s.sendSuccess(w, logBuffer.GetAll(), "")
}

// ============================================================================
// API HANDLERS - Log Data
// ============================================================================

func (s *HTTPServer) getRecentQSOs(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "50"
	}
	s.sendSuccess(w, s.ContactRepo.GetRecentQSOs(limit), "")
}

func (s *HTTPServer) getLogStats(w http.ResponseWriter, r *http.Request) {
	s.sendSuccess(w, s.ContactRepo.GetQSOStats(), "")
}

func (s *HTTPServer) getCallsignSpots(w http.ResponseWriter, r *http.Request) {
	callsign := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("call")))
	if callsign == "" {
		s.sendError(w, "missing call parameter")
		return
	}
	spots := s.FlexRepo.GetSpotsByCallsign(callsign, 10)
	if spots == nil {
		spots = []FlexSpot{}
	}
	s.sendSuccess(w, spots, "")
}

func (s *HTTPServer) getCallsignBandModes(w http.ResponseWriter, r *http.Request) {
	if s.ContactRepo == nil {
		s.sendError(w, "Log4OM database not configured")
		return
	}
	callsign := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("call")))
	if callsign == "" {
		s.sendError(w, "missing call parameter")
		return
	}
	info := s.ContactRepo.GetCallsignBandModes(callsign)
	s.sendSuccess(w, info, "")
}

func (s *HTTPServer) getDXCCProgress(w http.ResponseWriter, r *http.Request) {
	count := s.ContactRepo.GetDXCCCount()
	s.sendSuccess(w, map[string]interface{}{
		"worked":     count,
		"total":      340,
		"percentage": float64(count) / 340.0 * 100.0,
	}, "")
}

// ============================================================================
// API HANDLERS - Watchlist
// ============================================================================

func (s *HTTPServer) getWatchlist(w http.ResponseWriter, r *http.Request) {
	entries := s.filterWatchlistEntries(s.Watchlist.GetAll(), Cfg.General.ContestMode)
	s.sendSuccess(w, entries, "")
}

func (s *HTTPServer) addToWatchlist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Callsign string `json:"callsign"`
	}

	if err := s.decodeJSONBody(r, &req); err != nil {
		s.sendError(w, "Invalid request")
		return
	}

	if req.Callsign == "" {
		s.sendError(w, "Callsign is required")
		return
	}

	if err := s.Watchlist.Add(req.Callsign); err != nil {
		s.sendError(w, err.Error())
		return
	}

	s.Log.Infof("Added %s to watchlist", req.Callsign)
	s.broadcast <- WSMessage{Type: "watchlist", Data: s.Watchlist.GetAll()}
	s.sendSuccess(w, nil, "Callsign added to watchlist")
}

func (s *HTTPServer) removeFromWatchlist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Callsign string `json:"callsign"`
	}

	if err := s.decodeJSONBody(r, &req); err != nil {
		s.sendError(w, "Invalid request")
		return
	}

	if req.Callsign == "" {
		s.sendError(w, "Callsign is required")
		return
	}

	if err := s.Watchlist.Remove(req.Callsign); err != nil {
		s.sendError(w, err.Error())
		return
	}

	s.Log.Debugf("Removed %s from watchlist", req.Callsign)
	s.broadcast <- WSMessage{Type: "watchlist", Data: s.Watchlist.GetAll()}
	s.sendSuccess(w, nil, "Callsign removed from watchlist")
}

func (s *HTTPServer) setWatchlistNotify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Callsign string `json:"callsign"`
		Notify   bool   `json:"notify"`
	}

	if err := s.decodeJSONBody(r, &req); err != nil {
		s.sendError(w, "Invalid request")
		return
	}

	if req.Callsign == "" {
		s.sendError(w, "Callsign is required")
		return
	}

	if err := s.Watchlist.SetNotify(req.Callsign, req.Notify); err != nil {
		s.sendError(w, err.Error())
		return
	}

	status := "disabled"
	if req.Notify {
		status = "enabled"
	}
	s.Log.Infof("Gotify notifications %s for %s", status, req.Callsign)
	s.broadcast <- WSMessage{Type: "watchlist", Data: s.Watchlist.GetAll()}
	s.sendSuccess(w, nil, fmt.Sprintf("Notifications %s for %s", status, req.Callsign))
}

func (s *HTTPServer) getWatchlistSpotsWithStatus(w http.ResponseWriter, r *http.Request) {
	allSpots := s.FlexRepo.GetAllSpots("0")
	watchlistEntries := s.filterWatchlistEntries(s.Watchlist.GetAll(), Cfg.General.ContestMode)

	// Créer une map des callsigns watchlist pour recherche rapide
	watchlistCallsigns := make(map[string]bool)
	for _, entry := range watchlistEntries {
		watchlistCallsigns[entry.Callsign] = true
	}

	// Filtrer les spots qui correspondent à la watchlist
	var relevantSpots []FlexSpot
	for _, spot := range allSpots {
		for callsign := range watchlistCallsigns {
			if spot.DX == callsign || strings.HasPrefix(spot.DX, callsign) {
				relevantSpots = append(relevantSpots, spot)
				break
			}
		}
	}

	// Grouper par Band/Mode
	type BandModeKey struct {
		Band string
		Mode string
	}

	spotsByBandMode := make(map[BandModeKey][]FlexSpot)
	for _, spot := range relevantSpots {
		key := BandModeKey{Band: spot.Band, Mode: spot.Mode}
		spotsByBandMode[key] = append(spotsByBandMode[key], spot)
	}

	// Construire le résultat
	var watchlistSpots []WatchlistSpot
	for key, spots := range spotsByBandMode {
		// Collecter les callsigns uniques
		callsignSet := make(map[string]bool)
		for _, spot := range spots {
			callsignSet[spot.DX] = true
		}

		// Séparer contest/normal
		var contestCallsigns, normalCallsigns []string
		for callsign := range callsignSet {
			entry := s.Watchlist.GetEntry(callsign)
			if entry != nil && entry.IsContest {
				contestCallsigns = append(contestCallsigns, callsign)
			} else {
				normalCallsigns = append(normalCallsigns, callsign)
			}
		}

		// Récupérer les statuts worked
		workedMap := make(map[string]bool)

		if len(contestCallsigns) > 0 {
			for callsign, worked := range s.ContactRepo.GetWorkedCallsignsBandModeToday(contestCallsigns, key.Band, key.Mode) {
				workedMap[callsign] = worked
			}
		}

		if len(normalCallsigns) > 0 {
			for callsign, worked := range s.ContactRepo.GetWorkedCallsignsBandMode(normalCallsigns, key.Band, key.Mode) {
				workedMap[callsign] = worked
			}
		}

		// Créer les WatchlistSpot
		for _, spot := range spots {
			watchlistSpots = append(watchlistSpots, WatchlistSpot{
				DX:              spot.DX,
				FrequencyMhz:    spot.FrequencyMhz,
				Band:            spot.Band,
				Mode:            spot.Mode,
				SpotterCallsign: spot.SpotterCallsign,
				UTCTime:         spot.UTCTime,
				CountryName:     spot.CountryName,
				NewDXCC:         spot.NewDXCC,
				NewBand:         spot.NewBand,
				NewMode:         spot.NewMode,
				NewSlot:         spot.NewSlot,
				Worked:          spot.Worked,
				WorkedBandMode:  workedMap[spot.DX],
			})
		}
	}

	s.sendSuccess(w, watchlistSpots, "")
}

// ============================================================================
// API HANDLERS - Actions
// ============================================================================

func (s *HTTPServer) updateFilters(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Skimmer *bool `json:"skimmer,omitempty"`
		FT8     *bool `json:"ft8,omitempty"`
		FT4     *bool `json:"ft4,omitempty"`
		Beacon  *bool `json:"beacon,omitempty"`
	}

	if err := s.decodeJSONBody(r, &req); err != nil {
		s.sendError(w, "Invalid request")
		return
	}

	type filterConfig struct {
		value  *bool
		cfgPtr *bool
		onCmd  string
		offCmd string
	}

	// Trouver l'index du cluster maître dans Cfg.Clusters pour pouvoir le muter
	masterIdx := -1
	for i := range Cfg.Clusters {
		if Cfg.Clusters[i].Master && Cfg.Clusters[i].Enabled {
			masterIdx = i
			break
		}
	}
	if masterIdx == -1 && len(Cfg.Clusters) > 0 {
		masterIdx = 0
	}
	if masterIdx == -1 {
		s.sendError(w, "No cluster configured")
		return
	}

	filters := []filterConfig{
		{req.Skimmer, &Cfg.Clusters[masterIdx].Skimmer, "set/skimmer", "set/noskimmer"},
		{req.FT8, &Cfg.Clusters[masterIdx].FT8, "set/ft8", "set/noft8"},
		{req.FT4, &Cfg.Clusters[masterIdx].FT4, "set/ft4", "set/noft4"},
		{req.Beacon, &Cfg.Clusters[masterIdx].Beacon, "set/beacon", "set/nobeacon"},
	}

	for _, f := range filters {
		if f.value != nil {
			*f.cfgPtr = *f.value
			cmd := f.offCmd
			if *f.value {
				cmd = f.onCmd
			}
			s.MasterClient().CmdChan <- cmd
		}
	}

	s.sendSuccess(w, map[string]string{"message": "Filters updated successfully"}, "")
}

func (s *HTTPServer) toggleContestMode(w http.ResponseWriter, r *http.Request) {
	Cfg.General.ContestMode = !Cfg.General.ContestMode
	s.Log.Infof("Contest mode toggled to: %v", Cfg.General.ContestMode)

	if Cfg.General.ContestMode && s.Watchlist != nil {
		addedCount := 0
		for _, callsign := range Cfg.General.ContestCallsigns {
			if err := s.Watchlist.AddContest(callsign); err == nil {
				addedCount++
			}
		}
		s.Log.Infof("Added %d contest callsigns to watchlist", addedCount)
	}

	// Broadcast updates
	s.broadcast <- WSMessage{Type: "stats", Data: s.calculateStats()}
	s.broadcast <- WSMessage{Type: "watchlist", Data: s.Watchlist.GetAll()}

	status := "disabled"
	if Cfg.General.ContestMode {
		status = "enabled"
	}

	s.sendSuccess(w, map[string]interface{}{
		"contestMode":      Cfg.General.ContestMode,
		"contestPrefix":    Cfg.General.ContestPrefix,
		"contestCallsigns": Cfg.General.ContestCallsigns,
	}, fmt.Sprintf("Contest mode %s", status))
}

func (s *HTTPServer) handleSendCallsign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Callsign  string `json:"callsign"`
		Frequency string `json:"frequency"`
		Mode      string `json:"mode"`
	}

	if err := s.decodeJSONBody(r, &req); err != nil {
		s.sendError(w, "Invalid request")
		return
	}

	if req.Callsign == "" {
		s.sendError(w, "Callsign is required")
		return
	}

	// Envoyer le callsign à Log4OM
	SendUDPMessage([]byte("<CALLSIGN>" + req.Callsign))
	s.Log.Debugf("Sent callsign %s to Log4OM via UDP (127.0.0.1:2241)", req.Callsign)

	// Si configuré, envoyer freq/mode à Log4OM
	if Cfg.General.SendFreqModeToLog {
		s.sendFreqModeToLog4OM(req.Frequency, req.Mode)
	} else if req.Frequency != "" && s.isFlexConnected() {
		// Sinon, contrôler le FlexRadio directement
		s.tuneFlexRadio(req.Frequency, req.Mode)
	}

	s.sendSuccess(w, map[string]string{
		"callsign":  req.Callsign,
		"frequency": req.Frequency,
	}, "Callsign sent to Log4OM and radio tuned")
}

// handleTuneFromToast is called when the user clicks a Windows toast notification.
// It tunes the radio to the spot frequency, identical to clicking a spot in the UI.
func (s *HTTPServer) handleTuneFromToast(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	callsign := strings.ToUpper(strings.TrimSpace(q.Get("callsign")))
	frequency := strings.TrimSpace(q.Get("freq"))
	mode := strings.ToUpper(strings.TrimSpace(q.Get("mode")))

	if callsign != "" {
		SendUDPMessage([]byte("<CALLSIGN>" + callsign))
		s.Log.Debugf("Toast click: sent callsign %s to Log4OM", callsign)
	}

	if frequency != "" {
		if Cfg.General.SendFreqModeToLog {
			s.sendFreqModeToLog4OM(frequency, mode)
		} else if s.isFlexConnected() {
			s.tuneFlexRadio(frequency, mode)
		}
	}

	// Redirect to the dashboard so the browser opens/focuses on it
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *HTTPServer) sendFreqModeToLog4OM(frequency, mode string) {
	freqLog4OM := strings.Replace(frequency, ".", "", 1)

	// Frequency request
	xmlFreq := RemoteControlRequest{
		MessageId:            uuid.New().String(),
		RemoteControlMessage: "SetTxFrequency",
		Frequency:            freqLog4OM,
	}
	if xmlBytes, err := xml.MarshalIndent(xmlFreq, "", "  "); err == nil {
		SendUDPMessage(xmlBytes)
	} else {
		s.Log.Errorf("Failed to marshal frequency XML: %v", err)
	}

	// Mode request
	xmlMode := RemoteControlRequest{
		MessageId:            uuid.New().String(),
		RemoteControlMessage: "SetMode",
		Mode:                 mode,
	}
	if xmlBytes, err := xml.MarshalIndent(xmlMode, "", "  "); err == nil {
		SendUDPMessage(xmlBytes)
	} else {
		s.Log.Errorf("Failed to marshal mode XML: %v", err)
	}
}

func (s *HTTPServer) tuneFlexRadio(frequency, mode string) {
	tuneCmd := fmt.Sprintf("C%v|slice tune 0 %s", CommandNumber, frequency)
	s.FlexClient.Write(tuneCmd)
	CommandNumber++

	time.Sleep(time.Millisecond * 500)

	modeCmd := fmt.Sprintf("C%v|slice s 0 mode=%s", CommandNumber, mode)
	s.FlexClient.Write(modeCmd)
	CommandNumber++

	s.Log.Infof("Sent TUNE command to Flex: %s", tuneCmd)

	time.Sleep(time.Millisecond * 100)

	s.FlexClient.ZoomPanadapter(mode, frequency)
	s.FlexClient.AdjustAGC(mode)
	s.Log.Infof("AGC Mode adjusted to mode: %s", mode)
}

func (s *HTTPServer) shutdownApp(w http.ResponseWriter, r *http.Request) {
	s.Log.Info("Shutdown request received from dashboard")
	s.sendSuccess(w, map[string]string{"message": "Shutting down FlexDXCluster"}, "")

	go func() {
		time.Sleep(500 * time.Millisecond)
		GracefulShutdown(s.TCPClients, s.TCPServer, s.FlexClient, s.FlexRepo, s.ContactRepo)
		os.Exit(0)
	}()
}

// ============================================================================
// API HANDLERS - External Data
// ============================================================================

func (s *HTTPServer) HandleADXO(w http.ResponseWriter, r *http.Request) {
	if adxoCache.NeedsRefresh() {
		go refreshADXO(s.broadcast, s.Watchlist)
	}
	s.sendSuccess(w, adxoCache.Get(), "")
}

func (s *HTTPServer) HandleDXWorld(w http.ResponseWriter, r *http.Request) {
	if dxwCache.NeedsRefresh() {
		go refreshDXWorld(s.broadcast)
	}
	s.sendSuccess(w, dxwCache.Get(), "")
}

func (s *HTTPServer) HandleQRZ(w http.ResponseWriter, r *http.Request) {
	call := mux.Vars(r)["call"]
	result, err := QRZLookup(call)
	if err != nil {
		s.sendError(w, err.Error())
		return
	}
	s.sendSuccess(w, result, "")
}

func (s *HTTPServer) HandleSolarData(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("https://www.hamqsl.com/solarxml.php")
	if err != nil {
		s.Log.Errorf("Error fetching solar data: %v", err)
		s.sendError(w, "Failed to fetch solar data")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.Log.Errorf("Error reading solar data: %v", err)
		s.sendError(w, "Failed to read solar data")
		return
	}

	var solarXML SolarXML
	if err := xml.Unmarshal(body, &solarXML); err != nil {
		s.Log.Errorf("Error parsing solar XML: %v", err)
		s.sendError(w, "Failed to parse solar data")
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"sfi":      solarXML.Data.SolarFlux,
		"sunspots": solarXML.Data.Sunspots,
		"aIndex":   solarXML.Data.AIndex,
		"kIndex":   solarXML.Data.KIndex,
		"updated":  solarXML.Data.Updated,
	}, "")
}

// ============================================================================
// CONFIG API
// ============================================================================

func (s *HTTPServer) getConfigAPI(w http.ResponseWriter, r *http.Request) {
	s.sendSuccess(w, configToDTO(), "")
}

func (s *HTTPServer) saveConfigAPI(w http.ResponseWriter, r *http.Request) {
	var dto ConfigDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		s.sendError(w, "invalid request: "+err.Error())
		return
	}

	// Snapshot before modifying — the ConfigWatcher fires after Save() and would
	// see oldCfg == newCfg if we mutated Cfg first, so we do change detection here.
	oldLogLevel := Cfg.General.LogLevel
	oldSQLitePath := Cfg.SQLite.SQLitePath
	oldLogbookType := Cfg.Database.LogbookType
	oldClusters := make([]ClusterConfig, len(Cfg.Clusters))
	copy(oldClusters, Cfg.Clusters)

	Cfg.General.Callsign = dto.General.Callsign
	Cfg.General.Grid = dto.General.Grid
	Cfg.General.LogLevel = dto.General.LogLevel
	Cfg.General.SendFreqModeToLog = dto.General.SendFreqModeToLog
	Cfg.General.FlexRadioSpot = dto.General.FlexRadioSpot
	Cfg.General.TelnetServer = dto.General.TelnetServer
	Cfg.FTx.Enabled = dto.FTx.Enabled
	Cfg.FTx.Multicast = dto.FTx.Multicast
	Cfg.FTx.MulticastIP = dto.FTx.MulticastIP
	Cfg.FTx.Port = dto.FTx.Port
	Cfg.QRZ.Username = dto.QRZ.Username
	Cfg.QRZ.Password = dto.QRZ.Password
	Cfg.Flex.IP = dto.Flex.IP
	Cfg.Flex.Discover = dto.Flex.Discover
	Cfg.Flex.SpotLife = dto.Flex.SpotLife
	Cfg.Gotify.Enable = dto.Gotify.Enable
	Cfg.Gotify.URL = dto.Gotify.URL
	Cfg.Gotify.Token = dto.Gotify.Token
	Cfg.Gotify.NewDXCC = dto.Gotify.NewDXCC
	Cfg.Gotify.NewBand = dto.Gotify.NewBand
	Cfg.Gotify.NewMode = dto.Gotify.NewMode
	Cfg.Gotify.NewBandAndMode = dto.Gotify.NewBandAndMode
	Cfg.Gotify.WatchList = dto.Gotify.WatchList
	Cfg.Gotify.WindowsNotify = dto.Gotify.WindowsNotify

	// Replace the full cluster list from the DTO (handles add/remove correctly).
	newClusters := make([]ClusterConfig, 0, len(dto.Clusters))
	for _, dc := range dto.Clusters {
		// Preserve fields not exposed in the UI (LoginPrompt, Type) from the old entry.
		existing := ClusterConfig{
			LoginPrompt: "login:",
		}
		for _, old := range oldClusters {
			if old.Name == dc.Name || (old.Server == dc.Server && old.Port == dc.Port) {
				existing = old
				break
			}
		}
		existing.Name = dc.Name
		existing.Server = dc.Server
		existing.Port = dc.Port
		existing.Login = dc.Login
		existing.Password = dc.Password
		existing.Enabled = dc.Enabled
		existing.Master = dc.Master
		existing.Skimmer = dc.Skimmer
		existing.FT8 = dc.FT8
		existing.FT4 = dc.FT4
		existing.Beacon = dc.Beacon
		existing.Command = dc.Command
		newClusters = append(newClusters, existing)
	}
	Cfg.Clusters = newClusters

	if err := Cfg.Save(s.ConfigPath); err != nil {
		s.sendError(w, "failed to write config: "+err.Error())
		return
	}

	// Apply live changes immediately (ConfigWatcher would miss them since Cfg
	// was already mutated before the file-write event fires).
	if oldLogLevel != Cfg.General.LogLevel {
		applyLogLevel(Cfg.General.LogLevel)
	}

	if clusterTopologyChanged(oldClusters, Cfg.Clusters) {
		if active := Cfg.GetActiveClusters(); len(active) > 0 {
			go s.ReloadClusters(active)
		}
	} else {
		oldMaster := getClusterMaster(oldClusters)
		newMaster := getClusterMaster(Cfg.Clusters)
		if oldMaster != nil && newMaster != nil &&
			(oldMaster.FT8 != newMaster.FT8 || oldMaster.FT4 != newMaster.FT4 ||
				oldMaster.Skimmer != newMaster.Skimmer || oldMaster.Beacon != newMaster.Beacon) {
			if mc := s.MasterClient(); mc != nil {
				mc.ReloadFilters()
			}
		}
	}

	if oldSQLitePath != Cfg.SQLite.SQLitePath || oldLogbookType != Cfg.Database.LogbookType {
		go s.ReloadLogbook()
	}

	s.broadcast <- WSMessage{Type: "stats", Data: s.calculateStats()}
	s.sendSuccess(w, configToDTO(), "Configuration saved")
}

func (s *HTTPServer) testQRZConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid request: "+err.Error())
		return
	}
	if req.Username == "" || req.Password == "" {
		s.sendError(w, "username and password required")
		return
	}

	url := fmt.Sprintf("https://xmldata.qrz.com/xml/current/?username=%s&password=%s&agent=FlexDXClusterGui",
		req.Username, req.Password)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		s.sendError(w, "QRZ unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Session struct {
			Key     string `xml:"Key"`
			Error   string `xml:"Error"`
			Message string `xml:"Message"`
		} `xml:"Session"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		s.sendError(w, "QRZ response parse error: "+err.Error())
		return
	}
	if parsed.Session.Error != "" {
		s.sendError(w, "QRZ: "+parsed.Session.Error)
		return
	}
	if parsed.Session.Key == "" {
		s.sendError(w, "QRZ: no session key returned")
		return
	}
	s.sendSuccess(w, map[string]string{"message": parsed.Session.Message}, "QRZ authentication successful")
}

func (s *HTTPServer) testClusterConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Server string `json:"server"`
		Port   string `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid request: "+err.Error())
		return
	}
	if req.Server == "" || req.Port == "" {
		s.sendError(w, "server and port are required")
		return
	}

	addr := req.Server + ":" + req.Port
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		s.sendError(w, "Cannot reach "+addr+": "+err.Error())
		return
	}
	conn.Close()
	s.sendSuccess(w, nil, addr+" is reachable")
}

// ============================================================================
// LIVE RELOAD — clusters & logbook without restart
// ============================================================================

func (s *HTTPServer) ReloadClusters(clusters []ClusterConfig) {
	s.Log.Info("Reloading cluster connections...")

	s.clientsMu.RLock()
	old := make([]*TCPClient, len(s.TCPClients))
	copy(old, s.TCPClients)
	s.clientsMu.RUnlock()

	for _, c := range old {
		c.Close()
	}

	var newClients []*TCPClient
	for _, cfg := range clusters {
		name := cfg.Name
		if name == "" {
			name = cfg.Server
		}
		client := NewTCPClient(s.TCPServer, cfg, s.ContactRepo, s.SpotChan, s.ConsoleChan)
		newClients = append(newClients, client)
		s.Log.Infof("Cluster configured: %s (%s:%s)", name, cfg.Server, cfg.Port)
	}

	s.clientsMu.Lock()
	s.TCPClients = newClients
	s.clientsMu.Unlock()

	for _, c := range newClients {
		go c.StartClient()
	}

	s.Log.Infof("Cluster reload complete: %d client(s)", len(newClients))
	select {
	case s.broadcast <- WSMessage{Type: "stats", Data: s.calculateStats()}:
	default:
	}
}

func (s *HTTPServer) ReloadLogbook() {
	s.Log.Info("Reloading logbook connection...")

	var newRepo LogbookProvider
	if Cfg.Database.LogbookType == "hrd" {
		newRepo = NewHRDContactsRepository(Cfg.SQLite.SQLitePath)
	} else {
		newRepo = NewLog4OMContactsRepository(Cfg.SQLite.SQLitePath)
	}

	old := s.ContactRepo
	s.ContactRepo = newRepo

	s.clientsMu.RLock()
	for _, c := range s.TCPClients {
		c.ContactRepo = newRepo
	}
	s.clientsMu.RUnlock()

	if old != nil {
		old.Close()
	}

	if newRepo != nil {
		if Cfg.Database.LogbookType == "hrd" {
			s.Log.Infof("Logbook reloaded: Ham Radio Deluxe — %d contacts", newRepo.CountEntries())
		} else {
			s.Log.Infof("Logbook reloaded: Log4OM — %d contacts", newRepo.CountEntries())
		}
	} else {
		s.Log.Warn("Logbook reload: not connected — check sqlite_path in config.yml")
	}

	select {
	case s.broadcast <- WSMessage{Type: "logbookType", Data: Cfg.Database.LogbookType}:
	default:
	}
}

// ============================================================================
// STATS & MILESTONES
// ============================================================================

func (s *HTTPServer) calculateStats() Stats {
	allSpots := s.FlexRepo.GetAllSpots("0")

	newDXCCCount := 0
	for _, spot := range allSpots {
		if spot.NewDXCC {
			newDXCCCount++
		}
	}

	clusterStatus := "disconnected"
	clusterType := "unknown"
	if s.isClusterConnected() {
		clusterStatus = "connected"
		clusterType = s.MasterClient().ClusterType
	}

	flexStatus := "disconnected"
	if s.isFlexConnected() {
		flexStatus = "connected"
	}

	received, processed, rejected := GetSpotStats()

	// Construire la liste des clusters avec leur statut
	var clusterInfos []ClusterInfo
	for _, c := range s.TCPClients {
		status := "disconnected"
		if c.LoggedIn {
			status = "connected"
		}
		clusterInfos = append(clusterInfos, ClusterInfo{
			Name:   c.ClusterCfg.Name,
			Master: c.ClusterCfg.Master,
			Status: status,
			Type:   c.ClusterType,
		})
	}

	return Stats{
		TotalSpots:       len(allSpots),
		NewDXCC:          newDXCCCount,
		ConnectedClients: len(s.TCPServer.Clients),
		TotalContacts: func() int {
			if s.ContactRepo == nil {
				return 0
			}
			return s.ContactRepo.CountEntries()
		}(),
		ClusterStatus:    clusterStatus,
		ClusterType:      clusterType,
		Clusters:         clusterInfos,
		FlexStatus:       flexStatus,
		MyCallsign:       Cfg.General.Callsign,
		MyGrid:           Cfg.General.Grid,
		Filters: func() Filters {
			if m := getClusterMaster(Cfg.Clusters); m != nil {
				return Filters{Skimmer: m.Skimmer, FT8: m.FT8, FT4: m.FT4, Beacon: m.Beacon}
			}
			return Filters{}
		}(),
		SpotsReceived:    received,
		SpotsProcessed:   processed,
		SpotsRejected:    rejected,
		SpotSuccessRate:  GetSpotSuccessRate(),
		ContestMode:      Cfg.General.ContestMode,
		ContestPrefix:    Cfg.General.ContestPrefix,
		ContestCallsigns: Cfg.General.ContestCallsigns,
		LoTWReady:        lotwReady,
		LoTWCount:        lotwCount,
		FTxEnabled:       Cfg.FTx.Enabled,
	}
}

func (s *HTTPServer) checkQSOMilestones(todayCount int) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()

	if todayCount == s.lastQSOCount {
		return
	}

	milestones := []int{5, 10, 25, 50, 100, 200, 500}
	for _, milestone := range milestones {
		if todayCount >= milestone && s.lastQSOCount < milestone {
			s.broadcast <- WSMessage{
				Type: "milestone",
				Data: map[string]interface{}{
					"type":    "qso",
					"count":   milestone,
					"message": fmt.Sprintf("🎉 %d QSOs today!", milestone),
				},
			}
		}
	}

	s.lastQSOCount = todayCount
}

func (s *HTTPServer) checkBandOpening(spots []FlexSpot) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()

	bandCounts := make(map[string]int)
	for _, spot := range spots {
		bandCounts[spot.Band]++
	}

	monitoredBands := []string{"6M", "10M", "12M"}
	now := time.Now()

	for _, band := range monitoredBands {
		count := bandCounts[band]
		if count >= 20 {
			lastSeen, exists := s.lastBandOpening[band]
			if !exists || now.Sub(lastSeen) > 2*time.Hour {
				s.lastBandOpening[band] = now
				s.broadcast <- WSMessage{
					Type: "milestone",
					Data: map[string]interface{}{
						"type":    "band",
						"band":    band,
						"count":   count,
						"message": fmt.Sprintf("📡 %s opening detected! (%d spots)", band, count),
					},
				}
			}
		}
	}
}

// ============================================================================
// MIDDLEWARE & STATIC FILES
// ============================================================================

func (s *HTTPServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *HTTPServer) setupStaticFiles() {
	distFS, err := fs.Sub(frontendFiles, "frontend/dist")
	if err != nil {
		s.Log.Fatal("Cannot load frontend files:", err)
	}

	spaHandler := http.FileServer(http.FS(distFS))
	s.Router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path != "/" {
			file, err := distFS.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				file.Close()
				spaHandler.ServeHTTP(w, r)
				return
			}
		}

		r.URL.Path = "/"
		spaHandler.ServeHTTP(w, r)
	}))
}

func (s *HTTPServer) Start() {
	s.Log.Infof("HTTP Server starting on port %s", s.Port)
	s.Log.Infof("Dashboard available at http://localhost:%s", s.Port)

	if err := http.ListenAndServe(":"+s.Port, s.Router); err != nil {
		s.Log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
