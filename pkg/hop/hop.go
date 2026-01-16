package hop

import (
	"container/ring"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	gm "github.com/buger/goterm"
	"github.com/moeart/ntr/pkg/asn"
	"github.com/moeart/ntr/pkg/geoip"
	"github.com/moeart/ntr/pkg/icmp"
)

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
	dnsCache       map[string]string
	Asns           *asn.ASNs
	GeoIP          *geoip.GeoIP
	Lang           string
	UseQQWry       bool
}

type packet struct {
	Success      bool    `json:"success"`
	ResponseTime float64 `json:"respond_ms"`
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

// getStringDisplayWidth 计算字符串的显示宽度，中文字符计为2个宽度，英文字符计为1个宽度
func getStringDisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			// 中文字符占用2个宽度
			width += 2
		} else {
			// 其他字符占用1个宽度
			width += 1
		}
	}
	return width
}

// truncateString 安全地截断字符串，避免截断多字节字符，基于显示宽度
func truncateString(s string, maxWidth int) string {
	if getStringDisplayWidth(s) <= maxWidth {
		return s
	}

	// 确保不会截断UTF-8字符，基于显示宽度进行截断
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

func (h *HopStatistic) Render(ptrLookup bool, width int, destWidth int) {
	if h == nil {
		gm.Println("nil HopStatistic")
		return
	}
	maxLength := width - 1

	// 确定列格式，根据 IPv4/IPv6 调整 DESTINATION 列宽度
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

	// 计算其他列宽度
	lossWidth := 5
	sentWidth := 5
	lastWidth := 5
	bestWidth := 5
	avgWidth := 5
	wrstWidth := 5
	asnWidth := 7
	locationWidth := maxLength - 3 - 2 - destWidth - 2 - lossWidth - sentWidth - lastWidth - bestWidth - avgWidth - wrstWidth - 2 - asnWidth - 1

	// 构建格式化字符串
	format := fmt.Sprintf("%%3d  %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds  %%-%ds %%-%ds",
		destWidth, lossWidth, sentWidth, lastWidth, bestWidth, avgWidth, wrstWidth, asnWidth, locationWidth)

	// 获取目标地址
	var dest string
	if h.Targets == nil || len(h.Targets) == 0 || (len(h.Targets) > 0 && h.Targets[0] == "") {
		dest = "Request timed out"
	} else {
		dest = h.lookupAddr(ptrLookup, 0)
	}

	// 安全截断目标地址以适应列宽，避免截断多字节字符
	if getStringDisplayWidth(dest) > destWidth {
		dest = truncateString(dest, destWidth)
	}

	// 获取延迟值
	var last, best, avg, wrst string
	if !h.Last.Success {
		last = "*"
	} else {
		last = fmt.Sprintf("%.0f", h.Last.Elapsed.Seconds()*1000)
	}

	if !h.Best.Success {
		best = "*"
	} else {
		best = fmt.Sprintf("%.0f", h.Best.Elapsed.Seconds()*1000)
	}

	if h.Sent-h.Lost == 0 {
		avg = "*"
	} else {
		avg = fmt.Sprintf("%.0f", h.Avg())
	}

	if !h.Worst.Success {
		wrst = "*"
	} else {
		wrst = fmt.Sprintf("%.0f", h.Worst.Elapsed.Seconds()*1000)
	}

	// 获取 ASN 信息
	var asnStr string
	if h.Asns != nil && h.Targets != nil && len(h.Targets) > 0 && h.Targets[0] != "" {
		if a, err := h.Asns.LookupByIP(h.Targets[0]); err == nil && a != nil && a.Number != "AS0" {
			asnStr = a.Number
		} else {
			asnStr = "- -"
		}
	} else {
		asnStr = "- -"
	}

	// 获取 LOCATION 信息
	var locationStr string
	if h.Targets != nil && len(h.Targets) > 0 && h.Targets[0] != "" {
		if h.GeoIP != nil {
			if loc, err := h.GeoIP.LookupByIP(h.Targets[0], h.Lang, h.UseQQWry, h.Asns); err == nil && loc != nil {
				locationStr = loc.Format(h.Lang)
			} else {
				locationStr = "- -"
			}
		} else if h.Asns != nil {
			// 如果没有 GeoIP，但有 ASN 数据，也尝试获取位置信息
			if a, err := h.Asns.LookupByIP(h.Targets[0]); err == nil && a != nil && a.Country != "" {
				parts := []string{a.Country}
				if a.Description != "" {
					parts = append(parts, a.Description)
				}
				locationStr = strings.Join(parts, " ")
			} else {
				locationStr = "- -"
			}
		} else {
			locationStr = "- -"
		}
	} else {
		locationStr = "- -"
	}

	// 安全截断位置信息以适应列宽，避免截断多字节字符
	if getStringDisplayWidth(locationStr) > locationWidth {
		locationStr = truncateString(locationStr, locationWidth)
	}

	// 构建行内容
	line := fmt.Sprintf(format,
		h.TTL,
		dest,
		fmt.Sprintf("%.0f", h.Loss()),
		fmt.Sprintf("%d", h.Sent),
		last,
		best,
		avg,
		wrst,
		asnStr,      // ASN 列
		locationStr, // LOCATION 列
	)

	// 确保行内容不超过终端宽度，使用安全截断
	if getStringDisplayWidth(line) > maxLength {
		line = truncateString(line, maxLength)
	}

	gm.Printf("%s\n", line)
}

func (h *HopStatistic) lookupAddr(ptrLookup bool, index int) string {
	addr := "???"
	if h.Targets[index] != "" {
		addr = h.Targets[index]
		if ptrLookup {
			if h.dnsCache == nil {
				h.dnsCache = map[string]string{}
			}
			if key, ok := h.dnsCache[h.Targets[index]]; ok {
				addr = key
			} else {
				names, err := net.LookupAddr(h.Targets[index])
				if err == nil && len(names) > 0 {
					addr = names[0]
				}
			}
			h.dnsCache[h.Targets[index]] = addr
		}
	}
	return addr
}
