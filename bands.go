package main

import (
	"fmt"
	"strconv"
	"strings"
)

// BandDefinition définit une bande amateur avec ses limites de fréquence
type BandDefinition struct {
	Name       string
	MinFreqMHz float64
	MaxFreqMHz float64
	UseUSB     bool // true = USB, false = LSB
}

// AmateurBands définit toutes les bandes amateur radioamateur
var AmateurBands = []BandDefinition{
	{Name: "160M", MinFreqMHz: 1.800, MaxFreqMHz: 2.000, UseUSB: false},
	{Name: "80M", MinFreqMHz: 3.500, MaxFreqMHz: 3.800, UseUSB: false},
	{Name: "60M", MinFreqMHz: 5.330, MaxFreqMHz: 5.405, UseUSB: false},
	{Name: "40M", MinFreqMHz: 7.000, MaxFreqMHz: 7.200, UseUSB: false},
	{Name: "30M", MinFreqMHz: 10.100, MaxFreqMHz: 10.150, UseUSB: true}, // Digital only
	{Name: "20M", MinFreqMHz: 14.000, MaxFreqMHz: 14.350, UseUSB: true},
	{Name: "17M", MinFreqMHz: 18.068, MaxFreqMHz: 18.168, UseUSB: true},
	{Name: "15M", MinFreqMHz: 21.000, MaxFreqMHz: 21.450, UseUSB: true},
	{Name: "12M", MinFreqMHz: 24.890, MaxFreqMHz: 24.990, UseUSB: true},
	{Name: "10M", MinFreqMHz: 28.000, MaxFreqMHz: 29.700, UseUSB: true},
	{Name: "6M", MinFreqMHz: 50.000, MaxFreqMHz: 54.000, UseUSB: true},
}

// FrequencyToBand convertit une fréquence en MHz vers un nom de bande
// Retourne "N/A" si la fréquence ne correspond à aucune bande amateur
func FrequencyToBand(freqMHz float64) string {
	for _, band := range AmateurBands {
		if freqMHz >= band.MinFreqMHz && freqMHz < band.MaxFreqMHz {
			return band.Name
		}
	}
	return "N/A"
}

// FrequencyStringToBand convertit une fréquence string (ex: "14.195") vers un nom de bande
// Compatible avec les anciennes fonctions qui utilisent des strings
func FrequencyStringToBand(freqStr string) string {
	freqMHz, err := strconv.ParseFloat(freqStr, 64)
	if err != nil {
		return "N/A"
	}
	return FrequencyToBand(freqMHz)
}

// GetBandDefinition retourne la définition d'une bande par son nom
func GetBandDefinition(bandName string) *BandDefinition {
	for _, band := range AmateurBands {
		if band.Name == bandName {
			return &band
		}
	}
	return nil
}

// IsLSBBand retourne true si la bande utilise LSB par défaut
func IsLSBBand(bandName string) bool {
	band := GetBandDefinition(bandName)
	if band == nil {
		return false
	}
	return !band.UseUSB
}

// IsUSBBand retourne true si la bande utilise USB par défaut
func IsUSBBand(bandName string) bool {
	band := GetBandDefinition(bandName)
	if band == nil {
		return false
	}
	return band.UseUSB
}

// NormalizeSSBMode convertit "SSB" en "USB" ou "LSB" selon la bande
// Si le mode n'est pas "SSB", il est retourné tel quel
func NormalizeSSBMode(mode string, band string) string {
	if mode != "SSB" {
		return mode
	}

	if IsUSBBand(band) {
		return "USB"
	}
	return "LSB"
}

// NormalizeSSBModeByFrequency convertit "SSB" en "USB" ou "LSB" selon la fréquence
func NormalizeSSBModeByFrequency(mode string, freqMHz float64) string {
	if mode != "SSB" {
		return mode
	}

	band := FrequencyToBand(freqMHz)
	return NormalizeSSBMode(mode, band)
}

// GetBandFromFrequencyString extrait la bande depuis une string de fréquence formatée
// Compatible avec FreqMhztoHz (ex: "14.195000" ou "14195.000")
func GetBandFromFrequencyString(freqStr string) string {
	// Nettoyer la string
	freqStr = strings.TrimSpace(freqStr)

	// Essayer de parser comme float
	freqMHz, err := strconv.ParseFloat(freqStr, 64)
	if err != nil {
		return "N/A"
	}

	// Si la fréquence est en kHz (> 1000), convertir en MHz
	if freqMHz > 1000 {
		freqMHz = freqMHz / 1000.0
	}

	return FrequencyToBand(freqMHz)
}

// FreqMhztoHz convertit une fréquence depuis différents formats vers MHz string
// Fonction de compatibilité avec l'ancien code
func FreqMhztoHz(freq string) string {
	if freq == "" {
		return "0"
	}

	// Parser la fréquence
	freqFloat, err := strconv.ParseFloat(freq, 64)
	if err != nil {
		return "0"
	}

	// Si la fréquence est en kHz (> 1000), la garder telle quelle
	// Sinon la convertir en kHz
	if freqFloat < 1000 {
		freqFloat = freqFloat * 1000
	}

	return fmt.Sprintf("%.3f", freqFloat/1000.0)
}

// GetAllBandNames retourne la liste de tous les noms de bandes
func GetAllBandNames() []string {
	names := make([]string, len(AmateurBands))
	for i, band := range AmateurBands {
		names[i] = band.Name
	}
	return names
}

// IsBandValid vérifie si un nom de bande est valide
func IsBandValid(bandName string) bool {
	return GetBandDefinition(bandName) != nil
}

// GetBandFrequencyRange retourne la plage de fréquence d'une bande sous forme de string
func GetBandFrequencyRange(bandName string) string {
	band := GetBandDefinition(bandName)
	if band == nil {
		return ""
	}
	return fmt.Sprintf("%.3f - %.3f MHz", band.MinFreqMHz, band.MaxFreqMHz)
}
