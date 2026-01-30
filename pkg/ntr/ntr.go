package ntr

import (
	"container/ring"
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/moeart/ntr/pkg/asn"
	"github.com/moeart/ntr/pkg/geoip"
	"github.com/moeart/ntr/pkg/hop"
	"github.com/moeart/ntr/pkg/icmp"
	"github.com/moeart/ntr/pkg/render"
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
	startTime      string
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
		startTime:      time.Now().Format("2006-01-02 15:04:05"),
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

func (m *NTR) Render() {
	// Create render config
	renderConfig := &render.NTRRenderConfig{
		SrcAddress:     m.SrcAddress,
		Address:        m.Address,
		StartTime:      m.startTime,
		Statistic:      m.Statistic,
		Timeout:        m.timeout,
		Interval:       m.interval,
		MaxHops:        m.maxHops,
		MaxUnknownHops: m.maxUnknownHops,
		RingBufferSize: m.ringBufferSize,
		PtrLookup:      m.ptrLookup,
		EnableAsn:      m.enableAsn,
		Asns:           m.asns,
		EnableGeoIP:    m.enableGeoIP,
		GeoIP:          m.geoip,
		Lang:           m.lang,
		UseQQWry:       m.useQQWry,
	}

	// Render using the render package
	render.RenderNTR(renderConfig)
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
