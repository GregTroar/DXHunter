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

	// Start cleanup goroutine
	go sp.cleanupExpiredSpots()

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
	// ✅ AUTO-ADD contest callsigns with prefix to watchlist
	if Cfg.General.ContestMode && Cfg.General.ContestPrefix != "" && sp.HTTPServer != nil && sp.HTTPServer.Watchlist != nil {
		if strings.Contains(spot.DX, Cfg.General.ContestPrefix) {
			// Check if not already in watchlist
			if !sp.HTTPServer.Watchlist.Matches(spot.DX) {
				if err := sp.HTTPServer.Watchlist.AddContest(spot.DX); err == nil {
					Log.Infof("🏆 Auto-added contest callsign %s to watchlist (contains prefix '%s')",
						spot.DX, Cfg.General.ContestPrefix)

					// Broadcast updated watchlist to all WebSocket clients
					sp.HTTPServer.broadcast <- WSMessage{
						Type: "watchlist",
						Data: sp.HTTPServer.Watchlist.GetAll(),
					}
				}
			}
		}
	}

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
	flexSpot.Comment = flexSpot.Comment + " [" + flexSpot.Mode + "] [" + flexSpot.SpotterCallsign + "] [" + flexSpot.CountryName + "]"

	// Check if this spot is in the watchlist
	isInWatchlist := false
	if sp.HTTPServer != nil && sp.HTTPServer.Watchlist != nil {
		if sp.HTTPServer.Watchlist.Matches(flexSpot.DX) {
			flexSpot.InWatchlist = true
			isInWatchlist = true

			// Mark as seen and update last seen time
			sp.HTTPServer.Watchlist.MarkSeen(flexSpot.DX)
		}
	}

	sp.applySpotColors(&flexSpot, spot)
	sp.sendGotifyNotification(flexSpot)

	flexSpot.Comment = strings.ReplaceAll(flexSpot.Comment, " ", "\u00A0")

	srcFlexSpot, err := sp.FlexRepo.FindDXSameBand(flexSpot)
	if err != nil {
		Log.Debugf("Could not find the DX in the database: %v", err)
	}

	// Vérifier si le spot trouvé est valide (a un ID)
	if srcFlexSpot != nil && srcFlexSpot.ID == 0 {
		srcFlexSpot = nil
	}

	// Store the spot
	sp.handleSpotStorage(flexSpot, srcFlexSpot)

	// After creating the spot, if it's in watchlist, track it
	// We need to find the created spot to get its ID
	if isInWatchlist && sp.HTTPServer != nil && sp.HTTPServer.Watchlist != nil {
		// Try to find the spot we just created using CommandNumber
		createdSpot, err := sp.FlexRepo.FindSpotByCommandNumber(fmt.Sprintf("%d", flexSpot.CommandNumber))
		if err == nil && createdSpot != nil && createdSpot.ID > 0 {
			sp.HTTPServer.Watchlist.AddActiveSpot(flexSpot.DX, createdSpot.ID)

			// Send update to websocket clients
			sp.HTTPServer.broadcast <- WSMessage{
				Type: "watchlistUpdate",
				Data: sp.HTTPServer.Watchlist.GetAllWithActiveStatus(),
			}
		}
	}

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
		flexSpot.Priority = "1"
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
		flexSpot.Priority = "1"
		flexSpot.BackgroundColor = "#ff000000"
		flexSpot.Comment = flexSpot.Comment + " [New Band]"
	} else if !spot.NewBand && !spot.NewMode && !spot.NewDXCC && !spot.CallsignWorked && spot.NewSlot {
		flexSpot.Color = "#ff91d2ff"
		flexSpot.Priority = "2"
		flexSpot.BackgroundColor = "#ff000000"
		flexSpot.Comment = flexSpot.Comment + " [New Slot]"
	}
}

