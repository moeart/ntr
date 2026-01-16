//go:build linux
// +build linux

package ntr

import (
	"bytes"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

func GetTerminalSize() (int, int) {
	var ws syscall.Winsize
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(syscall.Stdout), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws))); errno == 0 {
		width := int(ws.Col)
		height := int(ws.Row)
		width -= 2 // Reduce width by 2 characters to avoid interface overflow

		// Ensure width is at least 78 and height is at least 25
		if width < 78 {
			width = 78
		}
		if height < 25 {
			height = 25
		}

		return width, height
	}

	// If API call fails, use stty size command as fallback
	cmd := exec.Command("stty", "size")
	cmd.Stdin = syscall.Stdin
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err == nil {
		// Parse output: height width
		output := out.String()
		var height, width int
		if _, err := fmt.Sscanf(output, "%d %d", &height, &width); err == nil {
			width -= 2 // Reduce width by 2 characters to avoid interface overflow

			// Ensure width is at least 78 and height is at least 25
			if width < 78 {
				width = 78
			}
			if height < 25 {
				height = 25
			}

			return width, height
		}
	}

	// If all fail, use default values
	width := 78
	height := 25

	return width, height
}
