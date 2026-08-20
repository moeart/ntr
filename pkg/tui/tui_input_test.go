package tui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/moeart/ntr/pkg/ntr"
)

func newInputTestUI(t *testing.T) *UI {
	t.Helper()
	source, _, err := ntr.NewNTR(
		"127.0.0.1", "", time.Second, time.Second, 0,
		3, 1, 2, false, false, false, "zh", false, false, false,
	)
	if err != nil {
		t.Fatalf("NewNTR: %v", err)
	}
	return New(source)
}

func TestCaptureInputShortcutsReturnWithoutBlocking(t *testing.T) {
	ui := newInputTestUI(t)

	if got := ui.captureInput(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone)); got != nil {
		t.Fatal("r should be consumed by the TUI")
	}
	if got := ui.captureInput(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone)); got != nil {
		t.Fatal("c should be consumed by the TUI")
	}
	if got := ui.captureInput(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)); got == nil || got.Key() != tcell.KeyPgUp {
		t.Fatal("Up should be translated to PageUp")
	}
	if got := ui.captureInput(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); got == nil || got.Key() != tcell.KeyPgDn {
		t.Fatal("Down should be translated to PageDown")
	}
	if got := ui.captureInput(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone)); got != nil {
		t.Fatal("q should be consumed by the TUI")
	}
}

func TestCaptureInputEscapeIsConsumed(t *testing.T) {
	ui := newInputTestUI(t)
	if got := ui.captureInput(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Fatal("Escape should be consumed by the TUI")
	}
}
