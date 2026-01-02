package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var Mutex sync.Mutex

func ParseFlags() (string, error) {
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

	// Validate the path first
	if err := ValidateConfigPath(configPath); err != nil {
		return "", err
	}

	// Return the configuration path
	return configPath, nil
}

func GracefulShutdown(tcpClient *TCPClient, tcpServer *TCPServer, flexClient *FlexClient, flexRepo *FlexDXClusterRepository, contactRepo *Log4OMContactsRepository) {
	Log.Info("Starting graceful shutdown...")

	// Fermer les clients
	if tcpClient != nil {
		tcpClient.Close()
	}
	if flexClient != nil {
		flexClient.Close()
	}

	// Fermer les serveurs
	if tcpServer != nil {
		// tcpServer.Close() si tu as une méthode close
	}

	// Fermer les bases de données
	if flexRepo != nil && flexRepo.db != nil {
		flexRepo.db.Close()
	}
	if contactRepo != nil && contactRepo.db != nil {
		contactRepo.db.Close()
	}

	// ✅ Fermer le log en dernier
	Log.Info("Shutdown complete")
	CloseLog()
}

func main() {

	// Generate our config based on the config supplied
	// by the user in the flags
	cfgPath, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}

	NewConfig(cfgPath)

	fmt.Printf("\n=== CONTEST MODE CONFIG ===\n")
	fmt.Printf("Contest Mode: %v\n", Cfg.General.ContestMode)
	fmt.Printf("Contest Prefix: %s\n", Cfg.General.ContestPrefix)
	fmt.Printf("Contest Callsigns: %v\n", Cfg.General.ContestCallsigns)
	fmt.Printf("===========================\n\n")

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

	// Load country.xml to get all the DXCC number
	Countries := LoadCountryFile()
	log.Debug("XML Country File has been loaded properly.")

	// Database to keep track of all spots
	fRepo := NewFlexDXDatabase("flex.sqlite")
	defer fRepo.db.Close()

	// Database connection to Log4OM
	cRepo := NewLog4OMContactsRepository(Cfg.SQLite.SQLitePath)
	defer cRepo.db.Close()
	contacts := cRepo.CountEntries()
	log.Infof("Log4OM Database Contains %v Contacts", contacts)

	// ✅ Créer le canal pour le traitement centralisé des spots
	SpotChanToHTTPServer := make(chan TelnetSpot, 100)

	// Initialize servers and clients
	TCPServer := NewTCPServer(Cfg.TelnetServer.Host, Cfg.TelnetServer.Port)
	TCPClient := NewTCPClient(TCPServer, Countries, cRepo, SpotChanToHTTPServer)
	FlexClient := NewFlexClient(*fRepo, TCPServer, nil, nil)

	// Initialize HTTP Server for Dashboard
	HTTPServer := NewHTTPServer(fRepo, cRepo, TCPServer, TCPClient, FlexClient, "8080")
	InitLogHook()

	log.Info("Running FlexDXCluster version 2.1")
	log.Infof("Callsign: %s", Cfg.General.Callsign)
	log.Info("Config hot reload enabled")

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

	// Start all services
	go FlexClient.StartFlexClient()
	go TCPClient.StartClient()
	go TCPServer.StartServer()
	go HTTPServer.Start()

	log.Infof("Telnet Server: %s:%s", Cfg.TelnetServer.Host, Cfg.TelnetServer.Port)
	log.Infof("Cluster: %s:%s", Cfg.Cluster.Server, Cfg.Cluster.Port)

	CheckSignal(TCPClient, TCPServer, FlexClient, fRepo, cRepo)

}
