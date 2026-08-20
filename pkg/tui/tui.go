package tui

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/moeart/ntr/pkg/hop"
	"github.com/moeart/ntr/pkg/ntr"
	"github.com/rivo/tview"
)

var (
	ntrTrack = tcell.NewRGBColor(128, 128, 128)
	// Normal primitives intentionally use the terminal's own palette. These
	// aliases make the policy explicit without changing ConEmu or any global
	// terminal color settings.
	ntrBackground = tcell.ColorDefault
	ntrForeground = tcell.ColorDefault
	ntrRed        = tcell.NewRGBColor(255, 80, 80)
	ntrYellow     = tcell.NewRGBColor(255, 220, 0)
)

// UI is the interactive terminal view for a running NTR probe.
type UI struct {
	source       *ntr.NTR
	app          *tview.Application
	title        *tview.TextView
	copyright    *tview.TextView
	meta         *tview.TextView
	header       *tview.Flex
	table        *tview.Table
	tableArea    *tview.Flex
	scrollbar    *tableScrollbar
	footer       *tview.Flex
	footerStatus *tview.TextView
	footerHelp   *tview.TextView
	root         *tview.Flex
	pollDur      time.Duration
	mu           sync.RWMutex
	latest       ntr.Snapshot
	colorEnabled bool
	stopOnce     sync.Once
}

type tableScrollbar struct {
	*tview.Box
	table       *tview.Table
	totalRows   int
	visibleRows int
}

func newTableScrollbar() *tableScrollbar {
	bar := &tableScrollbar{Box: tview.NewBox()}
	bar.SetBackgroundColor(ntrBackground)
	return bar
}

func (bar *tableScrollbar) SetMetrics(totalRows, visibleRows int) {
	bar.totalRows = totalRows
	bar.visibleRows = visibleRows
}

func scrollbarGeometry(totalRows, visibleRows, offset, height int) (top, size int) {
	if totalRows <= 0 || visibleRows <= 0 || totalRows <= visibleRows || height <= 0 {
		return 0, 0
	}
	if offset < 0 {
		offset = 0
	}
	maxOffset := totalRows - visibleRows
	if offset > maxOffset {
		offset = maxOffset
	}
	size = height * visibleRows / totalRows
	if size < 1 {
		size = 1
	}
	if size > height {
		size = height
	}
	if maxOffset > 0 {
		top = offset * (height - size) / maxOffset
	}
	return top, size
}

func (bar *tableScrollbar) Draw(screen tcell.Screen) {
	bar.Box.DrawForSubclass(screen, bar)
	x, y, width, height := bar.GetInnerRect()
	if width < 1 || height < 1 || bar.table == nil {
		return
	}
	offset, _ := bar.table.GetOffset()
	top, size := scrollbarGeometry(bar.totalRows, bar.visibleRows, offset, height)
	if size == 0 {
		return
	}
	trackStyle := tcell.StyleDefault.Foreground(ntrTrack).Background(ntrBackground)
	thumbStyle := tcell.StyleDefault.Foreground(ntrForeground).Background(ntrBackground)
	for row := 0; row < height; row++ {
		ch, style := '|', trackStyle
		if row >= top && row < top+size {
			ch, style = '#', thumbStyle
		}
		screen.SetContent(x+width-1, y+row, ch, nil, style)
	}
}

type columnSpec struct {
	title    string
	align    int
	maxWidth int
	prefix   string
	value    func(hop.Snapshot) string
}

func (c columnSpec) headerText() string {
	return formatCell(c.prefix+c.title, c.maxWidth, c.align)
}

func (c columnSpec) format(item hop.Snapshot) string {
	return formatCell(c.prefix+c.value(item), c.maxWidth, c.align)
}

func formatCell(text string, width int, align int) string {
	if width <= 0 {
		return text
	}
	if runewidth.StringWidth(text) > width {
		text = truncateCell(text, width)
	}
	padding := width - runewidth.StringWidth(text)
	switch align {
	case tview.AlignRight:
		return strings.Repeat(" ", padding) + text
	case tview.AlignCenter:
		left := padding / 2
		return strings.Repeat(" ", left) + text + strings.Repeat(" ", padding-left)
	default:
		return text + strings.Repeat(" ", padding)
	}
}

