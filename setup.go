package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type SetupRequest struct {
	Callsign    string            `json:"callsign"`
	Grid        string            `json:"grid"`
	FlexIP      string            `json:"flexIP"`
	SQLitePath  string            `json:"sqlitePath"`
	LogbookType string            `json:"logbookType"`
	Clusters    []SetupClusterDTO `json:"clusters"`
}

type SetupClusterDTO struct {
	Name    string `json:"name"`
	Server  string `json:"server"`
	Port    string `json:"port"`
	Enabled bool   `json:"enabled"`
	Master  bool   `json:"master"`
}

type SetupServer struct {
	router     *mux.Router
	configPath string
	done       chan struct{}
	srv        *http.Server
}

func NewSetupServer(configPath string, done chan struct{}) *SetupServer {
	s := &SetupServer{
		router:     mux.NewRouter(),
		configPath: configPath,
		done:       done,
	}
	s.setupRoutes()
	return s
}

func (s *SetupServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *SetupServer) setupRoutes() {
	s.router.Use(s.corsMiddleware)
	s.router.HandleFunc("/api/setup-required", s.handleSetupRequired).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/setup", s.handleSetup).Methods("POST", "OPTIONS")
	s.setupStaticFiles()
}

func (s *SetupServer) setupStaticFiles() {
	distFS, err := fs.Sub(frontendFiles, "frontend/dist")
	if err != nil {
		log.Fatal("Cannot load frontend files:", err)
	}
	spaHandler := http.FileServer(http.FS(distFS))
	s.router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func (s *SetupServer) handleSetupRequired(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"required": true})
}

func (s *SetupServer) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	callsign := strings.ToUpper(strings.TrimSpace(req.Callsign))

	cfg := &Config{}
	cfg.General.Callsign = callsign
	cfg.General.Grid = strings.ToUpper(strings.TrimSpace(req.Grid))
	cfg.General.LogLevel = "INFO"
	cfg.General.FlexRadioSpot = true
	cfg.General.TelnetServer = true
	cfg.General.DeleteLogFileAtStart = true
	cfg.General.ContestPrefix = "WWA"
	cfg.Flex.IP = strings.TrimSpace(req.FlexIP)
	cfg.Flex.Discover = req.FlexIP == ""
	cfg.Database.SQLite = true
	cfg.Database.LogbookType = req.LogbookType
	cfg.SQLite.SQLitePath = strings.TrimSpace(req.SQLitePath)
	cfg.TelnetServer.Host = "0.0.0.0"
	cfg.TelnetServer.Port = "7300"

	clusters := make([]ClusterConfig, len(req.Clusters))
	for i, cl := range req.Clusters {
		clusters[i] = ClusterConfig{
			Name:        cl.Name,
			Server:      cl.Server,
			Port:        cl.Port,
			Enabled:     cl.Enabled,
			Master:      cl.Master,
			Login:       callsign,
			LoginPrompt: "login:",
		}
	}
	cfg.Clusters = clusters

	if err := cfg.Save(s.configPath); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})

	go func() {
		time.Sleep(200 * time.Millisecond)
		s.done <- struct{}{}
	}()
}

func (s *SetupServer) Start() {
	s.srv = &http.Server{
		Addr:    ":8080",
		Handler: s.router,
	}
	log.Printf("Setup wizard available at http://localhost:8080")
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Setup server error: %v", err)
	}
}

func (s *SetupServer) Stop() {
	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.srv.Shutdown(ctx)
	}
}

func runSetupMode(cfgPath string) {
	done := make(chan struct{}, 1)
	setupSrv := NewSetupServer(cfgPath, done)
	go setupSrv.Start()
	log.Printf("Config not found — first-run setup wizard started")
	<-done
	setupSrv.Stop()
	time.Sleep(500 * time.Millisecond) // let OS release the port before main HTTP server binds
	log.Printf("Setup complete, starting services...")
}
