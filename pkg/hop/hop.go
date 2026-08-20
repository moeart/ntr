package hop

import (
	"container/ring"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/moeart/ntr/pkg/asn"
	"github.com/moeart/ntr/pkg/geoip"
	"github.com/moeart/ntr/pkg/icmp"
)

// HopStatistic - Contains statistics and information about a traceroute hop

type HopStatistic struct {
	Dest           *net.IPAddr
	Timeout        time.Duration
	PID            int
	Sent           int
	TTL            int
	Targets        []string
	Last           icmp.ICMPReturn
	Best           icmp.ICMPReturn
	Worst          icmp.ICMPReturn
	SumElapsed     time.Duration
	Lost           int
	Packets        *ring.Ring
	RingBufferSize int
	mu             sync.RWMutex
	dnsCache       map[string]string
	cacheMu        sync.Mutex
	lookupCache    map[string]lookupMetadata
	Asns           *asn.ASNs
	GeoIP          *geoip.GeoIP
	Lang           string
	UseQQWry       bool
}

// packet - Represents an ICMP packet result for JSON serialization

type packet struct {
	Success      bool    `json:"success"`
	ResponseTime float64 `json:"respond_ms"`
}

type lookupMetadata struct {
	asn      string
	location string
}

// Snapshot is an immutable view of one hop for terminal UIs and exporters.
// Record adds one probe result while holding the hop-local lock.
func (h *HopStatistic) Record(r icmp.ICMPReturn) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	h.Last = r
	h.Sent++
	h.Targets = appendTarget(h.Targets, r.Addr)
	if h.Packets != nil {
		h.Packets = h.Packets.Prev()
		h.Packets.Value = r
	}
	if !r.Success {
		h.Lost++
		return
	}
	h.SumElapsed += r.Elapsed
	if !h.Best.Success || h.Best.Elapsed > r.Elapsed {
		h.Best = r
	}
	if h.Worst.Elapsed < r.Elapsed {
		h.Worst = r
	}
}

// SetRouteInfo updates the destination metadata associated with a hop.
func (h *HopStatistic) SetRouteInfo(dest *net.IPAddr, pid int) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.Dest = dest
	h.PID = pid
	h.mu.Unlock()
}

func appendTarget(targets []string, address string) []string {
	if address == "" {
		return targets
	}
	for _, target := range targets {
		if target == address {
			return targets
		}
	}
	if len(targets) > 0 {
		filtered := targets[:0]
		for _, target := range targets {
			if target != "" {
				filtered = append(filtered, target)
			}
		}
		targets = filtered
	}
	return append(targets, address)
}

type Snapshot struct {
	TTL       int
	Address   string
	Target    string
	Sent      int
	Lost      int
	Loss      float64
	LastMS    float64
	BestMS    float64
	AverageMS float64
	WorstMS   float64
	ASN       string
	Location  string
}

// Snapshot returns display-ready values without exposing mutable ring-buffer state.
func (h *HopStatistic) Snapshot(ptrLookup bool) Snapshot {
	if h == nil {
		return Snapshot{}
	}

	h.mu.RLock()
	address := ""
	if len(h.Targets) > 0 {
		address = h.Targets[0]
	}
	view := Snapshot{
		TTL:       h.TTL,
		Address:   address,
		Sent:      h.Sent,
		Lost:      h.Lost,
		Loss:      h.Loss(),
		LastMS:    h.Last.Elapsed.Seconds() * 1000,
		BestMS:    h.Best.Elapsed.Seconds() * 1000,
		AverageMS: h.Avg(),
		WorstMS:   h.Worst.Elapsed.Seconds() * 1000,
	}
	h.mu.RUnlock()

	target, asnStr, locationStr := h.ResolveTarget(address, ptrLookup)
	view.Target = target
	view.ASN = asnStr
	view.Location = locationStr
	return view
}

// ResolveTarget uses the per-hop caches without reading mutable statistics.
// Callers can pass an address captured under their own statistics lock.
func (h *HopStatistic) ResolveTarget(address string, ptrLookup bool) (string, string, string) {
	target := address
	if address == "" {
		if h.Lang == "zh" {
			target = "请求超时"
		} else {
			target = "Request timed out"
		}
	} else if ptrLookup {
		target = h.lookupAddrForTarget(address)
	}
	asnStr, locationStr := h.lookupMetadataForIP(address)
	return target, asnStr, locationStr
}

