package main

import (
	"bytes"
	"encoding/binary"
	"os/exec"
	"runtime"
	"sync"

	"github.com/getlantern/systray"
)

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
		systray.SetIcon(generateIcon())
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

// generateIcon builds a 32×32 green-circle icon in ICO format (Windows BMP-in-ICO).
// No external files needed — generated at startup.
func generateIcon() []byte {
	const size = 32
	const cx, cy = size / 2, size / 2
	const radius = size/2 - 2

	// XOR mask: BGRA pixels stored bottom-up (BMP convention).
	pixels := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := x-cx, y-cy
			row := size - 1 - y // flip vertically for BMP
			i := (row*size + x) * 4
			if dx*dx+dy*dy <= radius*radius {
				pixels[i+0] = 94  // B
				pixels[i+1] = 197 // G
				pixels[i+2] = 34  // R
				pixels[i+3] = 255 // A (opaque)
			}
		}
	}

	// AND mask: 1 bit/pixel, rows padded to DWORD. All zeros → use XOR colour.
	andRowBytes := (size + 31) / 32 * 4
	andMask := make([]byte, size*andRowBytes)

	// BITMAPINFOHEADER (40 bytes)
	var bih bytes.Buffer
	bw := func(v interface{}) { binary.Write(&bih, binary.LittleEndian, v) } //nolint:errcheck
	bw(uint32(40))       // biSize
	bw(int32(size))      // biWidth
	bw(int32(size * 2))  // biHeight = XOR height + AND height (both size)
	bw(uint16(1))        // biPlanes
	bw(uint16(32))       // biBitCount
	bw(uint32(0))        // biCompression = BI_RGB
	bw(uint32(0))        // biSizeImage
	bw(int32(0))         // biXPelsPerMeter
	bw(int32(0))         // biYPelsPerMeter
	bw(uint32(0))        // biClrUsed
	bw(uint32(0))        // biClrImportant

	var imgData bytes.Buffer
	imgData.Write(bih.Bytes())
	imgData.Write(pixels)
	imgData.Write(andMask)
	img := imgData.Bytes()

	// ICO container: 6-byte header + 16-byte directory + image data.
	var ico bytes.Buffer
	iw := func(v interface{}) { binary.Write(&ico, binary.LittleEndian, v) } //nolint:errcheck
	iw(uint16(0))         // reserved
	iw(uint16(1))         // type = ICO
	iw(uint16(1))         // image count = 1
	ico.WriteByte(byte(size)) // width
	ico.WriteByte(byte(size)) // height
	ico.WriteByte(0)          // color count (0 = no palette)
	ico.WriteByte(0)          // reserved
	iw(uint16(1))             // planes
	iw(uint16(32))            // bits per pixel
	iw(uint32(len(img)))      // image data size
	iw(uint32(22))            // offset to image data (6 + 16 = 22)
	ico.Write(img)

	return ico.Bytes()
}
