package main

import "testing"

func TestFrequencyToBand(t *testing.T) {
	tests := []struct {
		freq float64
		want string
	}{
		{1.850, "160M"},
		{3.700, "80M"},
		{7.055, "40M"},
		{14.195, "20M"},
		{21.074, "15M"},
		{28.500, "10M"},
		{50.313, "6M"},
		{100.0, "N/A"},
	}

	for _, tt := range tests {
		got := FrequencyToBand(tt.freq)
		if got != tt.want {
			t.Errorf("FrequencyToBand(%.3f) = %s, want %s", tt.freq, got, tt.want)
		}
	}
}

func TestNormalizeSSBMode(t *testing.T) {
	tests := []struct {
		mode string
		band string
		want string
	}{
		{"SSB", "20M", "USB"},
		{"SSB", "40M", "LSB"},
		{"USB", "20M", "USB"},
		{"CW", "20M", "CW"},
		{"FT8", "40M", "FT8"},
	}

	for _, tt := range tests {
		got := NormalizeSSBMode(tt.mode, tt.band)
		if got != tt.want {
			t.Errorf("NormalizeSSBMode(%s, %s) = %s, want %s", tt.mode, tt.band, got, tt.want)
		}
	}
}
