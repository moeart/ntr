package render

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	gm "github.com/buger/goterm"
	"github.com/moeart/ntr/pkg/asn"
	"github.com/moeart/ntr/pkg/geoip"
	"github.com/moeart/ntr/pkg/hop"
	"golang.org/x/term"
)

// Tool name and copyright information, defined as global variables for access by other packages
var (
	ToolName      = "NTR - MoeArt's Network Traceroute"
	ToolCopyright = "(c)2016-2026 MoeArt OpenSource, www.acgdraw.com"
)

// Create window size change detection channel
var resizeChan = make(chan bool, 1)

// Once for clear screen operation
var clearScreenOnce sync.Once

// NTRRenderConfig contains configuration for rendering NTR data
type NTRRenderConfig struct {
	SrcAddress     string
	Address        string
	Statistic      map[int]*hop.HopStatistic
	Timeout        time.Duration
	Interval       time.Duration
	MaxHops        int
	MaxUnknownHops int
	RingBufferSize int
	PtrLookup      bool
	EnableAsn      bool
	Asns           *asn.ASNs
	EnableGeoIP    bool
	GeoIP          *geoip.GeoIP
	Lang           string
	UseQQWry       bool
}

// Render renders the NTR data to the terminal
func Render(m *NTRRenderConfig) {
	// Clear screen only once before first render
	clearScreenOnce.Do(ClearScreen)

	// Get terminal size
	width, _ := GetTerminalSize()
	maxLength := width - 3 // Prevent overflow

	// Print tool information

	// Print tool name and copyright information centered
	padding := (maxLength - len(ToolName)) / 2
	if padding > 0 {
		gm.Printf("%*s%s%*s\n", padding, "", ToolName, padding, "")
	} else {
		gm.Printf("%s\n", ToolName[:maxLength])
	}

	padding = (maxLength - len(ToolCopyright)) / 2
	if padding > 0 {
		gm.Printf("%*s%s%*s\n", padding, "", ToolCopyright, padding, "")
	} else {
		gm.Printf("%s\n", ToolCopyright[:maxLength])
	}

	// Print information line
	infoLeft := fmt.Sprintf("DEST: %s", m.Address)
	startTime := time.Now().Format("2006-01-02 15:04:05")
	infoRight := fmt.Sprintf("START: %s", startTime)
	paddingRight := maxLength - len(infoLeft) - len(infoRight)
	if paddingRight > 0 {
		gm.Printf("%s%s%s\n", infoLeft, strings.Repeat(" ", paddingRight), infoRight)
	} else {
		gm.Printf("%s\n", infoLeft[:maxLength])
	}

	// Determine column format, adjust DESTINATION column width based on IPv4/IPv6
	isIPv6 := net.ParseIP(m.Address).To4() == nil
	var destWidth int
	if isIPv6 {
		destWidth = 40
	} else {
		destWidth = 17
	}

	// Calculate other column widths
	lossWidth := 5
	sentWidth := 5
	lastWidth := 5
	bestWidth := 5
	avgWidth := 5
	wrstWidth := 5
	asnWidth := 7
	locationWidth := maxLength - 3 - 2 - destWidth - 2 - lossWidth - sentWidth - lastWidth - bestWidth - avgWidth - wrstWidth - 2 - asnWidth - 1

	// Build format string
	format := "%3s  %s %s %s %s %s %s %s  %s %s"

	// Print title bar
	var title string
	if m.Lang == "zh" {
		title = fmt.Sprintf(format,
			"#",
			padString("目标主机", destWidth),
			rightPadString("丢包%", lossWidth),
			rightPadString("发送", sentWidth),
			rightPadString("最近", lastWidth),
			rightPadString("最快", bestWidth),
			rightPadString("平均", avgWidth),
			rightPadString("最慢", wrstWidth),
			padString("ASN", asnWidth),
			padString("IP位置信息", locationWidth),
		)
	} else {
		title = fmt.Sprintf(format,
			"#",
			padString("DESTINATION", destWidth),
			rightPadString("LOSS%", lossWidth),
			rightPadString("SENT", sentWidth),
			rightPadString("LAST", lastWidth),
			rightPadString("BEST", bestWidth),
			rightPadString("AVG", avgWidth),
			rightPadString("WRST", wrstWidth),
			padString("ASN", asnWidth),
			padString("LOCATION", locationWidth),
		)
	}

	// Title bar contrast highlight effect
	gm.Println(gm.Background(gm.Color(title, gm.BLACK), gm.WHITE))

	// Print hop information
	foundTarget := false
	for i := 1; i <= m.MaxHops; i++ {
		hopStat := m.Statistic[i]
		if hopStat != nil && !foundTarget {
			// Check if current hop contains the target address
			for _, target := range hopStat.Targets {
				if target == m.Address {
					foundTarget = true
					break
				}
			}

			hopStat.Render(m.PtrLookup, width, destWidth, i)
			if foundTarget {
				break
			}
		}
	}
}