// Keep the original signature since CreateSpot doesn't return an ID
func (sp *SpotProcessor) handleSpotStorage(flexSpot FlexSpot, srcFlexSpot *FlexSpot) {
	if srcFlexSpot == nil {
		sp.FlexRepo.CreateSpot(flexSpot)
		CommandNumber++
		if sp.HTTPServer != nil {
			sp.HTTPServer.broadcast <- WSMessage{Type: "spots", Data: sp.FlexRepo.GetAllSpots("0")}
		}
	} else if srcFlexSpot.Band == flexSpot.Band {
		// Remove old spot from watchlist tracking before deleting
		if srcFlexSpot.ID > 0 {
			sp.RemoveSpotFromWatchlist(srcFlexSpot.ID)
		}

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

func (sp *SpotProcessor) sendGotifyNotification(flexSpot FlexSpot) {
	if !Cfg.Gotify.Enable {
		return
	}

	// Cas 1 : Nouveau DXCC - toujours notifier si activé dans la config
	if flexSpot.NewDXCC && Cfg.Gotify.NewDXCC {
		Gotify(flexSpot)
		Log.Debugf("🔔 Gotify notification sent: New DXCC - %s", flexSpot.DX)
		return
	}

	// Cas 2 : Callsign dans la watchlist ET non contacté
	if flexSpot.InWatchlist && !flexSpot.Worked && Cfg.Gotify.WatchList {
		Gotify(flexSpot)
		Log.Debugf("🔔 Gotify notification sent: Watchlist match (not worked) - %s", flexSpot.DX)
		return
	}

	// Tous les autres cas : pas de notification
	Log.Debugf("🔇 Gotify notification skipped for %s (InWatchlist=%v, Worked=%v, NewDXCC=%v)",
		flexSpot.DX, flexSpot.InWatchlist, flexSpot.Worked, flexSpot.NewDXCC)
}

// RemoveSpotFromWatchlist removes the spot from watchlist tracking when it's deleted
// This should be called when a spot is deleted from the database
func (sp *SpotProcessor) RemoveSpotFromWatchlist(spotID int) {
	if sp.HTTPServer != nil && sp.HTTPServer.Watchlist != nil {
		sp.HTTPServer.Watchlist.RemoveActiveSpot(spotID)

		// Send update to websocket clients
		sp.HTTPServer.broadcast <- WSMessage{
			Type: "watchlistUpdate",
			Data: sp.HTTPServer.Watchlist.GetAllWithActiveStatus(),
		}
	}
}

// RemoveSpotByFlexNumberFromWatchlist removes a spot by its flex number
// This should be called when receiving a spot deletion from FlexRadio
func (sp *SpotProcessor) RemoveSpotByFlexNumberFromWatchlist(flexSpotNumber string) {
	// Find the spot by flex number to get its ID
	spot, err := sp.FlexRepo.FindSpotByFlexSpotNumber(flexSpotNumber)
	if err == nil && spot != nil && spot.ID > 0 {
		sp.RemoveSpotFromWatchlist(spot.ID)
	}
}

// cleanupExpiredSpots runs periodically to remove expired spots from database
// This ensures spots are cleaned up even if FlexRadio messages are lost
func (sp *SpotProcessor) cleanupExpiredSpots() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	Log.Info("Started spot cleanup goroutine (runs every 30 seconds)")

	for {
		select {
		case <-sp.ctx.Done():
			Log.Info("Spot cleanup goroutine shutting down...")
			return
		case <-ticker.C:
			sp.performCleanup()
		}
	}
}

// performCleanup removes spots that have exceeded their lifetime
func (sp *SpotProcessor) performCleanup() {
	// Parse the configured spot lifetime (e.g., "900" for 900 seconds)
	var lifetimeSeconds int64
	fmt.Sscanf(Cfg.Flex.SpotLife, "%d", &lifetimeSeconds)

	if lifetimeSeconds <= 0 {
		lifetimeSeconds = 900 // Default to 15 minutes if not configured
	}

	// Get all spots from database
	allSpots := sp.FlexRepo.GetAllSpots("0")

	now := time.Now().Unix()
	removedCount := 0

	for _, spot := range allSpots {
		// Check if spot has exceeded its lifetime
		spotAge := now - spot.TimeStamp

		if spotAge > lifetimeSeconds {
			// Remove from watchlist tracking first
			if spot.ID > 0 {
				sp.RemoveSpotFromWatchlist(spot.ID)
			}

			// Delete from database
			if spot.FlexSpotNumber > 0 {
				sp.FlexRepo.DeleteSpotByFlexSpotNumber(fmt.Sprintf("%d", spot.FlexSpotNumber))
				if err == nil {
					removedCount++
					Log.Debugf("Cleaned up expired spot: %s (age: %d seconds, lifetime: %d seconds)",
						spot.DX, spotAge, lifetimeSeconds)

					// If FlexClient is connected, also remove from FlexRadio panadapter
					if sp.FlexClient != nil && sp.FlexClient.Enabled && sp.FlexClient.IsConnected {
						removeCmd := fmt.Sprintf("C%v|spot remove %v", CommandNumber, spot.FlexSpotNumber)
						sp.FlexClient.SendSpot(removeCmd)
						CommandNumber++
					}
				}
			}
		}
	}

	if removedCount > 0 {
		Log.Infof("Cleanup: Removed %d expired spots", removedCount)

		// Broadcast updated spots list to WebSocket clients
		if sp.HTTPServer != nil {
			sp.HTTPServer.broadcast <- WSMessage{
				Type: "spots",
				Data: sp.FlexRepo.GetAllSpots("0"),
			}
		}
	}
}
