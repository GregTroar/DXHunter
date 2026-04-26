package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var Cfg *Config

// ClusterConfig représente la configuration d'un cluster DX
type ClusterConfig struct {
	Name        string `yaml:"name"` // Nom affiché (ex: "EU Cluster")
	Server      string `yaml:"server"`
	Port        string `yaml:"port"`
	Login       string `yaml:"login"`
	Password    string `yaml:"password"`
	Skimmer     bool   `yaml:"skimmer"`
	FT8         bool   `yaml:"ft8"`
	FT4         bool   `yaml:"ft4"`
	Beacon      bool   `yaml:"beacon"`
	Command     string `yaml:"command"`
	LoginPrompt string `yaml:"login_prompt"`
	Enabled     bool   `yaml:"enabled"`
	Master      bool   `yaml:"master"` // Cluster maître : reçoit les commandes et détermine le ClusterType
	Type        string `yaml:"type"`   // Type forcé : dxspider, cc_cluster, ar_cluster (vide = auto-détection)
}

// GetActiveClusters retourne la liste des clusters activés.
func (c *Config) GetActiveClusters() []ClusterConfig {
	var active []ClusterConfig
	for _, cl := range c.Clusters {
		if cl.Enabled && cl.Server != "" {
			active = append(active, cl)
		}
	}
	if len(active) == 0 {
		return nil
	}
	// Si aucun master défini, le premier devient master
	hasMaster := false
	for _, cl := range active {
		if cl.Master {
			hasMaster = true
			break
		}
	}
	if !hasMaster {
		active[0].Master = true
	}
	return active
}

// getClusterMaster retourne le cluster maître d'une liste, ou nil si aucun.
func getClusterMaster(clusters []ClusterConfig) *ClusterConfig {
	for i := range clusters {
		if clusters[i].Master && clusters[i].Enabled {
			return &clusters[i]
		}
	}
	if len(clusters) > 0 && clusters[0].Enabled {
		return &clusters[0]
	}
	return nil
}

type Config struct {
	General struct {
		ContestMode                bool     `yaml:"contest_mode"`
		ContestPrefix              string   `yaml:"contest_prefix"`
		ContestCallsigns           []string `yaml:"contest_callsigns"`
		DeleteLogFileAtStart       bool     `yaml:"delete_log_file_at_start"`
		LogToFile                  bool     `yaml:"log_to_file"`
		Callsign                   string   `yaml:"callsign"`
		Grid                       string   `yaml:"grid"`
		LogLevel                   string   `yaml:"log_level"`
		TelnetServer               bool     `yaml:"telnetserver"`
		FlexRadioSpot              bool     `yaml:"flexradiospot"`
		SendFreqModeToLog          bool     `yaml:"sendFreqModeToLog4OM"`
		SpotColorNewEntity         string   `yaml:"spot_color_new_entity"`
		BackgroundColorNewEntity   string   `yaml:"background_color_new_entity"`
		SpotColorNewBand           string   `yaml:"spot_color_new_band"`
		BackgroundColorNewBand     string   `yaml:"background_color_new_band"`
		SpotColorNewMode           string   `yaml:"spot_color_new_mode"`
		BackgroundColorNewMode     string   `yaml:"background_color_new_mode"`
		SpotColorNewBandMode       string   `yaml:"spot_color_new_band_mode"`
		BackgroundColorNewBandMode string   `yaml:"background_color_new_band_mode"`
		SpotColorNewSlot           string   `yaml:"spot_color_new_slot"`
		BackgroundColorNewSlot     string   `yaml:"background_color_new_slot"`
		SpotColorMyCallsign        string   `yaml:"spot_color_my_callsign"`
		BackgroundColorMyCallsign  string   `yaml:"background_color_my_callsign"`
		SpotColorWorked            string   `yaml:"spot_color_worked"`
		BackgroundColorWorked      string   `yaml:"background_color_worked"`
	} `yaml:"general"`

	Database struct {
		MySQL         bool   `yaml:"mysql"`
		SQLite        bool   `yaml:"sqlite"`
		LogbookType   string `yaml:"logbook_type"` // "log4om" (default) or "hrd"
		MySQLUser     string `yaml:"mysql_db_user"`
		MySQLPassword string `yaml:"mysql_db_password"`
		MySQLDbName   string `yaml:"mysql_db_name"`
		MySQLHost     string `yaml:"mysql_host"`
		MySQLPort     string `yaml:"mysql_port"`
	} `yaml:"database"`

	SQLite struct {
		SQLitePath string `yaml:"sqlite_path"`
	} `yaml:"sqlite"`

	Clusters []ClusterConfig `yaml:"clusters"`

	Flex struct {
		Discover bool   `yaml:"discovery"`
		IP       string `yaml:"ip"`
		SpotLife string `yaml:"spot_life"`
	} `yaml:"flex"`

	TelnetServer struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"telnetserver"`

	FTx struct {
		Enabled     bool   `yaml:"enabled"`
		Multicast   bool   `yaml:"multicast"`
		MulticastIP string `yaml:"multicast_ip"`
		Port        int    `yaml:"port"`
	} `yaml:"ftx"`

	ClubLog struct {
		APIKey string `yaml:"api_key"`
	} `yaml:"clublog"`

	QRZ struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"qrz"`

	Gotify struct {
		Enable         bool   `yaml:"enable"`
		URL            string `yaml:"url"`
		Token          string `yaml:"token"`
		NewDXCC        bool   `yaml:"NewDXCC"`
		NewBand        bool   `yaml:"NewBand"`
		NewMode        bool   `yaml:"NewMode"`
		NewBandAndMode bool   `yaml:"NewBandAndMode"`
		WatchList      bool   `yaml:"Watchlist"`
		WindowsNotify  bool   `yaml:"windows_notify"`
	} `yaml:"gotify"`
}

