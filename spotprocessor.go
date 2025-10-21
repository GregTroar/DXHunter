package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SpotProcessor struct {
	FlexRepo   *FlexDXClusterRepository
	FlexClient *FlexClient
	HTTPServer *HTTPServer
	SpotChan   chan TelnetSpot
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewSpotProcessor(flexRepo *FlexDXClusterRepository, flexClient *FlexClient, httpServer *HTTPServer, spotChan chan TelnetSpot) *SpotProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	return &SpotProcessor{
		FlexRepo:   flexRepo,
		FlexClient: flexClient,
		HTTPServer: httpServer,
		SpotChan:   spotChan,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (sp *SpotProcessor) Start() {
	Log.Info("Starting Spot Processor...")

	for {
		select {
		case <-sp.ctx.Done():
			Log.Info("Spot Processor shutting down...")
			return
		case spot := <-sp.SpotChan:
			sp.processSpot(spot)
		}
	}
}

func (sp *SpotProcessor) Stop() {
	Log.Info("Stopping Spot Processor...")
	sp.cancel()
}

func (sp *SpotProcessor) processSpot(spot TelnetSpot) {
	freq := FreqMhztoHz(spot.Frequency)

	flexSpot := FlexSpot{
		CommandNumber:   CommandNumber,
		DX:              spot.DX,
		FrequencyMhz:    freq,
		FrequencyHz:     spot.Frequency,
		Band:            spot.Band,
		Mode:            spot.Mode,
		Source:          "FlexDXCluster",
		SpotterCallsign: spot.Spotter,
		TimeStamp:       time.Now().Unix(),
		UTCTime:         spot.Time,
		LifeTime:        Cfg.Flex.SpotLife,
		OriginalComment: spot.Comment,
		Comment:         spot.Comment,
		Color:           "#ffeaeaea",
		BackgroundColor: "#ff000000",
		Priority:        "5",
		NewDXCC:         spot.NewDXCC,
		NewBand:         spot.NewBand,
		NewMode:         spot.NewMode,
		NewSlot:         spot.NewSlot,
		Worked:          spot.CallsignWorked,
		InWatchlist:     false,
		CountryName:     spot.CountryName,
		DXCC:            spot.DXCC,
	}

	flexSpot.OriginalComment = spot.Comment
	flexSpot.Comment = flexSpot.Comment + " [" + flexSpot.Mode + "] [" + flexSpot.SpotterCallsign + "] [" + flexSpot.UTCTime + "]"

	if sp.HTTPServer != nil && sp.HTTPServer.Watchlist != nil {
		if sp.HTTPServer.Watchlist.Matches(flexSpot.DX) {
			flexSpot.InWatchlist = true

			// Mark as seen and update last seen time
			sp.HTTPServer.Watchlist.MarkSeen(flexSpot.DX)

			// Get entry to check if sound should be played
			entry := sp.HTTPServer.Watchlist.GetEntry(flexSpot.DX)
			if entry != nil {
				Log.Infof("🎯 Watchlist match: %s (LastSeen: %s)",
					flexSpot.DX, entry.LastSeenStr)

				// Send notification to websocket clients for sound alert
				if entry.PlaySound && sp.HTTPServer != nil {
					sp.HTTPServer.broadcast <- WSMessage{
						Type: "watchlistAlert",
						Data: map[string]interface{}{
							"callsign":    flexSpot.DX,
							"frequency":   flexSpot.FrequencyMhz,
							"band":        flexSpot.Band,
							"mode":        flexSpot.Mode,
							"countryName": flexSpot.CountryName,
							"playSound":   entry.PlaySound,
						},
					}
				}
			}
		}
	}

	sp.applySpotColors(&flexSpot, spot)
	Gotify(flexSpot)

	flexSpot.Comment = strings.ReplaceAll(flexSpot.Comment, " ", "\u00A0")

	srcFlexSpot, err := sp.FlexRepo.FindDXSameBand(flexSpot)
	if err != nil {
		Log.Debugf("Could not find the DX in the database: %v", err)
	}

	// Vérifier si le spot trouvé est valide (a un ID)
	if srcFlexSpot != nil && srcFlexSpot.ID == 0 {
		srcFlexSpot = nil
	}

	sp.handleSpotStorage(flexSpot, srcFlexSpot)

	if sp.FlexClient != nil && sp.FlexClient.Enabled && sp.FlexClient.IsConnected {
		sp.sendToFlexRadio(flexSpot, srcFlexSpot)
	}
}

func (sp *SpotProcessor) applySpotColors(flexSpot *FlexSpot, spot TelnetSpot) {
	if spot.NewDXCC {
		flexSpot.Priority = "1"
		flexSpot.Comment = flexSpot.Comment + " [New DXCC]"
		if Cfg.General.SpotColorNewEntity != "" {
			flexSpot.Color = Cfg.General.SpotColorNewEntity
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorNewEntity
		} else {
			flexSpot.Color = "#ff3bf908"
			flexSpot.BackgroundColor = "#ff000000"
		}
	} else if spot.DX == Cfg.General.Callsign {
		flexSpot.Priority = "1"
		if Cfg.General.SpotColorMyCallsign != "" {
			flexSpot.Color = Cfg.General.SpotColorMyCallsign
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorMyCallsign
		} else {
			flexSpot.Color = "#ffff0000"
			flexSpot.BackgroundColor = "#ff000000"
		}
	} else if spot.CallsignWorked {
		flexSpot.Priority = "5"
		flexSpot.Comment = flexSpot.Comment + " [Worked]"
		if Cfg.General.SpotColorWorked != "" {
			flexSpot.Color = Cfg.General.SpotColorWorked
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorWorked
		} else {
			flexSpot.Color = "#ff000000"
			flexSpot.BackgroundColor = "#ff00c0c0"
		}
	} else if spot.NewMode && spot.NewBand {
		flexSpot.Priority = "1"
		flexSpot.Comment = flexSpot.Comment + " [New Band & Mode]"
		if Cfg.General.SpotColorNewBandMode != "" {
			flexSpot.Color = Cfg.General.SpotColorNewBandMode
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorNewBandMode
		} else {
			flexSpot.Color = "#ffc603fc"
			flexSpot.BackgroundColor = "#ff000000"
		}
	} else if spot.NewMode && !spot.NewBand {
		flexSpot.Priority = "2"
		flexSpot.Comment = flexSpot.Comment + " [New Mode]"
		if Cfg.General.SpotColorNewMode != "" {
			flexSpot.Color = Cfg.General.SpotColorNewMode
			flexSpot.BackgroundColor = Cfg.General.BackgroundColorNewMode
		} else {
			flexSpot.Color = "#fff9a908"
			flexSpot.BackgroundColor = "#ff000000"
		}
	} else if spot.NewBand && !spot.NewMode {
		flexSpot.Color = "#fff9f508"
		flexSpot.Priority = "3"
		flexSpot.BackgroundColor = "#ff000000"
		flexSpot.Comment = flexSpot.Comment + " [New Band]"
	} else if !spot.NewBand && !spot.NewMode && !spot.NewDXCC && !spot.CallsignWorked && spot.NewSlot {
		flexSpot.Color = "#ff91d2ff"
		flexSpot.Priority = "5"
		flexSpot.BackgroundColor = "#ff000000"
		flexSpot.Comment = flexSpot.Comment + " [New Slot]"
	}
}

func (sp *SpotProcessor) handleSpotStorage(flexSpot FlexSpot, srcFlexSpot *FlexSpot) {
	if srcFlexSpot == nil {
		sp.FlexRepo.CreateSpot(flexSpot)
		CommandNumber++
		if sp.HTTPServer != nil {
			sp.HTTPServer.broadcast <- WSMessage{Type: "spots", Data: sp.FlexRepo.GetAllSpots("0")}
		}
	} else if srcFlexSpot.Band == flexSpot.Band {
		sp.FlexRepo.DeleteSpotByFlexSpotNumber(fmt.Sprintf("%d", srcFlexSpot.FlexSpotNumber))
		sp.FlexRepo.CreateSpot(flexSpot)
		CommandNumber++
	} else {
		sp.FlexRepo.CreateSpot(flexSpot)
		CommandNumber++
	}
}

func (sp *SpotProcessor) sendToFlexRadio(flexSpot FlexSpot, srcFlexSpot *FlexSpot) {
	var stringSpot string

	if srcFlexSpot == nil {
		stringSpot = fmt.Sprintf("C%v|spot add rx_freq=%v callsign=%s mode=%s source=%s spotter_callsign=%s timestamp=%v lifetime_seconds=%s comment=%s color=%s background_color=%s priority=%s",
			flexSpot.CommandNumber, flexSpot.FrequencyMhz, flexSpot.DX, flexSpot.Mode, flexSpot.Source, flexSpot.SpotterCallsign,
			flexSpot.TimeStamp, flexSpot.LifeTime, flexSpot.Comment, flexSpot.Color, flexSpot.BackgroundColor, flexSpot.Priority)
		CommandNumber++
		sp.FlexClient.SendSpot(stringSpot)
	} else if srcFlexSpot.Band == flexSpot.Band {
		stringSpot = fmt.Sprintf("C%v|spot remove %v", flexSpot.CommandNumber, srcFlexSpot.FlexSpotNumber)
		sp.FlexClient.SendSpot(stringSpot)
		CommandNumber++

		stringSpot = fmt.Sprintf("C%v|spot add rx_freq=%v callsign=%s mode=%s source=%s spotter_callsign=%s timestamp=%v lifetime_seconds=%s comment=%s color=%s background_color=%s priority=%s",
			flexSpot.CommandNumber, flexSpot.FrequencyMhz, flexSpot.DX, flexSpot.Mode, flexSpot.Source, flexSpot.SpotterCallsign,
			flexSpot.TimeStamp, flexSpot.LifeTime, flexSpot.Comment, flexSpot.Color, flexSpot.BackgroundColor, flexSpot.Priority)
		CommandNumber++
		sp.FlexClient.SendSpot(stringSpot)
	} else {
		stringSpot = fmt.Sprintf("C%v|spot add rx_freq=%v callsign=%s mode=%s source=%s spotter_callsign=%s timestamp=%v lifetime_seconds=%s comment=%s color=%s background_color=%s priority=%s",
			flexSpot.CommandNumber, flexSpot.FrequencyMhz, flexSpot.DX, flexSpot.Mode, flexSpot.Source, flexSpot.SpotterCallsign,
			flexSpot.TimeStamp, flexSpot.LifeTime, flexSpot.Comment, flexSpot.Color, flexSpot.BackgroundColor, flexSpot.Priority)
		CommandNumber++
		sp.FlexClient.SendSpot(stringSpot)
	}
}
