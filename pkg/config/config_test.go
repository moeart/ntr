package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigParsesDurationsAndPreservesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`network:
  timeout: 750ms
  interval: 125ms
  hop_sleep: 10ms
  max_hops: 7
display:
  language: en
  ptr_lookup: true
  ring_buffer_size: 32
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Network.Timeout != 750*time.Millisecond || cfg.Network.Interval != 125*time.Millisecond || cfg.Network.HopSleep != 10*time.Millisecond {
		t.Fatalf("duration settings were not parsed: %+v", cfg.Network)
	}
	if cfg.Network.MaxHops != 7 || cfg.Display.Language != "en" || !cfg.Display.PTRLookup || cfg.Display.RingBufferSize != 32 {
		t.Fatalf("custom settings were not applied: %+v / %+v", cfg.Network, cfg.Display)
	}
	if !cfg.ASN.Enable || !cfg.GeoIP.Enable {
		t.Fatal("omitted settings should retain defaults")
	}
}
