package geoip

import (
	"fmt"
	"io"
	"log"
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

// LookupByIP 查找与 IP 对应的地理位置
func (g *GeoIP) LookupByIP(ipStr string) (*Location, error) {
	if !g.initialized {
		return nil, fmt.Errorf("GeoIP database not initialized")
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

// Format 格式化显示地理位置信息
// 中国范围内不显示国家，其他国家显示国家
func (l *Location) Format() string {
	var parts []string

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
