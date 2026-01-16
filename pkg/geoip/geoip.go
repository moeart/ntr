package geoip

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xiaoqidun/qqwry"
)

const (
	// GeoIPURL 是获取 IP 到地理位置数据的官方数据源
	GeoIPURL = "https://github.com/metowolf/qqwry.dat/releases/latest/download/qqwry.dat"
	// BinaryDatabaseFile 是编译后的二进制数据库文件名
	BinaryDatabaseFile = "data/geoip.bin"
)

// GeoIP 封装 qqwry 库并提供查询功能
type GeoIP struct {
	initialized bool
}

// NewGeoIP 创建新的 GeoIP 实例并加载数据库
func NewGeoIP() (*GeoIP, error) {
	geoip := &GeoIP{}

	filePath := GetBinaryDatabasePath()
	err := qqwry.LoadFile(filePath)
	if err == nil {
		log.Println("Successfully loaded GeoIP database")
		geoip.initialized = true
		return geoip, nil
	}

	log.Printf("Warning: Could not load GeoIP database: %v", err)
	log.Println("GeoIP data will not be available. To enable GeoIP lookups, please run the update command.")
	log.Println("You can manually update the database by calling UpdateGeoIPDatabase()")

	geoip.initialized = false
	return geoip, nil
}

// UpdateGeoIPDatabase 从 metowolf/qqwry.dat 下载最新的 GeoIP 数据库
func UpdateGeoIPDatabase() error {
	log.Println("Starting GeoIP database update from server ...")

	// 确保 data 目录存在
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// 下载数据库文件
	log.Println("Downloading ASN database ...")
	if err := downloadFileWithProgress(GetBinaryDatabasePath(), GeoIPURL); err != nil {
		return fmt.Errorf("failed to download GeoIP database: %v", err)
	}

	log.Println("GeoIP database updated successfully")
	return nil
}

// isSpecialIP 检查是否是特殊IP地址并返回对应Location
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

	// 127.*.*.* 显示为本机环回地址
	if b1 == 127 {
		if lang == "en" {
			return &Location{Country: "Loopback Address"}
		}
		return &Location{Country: "本机环回地址"}
	}

	// 10.*.*.* 显示为本地局域网
	if b1 == 10 {
		if lang == "en" {
			return &Location{Country: "Local Area Network (A-class)"}
		}
		return &Location{Country: "本地局域网"}
	}

	// 172.16.*.* 到 172.31.*.*（整个B类）显示为本地局域网
	if b1 == 172 && (b2 >= 16 && b2 <= 31) {
		if lang == "en" {
			return &Location{Country: "Local Area Network (B-class)"}
		}
		return &Location{Country: "本地局域网"}
	}

	// 192.168.*.* 显示为本地局域网
	if b1 == 192 && b2 == 168 {
		// 192.168.1-255.1 显示为本地局域网网关
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

	// 169.254.*.* 显示为零配置局域网
	if b1 == 169 && b2 == 254 {
		if lang == "en" {
			return &Location{Country: "Zero-Configuration Local Area Network"}
		}
		return &Location{Country: "零配置局域网"}
	}

	// 11.*.*.* 显示为IDC机房内网
	if b1 == 11 {
		if lang == "en" {
			return &Location{Country: "IDC Intranet"}
		}
		return &Location{Country: "IDC机房内网"}
	}

	// 134.128-255.*.* 显示为中国电信 DCN内网
	if b1 == 134 && (b2 >= 128 && b2 <= 255) {
		if lang == "en" {
			return &Location{Country: "China Telecom DCN Intranet"}
		}
		return &Location{Country: "中国电信 DCN内网"}
	}

	// 220.110-111.*.* 显示为萌信通 DCN内网
	if b1 == 220 && (b2 == 110 || b2 == 111) {
		if lang == "en" {
			return &Location{Country: "Mengxintong DCN Intranet"}
		}
		return &Location{Country: "萌信通 DCN内网"}
	}

	// 100.64-127.*.* 显示为运营商CGNAT内网
	if b1 == 100 && (b2 >= 64 && b2 <= 127) {
		if lang == "en" {
			return &Location{Country: "Carrier CGNAT Intranet"}
		}
		return &Location{Country: "运营商CGNAT内网"}
	}

	return nil
}

// LookupByIP 查询IP地址信息
func (g *GeoIP) LookupByIP(ipStr string, lang string, useQQWry bool) (*Location, error) {
	// 首先检查是否是特殊IP地址
	specialLoc := isSpecialIP(ipStr, lang)
	if specialLoc != nil {
		return specialLoc, nil
	}

	// 根据条件决定是否使用QQWry查询
	if !useQQWry || !g.initialized {
		return &Location{Country: "N/A"}, nil
	}

	location, err := qqwry.QueryIP(ipStr)
	if err != nil {
		return nil, err
	}

	return &Location{
		Country:  location.Country,
		Province: location.Province,
		City:     location.City,
		District: location.District,
		ISP:      location.ISP,
	}, nil
}

// Location 表示地理位置信息
type Location struct {
	Country  string
	Province string
	City     string
	District string
	ISP      string
}

// Format 格式化输出
func (l *Location) Format(lang string) string {
	var parts []string

	// 检查是否是特殊IP地址（这些地址的Country字段包含了完整的描述）
	specialCases := []string{"本地局域网", "本地局域网网关", "零配置局域网", "本机环回地址", "IDC机房内网", "中国电信 DCN内网", "萌信通 DCN内网", "运营商CGNAT内网"}
	if lang == "en" {
		specialCases = []string{"Local Area Network", "Loopback Address", "Zero-Configuration Local Area Network", "IDC Intranet", "China Telecom DCN Intranet", "Mengxintong DCN Intranet", "Carrier CGNAT Intranet"}
	}

	for _, special := range specialCases {
		if strings.Contains(l.Country, special) {
			return l.Country
		}
	}

	// 中国范围内不显示国家
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

	return strings.Join(parts, " ")
}

// downloadFileWithProgress 下载文件并显示进度
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

	// 使用带缓冲的写入
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

			// 只在百分比变化时显示
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
	log.Printf("Download completed in %v", time.Since(startTime))
	return nil
}

// GetBinaryDatabasePath 获取二进制数据库文件路径
func GetBinaryDatabasePath() string {
	// 首先检查当前目录
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

	// 如果都不存在，返回默认路径
	return filepath.Join(currentDir, BinaryDatabaseFile)
}

// IsInitialized 检查数据库是否已初始化
func (g *GeoIP) IsInitialized() bool {
	return g.initialized
}
