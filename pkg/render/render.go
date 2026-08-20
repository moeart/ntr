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
	StartTime      string
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
	// Clear once, then repaint from the top. The frame itself is submitted in
	// one write below; mixing goterm's Screen buffer with fmt.Printf caused
	// visible tearing and partial frames.
	clearScreenOnce.Do(ClearScreen)

	width, _ := GetTerminalSize()
	maxLength := width - 3
	var frame strings.Builder

	padding := (maxLength - getStringDisplayWidth(ToolName)) / 2
	if padding > 0 {
		fmt.Fprintf(&frame, "%*s%s%*s\n", padding, "", ToolName, padding, "")
	} else {
		frame.WriteString(truncateString(ToolName, maxLength))
		frame.WriteByte('\n')
	}

	padding = (maxLength - getStringDisplayWidth(ToolCopyright)) / 2
	if padding > 0 {
		fmt.Fprintf(&frame, "%*s%s%*s\n", padding, "", ToolCopyright, padding, "")
	} else {
		frame.WriteString(truncateString(ToolCopyright, maxLength))
		frame.WriteByte('\n')
	}

	infoLeft := fmt.Sprintf("DEST: %s", m.Address)
	infoRight := fmt.Sprintf("START: %s", m.StartTime)
	paddingRight := maxLength - getStringDisplayWidth(infoLeft) - getStringDisplayWidth(infoRight)
	if paddingRight > 0 {
		fmt.Fprintf(&frame, "%s%s%s\n", infoLeft, strings.Repeat(" ", paddingRight), infoRight)
	} else {
		frame.WriteString(truncateString(infoLeft, maxLength))
		frame.WriteByte('\n')
	}

	isIPv6 := net.ParseIP(m.Address).To4() == nil
	destWidth := 17
	if isIPv6 {
		destWidth = 40
	}

	lossWidth := 5
	sentWidth := 5
	lastWidth := 5
	bestWidth := 5
	avgWidth := 5
	wrstWidth := 5
	asnWidth := 7
	locationWidth := maxLength - 3 - 2 - destWidth - 2 - lossWidth - sentWidth - lastWidth - bestWidth - avgWidth - wrstWidth - 2 - asnWidth - 1
	if locationWidth < 1 {
		locationWidth = 1
	}

	format := "%3s  %s %s %s %s %s %s %s  %s %s"
	var title string
	if m.Lang == "zh" {
		title = fmt.Sprintf(format, "#", padString("目标主机", destWidth), rightPadString("丢包%", lossWidth), rightPadString("发送", sentWidth), rightPadString("最近", lastWidth), rightPadString("最快", bestWidth), rightPadString("平均", avgWidth), rightPadString("最慢", wrstWidth), padString("ASN", asnWidth), padString("IP位置信息", locationWidth))
	} else {
		title = fmt.Sprintf(format, "#", padString("DESTINATION", destWidth), rightPadString("LOSS%", lossWidth), rightPadString("SENT", sentWidth), rightPadString("LAST", lastWidth), rightPadString("BEST", bestWidth), rightPadString("AVG", avgWidth), rightPadString("WRST", wrstWidth), padString("ASN", asnWidth), padString("LOCATION", locationWidth))
	}
	frame.WriteString(gm.Background(title, gm.BLACK))
	frame.WriteByte('\n')

	foundTarget := false
	for i := 1; i <= m.MaxHops; i++ {
		hopStat := m.Statistic[i]
		if hopStat == nil || foundTarget {
			continue
		}
		for _, target := range hopStat.Targets {
			if target == m.Address {
				foundTarget = true
				break
			}
		}
		if line := hopStat.Render(m.PtrLookup, width, destWidth, i); line != "" {
			frame.WriteString(line)
			frame.WriteByte('\n')
		}
	}

	// Home first, then erase anything left by a taller previous frame. This
	// avoids a full-screen clear on every update while still preventing stale
	// rows after a resize or route convergence.
	gm.Output.WriteString("\033[H")
	gm.Output.WriteString(frame.String())
	gm.Output.WriteString("\033[J")
	gm.Output.Flush()
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
func truncateString(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if getStringDisplayWidth(s) <= maxWidth {
		return s
	}

	var b strings.Builder
	currentWidth := 0
	for _, r := range s {
		runeWidth := 1
		if r >= 0x4e00 && r <= 0x9fff {
			runeWidth = 2
		}
		if currentWidth+runeWidth > maxWidth {
			break
		}
		b.WriteRune(r)
		currentWidth += runeWidth
	}
	return b.String()
}

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
