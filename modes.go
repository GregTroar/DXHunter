package main

import (
	"regexp"
	"strings"
)

// ModeRange définit une plage de fréquences pour un mode spécifique
type ModeRange struct {
	MinFreqMHz float64
	MaxFreqMHz float64
	Mode       string
}

// BandModeRanges définit les plages de modes pour chaque bande amateur
// Basé sur les plans de bandes IARU et pratiques courantes
var BandModeRanges = map[string][]ModeRange{
	"160M": {
		{MinFreqMHz: 1.800, MaxFreqMHz: 1.838, Mode: "CW"},
		{MinFreqMHz: 1.838, MaxFreqMHz: 1.843, Mode: "FT8"},
		{MinFreqMHz: 1.843, MaxFreqMHz: 2.000, Mode: "LSB"},
	},
	"80M": {
		{MinFreqMHz: 3.500, MaxFreqMHz: 3.560, Mode: "CW"},
		{MinFreqMHz: 3.560, MaxFreqMHz: 3.575, Mode: "FT8"},
		{MinFreqMHz: 3.575, MaxFreqMHz: 3.578, Mode: "FT4"},
		{MinFreqMHz: 3.578, MaxFreqMHz: 3.590, Mode: "RTTY"},
		{MinFreqMHz: 3.590, MaxFreqMHz: 3.800, Mode: "LSB"},
	},
	"60M": {
		{MinFreqMHz: 5.330, MaxFreqMHz: 5.357, Mode: "CW"},
		{MinFreqMHz: 5.357, MaxFreqMHz: 5.359, Mode: "FT8"},
		{MinFreqMHz: 5.359, MaxFreqMHz: 5.405, Mode: "USB"},
	},
	"40M": {
		{MinFreqMHz: 7.000, MaxFreqMHz: 7.040, Mode: "CW"},
		{MinFreqMHz: 7.040, MaxFreqMHz: 7.047, Mode: "RTTY"},
		{MinFreqMHz: 7.047, MaxFreqMHz: 7.050, Mode: "FT4"},
		{MinFreqMHz: 7.050, MaxFreqMHz: 7.100, Mode: "FT8"},
		{MinFreqMHz: 7.100, MaxFreqMHz: 7.200, Mode: "LSB"},
	},
	"30M": {
		{MinFreqMHz: 10.100, MaxFreqMHz: 10.130, Mode: "CW"},
		{MinFreqMHz: 10.130, MaxFreqMHz: 10.142, Mode: "FT8"},
		{MinFreqMHz: 10.142, MaxFreqMHz: 10.150, Mode: "FT4"},
	},
	"20M": {
		{MinFreqMHz: 14.000, MaxFreqMHz: 14.070, Mode: "CW"},
		{MinFreqMHz: 14.070, MaxFreqMHz: 14.078, Mode: "FT8"},
		{MinFreqMHz: 14.078, MaxFreqMHz: 14.083, Mode: "FT4"},
		{MinFreqMHz: 14.083, MaxFreqMHz: 14.100, Mode: "FT8"},
		{MinFreqMHz: 14.100, MaxFreqMHz: 14.112, Mode: "RTTY"},
		{MinFreqMHz: 14.112, MaxFreqMHz: 14.350, Mode: "USB"},
	},
	"17M": {
		{MinFreqMHz: 18.068, MaxFreqMHz: 18.090, Mode: "CW"},
		{MinFreqMHz: 18.090, MaxFreqMHz: 18.104, Mode: "FT8"},
		{MinFreqMHz: 18.104, MaxFreqMHz: 18.106, Mode: "FT4"},
		{MinFreqMHz: 18.106, MaxFreqMHz: 18.110, Mode: "RTTY"},
		{MinFreqMHz: 18.110, MaxFreqMHz: 18.168, Mode: "USB"},
	},
	"15M": {
		{MinFreqMHz: 21.000, MaxFreqMHz: 21.070, Mode: "CW"},
		{MinFreqMHz: 21.070, MaxFreqMHz: 21.100, Mode: "FT8"},
		{MinFreqMHz: 21.100, MaxFreqMHz: 21.130, Mode: "RTTY"},
		{MinFreqMHz: 21.130, MaxFreqMHz: 21.143, Mode: "FT4"},
		{MinFreqMHz: 21.143, MaxFreqMHz: 21.450, Mode: "USB"},
	},
	"12M": {
		{MinFreqMHz: 24.890, MaxFreqMHz: 24.910, Mode: "CW"},
		{MinFreqMHz: 24.910, MaxFreqMHz: 24.918, Mode: "FT8"},
		{MinFreqMHz: 24.918, MaxFreqMHz: 24.922, Mode: "FT4"},
		{MinFreqMHz: 24.922, MaxFreqMHz: 24.930, Mode: "FT4"},
		{MinFreqMHz: 24.930, MaxFreqMHz: 24.990, Mode: "USB"},
	},
	"10M": {
		{MinFreqMHz: 28.000, MaxFreqMHz: 28.070, Mode: "CW"},
		{MinFreqMHz: 28.070, MaxFreqMHz: 28.110, Mode: "FT8"},
		{MinFreqMHz: 28.110, MaxFreqMHz: 28.179, Mode: "RTTY"},
		{MinFreqMHz: 28.179, MaxFreqMHz: 28.190, Mode: "FT4"},
		{MinFreqMHz: 28.190, MaxFreqMHz: 29.000, Mode: "USB"},
		{MinFreqMHz: 29.000, MaxFreqMHz: 29.700, Mode: "FM"},
	},
	"6M": {
		{MinFreqMHz: 50.000, MaxFreqMHz: 50.100, Mode: "CW"},
		{MinFreqMHz: 50.100, MaxFreqMHz: 50.313, Mode: "USB"},
		{MinFreqMHz: 50.313, MaxFreqMHz: 50.318, Mode: "FT8"},
		{MinFreqMHz: 50.318, MaxFreqMHz: 50.323, Mode: "FT4"},
		{MinFreqMHz: 50.323, MaxFreqMHz: 51.000, Mode: "USB"},
		{MinFreqMHz: 51.000, MaxFreqMHz: 54.000, Mode: "FM"},
	},
}

