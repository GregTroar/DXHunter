package main

import (
	"encoding/xml"
)

type SolarData struct {
	SolarFlux   string `xml:"solarflux"`
	Sunspots    string `xml:"sunspots"`
	AIndex      string `xml:"aindex"`
	KIndex      string `xml:"kindex"`
	SolarWind   string `xml:"solarwind"`
	HeliumLine  string `xml:"heliumline"`
	ProtonFlux  string `xml:"protonflux"`
	GeomagField string `xml:"geomagfield"`
	Updated     string `xml:"updated"`
}

type SolarXML struct {
	XMLName xml.Name  `xml:"solar"`
	Data    SolarData `xml:"solardata"`
}
