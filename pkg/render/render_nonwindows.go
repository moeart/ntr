// +build !windows

package render

// This is a dummy implementation for non-Windows platforms
// It should never be called due to the runtime.GOOS check in GetTerminalSize
func getWindowsTerminalSize() (int, int) {
	return 0, 0
}
