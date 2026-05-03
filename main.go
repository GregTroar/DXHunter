package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var Mutex sync.Mutex

func ParseFlags() (string, bool, error) {
	// String that contains the configured configuration path
	var configPath string

	exe, _ := os.Executable()
	defaultCfgPath := filepath.Dir(exe)
	defaultCfgPath = filepath.Join(defaultCfgPath, "/config.yml")
	// Set up a CLI flag called "-config" to allow users
	// to supply the configuration file
	flag.StringVar(&configPath, "config", defaultCfgPath, "path to config file")

	// Actually parse the flags
	flag.Parse()

	// Check if config exists — missing config triggers first-run setup wizard
	if err := ValidateConfigPath(configPath); err != nil {
		// File doesn't exist: setup required (not a fatal error)
		if os.IsNotExist(err) {
			return configPath, true, nil
		}
		return "", false, err
	}

	// Return the configuration path
	return configPath, false, nil
}

func GracefulShutdown(tcpClients []*TCPClient, tcpServer *TCPServer, flexClient *FlexClient, flexRepo *FlexDXClusterRepository, contactRepo LogbookProvider) {
	Log.Info("Starting graceful shutdown...")

	for _, client := range tcpClients {
		if client != nil {
			client.Close()
		}
	}
	if flexClient != nil {
		flexClient.Close()
	}
	if flexRepo != nil && flexRepo.db != nil {
		flexRepo.db.Close()
	}
	if contactRepo != nil {
		contactRepo.Close()
	}

	Log.Info("Shutdown complete")
	CloseLog()
}

