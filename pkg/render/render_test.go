package render

import (
	"bufio"
	"bytes"
	"strings"
	"sync"
	"testing"

	gm "github.com/buger/goterm"
	"github.com/moeart/ntr/pkg/hop"
)

func TestRenderWritesOneCompleteFrame(t *testing.T) {
	var output bytes.Buffer
	originalOutput := gm.Output
	originalScreen := gm.Screen
	gm.Output = bufio.NewWriter(&output)
	gm.Screen.Reset()
	gm.Screen = new(bytes.Buffer)
	clearScreenOnce = sync.Once{}
	defer func() {
		gm.Output = originalOutput
		gm.Screen = originalScreen
	}()

	Render(&NTRRenderConfig{
		Address:        "192.0.2.10",
		StartTime:      "2026-08-19 12:00:00",
		Statistic:      map[int]*hop.HopStatistic{},
		MaxHops:        3,
		Lang:           "en",
		RingBufferSize: 4,
	})

	frame := output.String()
	if !strings.HasPrefix(frame, "\033[2J") {
		t.Fatalf("first frame did not clear the terminal: %q", frame[:min(len(frame), 20)])
	}
	if !strings.Contains(frame, "\033[H") || !strings.Contains(frame, "NTR - MoeArt's Network Traceroute") {
		t.Fatalf("frame is missing home cursor or title: %q", frame)
	}
	if strings.Count(frame, "NTR - MoeArt's Network Traceroute") != 1 {
		t.Fatalf("title was written more than once: %q", frame)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
