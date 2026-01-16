package geoip

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moeart/ntr/pkg/asn"
	"github.com/xiaoqidun/qqwry"
)

const (
	// GeoIPURL - Official source for IP to geolocation data
	GeoIPURL = "https://github.com/metowolf/qqwry.dat/releases/latest/download/qqwry.dat"
	// BinaryDatabaseFile - Compiled binary database filename
	BinaryDatabaseFile = "data/geoip.bin"
)

// GeoIP - Encapsulates qqwry library and provides lookup functionality
type GeoIP struct {
	initialized bool
}

// NewGeoIP - Create new GeoIP instance and load database
func NewGeoIP() (*GeoIP, error) {
	geoip := &GeoIP{}

	filePath := GetBinaryDatabasePath()
	err := qqwry.LoadFile(filePath)
	if err == nil {
		geoip.initialized = true
		return geoip, nil
	}

	geoip.initialized = false
	return geoip, nil
}

// UpdateGeoIPDatabase - Download latest GeoIP database from specified URL
func UpdateGeoIPDatabase(downloadURL string) error {
	// If no URL provided, use default
	if downloadURL == "" {
		downloadURL = GeoIPURL
	}

	fmt.Println("Starting GeoIP database update")

	// Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Download database file
	fmt.Printf("Downloading GeoIP database from %s ...\n", downloadURL)
	if err := downloadFileWithProgress(GetBinaryDatabasePath(), downloadURL); err != nil {
		return fmt.Errorf("failed to download GeoIP database: %v", err)
	}

	fmt.Println("GeoIP database updated successfully")
	return nil
}

// isSpecialIP - Check if IP is special and return corresponding Location
func isSpecialIP(ipStr string, lang string) *Location {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}

	b1, b2, b3, b4 := ip4[0], ip4[1], ip4[2], ip4[3]

	// 127.*.*.* - Loopback address
	if b1 == 127 {
		if lang == "en" {
			return &Location{Country: "Loopback Address"}
		}
		return &Location{Country: "本机环回地址"}
	}

	// 10.*.*.* - Local Area Network (A-class)
	if b1 == 10 {
		if lang == "en" {
			return &Location{Country: "Local Area Network (A-class)"}
		}
		return &Location{Country: "本地局域网"}
	}

	// 172.16.*.* to 172.31.*.* - Local Area Network (B-class)
	if b1 == 172 && (b2 >= 16 && b2 <= 31) {
		if lang == "en" {
			return &Location{Country: "Local Area Network (B-class)"}
		}
		return &Location{Country: "本地局域网"}
	}

	// 192.168.*.* - Local Area Network (C-class)
	if b1 == 192 && b2 == 168 {
		// 192.168.1-255.1 - Local Area Network Gateway
		if b4 == 1 && b3 >= 1 && b3 <= 255 {
			if lang == "en" {
				return &Location{Country: "Local Area Network Gateway"}
			}
			return &Location{Country: "本地局域网网关"}
		}
		if lang == "en" {
			return &Location{Country: "Local Area Network (C-class)"}
		}
		return &Location{Country: "本地局域网"}
	}

	// 169.254.*.* - Zero-Configuration Local Area Network
	if b1 == 169 && b2 == 254 {
		if lang == "en" {
			return &Location{Country: "Zero-Configuration Local Area Network"}
		}
		return &Location{Country: "零配置局域网"}
	}

	// 11.*.*.* - IDC Intranet
	if b1 == 11 {
		if lang == "en" {
			return &Location{Country: "IDC Intranet"}
		}
		return &Location{Country: "IDC机房内网"}
	}

	// 134.128-255.*.* - China Telecom DCN Intranet
	if b1 == 134 && (b2 >= 128 && b2 <= 255) {
		if lang == "en" {
			return &Location{Country: "China Telecom DCN Intranet"}
		}
		return &Location{Country: "中国电信 DCN内网"}
	}

	// 220.110-111.*.* - Mengxintong DCN Intranet
	if b1 == 220 && (b2 == 110 || b2 == 111) {
		if lang == "en" {
			return &Location{Country: "Mengxintong DCN Intranet"}
		}
		return &Location{Country: "萌信通 DCN内网"}
	}

	// 100.64-127.*.* - Carrier CGNAT Intranet
	if b1 == 100 && (b2 >= 64 && b2 <= 127) {
		if lang == "en" {
			return &Location{Country: "Carrier CGNAT Intranet"}
		}
		return &Location{Country: "运营商CGNAT内网"}
	}

	return nil
}