func main() {

	// Generate our config based on the config supplied
	// by the user in the flags
	cfgPath, setupRequired, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}

	if setupRequired {
		runSetupMode(cfgPath)
	}

	NewConfig(cfgPath)

	configWatcher, err := NewConfigWatcher(cfgPath)
	if err != nil {
		log.Fatalf("Could not create config watcher: %v", err)
	}
	defer configWatcher.Stop()

	if err := configWatcher.Start(); err != nil {
		log.Fatalf("Could not start config watcher: %v", err)
	}

	log := NewLog()
	defer CloseLog()

	DeleteDatabase("./flex.sqlite", log)

	// Load cty.plist — base DXCC disponible globalement via ctyDB
	ctyPath := resolveCtyPath()
	db, err := LoadCtyPlist(ctyPath)
	if err != nil {
		log.Fatalf("Failed to load cty.plist from %s: %v", ctyPath, err)
	}
	ctyDB = db
	log.Infof("cty.plist loaded: %d entries (from %s)", len(ctyDB.entries), ctyPath)

	// Mise à jour automatique de cty.plist au démarrage (en arrière-plan)
	go func() {
		result, err := UpdateCtyPlist(ctyPath)
		if err != nil {
			log.Errorf("cty.plist auto-update failed: %v", err)
			return
		}
		log.Debugf("cty.plist auto-update: %s", result.Message)
	}()

	// Database to keep track of all spots
	fRepo := NewFlexDXDatabase("flex.sqlite")
	defer fRepo.db.Close()

	// POTA cache (persistent across restarts)
	potaCachePath := resolveSiblingPath("pota.sqlite")
	if pc, err := NewPOTACache(potaCachePath); err != nil {
		log.Warnf("POTA cache unavailable: %v", err)
	} else {
		potaCache = pc
		defer pc.Close()
		log.Infof("POTA cache initialized at %s", potaCachePath)
	}

	// Logbook database connection (Log4OM SQLite, HRD SQLite, or MySQL)
	var cRepo LogbookProvider
	if Cfg.Database.LogbookType == "hrd" {
		cRepo = NewHRDContactsRepository(Cfg.SQLite.SQLitePath)
	} else {
		cRepo = NewLog4OMContactsRepository(Cfg.SQLite.SQLitePath)
	}
	if cRepo != nil {
		defer cRepo.Close()
		globalLogbookCache = NewLogbookCache(cRepo)
		globalLogbookCache.StartAutoRefresh(cRepo, 30*time.Second)
	}

	// ✅ Créer le canal pour le traitement centralisé des spots
	SpotChanToHTTPServer := make(chan TelnetSpot, 500)

	// Créer le canal console (partagé entre TCPClient et HTTPServer)
	// Buffer large : seuls les messages non-spot passent ici, mais on garde de la marge
	consoleChan := make(chan string, 500)

	// Initialize servers and clients
	TCPServer := NewTCPServer(Cfg.TelnetServer.Host, Cfg.TelnetServer.Port)

	// Multi-cluster : instancier un TCPClient par cluster actif
	clusters := Cfg.GetActiveClusters()
	if len(clusters) == 0 {
		log.Fatal("No cluster configured. Please check your config.yml")
	}

	var TCPClients []*TCPClient
	for _, clusterCfg := range clusters {
		clusterCfg := clusterCfg // capture locale pour éviter le piège de closure Go
		name := clusterCfg.Name
		if name == "" {
			name = clusterCfg.Server
		}
		client := NewTCPClient(TCPServer, clusterCfg, cRepo, SpotChanToHTTPServer, consoleChan)
		TCPClients = append(TCPClients, client)
		log.Infof("Configured cluster: %s (%s:%s)", name, clusterCfg.Server, clusterCfg.Port)
	}

	FlexClient := NewFlexClient(*fRepo, TCPServer, nil, nil)

	// Initialize HTTP Server for Dashboard
	HTTPServer := NewHTTPServer(fRepo, cRepo, TCPServer, TCPClients, FlexClient, "8080", cfgPath, consoleChan, SpotChanToHTTPServer)
	InitLogHook()
	log.Info("Running FlexDXCluster version 2.41")
	if cRepo != nil {
		if Cfg.Database.LogbookType == "hrd" {
			log.Infof("Logbook: Ham Radio Deluxe — %d contacts", cRepo.CountEntries())
		} else {
			log.Infof("Logbook: Log4OM — %d contacts", cRepo.CountEntries())
		}
	} else {
		log.Warn("Logbook: not connected — check sqlite_path in config.yml")
	}

	// Download LoTW user list in background
	go LoadLoTWUsers()
	log.Infof("Callsign: %s", Cfg.General.Callsign)

	if Cfg.General.ContestMode {
		log.Infof("🏆 Contest Mode: ENABLED")
		log.Infof("🏆 Contest Prefix: %s", Cfg.General.ContestPrefix)
		if len(Cfg.General.ContestCallsigns) > 0 {
			log.Infof("🏆 Contest Special Callsigns: %v", Cfg.General.ContestCallsigns)
		} else {
			log.Info("🏆 No special contest callsigns configured")
		}

		// ✅ AUTO-ADD contest callsigns to watchlist at startup
		if len(Cfg.General.ContestCallsigns) > 0 {
			log.Info("📋 Adding contest callsigns to watchlist...")
			addedCount := 0
			for _, callsign := range Cfg.General.ContestCallsigns {
				if err := HTTPServer.Watchlist.AddContest(callsign); err != nil {
					log.Debugf("Contest callsign %s already in watchlist or error: %v", callsign, err)
				} else {
					log.Infof("✅ Added contest callsign %s to watchlist", callsign)
					addedCount++
				}
			}
			if addedCount > 0 {
				log.Infof("📋 Added %d contest callsigns to watchlist", addedCount)
			}
		}
	} else {
		log.Info("Contest Mode: DISABLED")
	}

	log.Debugf("Gotify Push Enabled: %v", Cfg.Gotify.Enable)
	if Cfg.Gotify.Enable {
		log.Debugf("Gotify Push NewDXCC: %v - NewBand: %v - NewMode: %v - NewBandAndMode: %v", Cfg.Gotify.NewDXCC, Cfg.Gotify.NewBand, Cfg.Gotify.NewMode, Cfg.Gotify.NewBandAndMode)
	}

	FlexClient.HTTPServer = HTTPServer

	spotProcessor := NewSpotProcessor(fRepo, FlexClient, HTTPServer, SpotChanToHTTPServer)

	go spotProcessor.Start()

	// Initialize ClubLog client si cle configuree
	if Cfg.ClubLog.APIKey != "" {
		clubLogClient = NewClubLogClient(Cfg.ClubLog.APIKey)
		if clubLogCache != nil {
			clubLogClient.SetCache(clubLogCache)
		}
		log.Infof("ClubLog API key configured")
	}

	// Start ADXO activations refresher
	StartADXORefresher(HTTPServer.broadcast, HTTPServer.Watchlist)

	// Start DX-World news refresher
	StartDXWorldRefresher(HTTPServer.broadcast)

	// Bridge telnet client commands → master cluster TCPClient
	// Commands arriving from external telnet clients (TCPServer.CmdChan) are forwarded
	// to the master cluster connection so they reach the DX cluster.
	go func() {
		for cmd := range TCPServer.CmdChan {
			master := HTTPServer.MasterClient()
			if master != nil {
				select {
				case master.CmdChan <- cmd:
					log.Debugf("Bridged telnet command to master cluster: %s", cmd)
				default:
					log.Warn("Master cluster CmdChan full, dropping bridged command")
				}
			}
		}
	}()

	// Start all services
	go FlexClient.StartFlexClient()
	for _, client := range TCPClients {
		go client.StartClient()
	}
	go TCPServer.StartServer()
	go HTTPServer.Start()

	log.Infof("Telnet Server: %s:%s", Cfg.TelnetServer.Host, Cfg.TelnetServer.Port)
	for _, cl := range clusters {
		name := cl.Name
		if name == "" {
			name = cl.Server
		}
		log.Infof("Cluster [%s]: %s:%s", name, cl.Server, cl.Port)
	}

	runTray(TCPClients, TCPServer, FlexClient, fRepo, cRepo)
}

// resolveCtyPath retourne le chemin vers cty.plist :
// d'abord à côté de l'exécutable, sinon dans le répertoire courant.
// resolveSiblingPath retourne le chemin d'un fichier a cote de l'executable
func resolveSiblingPath(filename string) string {
	exe, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(exe), filename)
	}
	return filename
}

func resolveCtyPath() string {
	exe, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(exe), "cty.plist")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "cty.plist"
}