// GuessMode devine le mode depuis la fréquence en MHz
// Retourne une string vide si le mode ne peut pas être deviné
func GuessMode(freqMHz float64) string {
	// D'abord déterminer la bande
	band := FrequencyToBand(freqMHz)
	if band == "N/A" {
		if freqMHz < 10.0 {
			return "LSB"
		}
		return "USB"
	}

	// Chercher dans les plages de modes de la bande
	return GuessModeForBand(freqMHz, band)
}

// GuessModeForBand devine le mode pour une bande spécifique
func GuessModeForBand(freqMHz float64, band string) string {
	ranges, exists := BandModeRanges[band]
	if !exists {
		// Bande sans plages définies - utiliser le mode par défaut
		if IsUSBBand(band) {
			return "USB"
		}
		return "LSB"
	}

	for _, r := range ranges {
		if freqMHz >= r.MinFreqMHz && freqMHz < r.MaxFreqMHz {
			return r.Mode
		}
	}

	// Si aucune plage ne correspond, utiliser le mode par défaut de la bande
	if IsUSBBand(band) {
		return "USB"
	}
	return "LSB"
}

// ExtractModeFromComment essaie d'extraire le mode depuis un commentaire de spot
// Retourne une string vide si aucun mode n'est détecté
func ExtractModeFromComment(comment string) string {
	if comment == "" {
		return ""
	}

	commentUpper := strings.ToUpper(comment)

	// 1. Détecter FT8/FT4 avec leurs patterns typiques (dB + Hz)
	if strings.Contains(commentUpper, "FT8") ||
		(strings.Contains(commentUpper, "DB") && strings.Contains(commentUpper, "HZ")) {
		return "FT8"
	}

	if strings.Contains(commentUpper, "FT4") {
		return "FT4"
	}

	// 2. Détecter CW avec WPM (Words Per Minute)
	if strings.Contains(commentUpper, "WPM") || strings.Contains(commentUpper, " CW ") ||
		strings.HasSuffix(commentUpper, "CW") || strings.HasPrefix(commentUpper, "CW ") {
		return "CW"
	}

	// 3. Autres modes digitaux
	digitalModes := []string{"RTTY", "PSK31", "PSK63", "PSK", "MFSK", "OLIVIA", "JT65", "JT9"}
	for _, mode := range digitalModes {
		if strings.Contains(commentUpper, mode) {
			return mode
		}
	}

	// 4. Modes voice - chercher comme mot complet
	voiceModes := []string{"USB", "LSB", "SSB", "FM", "AM"}
	for _, mode := range voiceModes {
		// Chercher le mode comme mot complet (pas dans "SSBC" par exemple)
		if strings.Contains(commentUpper, " "+mode+" ") ||
			strings.HasPrefix(commentUpper, mode+" ") ||
			strings.HasSuffix(commentUpper, " "+mode) ||
			commentUpper == mode {
			return mode
		}
	}

	return ""
}

