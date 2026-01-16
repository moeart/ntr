//go:build windows
// +build windows

package ntr

import (
	"bytes"
	"container/ring"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	gm "github.com/buger/goterm"
	"github.com/moeart/ntr/pkg/asn"
	"github.com/moeart/ntr/pkg/geoip"
	"github.com/moeart/ntr/pkg/hop"
	"github.com/moeart/ntr/pkg/icmp"
)

// Tool name and copyright information, defined as global variables for access by other packages
var (
	ToolName      = "NTR - MoeArt's Network Traceroute"
	ToolCopyright = "(c)2016-2026 MoeArt OpenSource, www.acgdraw.com"
)

type NTR struct {
	SrcAddress     string `json:"source"`
	mutex          *sync.RWMutex
	timeout        time.Duration
	interval       time.Duration
	Address        string `json:"destination"`
	hopsleep       time.Duration
	Statistic      map[int]*hop.HopStatistic `json:"statistic"`
	ringBufferSize int
	maxHops        int
	maxUnknownHops int
	ptrLookup      bool
	enableAsn      bool
	asns           *asn.ASNs
	enableGeoIP    bool
	geoip          *geoip.GeoIP
	lang           string
	useQQWry       bool
}

func NewNTR(addr, srcAddr string, timeout time.Duration, interval time.Duration,
	hopsleep time.Duration, maxHops, maxUnknownHops, ringBufferSize int, ptr bool, enableAsn bool, enableGeoIP bool, lang string, useQQWry bool, forceIPv4 bool, forceIPv6 bool) (*NTR, chan struct{}, error) {
	if net.ParseIP(addr) == nil {
		// Domain resolution
		if forceIPv4 {
			// Force resolve IPv4 address
			ipAddr, err := net.ResolveIPAddr("ip4", addr)
			if err != nil {
				return nil, nil, fmt.Errorf("no IPv4 address found for host: %s", err)
			}
			addr = ipAddr.IP.String()
		} else if forceIPv6 {
			// Force resolve IPv6 address
			ipAddr, err := net.ResolveIPAddr("ip6", addr)
			if err != nil {
				return nil, nil, fmt.Errorf("no IPv6 address found for host: %s", err)
			}
			addr = ipAddr.IP.String()
		} else {
			// Default resolution behavior: IPv6 first (if available)
			// First try to resolve IPv6 address
			ipv6Addr, err := net.ResolveIPAddr("ip6", addr)
			if err == nil {
				addr = ipv6Addr.IP.String()
			} else {
				// IPv6 resolution failed, try to resolve IPv4 address
				ipv4Addr, err := net.ResolveIPAddr("ip4", addr)
				if err != nil {
					return nil, nil, fmt.Errorf("no valid IP address found for host: %s", err)
				}
				addr = ipv4Addr.IP.String()
			}
		}
	}
	if srcAddr == "" {
		if net.ParseIP(addr).To4() != nil {
			srcAddr = "0.0.0.0"
		} else {
			srcAddr = "::"
		}
	}

	ntr := &NTR{
		SrcAddress:     srcAddr,
		interval:       interval,
		timeout:        timeout,
		hopsleep:       hopsleep,
		Address:        addr,
		mutex:          &sync.RWMutex{},
		Statistic:      map[int]*hop.HopStatistic{},
		maxHops:        maxHops,
		ringBufferSize: ringBufferSize,
		maxUnknownHops: maxUnknownHops,
		ptrLookup:      ptr,
		enableAsn:      enableAsn,
		enableGeoIP:    enableGeoIP,
		lang:           lang,
		useQQWry:       useQQWry,
	}

	if enableAsn {
		// Load ASN database
		var err error
		ntr.asns, err = asn.NewASNs()
		if err != nil {
			return nil, nil, err
		}
	}

	if enableGeoIP {
		// Load GeoIP database
		var err error
		ntr.geoip, err = geoip.NewGeoIP()
		if err != nil {
			return nil, nil, err
		}
	}

	return ntr, make(chan struct{}, 1), nil
}

