package hop

import (
	"container/ring"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"time"

	gm "github.com/buger/goterm"
	"github.com/tonobo/mtr/pkg/icmp"
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

func (h *HopStatistic) Render(ptrLookup bool, width int, destWidth int) {
	maxLength := width - 1

	// 确定列格式，根据 IPv4/IPv6 调整 DESTINATION 列宽度
	isIPv6 := h.Dest.IP.To4() == nil
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
	if h.Targets == nil || len(h.Targets) == 0 || h.Targets[0] == "" {
		dest = "Request timed out"
	} else {
		dest = h.lookupAddr(ptrLookup, 0)
	}

	// 截断目标地址以适应列宽
	if len(dest) > destWidth {
		dest = dest[:destWidth]
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
		"- -", // ASN 列暂时留空
		"",    // LOCATION 列暂时留空
	)

	// 确保行内容不超过终端宽度
	if len(line) > maxLength {
		line = line[:maxLength]
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