type ConfigWatcher struct {
	watcher    *fsnotify.Watcher
	configPath string
	mu         sync.RWMutex
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func NewConfig(configPath string) *Config {
	Cfg = &Config{}

	file, err := os.Open(configPath)
	if err != nil {
		log.Println("could not open config file")
	}
	defer file.Close()
	d := yaml.NewDecoder(file)

	if err := d.Decode(&Cfg); err != nil {
		log.Println("could not decode config file")
	}

	return Cfg
}

func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory, not a normal file", path)
	}
	return nil
}

func NewConfigWatcher(configPath string) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &ConfigWatcher{
		watcher:    watcher,
		configPath: configPath,
	}, nil
}

func (cw *ConfigWatcher) Start() error {
	if err := cw.watcher.Add(cw.configPath); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-cw.watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					Log.Info("Config file modified, reloading...")
					cw.reloadConfig()
				}
			case err, ok := <-cw.watcher.Errors:
				if !ok {
					return
				}
				Log.Errorf("Config watcher error: %v", err)
			}
		}
	}()

	return nil
}

func (cw *ConfigWatcher) reloadConfig() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	newCfg := &Config{}
	file, err := os.Open(cw.configPath)
	if err != nil {
		Log.Errorf("Could not reload config: %v", err)
		return
	}
	defer file.Close()

	d := yaml.NewDecoder(file)
	if err := d.Decode(newCfg); err != nil {
		Log.Errorf("Could not decode reloaded config: %v", err)
		return
	}

	// Sauvegarder l'ancienne config
	oldCfg := Cfg

	// Appliquer la nouvelle config
	Cfg = newCfg

	// Vérifier les changements qui nécessitent des actions
	cw.applyConfigChanges(oldCfg, newCfg)

	Log.Info("✅ Config reloaded successfully")
}

func (cw *ConfigWatcher) applyConfigChanges(oldCfg, newCfg *Config) {
	// Log level
	if oldCfg.General.LogLevel != newCfg.General.LogLevel {
		switch newCfg.General.LogLevel {
		case "DEBUG":
			Log.SetLevel(log.DebugLevel)
		case "INFO":
			Log.SetLevel(log.InfoLevel)
		case "WARN":
			Log.SetLevel(log.WarnLevel)
		default:
			Log.SetLevel(log.InfoLevel)
		}
		Log.Infof("Log level changed to %s", newCfg.General.LogLevel)
	}

	// Gotify
	if oldCfg.Gotify.Enable != newCfg.Gotify.Enable {
		Log.Infof("Gotify notifications %s", map[bool]string{true: "enabled", false: "disabled"}[newCfg.Gotify.Enable])
	}

	// Clusters — détecter un changement de filtre sur le cluster maître
	oldMaster := getClusterMaster(oldCfg.Clusters)
	newMaster := getClusterMaster(newCfg.Clusters)
	if oldMaster != nil && newMaster != nil &&
		(oldMaster.FT8 != newMaster.FT8 ||
			oldMaster.FT4 != newMaster.FT4 ||
			oldMaster.Skimmer != newMaster.Skimmer ||
			oldMaster.Beacon != newMaster.Beacon) {
		Log.Info("Cluster filters changed, applying")
		httpServerInstance.MasterClient().ReloadFilters()
	}
}

func (cw *ConfigWatcher) Stop() {
	cw.watcher.Close()
}