func (m *NTR) registerStatistic(ttl int, r icmp.ICMPReturn) *hop.HopStatistic {
	s, ok := m.Statistic[ttl]
	if !ok {
		s = &hop.HopStatistic{
			Sent:           0,
			TTL:            ttl,
			Timeout:        m.timeout,
			Last:           r,
			Worst:          r,
			Lost:           0,
			Packets:        ring.New(m.ringBufferSize),
			RingBufferSize: m.ringBufferSize,
			Targets:        []string{}, // Initialize Targets field to prevent nil pointer reference
			Asns:           m.asns,     // Set ASNs field
			GeoIP:          m.geoip,    // Set GeoIP field
			Lang:           m.lang,
			UseQQWry:       m.useQQWry,
		}
		m.Statistic[ttl] = s
	}

	s.Last = r
	s.Sent++

	s.Targets = addTarget(s.Targets, r.Addr)

	s.Packets = s.Packets.Prev()
	s.Packets.Value = r

	if !r.Success {
		s.Lost++
		return s // do not count failed into statistics
	}

	s.SumElapsed = r.Elapsed + s.SumElapsed

	if !s.Best.Success || s.Best.Elapsed > r.Elapsed {
		s.Best = r
	}
	if s.Worst.Elapsed < r.Elapsed {
		s.Worst = r
	}

	return s
}

func addTarget(currentTargets []string, toAdd string) []string {
	for _, t := range currentTargets {
		if t == toAdd {
			// already added
			return currentTargets
		}
	}

	var newTargets []string
	if len(currentTargets) > 0 {
		// do not add no-ip target
		if toAdd == "" {
			return currentTargets
		}

		// remove no-ip target
		for _, t := range currentTargets {
			if t != "" {
				newTargets = append(newTargets, t)
			}
		}
	} else {
		newTargets = currentTargets
	}

	// add the new one
	return append(newTargets, toAdd)
}

// TODO: aggregates everything using the first target even when there are multiple
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

// Use Windows API to get console size directly, more accurate and faster
func GetTerminalSize() (int, int) {
	var csbi consoleScreenBufferInfo
	r1, _, _ := getConsoleScreenBuf.Call(
		uintptr(syscall.Stdout),
		uintptr(unsafe.Pointer(&csbi)),
	)
	if r1 != 0 {
		width := int(csbi.SrWindow.Right - csbi.SrWindow.Left + 1)
		height := int(csbi.SrWindow.Bottom - csbi.SrWindow.Top + 1)
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

		var width, height int
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

		// Ensure width is at least 78 and height is at least 25
		if width < 78 {
			width = 78
		}
		if height < 25 {
			height = 25
		}

		return width, height
	}

	// If all fail, use goterm's default method
	width := gm.Width() - 2
	height := gm.Height()

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

func (m *NTR) Render(offset int) {

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
	if m.lang == "zh" {
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
	for i := 1; i <= len(m.Statistic); i++ {
		m.mutex.RLock()
		hopStat := m.Statistic[i]
		if hopStat != nil && !foundTarget {
			// Check if current hop contains the target address
			for _, target := range hopStat.Targets {
				if target == m.Address {
					foundTarget = true
					break
				}
			}

			hopStat.Render(m.ptrLookup, width, destWidth)

			if foundTarget {
				m.mutex.RUnlock()
				break
			}
		}
		m.mutex.RUnlock()
	}
}

func (m *NTR) Run(ch chan struct{}, count int) {
	// Ignore count parameter, let the program run continuously until Ctrl+C is pressed
	m.discover(ch)
}

// discover discovers all hops on the route
func (m *NTR) discover(ch chan struct{}) {
	// Sequences are incrementing as we don't won't to get old replys which might be from a previous run (where we timed out and continued).
	// We can't use the process id as unique identifier as there might be multiple runs within a single binary, thus we use a fixed pseudo random number.
	rand.Seed(time.Now().UnixNano())
	seq := rand.Intn(math.MaxUint16)
	id := rand.Intn(math.MaxUint16) & 0xffff

	ipAddr := net.IPAddr{IP: net.ParseIP(m.Address)}

	for {
		time.Sleep(m.interval)
		var wg sync.WaitGroup
		for ttl := 1; ttl <= m.maxHops; ttl++ {
			wg.Add(1)
			go func(ttlVal int, seqVal int) {
				defer wg.Done()
				var hopReturn icmp.ICMPReturn
				if ipAddr.IP.To4() != nil {
					hopReturn, _ = icmp.SendDiscoverICMP(m.SrcAddress, &ipAddr, ttlVal, id, m.timeout, seqVal)
				} else {
					hopReturn, _ = icmp.SendDiscoverICMPv6(m.SrcAddress, &ipAddr, ttlVal, id, m.timeout, seqVal)
				}
				m.mutex.Lock()
				s := m.registerStatistic(ttlVal, hopReturn)
				s.Dest = &ipAddr
				s.PID = id
				m.mutex.Unlock()
				ch <- struct{}{}
			}(ttl, seq+ttl)
		}
		seq += m.maxHops
		wg.Wait()
	}
}