// LookupByIP - Lookup IP address information
func (g *GeoIP) LookupByIP(ipStr string, lang string, useQQWry bool, asns *asn.ASNs) (*Location, error) {
	// First check if it's a special IP address
	specialLoc := isSpecialIP(ipStr, lang)
	if specialLoc != nil {
		return specialLoc, nil
	}

	// Determine whether to use QQWry based on conditions
	if useQQWry && g.initialized {
		location, err := qqwry.QueryIP(ipStr)
		if err == nil {
			return &Location{
				Country:  location.Country,
				Province: location.Province,
				City:     location.City,
				District: location.District,
				ISP:      location.ISP,
			}, nil
		}
	}

	// If QQWry database is unavailable, try to get information from ASN data
	if asns != nil {
		if a, err := asns.LookupByIP(ipStr); err == nil && a != nil {
			// Extract country code and description from ASN data
			// ASN's Country field is usually 2-letter country code, Description contains ISP info
			return &Location{
				Country: a.Country,
				ISP:     a.Description,
			}, nil
		}
	}

	return &Location{Country: "N/A"}, nil
}

// Location - Represents geographical location information
type Location struct {
	Country  string
	Province string
	City     string
	District string
	ISP      string
}

// Format - Format location for display
func (l *Location) Format(lang string) string {
	var parts []string

	// Check if it's a special IP address (Country field contains complete description)
	specialCases := []string{"本地局域网", "本地局域网网关", "零配置局域网", "本机环回地址", "IDC机房内网", "中国电信 DCN内网", "萌信通 DCN内网", "运营商CGNAT内网"}
	if lang == "en" {
		specialCases = []string{"Local Area Network", "Loopback Address", "Zero-Configuration Local Area Network", "IDC Intranet", "China Telecom DCN Intranet", "Mengxintong DCN Intranet", "Carrier CGNAT Intranet"}
	}

	for _, special := range specialCases {
		if strings.Contains(l.Country, special) {
			return l.Country
		}
	}

	// If data is from ASN (usually 2-letter country code, Province/City empty)
	if l.Country != "" && l.Province == "" && l.City == "" && l.District == "" {
		if l.Country != "N/A" {
			parts = append(parts, l.Country)
		}
		if l.ISP != "" {
			parts = append(parts, l.ISP)
		}
	} else {
		// For China, don't display country name
		if l.Country == "中国" {
			if l.Province != "" {
				parts = append(parts, l.Province)
			}
			if l.City != "" {
				parts = append(parts, l.City)
			}
			if l.District != "" {
				parts = append(parts, l.District)
			}
		} else {
			if l.Country != "" {
				parts = append(parts, l.Country)
			}
			if l.Province != "" {
				parts = append(parts, l.Province)
			}
			if l.City != "" {
				parts = append(parts, l.City)
			}
			if l.District != "" {
				parts = append(parts, l.District)
			}
		}

		if l.ISP != "" {
			parts = append(parts, l.ISP)
		}
	}

	// If no information available, return N/A
	if len(parts) == 0 {
		return "N/A"
	}

	return strings.Join(parts, " ")
}

// downloadFileWithProgress - Download file with progress display
func downloadFileWithProgress(filePath, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status code: %d", resp.StatusCode)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Use buffered writing
	buf := make([]byte, 32*1024)
	var downloaded int64
	startTime := time.Now()
	var lastPercent int = -1

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			downloaded += int64(n)

			// Only display when percentage changes
			if resp.ContentLength > 0 {
				currentPercent := int(float64(downloaded) / float64(resp.ContentLength) * 100)
				if currentPercent != lastPercent {
					fmt.Printf("\rDownloading: %d%%", currentPercent)
					lastPercent = currentPercent
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Printf("Download completed in %v", time.Since(startTime))
	return nil
}

// GetBinaryDatabasePath - Get binary database file path
func GetBinaryDatabasePath() string {
	// First check current directory
	currentDir, _ := os.Getwd()
	paths := []string{
		filepath.Join(currentDir, BinaryDatabaseFile),
		filepath.Join(currentDir, "data", "geoip.bin"),
		BinaryDatabaseFile,
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// If none exist, return default path
	return filepath.Join(currentDir, BinaryDatabaseFile)
}

// IsInitialized - Check if database is initialized
func (g *GeoIP) IsInitialized() bool {
	return g.initialized
}
