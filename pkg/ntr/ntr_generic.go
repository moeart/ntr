//go:build !windows && !linux && !darwin
// +build !windows,!linux,!darwin

package ntr

import (
	"github.com/buger/goterm"
)

func GetTerminalSize() (int, int) {
	// Use goterm's default method for other platforms
	width := goterm.Width() - 2
	height := goterm.Height()

	// Ensure width is at least 78 and height is at least 25
	if width < 78 {
		width = 78
	}
	if height < 25 {
		height = 25
	}

	return width, height
}
