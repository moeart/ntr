package ntr

import (
	"container/ring"
	"context"
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
	activeHops     int
	maxUnknownHops int
	ptrLookup      bool
	enableAsn      bool
	asns           *asn.ASNs
	enableGeoIP    bool
	geoip          *geoip.GeoIP
	lang           string
	useQQWry       bool
	startTime      string
	rounds         int
	totalSent      int
	totalReceived  int
	lastActivity   time.Time
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewNTR(addr, srcAddr string, timeout time.Duration, interval time.Duration,
	hopsleep time.Duration, maxHops, maxUnknownHops, ringBufferSize int, ptr bool, enableAsn bool, enableGeoIP bool, lang string, useQQWry bool, forceIPv4 bool, forceIPv6 bool) (*NTR, chan struct{}, error) {
	parsedIP := net.ParseIP(addr)
	if parsedIP != nil {
		if forceIPv4 && parsedIP.To4() == nil {
			return nil, nil, fmt.Errorf("target %q is not an IPv4 address", addr)
		}
		if forceIPv6 && parsedIP.To4() != nil {
			return nil, nil, fmt.Errorf("target %q is not an IPv6 address", addr)
		}
	}
	if timeout <= 0 {
		return nil, nil, fmt.Errorf("timeout must be greater than zero")
	}
	if interval < 0 || hopsleep < 0 {
		return nil, nil, fmt.Errorf("interval and hop sleep cannot be negative")
	}
	if maxHops < 1 || maxHops > 255 {
		return nil, nil, fmt.Errorf("max hops must be between 1 and 255")
	}
	if maxUnknownHops < 0 {
		return nil, nil, fmt.Errorf("max unknown hops cannot be negative")
	}
	if ringBufferSize < 1 {
		return nil, nil, fmt.Errorf("ring buffer size must be greater than zero")
	}

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

	ctx, cancel := context.WithCancel(context.Background())
	ntr := &NTR{
		SrcAddress:     srcAddr,
		interval:       interval,
		timeout:        timeout,
		hopsleep:       hopsleep,
		Address:        addr,
		mutex:          &sync.RWMutex{},
		Statistic:      map[int]*hop.HopStatistic{},
		maxHops:        maxHops,
		activeHops:     maxHops,
		ringBufferSize: ringBufferSize,
		maxUnknownHops: maxUnknownHops,
		ptrLookup:      ptr,
		enableAsn:      enableAsn,
		enableGeoIP:    enableGeoIP,
		lang:           lang,
		useQQWry:       useQQWry,
		startTime:      time.Now().Format("2006-01-02 15:04:05"),
		ctx:            ctx,
		cancel:         cancel,
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

	m.totalSent++
	if r.Success {
		m.totalReceived++
	}
	m.lastActivity = time.Now()
	s.Record(r)
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
	// Keep the statistics map and each HopStatistic stable while the renderer
	// traverses it. Probe goroutines update the same objects under this lock.
	m.mutex.RLock()
	defer m.mutex.RUnlock()

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
	// Ignore count parameter, let the program run continuously until stopped.
	m.discover(ch)
}

// Stop requests a graceful stop of the discovery loop.
func (m *NTR) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

type Snapshot struct {
	SrcAddress    string
	Address       string
	StartTime     string
	Lang          string
	Timeout       time.Duration
	Interval      time.Duration
	MaxHops       int
	ActiveHops    int
	Rounds        int
	TotalSent     int
	TotalReceived int
	LastActivity  time.Time
	Running       bool
	Hops          []hop.Snapshot
}

// Snapshot returns a consistent, read-only view for a UI or exporter.
func (m *NTR) Snapshot() Snapshot {
	view := Snapshot{Hops: make([]hop.Snapshot, 0)}
	statistics := make([]*hop.HopStatistic, 0)

	// Capture the map membership and immutable run settings quickly. Each hop
	// owns a small lock for its mutable counters; expensive lookups happen after
	// releasing the NTR map lock and reuse the original hop caches.
	m.mutex.RLock()
	view.SrcAddress = m.SrcAddress
	view.Address = m.Address
	view.StartTime = m.startTime
	view.Lang = m.lang
	view.Timeout = m.timeout
	view.Interval = m.interval
	view.MaxHops = m.maxHops
	view.ActiveHops = m.activeHops
	view.Rounds = m.rounds
	view.TotalSent = m.totalSent
	view.TotalReceived = m.totalReceived
	view.LastActivity = m.lastActivity
	view.Running = m.ctx.Err() == nil
	ptrLookup := m.ptrLookup
	limit := m.activeHops
	if limit < 1 || limit > m.maxHops {
		limit = m.maxHops
	}
	for ttl := 1; ttl <= limit; ttl++ {
		if statistic := m.Statistic[ttl]; statistic != nil {
			statistics = append(statistics, statistic)
		}
	}
	m.mutex.RUnlock()

	for _, statistic := range statistics {
		view.Hops = append(view.Hops, statistic.Snapshot(ptrLookup))
	}
	return view
}

// discover discovers all hops on the route
func (m *NTR) discover(ch chan struct{}) {
	// Sequence numbers distinguish replies from probes that timed out in a
	// previous round. Keep the pseudo-random source local to this run instead
	// of mutating math/rand's process-global source.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	seq := rng.Intn(math.MaxUint16)
	id := rng.Intn(math.MaxUint16) & 0xffff

	ipAddr := net.IPAddr{IP: net.ParseIP(m.Address)}

	// A zero hop_sleep used to launch every TTL at once. That creates a burst
	// of raw-socket opens and ICMP packets, which is both unfriendly to the
	// network and the main cause of the observed abnormal TTL request speed.
	// In that case spread one round over interval instead of disabling pacing.
	hopDelay := m.hopsleep
	if hopDelay <= 0 {
		if m.interval > 0 && m.maxHops > 1 {
			hopDelay = m.interval / time.Duration(m.maxHops)
		}
		if hopDelay <= 0 {
			hopDelay = time.Millisecond
		}
	}

	probeLimit := m.maxHops
	unknownRounds := 0
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		roundStart := time.Now()
		roundTargetTTL := 0
		var wg sync.WaitGroup
		for ttl := 1; ttl <= probeLimit; ttl++ {
			if ttl > 1 {
				timer := time.NewTimer(hopDelay)
				select {
				case <-timer.C:
				case <-m.ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					wg.Wait()
					return
				}
			}
			select {
			case <-m.ctx.Done():
				wg.Wait()
				return
			default:
			}

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
				if hopReturn.Success && hopReturn.Addr == m.Address && (roundTargetTTL == 0 || ttlVal < roundTargetTTL) {
					roundTargetTTL = ttlVal
				}
				m.mutex.Unlock()

				// A notification only means that the view is stale. Coalesce bursts
				// instead of blocking probe goroutines behind a one-slot channel.
				select {
				case ch <- struct{}{}:
				default:
				}
			}(ttl, (seq+ttl)&math.MaxUint16)
		}
		seq = (seq + probeLimit) & math.MaxUint16
		wg.Wait()

		// Once the destination replies, probing higher TTLs only creates
		// unnecessary traffic. If the route later changes and the destination
		// disappears, expand the limit gradually instead of jumping back to a
		// full burst immediately.
		if roundTargetTTL > 0 {
			probeLimit = roundTargetTTL
			unknownRounds = 0
		} else if probeLimit < m.maxHops {
			unknownRounds++
			threshold := m.maxUnknownHops
			if threshold < 1 {
				threshold = 1
			}
			if unknownRounds >= threshold {
				probeLimit++
				unknownRounds = 0
			}
		}
		m.mutex.Lock()
		m.activeHops = probeLimit
		m.rounds++
		m.mutex.Unlock()

		// interval is the minimum period between round starts. A slow round
		// (for example, many timeouts) is never overlapped by the next one.
		if remaining := m.interval - time.Since(roundStart); remaining > 0 {
			timer := time.NewTimer(remaining)
			select {
			case <-timer.C:
			case <-m.ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			}
		}
	}
}
