package main

import (
	"flag"
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

func main() {

	// Generate our config based on the config supplied
	// by the user in the flags
	cfgPath, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}

	cfg := NewConfig(cfgPath)

	log := NewLog()
	log.Info("Running FlexDXCluster version 2.0")
	log.Infof("Callsign: %s", cfg.General.Callsign)

	DeleteDatabase("./flex.sqlite", log)

	log.Debugf("Gotify Push Enabled: %v", cfg.Gotify.Enable)
	if cfg.Gotify.Enable {
		log.Debugf("Gotify Push NewDXCC: %v - NewBand: %v - NewMode: %v - NewBandAndMode: %v", cfg.Gotify.NewDXCC, cfg.Gotify.NewBand, cfg.Gotify.NewMode, cfg.Gotify.NewBandAndMode)
	}

	// Load country.xml to get all the DXCC number
	Countries := LoadCountryFile()
	log.Debug("XML Country File has been loaded properly.")

	// Database to keep track of all spots
	fRepo := NewFlexDXDatabase("flex.sqlite")
	defer fRepo.db.Close()

	// Database connection to Log4OM
	cRepo := NewLog4OMContactsRepository(cfg.SQLite.SQLitePath)
	defer cRepo.db.Close()
	contacts := cRepo.CountEntries()
	log.Infof("Log4OM Database Contains %v Contacts", contacts)
	defer cRepo.db.Close()

	// Initialize servers and clients
	TCPServer := NewTCPServer(cfg.TelnetServer.Host, cfg.TelnetServer.Port)
	TCPClient := NewTCPClient(TCPServer, Countries, cRepo)
	FlexClient := NewFlexClient(*fRepo, TCPServer, TCPClient.SpotChanToFlex, nil)

	// Initialize HTTP Server for Dashboard
	HTTPServer := NewHTTPServer(fRepo, cRepo, TCPServer, TCPClient, FlexClient, "8080")

	FlexClient.HTTPServer = HTTPServer

	// Start all services
	go FlexClient.StartFlexClient()
	go TCPClient.StartClient()
	go TCPServer.StartServer()
	go HTTPServer.Start()

	log.Infof("Telnet Server: %s:%s", cfg.TelnetServer.Host, cfg.TelnetServer.Port)
	log.Infof("Cluster: %s:%s", cfg.Cluster.Server, cfg.Cluster.Port)

	CheckSignal(TCPClient, TCPServer, FlexClient, fRepo, cRepo)

}
