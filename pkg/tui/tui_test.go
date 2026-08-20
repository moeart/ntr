package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/moeart/ntr/pkg/hop"
	"github.com/moeart/ntr/pkg/ntr"
	"github.com/rivo/tview"
)

func TestColumnsShrinkForSmallTerminals(t *testing.T) {
	for _, width := range []int{80, 110, 140} {
		columns := columnsForWidth(width, false)
		if got := len(columns); got != 10 {
			t.Fatalf("%d-column layout has %d columns, want original 10", width, got)
		}
		if columns[8].title != "ASN" || columns[9].title != "IP位置信息" {
			t.Fatalf("%d-column layout dropped ASN/location: %#v", width, []string{columns[8].title, columns[9].title})
		}
	}
	small := columnsForWidth(80, false)
	if small[4].maxWidth != 5 || small[7].maxWidth != 5 {
		t.Fatalf("original metric widths changed: last=%d worst=%d", small[4].maxWidth, small[7].maxWidth)
	}
	if small[9].maxWidth < 1 {
		t.Fatalf("location column disappeared on a small terminal")
	}
}

func TestStatusTextContainsLiveCounters(t *testing.T) {
	snapshot := ntr.Snapshot{
		MaxHops:       25,
		ActiveHops:    1,
		Rounds:        3,
		TotalSent:     7,
		TotalReceived: 7,
		Running:       true,
		LastActivity:  time.Date(2026, 8, 19, 22, 48, 42, 0, time.Local),
	}
	status, help := statusFooter(snapshot, 140)
	combined := status + " " + help
	for _, want := range []string{"状态:运行中", "轮次:3", "TTL:1/25", "发包:7", "回包:7", "22:48:42", "[Q]退出", "[UP/DN]翻页"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("footer %q does not contain %q", combined, want)
		}
	}
	if strings.Contains(combined, "选中") || strings.Contains(combined, "Selected") {
		t.Fatalf("footer still exposes a selected row: %q", combined)
	}
}

func TestDisplayTargetUsesTimeoutLabel(t *testing.T) {
	if got := displayTarget(hop.Snapshot{Loss: 100}, false); got != "请求超时" {
		t.Fatalf("timeout target = %q, want 请求超时", got)
	}
	if got := displayTarget(hop.Snapshot{Target: "192.0.2.1"}, false); got != "192.0.2.1" {
		t.Fatalf("successful target = %q", got)
	}
	if got := displayTarget(hop.Snapshot{}, false); got != "-" {
		t.Fatalf("unknown target = %q, want -", got)
	}
}

func TestEnglishColumnsAlignWithBodyValues(t *testing.T) {
	columns := columnsForWidth(140, true)
	if columns[0].align != tview.AlignCenter {
		t.Fatalf("row number alignment = %d, want center", columns[0].align)
	}
	for _, index := range []int{2, 3, 4, 5, 6, 7} {
		if columns[index].align != tview.AlignRight {
			t.Fatalf("column %d alignment = %d, want right", index, columns[index].align)
		}
	}
	if columns[1].title != "HOST" || columns[9].title != "IP LOCATION" {
		t.Fatalf("English titles not applied: %q, %q", columns[1].title, columns[9].title)
	}
	if got := displayTarget(hop.Snapshot{Loss: 100}, true); got != "TIMEOUT" {
		t.Fatalf("English timeout label = %q", got)
	}
}

func TestConfigureTerminalDefaultsUsesASCIIBorders(t *testing.T) {
	configureTerminalDefaults()
	if tview.Borders.Horizontal != '-' || tview.Borders.Vertical != '|' || tview.Borders.Cross != '+' {
		t.Fatalf("TUI borders are not ASCII compatible: %#v", tview.Borders)
	}
	if tcell.ColorDefault == tcell.ColorBlack {
		t.Fatal("test assumption invalid: ColorDefault must remain terminal-controlled")
	}
}

func TestKommanderStylesUseExplicitColors(t *testing.T) {
	bodyFG, bodyBG, _ := bodyStyle(false, tcell.ColorDefault).Decompose()
	if bodyFG != ntrForeground || bodyBG != ntrBackground {
		t.Fatalf("body style = fg %v bg %v, want explicit black on white", bodyFG, bodyBG)
	}
	headerFG, headerBG, _ := headerStyle(false).Decompose()
	if headerFG != tcell.ColorBlack || headerBG != tcell.ColorWhite {
		t.Fatalf("header style = fg %v bg %v, want original black on white title bar", headerFG, headerBG)
	}
}

func TestCompactFooterDoesNotUseLongHint(t *testing.T) {
	snapshot := ntr.Snapshot{Lang: "en", MaxHops: 25, Running: true}
	_, help := statusFooter(snapshot, 80)
	if help != "[Q]uit [R]efresh [C]olor [UP/DN]Page" {
		t.Fatalf("compact English help = %q", help)
	}
	if strings.Contains(help, "j/k") {
		t.Fatalf("compact English help exposes removed navigation keys: %q", help)
	}
}

func TestMetaTextContainsDestinationAndStart(t *testing.T) {
	snapshot := ntr.Snapshot{
		Address:   "127.0.0.1",
		StartTime: "2026-08-19 23:09:19",
	}
	meta := metaText(snapshot, 80)
	if !strings.Contains(meta, "DEST:  127.0.0.1") || !strings.Contains(meta, "START: 2026-08-19 23:09:19") {
		t.Fatalf("meta line missing destination or start time: %q", meta)
	}
}
