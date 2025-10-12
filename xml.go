package main

import (
	_ "embed"
	"encoding/xml"
	"regexp"
	"strings"
	"unicode/utf8"
)

//go:embed country.xml
var countryXMLData []byte

type Countries struct {
	XMLName   xml.Name  `xml:"Countries"`
	Countries []Country `xml:"Country"`
}

type Country struct {
	XMLName           xml.Name          `xml:"Country"`
	ArrlPrefix        string            `xml:"ArrlPrefix"`
	Comment           string            `xml:"Comment"`
	Continent         string            `xml:"Continent"`
	CountryName       string            `xml:"CountryName"`
	CqZone            string            `xml:"CqZone"`
	CqZoneList        string            `xml:"CqZoneList"`
	Dxcc              string            `xml:"Dxcc"`
	ItuZone           string            `xml:"ItuZone"`
	IaruRegion        string            `xml:"IaruRegion"`
	ItuZoneList       string            `xml:"ItuZoneList"`
	Latitude          string            `xml:"Latitude"`
	Longitude         string            `xml:"Longitude"`
	Active            string            `xml:"Active"`
	CountryTag        string            `xml:"CountryTag"`
	CountryPrefixList CountryPrefixList `xml:"CountryPrefixList"`
}

type CountryPrefixList struct {
	XMLName           xml.Name        `xml:"CountryPrefixList"`
	CountryPrefixList []CountryPrefix `xml:"CountryPrefix"`
}

type CountryPrefix struct {
	XMLName    xml.Name `xml:"CountryPrefix"`
	PrefixList string   `xml:"PrefixList"`
	StartDate  string   `xml:"StartDate"`
	EndDate    string   `xml:"EndDate"`
}

type DXCC struct {
	Callsign        string
	CountryName     string
	DXCC            string
	RegEx           string
	RegExSplit      []string
	RegExCharacters int
	Ended           bool
}

func LoadCountryFile() Countries {
	var countries Countries

	// Utiliser les données embarquées
	err := xml.Unmarshal(countryXMLData, &countries)
	if err != nil {
		Log.Fatalf("Failed to parse embedded country.xml: %v", err)
	}

	return countries
}

func GetDXCC(dxCall string, Countries Countries) DXCC {
	DXCCList := []DXCC{}
	d := DXCC{}

	// Get all the matching DXCC for current callsign
	for i := 0; i < len(Countries.Countries); i++ {
		if len(Countries.Countries[i].CountryPrefixList.CountryPrefixList) == 0 {
			continue
		}

		lastPrefix := Countries.Countries[i].CountryPrefixList.CountryPrefixList[len(Countries.Countries[i].CountryPrefixList.CountryPrefixList)-1]
		regExp := regexp.MustCompile(lastPrefix.PrefixList)

		match := regExp.FindStringSubmatch(dxCall)
		if len(match) != 0 {
			d = DXCC{
				Callsign:    dxCall,
				CountryName: Countries.Countries[i].CountryName,
				DXCC:        Countries.Countries[i].Dxcc,
				RegEx:       lastPrefix.PrefixList,
			}

			if lastPrefix.EndDate == "" {
				d.Ended = false
			} else {
				d.Ended = true
			}

			DXCCList = append(DXCCList, d)
		}
	}

	for i := 0; i < len(DXCCList); i++ {
		DXCCList[i].RegExSplit = strings.Split(DXCCList[i].RegEx, "|")

		for j := 0; j < len(DXCCList[i].RegExSplit); j++ {
			regExp := regexp.MustCompile(DXCCList[i].RegExSplit[j])
			matched := regExp.FindStringSubmatch(dxCall)
			if len(matched) > 0 {
				DXCCList[i].RegExCharacters = utf8.RuneCountInString(DXCCList[i].RegExSplit[j])
			}
		}
	}

	if len(DXCCList) > 0 {
		DXCCMatch := DXCCList[0]
		higherMatch := 0

		if len(DXCCList) > 1 {
			for i := 0; i < len(DXCCList); i++ {
				if DXCCList[i].RegExCharacters > higherMatch && !DXCCList[i].Ended {
					DXCCMatch = DXCCList[i]
					higherMatch = DXCCList[i].RegExCharacters
				}
			}
		} else {
			DXCCMatch = DXCCList[0]
		}

		return DXCCMatch
	} else {
		Log.Warnf("Could not find %s in country list", dxCall)
	}

	return DXCC{}
}
