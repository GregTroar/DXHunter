package main

import (
	"embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
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

type HTTPServer struct {
	Router          *mux.Router
	FlexRepo        *FlexDXClusterRepository
	ContactRepo     *Log4OMContactsRepository
	TCPServer       *TCPServer
	TCPClient       *TCPClient
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
}

type Stats struct {
	TotalSpots       int     `json:"totalSpots"`
	ActiveSpotters   int     `json:"activeSpotters"`
	NewDXCC          int     `json:"newDXCC"`
	ConnectedClients int     `json:"connectedClients"`
	TotalContacts    int     `json:"totalContacts"`
	ClusterStatus    string  `json:"clusterStatus"`
	FlexStatus       string  `json:"flexStatus"`
	MyCallsign       string  `json:"myCallsign"`
	Mode             string  `json:"mode"`
	Filters          Filters `json:"filters"`
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

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type SendCallsignRequest struct {
	Callsign string `json:"callsign"`
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
	Worked          bool   `json:"worked"`
	WorkedBandMode  bool   `json:"workedBandMode"`
}

type RemoteControlRequestFreq struct {
	XMLName              xml.Name `xml:"RemoteControlRequest"`
	MessageId            string   `xml:"MessageId"`
	RemoteControlMessage string   `xml:"RemoteControlMessage"`
	Frequency            string   `xml:"Frequency"`
}

type RemoteControlRequestMode struct {
	XMLName              xml.Name `xml:"RemoteControlRequest"`
	MessageId            string   `xml:"MessageId"`
	RemoteControlMessage string   `xml:"RemoteControlMessage"`
	Mode                 string   `xml:"Mode"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

func NewHTTPServer(flexRepo *FlexDXClusterRepository, contactRepo *Log4OMContactsRepository,
	tcpServer *TCPServer, tcpClient *TCPClient, flexClient *FlexClient, port string) *HTTPServer {

	server := &HTTPServer{
		Router:          mux.NewRouter(),
		FlexRepo:        flexRepo,
		ContactRepo:     contactRepo,
		TCPServer:       tcpServer,
		TCPClient:       tcpClient,
		FlexClient:      flexClient,
		Port:            port,
		Log:             Log,
		wsClients:       make(map[*websocket.Conn]bool),
		broadcast:       make(chan WSMessage, 256),
		Watchlist:       NewWatchlist("watchlist.json"),
		lastQSOCount:    0,
		lastBandOpening: make(map[string]time.Time),
	}

	server.setupRoutes()
	go server.handleBroadcasts()
	go server.broadcastUpdates()

	return server
}

func (s *HTTPServer) setupRoutes() {
	// Enable CORS
	s.Router.Use(s.corsMiddleware)

	// API Routes
	api := s.Router.PathPrefix("/api").Subrouter()

	api.HandleFunc("/stats", s.getStats).Methods("GET", "OPTIONS")
	api.HandleFunc("/spots", s.getSpots).Methods("GET", "OPTIONS")
	api.HandleFunc("/spots/{id}", s.getSpotByID).Methods("GET", "OPTIONS")
	api.HandleFunc("/spotters", s.getTopSpotters).Methods("GET", "OPTIONS")
	api.HandleFunc("/contacts", s.getContacts).Methods("GET", "OPTIONS")
	api.HandleFunc("/filters", s.updateFilters).Methods("POST", "OPTIONS")
	api.HandleFunc("/shutdown", s.shutdownApp).Methods("POST", "OPTIONS")
	api.HandleFunc("/send-callsign", s.handleSendCallsign).Methods("POST", "OPTIONS")
	api.HandleFunc("/watchlist", s.getWatchlist).Methods("GET", "OPTIONS")
	api.HandleFunc("/watchlist/add", s.addToWatchlist).Methods("POST", "OPTIONS")
	api.HandleFunc("/watchlist/remove", s.removeFromWatchlist).Methods("DELETE", "OPTIONS")
	api.HandleFunc("/solar", s.HandleSolarData).Methods("GET", "OPTIONS")
	api.HandleFunc("/log/recent", s.getRecentQSOs).Methods("GET", "OPTIONS")
	api.HandleFunc("/log/stats", s.getLogStats).Methods("GET", "OPTIONS")
	api.HandleFunc("/log/dxcc-progress", s.getDXCCProgress).Methods("GET", "OPTIONS")
	api.HandleFunc("/watchlist/spots", s.getWatchlistSpotsWithStatus).Methods("GET", "OPTIONS")

	// WebSocket endpoint
	api.HandleFunc("/ws", s.handleWebSocket).Methods("GET")

	s.setupStaticFiles()
}

func (s *HTTPServer) setupStaticFiles() {
	// Obtenir le sous-système de fichiers depuis dist/
	distFS, err := fs.Sub(frontendFiles, "frontend/dist")
	if err != nil {
		s.Log.Fatal("Cannot load frontend files:", err)
	}

	spaHandler := http.FileServer(http.FS(distFS))
	s.Router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		path := r.URL.Path

		// Vérifier si le fichier existe
		if path != "/" {
			file, err := distFS.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				file.Close()
				spaHandler.ServeHTTP(w, r)
				return
			}
		}

		// Si pas trouvé ou racine, servir index.html
		r.URL.Path = "/"
		spaHandler.ServeHTTP(w, r)
	}))
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

	// Send initial data
	s.sendInitialData(conn)

	// Keep connection alive and handle client messages
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

	// Read messages from client (for ping/pong)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.Log.Errorf("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (s *HTTPServer) sendInitialData(conn *websocket.Conn) {
	// Send initial stats
	stats := s.calculateStats()
	conn.WriteJSON(WSMessage{Type: "stats", Data: stats})

	// Send initial spots
	spots := s.FlexRepo.GetAllSpots("0")
	conn.WriteJSON(WSMessage{Type: "spots", Data: spots})

	// Send initial spotters
	spotters := s.FlexRepo.GetSpotters()
	conn.WriteJSON(WSMessage{Type: "spotters", Data: spotters})

	// Send initial watchlist
	watchlist := s.Watchlist.GetAll()
	conn.WriteJSON(WSMessage{Type: "watchlist", Data: watchlist})

	// Send initial log data
	qsos := s.ContactRepo.GetRecentQSOs("13")
	conn.WriteJSON(WSMessage{Type: "log", Data: qsos})

	logStats := s.ContactRepo.GetQSOStats()
	conn.WriteJSON(WSMessage{Type: "logStats", Data: logStats})

	dxccCount := s.ContactRepo.GetDXCCCount()
	dxccData := map[string]interface{}{
		"worked":     dxccCount,
		"total":      340,
		"percentage": float64(dxccCount) / 340.0 * 100.0,
	}
	conn.WriteJSON(WSMessage{Type: "dxccProgress", Data: dxccData})
}

func (s *HTTPServer) handleBroadcasts() {
	for msg := range s.broadcast {
		s.wsMutex.RLock()
		for client := range s.wsClients {
			err := client.WriteJSON(msg)
			if err != nil {
				s.Log.Errorf("WebSocket write error: %v", err)
				client.Close()
				s.wsMutex.RUnlock()
				s.wsMutex.Lock()
				delete(s.wsClients, client)
				s.wsMutex.Unlock()
				s.wsMutex.RLock()
			}
		}
		s.wsMutex.RUnlock()
	}
}

func (s *HTTPServer) broadcastUpdates() {
	statsTicker := time.NewTicker(1 * time.Second)
	logTicker := time.NewTicker(10 * time.Second)
	cleanupTicker := time.NewTicker(5 * time.Minute) // ✅ AJOUTER

	defer statsTicker.Stop()
	defer logTicker.Stop()
	defer cleanupTicker.Stop() // ✅ AJOUTER

	for {
		select {
		case <-statsTicker.C:
			s.wsMutex.RLock()
			clientCount := len(s.wsClients)
			s.wsMutex.RUnlock()

			if clientCount == 0 {
				continue
			}

			// Broadcast stats
			stats := s.calculateStats()
			s.broadcast <- WSMessage{Type: "stats", Data: stats}

			// Broadcast spots
			spots := s.FlexRepo.GetAllSpots("0")
			s.checkBandOpening(spots)
			s.broadcast <- WSMessage{Type: "spots", Data: spots}

			// Broadcast spotters
			spotters := s.FlexRepo.GetSpotters()
			s.broadcast <- WSMessage{Type: "spotters", Data: spotters}

		case <-logTicker.C:
			s.wsMutex.RLock()
			clientCount := len(s.wsClients)
			s.wsMutex.RUnlock()

			if clientCount == 0 {
				continue
			}

			// Broadcast log data every 10 seconds
			qsos := s.ContactRepo.GetRecentQSOs("13")
			s.broadcast <- WSMessage{Type: "log", Data: qsos}

			stats := s.ContactRepo.GetQSOStats()
			s.checkQSOMilestones(stats.Today)
			s.broadcast <- WSMessage{Type: "logStats", Data: stats}

			dxccCount := s.ContactRepo.GetDXCCCount()
			dxccData := map[string]interface{}{
				"worked":     dxccCount,
				"total":      340,
				"percentage": float64(dxccCount) / 340.0 * 100.0,
			}

			s.broadcast <- WSMessage{Type: "dxccProgress", Data: dxccData}
		}
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

	// ✅ Seulement surveiller 6M, 10M et 12M
	monitoredBands := []string{"6M", "10M", "12M"}

	now := time.Now()
	for _, band := range monitoredBands {
		count := bandCounts[band]
		if count >= 20 { // Si 20+ spots sur une bande
			lastSeen, exists := s.lastBandOpening[band]
			// Notifier si première fois ou si pas vu depuis 2 heures
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

func (s *HTTPServer) getRecentQSOs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "50"
	}

	qsos := s.ContactRepo.GetRecentQSOs(limitStr)
	s.sendJSON(w, APIResponse{Success: true, Data: qsos})
}

func (s *HTTPServer) getLogStats(w http.ResponseWriter, r *http.Request) {
	stats := s.ContactRepo.GetQSOStats()
	s.sendJSON(w, APIResponse{Success: true, Data: stats})
}

func (s *HTTPServer) getDXCCProgress(w http.ResponseWriter, r *http.Request) {
	workedCount := s.ContactRepo.GetDXCCCount()

	data := map[string]interface{}{
		"worked":     workedCount,
		"total":      340,
		"percentage": float64(workedCount) / 340.0 * 100.0,
	}

	s.sendJSON(w, APIResponse{Success: true, Data: data})
}

func (s *HTTPServer) calculateStats() Stats {
	allSpots := s.FlexRepo.GetAllSpots("0")
	spotters := s.FlexRepo.GetSpotters()
	contacts := s.ContactRepo.CountEntries()

	newDXCCCount := 0
	for _, spot := range allSpots {
		if spot.NewDXCC {
			newDXCCCount++
		}
	}

	clusterStatus := "disconnected"
	if s.TCPClient != nil && s.TCPClient.LoggedIn {
		clusterStatus = "connected"
	}

	flexStatus := "disconnected"
	if s.FlexClient != nil && s.FlexClient.IsConnected {
		flexStatus = "connected"
	}

	return Stats{
		TotalSpots:       len(allSpots),
		ActiveSpotters:   len(spotters),
		NewDXCC:          newDXCCCount,
		ConnectedClients: len(s.TCPServer.Clients),
		TotalContacts:    contacts,
		ClusterStatus:    clusterStatus,
		FlexStatus:       flexStatus,
		MyCallsign:       Cfg.General.Callsign,
		Filters: Filters{
			Skimmer: Cfg.Cluster.Skimmer,
			FT8:     Cfg.Cluster.FT8,
			FT4:     Cfg.Cluster.FT4,
			Beacon:  Cfg.Cluster.Beacon,
		},
	}
}

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

func (s *HTTPServer) getStats(w http.ResponseWriter, r *http.Request) {
	stats := s.calculateStats()
	s.sendJSON(w, APIResponse{Success: true, Data: stats})
}

func (s *HTTPServer) getSpots(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "50"
	}

	spots := s.FlexRepo.GetAllSpots(limitStr)
	s.sendJSON(w, APIResponse{Success: true, Data: spots})
}

func (s *HTTPServer) getSpotByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	spot, err := s.FlexRepo.FindSpotByFlexSpotNumber(id)
	if err != nil {
		s.sendJSON(w, APIResponse{Success: false, Error: "Spot not found"})
		return
	}

	s.sendJSON(w, APIResponse{Success: true, Data: spot})
}

func (s *HTTPServer) getTopSpotters(w http.ResponseWriter, r *http.Request) {
	spotters := s.FlexRepo.GetSpotters()
	s.sendJSON(w, APIResponse{Success: true, Data: spotters})
}

func (s *HTTPServer) getContacts(w http.ResponseWriter, r *http.Request) {
	count := s.ContactRepo.CountEntries()
	data := map[string]interface{}{"totalContacts": count}
	s.sendJSON(w, APIResponse{Success: true, Data: data})
}

type FilterRequest struct {
	Skimmer *bool `json:"skimmer,omitempty"`
	FT8     *bool `json:"ft8,omitempty"`
	FT4     *bool `json:"ft4,omitempty"`
	Beacon  *bool `json:"beacon,omitempty"`
}

func (s *HTTPServer) updateFilters(w http.ResponseWriter, r *http.Request) {
	var req FilterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendJSON(w, APIResponse{Success: false, Error: "Invalid request"})
		return
	}

	commands := []string{}

	if req.Skimmer != nil {
		if *req.Skimmer {
			commands = append(commands, "set/skimmer")
			Cfg.Cluster.Skimmer = true
		} else {
			commands = append(commands, "set/noskimmer")
			Cfg.Cluster.Skimmer = false
		}
	}

	if req.FT8 != nil {
		if *req.FT8 {
			commands = append(commands, "set/ft8")
			Cfg.Cluster.FT8 = true
		} else {
			commands = append(commands, "set/noft8")
			Cfg.Cluster.FT8 = false
		}
	}

	if req.FT4 != nil {
		if *req.FT4 {
			commands = append(commands, "set/ft4")
			Cfg.Cluster.FT4 = true
		} else {
			commands = append(commands, "set/noft4")
			Cfg.Cluster.FT4 = false
		}
	}

	if req.Beacon != nil {
		if *req.Beacon {
			commands = append(commands, "set/beacon")
			Cfg.Cluster.Beacon = true
		} else {
			commands = append(commands, "set/nobeacon")
			Cfg.Cluster.Beacon = false
		}
	}

	for _, cmd := range commands {
		s.TCPClient.CmdChan <- cmd
	}

	s.sendJSON(w, APIResponse{Success: true, Data: map[string]string{"message": "Filters updated successfully"}})
}

func (s *HTTPServer) getWatchlist(w http.ResponseWriter, r *http.Request) {
	callsigns := s.Watchlist.GetAll()
	s.sendJSON(w, APIResponse{Success: true, Data: callsigns})
}

func (s *HTTPServer) addToWatchlist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Callsign string `json:"callsign"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendJSON(w, APIResponse{Success: false, Error: "Invalid request"})
		return
	}

	if req.Callsign == "" {
		s.sendJSON(w, APIResponse{Success: false, Error: "Callsign is required"})
		return
	}

	if err := s.Watchlist.Add(req.Callsign); err != nil {
		s.sendJSON(w, APIResponse{Success: false, Error: err.Error()})
		return
	}

	s.Log.Infof("Added %s to watchlist", req.Callsign)

	// Broadcast updated watchlist to all clients
	s.broadcast <- WSMessage{Type: "watchlist", Data: s.Watchlist.GetAll()}

	s.sendJSON(w, APIResponse{Success: true, Message: "Callsign added to watchlist"})
}

func (s *HTTPServer) removeFromWatchlist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Callsign string `json:"callsign"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendJSON(w, APIResponse{Success: false, Error: "Invalid request"})
		return
	}

	if req.Callsign == "" {
		s.sendJSON(w, APIResponse{Success: false, Error: "Callsign is required"})
		return
	}

	if err := s.Watchlist.Remove(req.Callsign); err != nil {
		s.sendJSON(w, APIResponse{Success: false, Error: err.Error()})
		return
	}

	s.Log.Debugf("Removed %s from watchlist", req.Callsign)

	// Broadcast updated watchlist to all clients
	s.broadcast <- WSMessage{Type: "watchlist", Data: s.Watchlist.GetAll()}

	s.sendJSON(w, APIResponse{Success: true, Message: "Callsign removed from watchlist"})
}

func (s *HTTPServer) getWatchlistSpotsWithStatus(w http.ResponseWriter, r *http.Request) {
	// Récupérer tous les spots
	allSpots := s.FlexRepo.GetAllSpots("0")

	// Récupérer la watchlist
	watchlistCallsigns := s.Watchlist.GetAll()

	// Filtrer les spots de la watchlist
	var relevantSpots []FlexSpot

	for _, spot := range allSpots {
		isInWatchlist := false

		for _, pattern := range watchlistCallsigns {
			if spot.DX == pattern || strings.HasPrefix(spot.DX, pattern) {
				isInWatchlist = true
				break
			}
		}

		if isInWatchlist {
			relevantSpots = append(relevantSpots, spot)
		}
	}

	type BandModeKey struct {
		Band string
		Mode string
	}

	spotsByBandMode := make(map[BandModeKey][]FlexSpot)

	for _, spot := range relevantSpots {
		key := BandModeKey{Band: spot.Band, Mode: spot.Mode}
		spotsByBandMode[key] = append(spotsByBandMode[key], spot)
	}

	var watchlistSpots []WatchlistSpot

	for key, spots := range spotsByBandMode {
		// Extraire les callsigns uniques
		callsignSet := make(map[string]bool)
		for _, spot := range spots {
			callsignSet[spot.DX] = true
		}

		callsigns := make([]string, 0, len(callsignSet))
		for callsign := range callsignSet {
			callsigns = append(callsigns, callsign)
		}

		workedMap := s.ContactRepo.GetWorkedCallsignsBandMode(callsigns, key.Band, key.Mode)

		// Construire les résultats
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
				Worked:          spot.Worked,
				WorkedBandMode:  workedMap[spot.DX],
			})
		}
	}

	s.sendJSON(w, APIResponse{Success: true, Data: watchlistSpots})
}

func (s *HTTPServer) HandleSolarData(w http.ResponseWriter, r *http.Request) {
	// Récupérer les données depuis hamqsl.com
	resp, err := http.Get("https://www.hamqsl.com/solarxml.php")
	if err != nil {
		s.Log.Errorf("Error fetching solar data: %v", err)
		s.sendJSON(w, APIResponse{Success: false, Error: "Failed to fetch solar data"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.Log.Errorf("Error reading solar data: %v", err)
		s.sendJSON(w, APIResponse{Success: false, Error: "Failed to read solar data"})
		return
	}

	var solarXML SolarXML
	err = xml.Unmarshal(body, &solarXML)
	if err != nil {
		s.Log.Errorf("Error parsing solar XML: %v", err)
		s.sendJSON(w, APIResponse{Success: false, Error: "Failed to parse solar data"})
		return
	}

	response := map[string]interface{}{
		"sfi":      solarXML.Data.SolarFlux,
		"sunspots": solarXML.Data.Sunspots,
		"aIndex":   solarXML.Data.AIndex,
		"kIndex":   solarXML.Data.KIndex,
		"updated":  solarXML.Data.Updated,
	}

	s.sendJSON(w, APIResponse{Success: true, Data: response})
}

func (s *HTTPServer) handleSendCallsign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Callsign  string `json:"callsign"`
		Frequency string `json:"frequency"`
		Mode      string `json:"mode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendJSON(w, APIResponse{Success: false, Error: "Invalid request"})
		return
	}

	if req.Callsign == "" {
		s.sendJSON(w, APIResponse{Success: false, Error: "Callsign is required"})
		return
	}

	SendUDPMessage([]byte("<CALLSIGN>" + req.Callsign))
	s.Log.Infof("Sent callsign %s to Log4OM via UDP (127.0.0.1:2241)", req.Callsign)

	if Cfg.General.sendFreqModeToLog4OM {
		freqLog4OM := strings.Replace(req.Frequency, ".", "", 1)

		xmlRequestFreq := RemoteControlRequestFreq{
			MessageId:            uuid.New().String(), // Generate a new unique ID
			RemoteControlMessage: "SetTxFrequency",    // Note: Typo matches your required format
			Frequency:            freqLog4OM,
		}

		xmlRequestMode := RemoteControlRequestMode{
			MessageId:            uuid.New().String(), // Generate a new unique ID
			RemoteControlMessage: "SetMode",           // Note: Typo matches your required format
			Mode:                 req.Mode,
		}

		xmlBytesFreq, err := xml.MarshalIndent(xmlRequestFreq, "", "  ")
		if err != nil {
			s.Log.Errorf("Failed to marshal XML: %v", err)
		} else {
			SendUDPMessage([]byte(xmlBytesFreq))
		}

		xmlBytesMode, err := xml.MarshalIndent(xmlRequestMode, "", "  ")
		if err != nil {
			s.Log.Errorf("Failed to marshal XML: %v", err)
		} else {
			SendUDPMessage([]byte(xmlBytesMode))
		}
	}

	if req.Frequency != "" && s.FlexClient != nil && s.FlexClient.IsConnected {
		tuneCmd := fmt.Sprintf("C%v|slice tune 0 %s", CommandNumber, req.Frequency)
		s.FlexClient.Write(tuneCmd)
		CommandNumber++
		time.Sleep(time.Millisecond * 500)
		modeCmd := fmt.Sprintf("C%v|slice s 0 mode=%s", CommandNumber, req.Mode)
		s.FlexClient.Write(modeCmd)
		CommandNumber++
		s.Log.Infof("Sent TUNE command to Flex: %s", tuneCmd)
	}

	s.sendJSON(w, APIResponse{
		Success: true,
		Message: "Callsign sent to Log4OM and radio tuned",
		Data:    map[string]string{"callsign": req.Callsign, "frequency": req.Frequency},
	})
}

func (s *HTTPServer) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *HTTPServer) shutdownApp(w http.ResponseWriter, r *http.Request) {
	s.Log.Info("Shutdown request received from dashboard")

	s.sendJSON(w, APIResponse{Success: true, Data: map[string]string{"message": "Shutting down FlexDXCluster"}})

	go func() {
		time.Sleep(500 * time.Millisecond)
		s.Log.Info("Initiating shutdown...")
		os.Exit(0)
	}()
}

func (s *HTTPServer) Start() {
	s.Log.Infof("HTTP Server starting on port %s", s.Port)
	s.Log.Infof("Dashboard available at http://localhost:%s", s.Port)

	if err := http.ListenAndServe(":"+s.Port, s.Router); err != nil {
		s.Log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
