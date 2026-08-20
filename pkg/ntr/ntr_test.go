package ntr

import (
	"container/ring"
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/moeart/ntr/pkg/hop"
	"github.com/moeart/ntr/pkg/icmp"
)

func TestNewNTRRejectsInvalidRuntimeParameters(t *testing.T) {
	cases := []struct {
		name string
		call func() error
	}{
		{"timeout", func() error {
			_, _, err := NewNTR("127.0.0.1", "", 0, time.Second, 0, 5, 1, 8, false, false, false, "en", false, false, false)
			return err
		}},
		{"interval", func() error {
			_, _, err := NewNTR("127.0.0.1", "", time.Second, -time.Millisecond, 0, 5, 1, 8, false, false, false, "en", false, false, false)
			return err
		}},
		{"max hops", func() error {
			_, _, err := NewNTR("127.0.0.1", "", time.Second, time.Second, 0, 0, 1, 8, false, false, false, "en", false, false, false)
			return err
		}},
		{"ring buffer", func() error {
			_, _, err := NewNTR("127.0.0.1", "", time.Second, time.Second, 0, 5, 1, 0, false, false, false, "en", false, false, false)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestRegisterStatisticTracksLossAndLatency(t *testing.T) {
	m := &NTR{
		timeout:        time.Second,
		Statistic:      make(map[int]*hop.HopStatistic),
		ringBufferSize: 4,
		mutex:          nil,
	}
	m.registerStatistic(1, icmp.ICMPReturn{Success: true, Addr: "192.0.2.1", Elapsed: 10 * time.Millisecond})
	m.registerStatistic(1, icmp.ICMPReturn{Success: false})
	m.registerStatistic(1, icmp.ICMPReturn{Success: true, Addr: "192.0.2.1", Elapsed: 30 * time.Millisecond})

	s := m.Statistic[1]
	if s == nil || s.Sent != 3 || s.Lost != 1 {
		t.Fatalf("unexpected counters: %#v", s)
	}
	if got := s.Avg(); got != 20 {
		t.Fatalf("average latency = %v, want 20", got)
	}
	if got := s.Loss(); math.Abs(got-100.0/3.0) > 1e-12 {
		t.Fatalf("loss = %v, want %v", got, 100.0/3.0)
	}
	if len(s.Targets) != 1 || s.Targets[0] != "192.0.2.1" {
		t.Fatalf("targets were not deduplicated: %#v", s.Targets)
	}
}

func TestSnapshotUsesActiveHops(t *testing.T) {
	m := &NTR{
		ctx:            context.Background(),
		mutex:          &sync.RWMutex{},
		Statistic:      make(map[int]*hop.HopStatistic),
		maxHops:        25,
		activeHops:     1,
		timeout:        time.Second,
		ringBufferSize: 2,
	}
	m.registerStatistic(1, icmp.ICMPReturn{Success: true, Addr: "127.0.0.1", Elapsed: time.Millisecond})
	m.registerStatistic(2, icmp.ICMPReturn{Success: true, Addr: "127.0.0.1", Elapsed: time.Millisecond})

	snapshot := m.Snapshot()
	if len(snapshot.Hops) != 1 || snapshot.Hops[0].TTL != 1 {
		t.Fatalf("snapshot retained stale hops: %#v", snapshot.Hops)
	}
}

func TestRegisterStatisticInitializesPacketRing(t *testing.T) {
	m := &NTR{timeout: time.Second, Statistic: make(map[int]*hop.HopStatistic), ringBufferSize: 2}
	m.registerStatistic(1, icmp.ICMPReturn{Success: false})
	if m.Statistic[1].Packets == nil || m.Statistic[1].Packets.Len() != 2 {
		t.Fatal("packet ring was not initialized")
	}
	_ = ring.New(1) // keep the test explicit about the ring type used by statistics
}
