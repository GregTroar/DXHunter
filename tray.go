package main

import (
	_ "embed"
	"os/exec"
	"runtime"
	"sync"

	"github.com/getlantern/systray"
)

//go:embed images/FlexDXCluster.ico
var appIcon []byte

// runTray sets up the system-tray icon and blocks (must be called from main goroutine).
// It is the replacement for CheckSignal: the app stays alive until the user picks
// "Quit" from the tray menu or sends an OS interrupt signal.
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

	systray.Run(func() {
		systray.SetIcon(appIcon)
		systray.SetTitle("FlexDXCluster")
		systray.SetTooltip("FlexDXCluster — Ham radio DX cluster")

		mOpen := systray.AddMenuItem("Open FlexDXCluster", "Open the dashboard in your browser")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Exit FlexDXCluster")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					openBrowser("http://localhost:8080")
				case <-mQuit.ClickedCh:
					doShutdown()
					systray.Quit()
					return
				}
			}
		}()
	}, doShutdown)
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

