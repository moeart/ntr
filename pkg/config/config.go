package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the overall configuration

type Config struct {
	ASN     ASNConfig     `yaml:"asn"`
	GeoIP   GeoIPConfig   `yaml:"geoip"`
	Network NetworkConfig `yaml:"network"`
	Display DisplayConfig `yaml:"display"`
}

// ASNConfig represents the ASN configuration
type ASNConfig struct {
	DownloadURL string `yaml:"download_url"`
	Enable      bool   `yaml:"enable"`
}

// GeoIPConfig represents the GeoIP configuration
type GeoIPConfig struct {
	DownloadURL string `yaml:"download_url"`
	Enable      bool   `yaml:"enable"`
	UseQQWry    bool   `yaml:"use_qqwry"`
}

// NetworkConfig represents the network configuration
type NetworkConfig struct {
	IPPreference   string        `yaml:"ip_preference"`
	MaxHops        int           `yaml:"max_hops"`
	MaxUnknownHops int           `yaml:"max_unknown_hops"`
	Timeout        time.Duration `yaml:"timeout"`
	Interval       time.Duration `yaml:"interval"`
	HopSleep       time.Duration `yaml:"hop_sleep"`
}

// DisplayConfig represents the display configuration
type DisplayConfig struct {
	Language       string `yaml:"language"`
	PTRLookup      bool   `yaml:"ptr_lookup"`
	RingBufferSize int    `yaml:"ring_buffer_size"`
}

// GetDefaultConfig returns the default configuration
func GetDefaultConfig() *Config {
	return &Config{
		ASN: ASNConfig{
			DownloadURL: "https://iptoasn.com/data/ip2asn-combined.tsv.gz",
			Enable:      true,
		},
		GeoIP: GeoIPConfig{
			DownloadURL: "https://v6.gh-proxy.org/https://github.com/metowolf/qqwry.dat/releases/latest/download/qqwry.dat",
			Enable:      true,
			UseQQWry:    true,
		},
		Network: NetworkConfig{
			IPPreference:   "auto",
			MaxHops:        25,
			MaxUnknownHops: 10,
			Timeout:        1000 * time.Millisecond,
			Interval:       200 * time.Millisecond,
			HopSleep:       0 * time.Millisecond,
		},
		Display: DisplayConfig{
			Language:       "zh",
			PTRLookup:      false,
			RingBufferSize: 128,
		},
	}
}

// LoadConfig loads the configuration from a YAML file
func LoadConfig(filePath string) (*Config, error) {
	// Start with default configuration
	config := GetDefaultConfig()

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// If config file doesn't exist, return default config
		return config, nil
	}

	// Read the config file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the YAML content
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// LoadConfigFromDefaultPath loads the configuration from the default path
func LoadConfigFromDefaultPath() (*Config, error) {
	// Check current directory first
	currentDir, _ := os.Getwd()
	paths := []string{
		filepath.Join(currentDir, "config.yaml"),
		filepath.Join(currentDir, "conf", "config.yaml"),
		filepath.Join("/etc", "ntr", "config.yaml"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return LoadConfig(path)
		}
	}

	// If no config file found, return default config
	return GetDefaultConfig(), nil
}

// GetASNDownloadURL returns the ASN download URL from configuration
func (c *Config) GetASNDownloadURL() string {
	return c.ASN.DownloadURL
}

// GetGeoIPDownloadURL returns the GeoIP download URL from configuration
func (c *Config) GetGeoIPDownloadURL() string {
	return c.GeoIP.DownloadURL
}

// IsASNEnabled returns whether ASN lookup is enabled
func (c *Config) IsASNEnabled() bool {
	return c.ASN.Enable
}

// IsGeoIPEnabled returns whether GeoIP lookup is enabled
func (c *Config) IsGeoIPEnabled() bool {
	return c.GeoIP.Enable
}

// UseQQWry returns whether to use QQWry database for GeoIP lookup
func (c *Config) UseQQWry() bool {
	return c.GeoIP.UseQQWry
}

// GetIPPreference returns the IP protocol preference
func (c *Config) GetIPPreference() string {
	return c.Network.IPPreference
}

// GetLanguage returns the display language
func (c *Config) GetLanguage() string {
	return c.Display.Language
}