func truncateCell(text string, width int) string {
	if width <= 0 {
		return ""
	}
	var result strings.Builder
	used := 0
	for _, r := range text {
		runeWidth := runewidth.RuneWidth(r)
		if used+runeWidth > width {
			break
		}
		result.WriteRune(r)
		used += runeWidth
	}
	return result.String()
}

func isEnglish(lang string) bool {
	return strings.EqualFold(strings.TrimSpace(lang), "en")
}

// New creates the original table-oriented NTR interface. The original ten-column
// presentation is kept intact while tview/tcell provide the event loop, mouse
// handling, keyboard navigation, colors, and terminal rendering.
func New(source *ntr.NTR) *UI {
	configureTerminalDefaults()

	title := tview.NewTextView().SetTextAlign(tview.AlignCenter).SetWrap(false).SetScrollable(false)
	title.SetTextColor(ntrForeground)
	title.SetBackgroundColor(ntrBackground)

	copyright := tview.NewTextView().SetTextAlign(tview.AlignCenter).SetWrap(false).SetScrollable(false)
	copyright.SetTextColor(ntrForeground)
	copyright.SetBackgroundColor(ntrBackground)

	meta := tview.NewTextView().SetWrap(false).SetScrollable(false)
	meta.SetTextColor(ntrForeground)
	meta.SetBackgroundColor(ntrBackground)

	header := tview.NewFlex().SetDirection(tview.FlexRow)
	header.SetBackgroundColor(ntrBackground)
	header.AddItem(title, 1, 0, false)
	header.AddItem(copyright, 1, 0, false)
	header.AddItem(meta, 1, 0, false)

	table := tview.NewTable()
	table.SetBackgroundColor(ntrBackground)
	table.SetBorders(false)
	table.SetSeparator(' ')
	table.SetSelectable(false, false)
	table.SetFixed(1, 0)
	table.SetEvaluateAllRows(true)

	scrollbar := newTableScrollbar()
	scrollbar.table = table
	tableArea := tview.NewFlex().SetDirection(tview.FlexColumn)
	tableArea.SetBackgroundColor(ntrBackground)
	tableArea.AddItem(table, 0, 1, true)
	// Keep the scrollbar slot stable across terminal resize events. The custom
	// primitive draws nothing when there is no overflow, avoiding Flex mutation
	// while ConEmu is emitting a resize burst.
	tableArea.AddItem(scrollbar, 1, 0, false)

	footerStatus := tview.NewTextView().SetWrap(false).SetScrollable(false)
	footerStatus.SetTextColor(ntrForeground)
	footerStatus.SetBackgroundColor(ntrBackground)
	footerStatus.SetTextAlign(tview.AlignLeft)

	footerHelp := tview.NewTextView().SetWrap(false).SetScrollable(false)
	footerHelp.SetTextColor(ntrForeground)
	footerHelp.SetBackgroundColor(ntrBackground)
	footerHelp.SetTextAlign(tview.AlignRight)

	footer := tview.NewFlex().SetDirection(tview.FlexColumn)
	footer.SetBackgroundColor(ntrBackground)
	// Reserve enough width for the complete shortcut hint. The previous 2:1
	// split clipped the right side in Kommander-sized windows.
	footer.AddItem(footerStatus, 0, 1, false)
	footer.AddItem(footerHelp, 1, 0, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.SetBackgroundColor(ntrBackground)
	root.AddItem(header, 3, 0, false)
	root.AddItem(tableArea, 0, 1, true)
	root.AddItem(footer, 1, 0, false)

	ui := &UI{
		source:       source,
		app:          tview.NewApplication().SetRoot(root, true).SetTitle("NTR").EnableMouse(false),
		title:        title,
		copyright:    copyright,
		meta:         meta,
		header:       header,
		table:        table,
		tableArea:    tableArea,
		scrollbar:    scrollbar,
		footer:       footer,
		footerStatus: footerStatus,
		footerHelp:   footerHelp,
		root:         root,
		pollDur:      250 * time.Millisecond,
		colorEnabled: true,
	}

	ui.app.SetInputCapture(ui.captureInput)
	// tview.Application already clears the screen before every draw. Avoid an
	// additional BeforeDraw screen operation because ConEmu can emit a burst of
	// resize events while its host window is being resized.
	ui.render(source.Snapshot())
	return ui
}

func configureTerminalDefaults() {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	tview.Styles.ContrastBackgroundColor = tcell.ColorDefault
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorDefault
	tview.Styles.PrimaryTextColor = tcell.ColorDefault
	tview.Styles.SecondaryTextColor = tcell.ColorDefault
	tview.Styles.TertiaryTextColor = tcell.ColorDefault
	tview.Styles.InverseTextColor = tcell.ColorDefault
	tview.Styles.ContrastSecondaryTextColor = tcell.ColorDefault
	tview.Styles.BorderColor = tcell.ColorDefault

	// Keep the fallback safe for terminals that do not render box-drawing
	// characters at one cell wide. The main layout itself has no outer boxes.
	tview.Borders.Horizontal = '-'
	tview.Borders.Vertical = '|'
	tview.Borders.TopLeft = '+'
	tview.Borders.TopRight = '+'
	tview.Borders.BottomLeft = '+'
	tview.Borders.BottomRight = '+'
	tview.Borders.LeftT = '+'
	tview.Borders.RightT = '+'
	tview.Borders.TopT = '+'
	tview.Borders.BottomT = '+'
	tview.Borders.Cross = '+'
	tview.Borders.HorizontalFocus = '='
	tview.Borders.VerticalFocus = '|'
	tview.Borders.TopLeftFocus = '+'
	tview.Borders.TopRightFocus = '+'
	tview.Borders.BottomLeftFocus = '+'
	tview.Borders.BottomRightFocus = '+'
}

// Run starts the TUI event loop and returns after q, Esc, Ctrl+C, or an
// application-level stop. The caller remains responsible for stopping NTR.
func (ui *UI) Run() error {
	stopped := make(chan struct{})
	go ui.poll(stopped)
	err := ui.app.Run()
	close(stopped)
	return err
}

// Stop closes the TUI event loop.
func (ui *UI) Stop() {
	ui.requestStop()
}

func (ui *UI) requestStop() {
	// tview.Stop waits for the event loop to consume its screen replacement.
	// Calling it directly from SetInputCapture deadlocks that same loop.
	ui.stopOnce.Do(func() {
		go ui.app.Stop()
	})
}

func (ui *UI) poll(stopped <-chan struct{}) {
	ticker := time.NewTicker(ui.pollDur)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			snapshot := ui.source.Snapshot()
			ui.app.QueueUpdateDraw(func() {
				ui.render(snapshot)
			})
		case <-stopped:
			return
		}
	}
}