// SnapshotWithMetadata builds a view from already captured values and avoids
// performing any external lookup.
func (h *HopStatistic) SnapshotWithMetadata(address, target, asnStr, locationStr string) Snapshot {
	if h == nil {
		return Snapshot{}
	}
	h.mu.RLock()
	view := Snapshot{
		TTL:       h.TTL,
		Address:   address,
		Target:    target,
		Sent:      h.Sent,
		Lost:      h.Lost,
		Loss:      h.Loss(),
		LastMS:    h.Last.Elapsed.Seconds() * 1000,
		BestMS:    h.Best.Elapsed.Seconds() * 1000,
		AverageMS: h.Avg(),
		WorstMS:   h.Worst.Elapsed.Seconds() * 1000,
		ASN:       asnStr,
		Location:  locationStr,
	}
	h.mu.RUnlock()
	return view
}

func (h *HopStatistic) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Sent             int       `json:"sent"`
		Target           string    `json:"target"`
		Last             float64   `json:"last_ms"`
		Best             float64   `json:"best_ms"`
		Worst            float64   `json:"worst_ms"`
		Loss             float64   `json:"loss_percent"`
		Avg              float64   `json:"avg_ms"`
		Stdev            float64   `json:"stdev_ms"`
		PacketBufferSize int       `json:"packet_buffer_size"`
		TTL              int       `json:"ttl"`
		Packets          []*packet `json:"packet_list_ms"`
	}{
		Sent:             h.Sent,
		TTL:              h.TTL,
		Loss:             h.Loss(),
		Target:           fmt.Sprintf("%v", h.Targets),
		PacketBufferSize: h.RingBufferSize,
		Last:             h.Last.Elapsed.Seconds() * 1000,
		Best:             h.Best.Elapsed.Seconds() * 1000,
		Worst:            h.Worst.Elapsed.Seconds() * 1000,
		Avg:              h.Avg(),
		Stdev:            h.Stdev(),
		Packets:          h.packets(),
	})
}

func (h *HopStatistic) Avg() float64 {
	avg := 0.0
	if !(h.Sent-h.Lost == 0) {
		avg = h.SumElapsed.Seconds() * 1000 / float64(h.Sent-h.Lost)
	}
	return avg
}

func (h *HopStatistic) Stdev() float64 {
	avg := h.Avg()
	result := 0.0
	n := 0

	for _, p := range h.packets() {
		if p == nil || !p.Success {
			continue
		}

		n++
		distance := math.Abs(p.ResponseTime - avg)
		dsquare := distance * distance
		result += dsquare
	}

	result = result / float64(n)
	result = math.Sqrt(result)

	if math.IsNaN(result) {
		result = 0
	}

	return result
}

func (h *HopStatistic) Loss() float64 {
	return float64(h.Lost) / float64(h.Sent) * 100.0
}

