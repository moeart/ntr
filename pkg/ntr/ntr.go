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
}

func NewNTR(addr, srcAddr string, timeout time.Duration, interval time.Duration,
	hopsleep time.Duration, maxHops, maxUnknownHops, ringBufferSize int, ptr bool, enableAsn bool, enableGeoIP bool) (*NTR, chan struct{}, error) {
	if net.ParseIP(addr) == nil {
		addrs, err := net.LookupHost(addr)
		if err != nil || len(addrs) == 0 {
			return nil, nil, fmt.Errorf("invalid host or ip provided: %s", err)
		}
		addr = addrs[0]
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
	}

	if enableAsn {
		// 加载 ASN 数据库
		var err error
		ntr.asns, err = asn.NewASNs()
		if err != nil {
			return nil, nil, err
		}
	}

	if enableGeoIP {
		// 加载 GeoIP 数据库
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
			Targets:        []string{}, // 初始化 Targets 字段，防止 nil 指针引用
			Asns:           m.asns,     // 设置 ASNs 字段
			GeoIP:          m.geoip,    // 设置 GeoIP 字段
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
// 定义 Windows API 函数和结构体
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

// 使用 Windows API 直接获取控制台尺寸，更准确和快速
func GetTerminalSize() (int, int) {
	var csbi consoleScreenBufferInfo
	r1, _, _ := getConsoleScreenBuf.Call(
		uintptr(syscall.Stdout),
		uintptr(unsafe.Pointer(&csbi)),
	)
	if r1 != 0 {
		width := int(csbi.SrWindow.Right - csbi.SrWindow.Left + 1)
		height := int(csbi.SrWindow.Bottom - csbi.SrWindow.Top + 1)
		width -= 2 // 横向减少2个字符，避免界面超出边界

		// 确保宽度至少是 78，高度至少是 25
		if width < 78 {
			width = 78
		}
		if height < 25 {
			height = 25
		}

		return width, height
	}

	// 如果 API 调用失败，使用 mode con 命令作为备用方法
	cmd := exec.Command("mode", "con")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err == nil {
		// 解析输出
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

		// 确保宽度至少是 78，高度至少是 25
		if width < 78 {
			width = 78
		}
		if height < 25 {
			height = 25
		}

		return width, height
	}

	// 如果都失败，使用 goterm 的默认方法
	width := gm.Width() - 2
	height := gm.Height()

	// 确保宽度至少是 78，高度至少是 25
	if width < 78 {
		width = 78
	}
	if height < 25 {
		height = 25
	}

	return width, height
}

// 检测窗口大小变化的函数
func monitorWindowResize(resizeChan chan bool) {
	prevWidth, _ := GetTerminalSize()
	for {
		time.Sleep(200 * time.Millisecond) // 每 200ms 检查一次
		currWidth, _ := GetTerminalSize()
		if currWidth != prevWidth {
			resizeChan <- true
			prevWidth = currWidth
		}
	}
}

func (m *NTR) Render(offset int) {

	// 获取终端尺寸
	width, _ := GetTerminalSize()
	maxLength := width - 3 // 防止溢出

	// 打印工具信息
	toolName := "NTR - MoeArt's Network Traceroute - Special Edition for Laba Festival"
	toolCopyright := "(c)2020 - 2025 MoeArt OpenSource, www.acgdraw.com"

	// 居中打印工具名称和版权信息
	padding := (maxLength - len(toolName)) / 2
	if padding > 0 {
		gm.Printf("%*s%s%*s\n", padding, "", toolName, padding, "")
	} else {
		gm.Printf("%s\n", toolName[:maxLength])
	}

	padding = (maxLength - len(toolCopyright)) / 2
	if padding > 0 {
		gm.Printf("%*s%s%*s\n", padding, "", toolCopyright, padding, "")
	} else {
		gm.Printf("%s\n", toolCopyright[:maxLength])
	}

	// 打印信息行
	infoLeft := fmt.Sprintf("Dest: %s", m.Address)
	startTime := time.Now().Format("2006-01-02 15:04:05")
	infoRight := fmt.Sprintf("ST: %s", startTime)
	paddingRight := maxLength - len(infoLeft) - len(infoRight)
	if paddingRight > 0 {
		gm.Printf("%s%s%s\n", infoLeft, strings.Repeat(" ", paddingRight), infoRight)
	} else {
		gm.Printf("%s\n", infoLeft[:maxLength])
	}

	// 确定列格式，根据 IPv4/IPv6 调整 DESTINATION 列宽度
	isIPv6 := net.ParseIP(m.Address).To4() == nil
	var destWidth int
	if isIPv6 {
		destWidth = 40
	} else {
		destWidth = 17
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
	format := fmt.Sprintf("%%3s  %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds  %%-%ds %%-%ds",
		destWidth, lossWidth, sentWidth, lastWidth, bestWidth, avgWidth, wrstWidth, asnWidth, locationWidth)

	// 打印标题栏
	title := fmt.Sprintf(format,
		"#",
		"DESTINATION",
		"LOSS%",
		"SENT",
		"LAST",
		"BEST",
		"AVG",
		"WRST",
		"ASN",
		"LOCATION",
	)

	// 标题栏反差高亮效果
	gm.Println(gm.Background(gm.Color(title, gm.BLACK), gm.WHITE))

	// 打印跳数信息
	foundTarget := false
	for i := 1; i <= len(m.Statistic); i++ {
		m.mutex.RLock()
		hopStat := m.Statistic[i]
		if hopStat != nil && !foundTarget {
			// 检查当前跳点是否包含目标地址
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
	// 忽略 count 参数，让程序持续运行直到用户按下 Ctrl+C
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
		for ttl := 1; ttl < m.maxHops; ttl++ {
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