func (ui *UI) captureInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlC, tcell.KeyEscape:
		ui.requestStop()
		return nil
	case tcell.KeyUp:
		return tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)
	case tcell.KeyDown:
		return tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		return event
	case tcell.KeyRune:
		switch event.Rune() {
		case 'q', 'Q':
			ui.requestStop()
			return nil
		case 'r', 'R':
			ui.mu.RLock()
			snapshot := ui.latest
			ui.mu.RUnlock()
			ui.render(snapshot)
			// Returning nil lets tview's event loop perform its normal draw.
			return nil
		case 'c', 'C':
			ui.mu.Lock()
			ui.colorEnabled = !ui.colorEnabled
			snapshot := ui.latest
			ui.mu.Unlock()
			ui.render(snapshot)
			// Returning nil lets tview's event loop perform its normal draw.
			return nil

		}
	}
	return event
}

func (ui *UI) render(snapshot ntr.Snapshot) {
	_, _, width, _ := ui.root.GetRect()
	english := isEnglish(snapshot.Lang)
	ui.syncScrollbar(len(snapshot.Hops))
	if ui.tableArea.GetItemCount() > 1 && width > 1 {
		width--
	}
	columns := columnsForWidth(width, english, snapshot.Address)

	ui.mu.Lock()
	ui.latest = snapshot
	colorEnabled := ui.colorEnabled
	ui.mu.Unlock()

	ui.title.SetText("NTR - MoeArt's Network Traceroute")
	ui.copyright.SetText("(c)2016-2026 MoeArt OpenSource, www.acgdraw.com")
	ui.meta.SetText(metaText(snapshot, width))

	ui.table.Clear()
	for column, spec := range columns {
		// Header and body use the same alignment for each column. This is
		// important for mixed CJK/Latin text and makes numeric headings line up
		// with the numeric values below them.
		cell := tview.NewTableCell(spec.headerText()).SetAlign(spec.align)
		cell.NotSelectable = true
		cell.MaxWidth = spec.maxWidth
		cell.Expansion = 0
		cell.Transparent = false
		cell.Style = headerStyle(colorEnabled)
		ui.table.SetCell(0, column, cell)
	}

	if len(snapshot.Hops) == 0 {
		for column := range columns {
			cell := tview.NewTableCell("")
			if column == 0 {
				cell.SetText("等待探测回包")
			}
			cell.Transparent = false
			cell.Style = bodyStyle(colorEnabled, tcell.ColorDefault)
			ui.table.SetCell(1, column, cell)
		}
	} else {
		for row, item := range snapshot.Hops {
			ui.setRow(row+1, item, columns, colorEnabled)
		}
	}
	ui.updateFooter(snapshot)
}