func (h *HopStatistic) packets() []*packet {
	v := make([]*packet, h.RingBufferSize)
	i := 0
	h.Packets.Do(func(f interface{}) {
		if f == nil {
			v[i] = nil
			i++
			return
		}
		x := f.(icmp.ICMPReturn)
		if x.Success {
			v[i] = &packet{
				Success:      true,
				ResponseTime: x.Elapsed.Seconds() * 1000,
			}
		} else {
			v[i] = &packet{
				Success:      false,
				ResponseTime: 0.0,
			}
		}
		i++
	})
	return v
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

// truncateString - Safely truncate string without breaking multi-byte characters, based on display width
func truncateString(s string, maxWidth int) string {
	if getStringDisplayWidth(s) <= maxWidth {
		return s
	}

	// Ensure UTF-8 characters are not broken, truncate based on display width
	var truncated string
	currentWidth := 0
	for _, r := range s {
		runeWidth := 1
		if r >= 0x4e00 && r <= 0x9fff {
			runeWidth = 2
		}

		if currentWidth+runeWidth > maxWidth {
			break
		}

		truncated += string(r)
		currentWidth += runeWidth
	}

	return truncated
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

func (h *HopStatistic) Render(ptrLookup bool, width int, destWidth int, ttl int) string {
	if h == nil {
		return ""
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	maxLength := width - 1

	// Determine column format, adjust DESTINATION column width based on IPv4/IPv6
	var isIPv6 bool
	if h.Dest != nil && h.Dest.IP != nil {
		isIPv6 = h.Dest.IP.To4() == nil
	} else {
		isIPv6 = false
	}

	if destWidth == 0 {
		if isIPv6 {
			destWidth = 40
		} else {
			destWidth = 17
		}
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

	// Build format string - removed unused variable

	// Get destination address
	var dest string
	if h.Targets == nil || len(h.Targets) == 0 || (len(h.Targets) > 0 && h.Targets[0] == "") {
		if h.Lang == "zh" {
			dest = "请求超时"
		} else {
			dest = "Request timed out"
		}
	} else {
		dest = h.lookupAddr(ptrLookup, 0)
	}

	// Safely truncate destination address to fit column width, avoid breaking multi-byte characters
	if getStringDisplayWidth(dest) > destWidth {
		dest = truncateString(dest, destWidth)
	}
	dest = padString(dest, destWidth)

	// Get delay values with right alignment
	var last, best, avg, wrst, loss, sent string
	if !h.Last.Success {
		last = rightPadString("*", lastWidth)
	} else {
		last = rightPadString(fmt.Sprintf("%.0f", h.Last.Elapsed.Seconds()*1000), lastWidth)
	}

	if !h.Best.Success {
		best = rightPadString("*", bestWidth)
	} else {
		best = rightPadString(fmt.Sprintf("%.0f", h.Best.Elapsed.Seconds()*1000), bestWidth)
	}

	if h.Sent-h.Lost == 0 {
		avg = rightPadString("*", avgWidth)
	} else {
		avg = rightPadString(fmt.Sprintf("%.0f", h.Avg()), avgWidth)
	}

	if !h.Worst.Success {
		wrst = rightPadString("*", wrstWidth)
	} else {
		wrst = rightPadString(fmt.Sprintf("%.0f", h.Worst.Elapsed.Seconds()*1000), wrstWidth)
	}

	// Format loss percentage and sent count with right alignment
	loss = rightPadString(fmt.Sprintf("%.0f", h.Loss()), lossWidth)
	sent = rightPadString(fmt.Sprintf("%d", h.Sent), sentWidth)

	// ASN and location lookups are cached per hop/IP. Rendering is frequent,
	// while the underlying database/DNS lookups are not free.
	asnStr, locationStr := h.lookupMetadataForTarget()
	asnStr = padString(asnStr, asnWidth)

	// Safely truncate location information to fit column width, avoid breaking multi-byte characters
	if getStringDisplayWidth(locationStr) > locationWidth {
		locationStr = truncateString(locationStr, locationWidth)
	}
	locationStr = padString(locationStr, locationWidth)

	// Build row content
	line := fmt.Sprintf("%3d  %s %s %s %s %s %s %s  %s %s",
		ttl,
		dest,
		loss,
		sent,
		last,
		best,
		avg,
		wrst,
		asnStr,      // ASN column
		locationStr, // LOCATION column
	)

	// Ensure line content does not exceed terminal width, use safe truncation
	if getStringDisplayWidth(line) > maxLength {
		line = truncateString(line, maxLength)
	}
	return line
}

func (h *HopStatistic) lookupAddr(ptrLookup bool, index int) string {
	if index < 0 || index >= len(h.Targets) || h.Targets[index] == "" {
		return "???"
	}
	if !ptrLookup {
		return h.Targets[index]
	}
	return h.lookupAddrForTarget(h.Targets[index])
}

func (h *HopStatistic) lookupAddrForTarget(addr string) string {
	if addr == "" {
		return "???"
	}
	h.cacheMu.Lock()
	if h.dnsCache != nil {
		if cached, ok := h.dnsCache[addr]; ok {
			h.cacheMu.Unlock()
			return cached
		}
	}
	h.cacheMu.Unlock()

	resolved := addr
	if names, err := net.LookupAddr(addr); err == nil && len(names) > 0 {
		resolved = names[0]
	}
	h.cacheMu.Lock()
	if h.dnsCache == nil {
		h.dnsCache = make(map[string]string)
	}
	h.dnsCache[addr] = resolved
	h.cacheMu.Unlock()
	return resolved
}

func (h *HopStatistic) lookupMetadataForTarget() (string, string) {
	if len(h.Targets) == 0 {
		return "- -", "- -"
	}
	return h.lookupMetadataForIP(h.Targets[0])
}

func (h *HopStatistic) lookupMetadataForIP(target string) (string, string) {
	if target == "" {
		return "- -", "- -"
	}
	h.cacheMu.Lock()
	if cached, ok := h.lookupCache[target]; ok {
		h.cacheMu.Unlock()
		return cached.asn, cached.location
	}
	h.cacheMu.Unlock()

	asnStr := "- -"
	locationStr := "- -"
	if h.Asns != nil {
		if a, err := h.Asns.LookupByIP(target); err == nil && a != nil {
			if a.Number != "" && a.Number != "AS0" {
				asnStr = a.Number
			}
			if h.GeoIP == nil && a.Country != "" {
				parts := []string{a.Country}
				if a.Description != "" {
					parts = append(parts, a.Description)
				}
				locationStr = strings.Join(parts, " ")
			}
		}
	}
	if h.GeoIP != nil {
		if loc, err := h.GeoIP.LookupByIP(target, h.Lang, h.UseQQWry, h.Asns); err == nil && loc != nil {
			locationStr = loc.Format(h.Lang)
		}
	}

	h.cacheMu.Lock()
	if h.lookupCache == nil {
		h.lookupCache = make(map[string]lookupMetadata)
	}
	h.lookupCache[target] = lookupMetadata{asn: asnStr, location: locationStr}
	h.cacheMu.Unlock()
	return asnStr, locationStr
}
