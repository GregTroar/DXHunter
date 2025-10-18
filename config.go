package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

var Cfg *Config

type Config struct {
	General struct {
		DeleteLogFileAtStart       bool   `yaml:"delete_log_file_at_start"`
		LogToFile                  bool   `yaml:"log_to_file"`
		Callsign                   string `yaml:"callsign"`
		LogLevel                   string `yaml:"log_level"`
		TelnetServer               bool   `yaml:"telnetserver"`
		FlexRadioSpot              bool   `yaml:"flexradiospot"`
		SendFreqModeToLog          bool   `yaml:"sendFreqModeToLog4OM"`
		SpotColorNewEntity         string `yaml:"spot_color_new_entity"`
		BackgroundColorNewEntity   string `yaml:"background_color_new_entity"`
		SpotColorNewBand           string `yaml:"spot_color_new_band"`
		BackgroundColorNewBand     string `yaml:"background_color_new_band"`
		SpotColorNewMode           string `yaml:"spot_color_new_mode"`
		BackgroundColorNewMode     string `yaml:"background_color_new_mode"`
		SpotColorNewBandMode       string `yaml:"spot_color_new_band_mode"`
		BackgroundColorNewBandMode string `yaml:"background_color_new_band_mode"`
		SpotColorNewSlot           string `yaml:"spot_color_new_slot"`
		BackgroundColorNewSlot     string `yaml:"background_color_new_slot"`
		SpotColorMyCallsign        string `yaml:"spot_color_my_callsign"`
		BackgroundColorMyCallsign  string `yaml:"background_color_my_callsign"`
		SpotColorWorked            string `yaml:"spot_color_worked"`
		BackgroundColorWorked      string `yaml:"background_color_worked"`
	} `yaml:"general"`

	Database struct {
		MySQL         bool   `yaml:"mysql"`
		SQLite        bool   `yaml:"sqlite"`
		MySQLUser     string `yaml:"mysql_db_user"`
		MySQLPassword string `yaml:"mysql_db_password"`
		MySQLDbName   string `yaml:"mysql_db_name"`
		MySQLHost     string `yaml:"mysql_host"`
		MySQLPort     string `yaml:"mysql_port"`
	} `yaml:"database"`

	SQLite struct {
		SQLitePath string `yaml:"sqlite_path"`
	} `yaml:"sqlite"`

	Cluster struct {
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
	} `yaml:"cluster"`

	Flex struct {
		Discover bool   `yaml:"discovery"`
		IP       string `yaml:"ip"`
		SpotLife string `yaml:"spot_life"`
	} `yaml:"flex"`

	TelnetServer struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"telnetserver"`

	Gotify struct {
		Enable         bool   `yaml:"enable"`
		URL            string `yaml:"url"`
		Token          string `yaml:"token"`
		NewDXCC        bool   `yaml:"NewDXCC"`
		NewBand        bool   `yaml:"NewBand"`
		NewMode        bool   `yaml:"NewMode"`
		NewBandAndMode bool   `yaml:"NewBandAndMode"`
	} `yaml:"gotify"`
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