func (ui *UI) syncScrollbar(hopCount int) {
	_, _, _, height := ui.tableArea.GetRect()
	if height <= 0 {
		return
	}
	totalRows := hopCount + 1
	ui.scrollbar.SetMetrics(totalRows, height)
}

func (ui *UI) updateFooter(snapshot ntr.Snapshot) {
	_, _, width, _ := ui.root.GetRect()
	status, help := statusFooter(snapshot, width)
	ui.footerStatus.SetText(status)
	ui.footerHelp.SetText(help)
	helpWidth := runewidth.StringWidth(help) + 1
	if helpWidth < 1 {
		helpWidth = 1
	}
	if width > 1 && helpWidth >= width {
		helpWidth = width - 1
	}
	ui.footer.ResizeItem(ui.footerStatus, 0, 1)
	ui.footer.ResizeItem(ui.footerHelp, helpWidth, 0)
}

func statusFooter(snapshot ntr.Snapshot, width int) (string, string) {
	english := isEnglish(snapshot.Lang)
	state := "运行中"
	if english {
		state = "RUNNING"
	}
	if !snapshot.Running {
		if english {
			state = "STOPPED"
		} else {
			state = "已停止"
		}
	}
	last := "--"
	if !snapshot.LastActivity.IsZero() {
		last = snapshot.LastActivity.Format("15:04:05")
	}

	var status, help string
	if english {
		status = fmt.Sprintf("State:%s Round:%d TTL:%d/%d Sent:%d Recv:%d Last:%s", state, snapshot.Rounds, snapshot.ActiveHops, snapshot.MaxHops, snapshot.TotalSent, snapshot.TotalReceived, last)
		help = "[Q]uit [R]efresh [C]olor [UP/DN]Page"
		if width > 0 && width < 90 {
			status = fmt.Sprintf("RUN R:%d TTL:%d/%d S:%d R:%d %s", snapshot.Rounds, snapshot.ActiveHops, snapshot.MaxHops, snapshot.TotalSent, snapshot.TotalReceived, last)
		}
	} else {
		status = fmt.Sprintf("状态:%s | 轮次:%d | TTL:%d/%d | 发包:%d | 回包:%d | 最近:%s", state, snapshot.Rounds, snapshot.ActiveHops, snapshot.MaxHops, snapshot.TotalSent, snapshot.TotalReceived, last)
		help = "[Q]退出 [R]刷新 [C]颜色 [UP/DN]翻页"
		if width > 0 && width < 90 {
			status = fmt.Sprintf("运行 轮:%d TTL:%d/%d 发:%d 回:%d 最:%s", snapshot.Rounds, snapshot.ActiveHops, snapshot.MaxHops, snapshot.TotalSent, snapshot.TotalReceived, last)
		}
	}
	return status, help
}