// DetermineMode détermine le mode d'un spot en utilisant plusieurs sources
// Priorité : mode explicite > commentaire > fréquence
func DetermineMode(explicitMode string, comment string, freqMHz float64) string {
	// 1. Si un mode explicite est fourni, le normaliser
	if explicitMode != "" {
		explicitMode = strings.ToUpper(explicitMode)

		// Normaliser SSB si nécessaire
		if explicitMode == "SSB" {
			return NormalizeSSBModeByFrequency(explicitMode, freqMHz)
		}

		return explicitMode
	}

	// 2. Essayer d'extraire depuis le commentaire
	modeFromComment := ExtractModeFromComment(comment)
	if modeFromComment != "" {
		// Normaliser SSB si nécessaire
		if modeFromComment == "SSB" {
			return NormalizeSSBModeByFrequency(modeFromComment, freqMHz)
		}
		return modeFromComment
	}

	// 3. Deviner depuis la fréquence
	return GuessMode(freqMHz)
}

// IsCWMode retourne true si le mode est CW
func IsCWMode(mode string) bool {
	return strings.ToUpper(mode) == "CW"
}

// IsDigitalMode retourne true si le mode est digital (FT8, FT4, RTTY, etc.)
func IsDigitalMode(mode string) bool {
	modeUpper := strings.ToUpper(mode)
	digitalModes := []string{"FT8", "FT4", "RTTY", "PSK31", "PSK63", "PSK", "MFSK", "OLIVIA", "CONTESTIA", "JT65", "JT9"}

	for _, dm := range digitalModes {
		if modeUpper == dm {
			return true
		}
	}
	return false
}

// IsPhoneMode retourne true si le mode est phone (SSB, USB, LSB, FM, AM)
func IsPhoneMode(mode string) bool {
	modeUpper := strings.ToUpper(mode)
	phoneModes := []string{"SSB", "USB", "LSB", "FM", "AM"}

	for _, pm := range phoneModes {
		if modeUpper == pm {
			return true
		}
	}
	return false
}

// IsSSBMode retourne true si le mode est une variante de SSB (SSB, USB, LSB)
func IsSSBMode(mode string) bool {
	modeUpper := strings.ToUpper(mode)
	return modeUpper == "SSB" || modeUpper == "USB" || modeUpper == "LSB"
}

// NormalizeModeString normalise un mode (majuscules, trim)
func NormalizeModeString(mode string) string {
	return strings.ToUpper(strings.TrimSpace(mode))
}

// ValidateModeForBand vérifie si un mode est valide pour une bande donnée
// Retourne true si le mode est acceptable sur cette bande
func ValidateModeForBand(mode string, band string) bool {
	// Certaines bandes ont des restrictions
	if band == "30M" {
		// 30M = CW et digital uniquement (pas de phone)
		return IsCWMode(mode) || IsDigitalMode(mode)
	}

	// Les autres bandes acceptent généralement tous les modes
	return true
}

// GetModeColor retourne une couleur CSS pour un mode (pour l'UI)
func GetModeColor(mode string) string {
	modeUpper := strings.ToUpper(mode)

	switch {
	case modeUpper == "CW":
		return "#10b981" // Green
	case modeUpper == "FT8" || modeUpper == "FT4":
		return "#8b5cf6" // Purple
	case modeUpper == "RTTY":
		return "#f59e0b" // Amber
	case IsSSBMode(modeUpper):
		return "#3b82f6" // Blue
	case modeUpper == "FM":
		return "#ec4899" // Pink
	default:
		return "#6b7280" // Gray
	}
}

// ParseModeFromRawSpot essaie d'extraire le mode depuis un spot brut
// Utilise une regex pour trouver les patterns courants
func ParseModeFromRawSpot(rawSpot string) string {
	// Pattern pour détecter les modes dans les spots
	// Ex: "DX de F4BPO:     14.074  VK9DWX       FT8  1234 Hz"
	modeRegex := regexp.MustCompile(`\b(CW|SSB|USB|LSB|FM|AM|FT8|FT4|RTTY|PSK\d*)\b`)

	match := modeRegex.FindString(strings.ToUpper(rawSpot))
	if match != "" {
		return match
	}

	return ""
}
