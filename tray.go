//go:build windows

package main

import (
	_ "embed"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/getlantern/systray"
)

//go:embed images/FlexDXCluster.ico
var appIcon []byte

// runTray sets up the system-tray icon and blocks (must be called from main goroutine).
func runTray(
	tcpClients []*TCPClient,
	tcpServer *TCPServer,
	flexClient *FlexClient,
	flexRepo *FlexDXClusterRepository,
	contactRepo LogbookProvider,
) {
	var once sync.Once
	doShutdown := func() {
		once.Do(func() {
			GracefulShutdown(tcpClients, tcpServer, flexClient, flexRepo, contactRepo)
		})
	}

	// OS signals quit the tray cleanly.
	go func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
		<-sigchan
		systray.Quit()
	}()

	systray.Run(func() {
		systray.SetIcon(appIcon)
		systray.SetTitle("DXHunter")
		systray.SetTooltip("DXHunter — Ham radio DX cluster")

		mOpen := systray.AddMenuItem("Open DXHunter", "Open the dashboard in your browser")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Exit DXHunter")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://localhost:8080").Start()
				case <-mQuit.ClickedCh:
					doShutdown()
					systray.Quit()
					return
				}
			}
		}()
	}, doShutdown)
}