func columnsForWidth(width int, english bool, addresses ...string) []columnSpec {
	if width <= 0 {
		width = 80
	}
	address := ""
	if len(addresses) > 0 {
		address = addresses[0]
	}
	destWidth := 17
	if ip := net.ParseIP(address); ip != nil && ip.To4() == nil {
		destWidth = 40
	}
	const (
		rowNumberWidth = 3
		metricWidth    = 5
		asnWidth       = 7
		columnGaps     = 9 // tview draws one separator between each pair of columns.
	)
	// The original renderer reserves one extra leading space before the target
	// and ASN columns, producing its characteristic two-space visual gaps.
	locationWidth := width - rowNumberWidth - (destWidth + 1) - 6*metricWidth - (asnWidth + 1) - columnGaps
	if locationWidth < 1 {
		locationWidth = 1
	}

	targetTitle, lossTitle, sentTitle := "目标主机", "丢包%", "发送"
	lastTitle, bestTitle, averageTitle, worstTitle := "最近", "最快", "平均", "最慢"
	locationTitle := "IP位置信息"
	if english {
		targetTitle, lossTitle, sentTitle = "HOST", "LOSS%", "SENT"
		lastTitle, bestTitle, averageTitle, worstTitle = "LAST", "BEST", "AVG", "WORST"
		locationTitle = "IP LOCATION"
	}
	return []columnSpec{
		{title: "#", align: tview.AlignCenter, maxWidth: rowNumberWidth, value: func(h hop.Snapshot) string { return fmt.Sprintf("%d", h.TTL) }},
		{title: targetTitle, align: tview.AlignLeft, maxWidth: destWidth + 1, prefix: " ", value: func(h hop.Snapshot) string { return displayTarget(h, english) }},
		{title: lossTitle, align: tview.AlignRight, maxWidth: metricWidth, value: func(h hop.Snapshot) string { return fmt.Sprintf("%.0f", h.Loss) }},
		{title: sentTitle, align: tview.AlignRight, maxWidth: metricWidth, value: func(h hop.Snapshot) string { return fmt.Sprintf("%d", h.Sent) }},
		{title: lastTitle, align: tview.AlignRight, maxWidth: metricWidth, value: func(h hop.Snapshot) string { return formatMilliseconds(h.LastMS, h.Sent-h.Lost > 0) }},
		{title: bestTitle, align: tview.AlignRight, maxWidth: metricWidth, value: func(h hop.Snapshot) string { return formatMilliseconds(h.BestMS, h.BestMS > 0) }},
		{title: averageTitle, align: tview.AlignRight, maxWidth: metricWidth, value: func(h hop.Snapshot) string { return formatMilliseconds(h.AverageMS, h.AverageMS > 0) }},
		{title: worstTitle, align: tview.AlignRight, maxWidth: metricWidth, value: func(h hop.Snapshot) string { return formatMilliseconds(h.WorstMS, h.WorstMS > 0) }},
		{title: "ASN", align: tview.AlignLeft, maxWidth: asnWidth + 1, prefix: " ", value: func(h hop.Snapshot) string { return h.ASN }},
		{title: locationTitle, align: tview.AlignLeft, maxWidth: locationWidth, value: func(h hop.Snapshot) string { return h.Location }},
	}
}

func (ui *UI) setRow(row int, item hop.Snapshot, columns []columnSpec, colorEnabled bool) {
	color := tcell.ColorDefault
	if colorEnabled {
		switch {
		case item.Loss >= 100:
			color = ntrRed
		case item.Loss > 0:
			color = ntrYellow
		}
	}
	style := bodyStyle(colorEnabled, color)
	for column, spec := range columns {
		cell := tview.NewTableCell(spec.format(item)).SetAlign(spec.align)
		cell.MaxWidth = spec.maxWidth
		cell.Expansion = 0
		cell.Transparent = false
		cell.Style = style
		ui.table.SetCell(row, column, cell)
	}
}

func displayTarget(item hop.Snapshot, english bool) string {
	if item.Target != "" {
		return item.Target
	}
	if item.Loss >= 100 {
		if english {
			return "TIMEOUT"
		}
		return "请求超时"
	}
	return "-"
}

func headerStyle(_ bool) tcell.Style {
	// Kommander may expose a light desktop through terminal transparency. Keep
	// the table header on a dark band with explicit white text instead of using
	// ColorDefault, which can become gray or invisible on that background.
	// Original render.go emits ANSI black foreground on white background for the
	// title bar. Keep that mapping local to header cells so tview remains the
	// rendering engine and the terminal's palette is not changed globally.
	return tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
}

func bodyStyle(colorEnabled bool, color tcell.Color) tcell.Style {
	style := tcell.StyleDefault.Foreground(ntrForeground).Background(ntrBackground)
	if colorEnabled && color != tcell.ColorDefault {
		style = style.Foreground(color)
	}
	return style
}

func metaText(snapshot ntr.Snapshot, width int) string {
	left := fmt.Sprintf("DEST:  %s", snapshot.Address)
	right := fmt.Sprintf("START: %s", snapshot.StartTime)
	gap := width - len(left) - len(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}

func formatMilliseconds(value float64, valid bool) string {
	if !valid || value <= 0 {
		return "*"
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
}