// GetTerminalSize - Get terminal size, compatible with Windows, Linux and macOS
func GetTerminalSize() (int, int) {
	var width, height int

	switch runtime.GOOS {
	case "windows":
		// Windows implementation
		// Use the Windows-specific implementation from ntr_windows.go
		width, height = getWindowsTerminalSize()
	case "linux", "darwin":
		// Linux and macOS implementation
		// Use goterm's method for cross-platform compatibility
		width = gm.Width() - 2
		height = gm.Height()
	default:
		// Fallback for other platforms
		width = gm.Width() - 2
		height = gm.Height()
	}

	// Ensure width is at least 78 and height is at least 25
	if width < 78 {
		width = 78
	}
	if height < 25 {
		height = 25
	}

	return width, height
}

// getStringDisplayWidth - Calculate string display width, Chinese characters count as 2, English characters count as 1
func getStringDisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			// Chinese characters occupy 2 widths
			width += 2
		} else {
			// Other characters occupy 1 width
			width += 1
		}
	}
	return width
}

// padString - Pad string to specified width, supporting Chinese characters (left alignment)
func padString(s string, width int) string {
	currentWidth := getStringDisplayWidth(s)
	if currentWidth >= width {
		return s
	}

	padding := width - currentWidth
	return s + strings.Repeat(" ", padding)
}

// rightPadString - Pad string to specified width, supporting Chinese characters (right alignment)
func rightPadString(s string, width int) string {
	currentWidth := getStringDisplayWidth(s)
	if currentWidth >= width {
		return s
	}

	padding := width - currentWidth
	return strings.Repeat(" ", padding) + s
}

// Function to detect window size changes
func monitorWindowResize(resizeChan chan bool) {
	prevWidth, _ := GetTerminalSize()
	for {
		time.Sleep(200 * time.Millisecond) // Check every 200ms
		currWidth, _ := GetTerminalSize()
		if currWidth != prevWidth {
			resizeChan <- true
			prevWidth = currWidth
		}
	}
}

// RenderNTR renders the NTR data to the terminal
func RenderNTR(m *NTRRenderConfig) {
	Render(m)
}

// GetResizeChan returns the resize channel
func GetResizeChan() chan bool {
	return resizeChan
}

// WatchWindowSize starts watching for window size changes
func WatchWindowSize() {
	// Use polling to detect window size changes
	prevWidth, _ := GetTerminalSize()
	for {
		time.Sleep(200 * time.Millisecond) // Check every 200ms
		currWidth, _ := GetTerminalSize()
		if currWidth != prevWidth {
			resizeChan <- true
			prevWidth = currWidth
		}
	}
}

// ClearScreen clears the screen
func ClearScreen() {
	gm.Clear()
	gm.Flush()
}

// MoveCursor moves the cursor to the specified position
func MoveCursor(x, y int) {
	gm.MoveCursor(x, y)
}

// Flush flushes the output
func Flush() {
	gm.Flush()
}

// SetupInterruptHandler 设置中断处理器
func SetupInterruptHandler(onInterrupt func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		onInterrupt()
		os.Exit(0)
	}()
}

// RestoreTerminal 恢复终端状态
func RestoreTerminal() {
	if oldState, err := term.GetState(int(os.Stdin.Fd())); err == nil {
		term.Restore(int(os.Stdin.Fd()), oldState)
	}
}
