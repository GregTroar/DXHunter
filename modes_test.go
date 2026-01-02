package main

import "testing"

func TestGuessMode(t *testing.T) {
	tests := []struct {
		freq float64
		want string
	}{
		{1.835, "CW"},
		{1.840, "FT8"},
		{1.850, "LSB"},
		{7.025, "CW"},
		{7.074, "FT8"},
		{7.150, "LSB"},
		{14.074, "FT8"},
		{14.200, "USB"},
	}

	for _, tt := range tests {
		got := GuessMode(tt.freq)
		if got != tt.want {
			t.Errorf("GuessMode(%.3f) = %s, want %s", tt.freq, got, tt.want)
		}
	}
}

func TestExtractModeFromComment(t *testing.T) {
	tests := []struct {
		comment string
		want    string
	}{
		{"CQ 599 TX", "CW"},
		{"-12 dB 1234 Hz", "FT8"},
		{"FT4 loud signal", "FT4"},
		{"RTTY contest", "RTTY"},
		{"USB nice signal", "USB"},
		{"Random comment", ""},
	}

	for _, tt := range tests {
		got := ExtractModeFromComment(tt.comment)
		if got != tt.want {
			t.Errorf("ExtractModeFromComment(%q) = %s, want %s", tt.comment, got, tt.want)
		}
	}
}
