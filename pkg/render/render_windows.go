//go:build windows
// +build windows

package render

import (
	"bytes"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"unsafe"

	gm "github.com/buger/goterm"
)

// Define Windows API functions and structures
var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getConsoleScreenBuf = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type coord struct {
	X, Y int16
}

type smallRect struct {
	Left, Top, Right, Bottom int16
}

type consoleScreenBufferInfo struct {
	DwSize              coord
	DwCursorPosition    coord
	WAttributes         uint16
	SrWindow            smallRect
	DwMaximumWindowSize coord
}

// getWindowsTerminalSize - Windows-specific implementation to get terminal size
func getWindowsTerminalSize() (int, int) {
	var width, height int

	// Windows implementation
	var csbi consoleScreenBufferInfo
	r1, _, _ := getConsoleScreenBuf.Call(
		uintptr(syscall.Stdout),
		uintptr(unsafe.Pointer(&csbi)),
	)
	if r1 != 0 {
		width = int(csbi.SrWindow.Right - csbi.SrWindow.Left + 1)
		height = int(csbi.SrWindow.Bottom - csbi.SrWindow.Top + 1)
		width -= 2 // Reduce width by 2 characters to avoid interface overflow
	} else {
		// If API call fails, use mode con command as fallback
		cmd := exec.Command("mode", "con")
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err == nil {
			// Parse output
			output := out.String()
			widthRegex := regexp.MustCompile(`Columns:\s*(\d+)`)
			heightRegex := regexp.MustCompile(`Lines:\s*(\d+)`)

			widthMatch := widthRegex.FindStringSubmatch(output)
			heightMatch := heightRegex.FindStringSubmatch(output)

			if len(widthMatch) > 1 {
				width, _ = strconv.Atoi(widthMatch[1])
				width -= 2
			} else {
				width = 78
			}

			if len(heightMatch) > 1 {
				height, _ = strconv.Atoi(heightMatch[1])
			} else {
				height = 25
			}
		} else {
			// If all fail, use goterm's default method
			width = gm.Width() - 2
			height = gm.Height()
		}
	}

	return width, height
}
