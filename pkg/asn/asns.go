package asn

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// IPToASNURL 是获取 IP 到 ASN 数据的官方数据源
	IPToASNURL = "https://iptoasn.com/data/ip2asn-combined.tsv.gz"
	// BinaryDatabaseFile 是编译后的二进制数据库文件名
	BinaryDatabaseFile = "data/ipasn.bin"
	// TempTSVFile 是临时存储下载并解压缩后的 TSV 文件
	TempTSVFile = "ip2asn-combined.tsv"
	// TempGZFile 是临时存储下载的 gzip 文件
	TempGZFile = "ip2asn-combined.tsv.gz"
)

// ASNs 存储所有 ASN 信息并提供查询功能
type ASNs struct {
	asns  []*ASN
	cache sync.Map
}

// NewASNs 创建新的 ASNs 实例并加载 ASN 数据库
func NewASNs() (*ASNs, error) {
	asns := &ASNs{}

	// 尝试加载二进制数据库
	filePath := GetBinaryDatabasePath()
	if err := asns.loadFromBinary(filePath); err == nil {
		log.Println("Successfully loaded ASN database from binary file")
		return asns, nil
	}

	log.Println("Binary database not found or corrupted, checking for TSV file")
	tsvPath := getTSVPath()
	if err := asns.loadFromTSV(tsvPath); err == nil {
		log.Println("TSV file loaded successfully, compiling to binary")
		if err := asns.saveToBinary(filePath); err != nil {
			log.Printf("Failed to compile binary database: %v", err)
		}
		return asns, nil
	}

	log.Println("TSV file not found, downloading latest database")
	if err := UpdateASNDatabase(); err != nil {
		return nil, err
	}

	// 下载和编译完成后，再次尝试加载二进制数据库
	if err := asns.loadFromBinary(filePath); err == nil {
		log.Println("Successfully loaded ASN database from binary file")
		return asns, nil
	}

	return nil, fmt.Errorf("failed to load ASN database from any source")
}

// UpdateASNDatabase 从 iptoasn.com 下载最新的 ASN 数据库
func UpdateASNDatabase() error {
	log.Println("Starting ASN database update from iptoasn.com")

	// 确保 data 目录存在
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// 下载 gzip 文件
	log.Println("Downloading ip2asn-combined.tsv.gz...")
	if err := downloadFile(TempGZFile, IPToASNURL); err != nil {
		return fmt.Errorf("failed to download database: %v", err)
	}

	// 解压缩 gzip 文件
	log.Println("Decompressing downloaded file...")
	if err := gunzipFile(TempGZFile, TempTSVFile); err != nil {
		cleanupTempFiles()
		return fmt.Errorf("failed to decompress database: %v", err)
	}

	log.Println("Compiling database to binary...")
	asns := &ASNs{}
	if err := asns.loadFromTSV(TempTSVFile); err != nil {
		cleanupTempFiles()
		return fmt.Errorf("failed to load TSV file: %v", err)
	}

	if err := asns.saveToBinary(GetBinaryDatabasePath()); err != nil {
		cleanupTempFiles()
		return fmt.Errorf("failed to compile binary database: %v", err)
	}

	cleanupTempFiles()
	log.Println("Database updated and compiled successfully")
	return nil
}

// downloadFile 下载文件并显示进度
func downloadFile(filePath, url string) error {
	// 发送 HTTP 请求
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status code: %d", resp.StatusCode)
	}

	// 创建文件
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Printf("Failed to close output file: %v", err)
		}
	}()

	// 下载文件并显示进度
	var downloaded int64
	buffer := make([]byte, 8192)
	var lastPercent int = -1

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, err := out.Write(buffer[:n]); err != nil {
				return err
			}
			downloaded += int64(n)

			// 计算当前百分比，只在整数百分比变化时显示
			currentPercent := int(float64(downloaded) / float64(resp.ContentLength) * 100)
			if currentPercent != lastPercent {
				fmt.Printf("\rDownloading: %d%%", currentPercent)
				lastPercent = currentPercent
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// 下载完成后换行
	fmt.Println()

	return nil
}

// showDownloadProgress 显示下载进度（在一行刷新）
func showDownloadProgress(downloaded, total int64, startTime time.Time) {
	if total <= 0 {
		fmt.Printf("\rDownloaded: %0.2f MB                                                                 ", float64(downloaded)/(1024*1024))
		return
	}

	percent := float64(downloaded) / float64(total) * 100
	elapsed := time.Since(startTime).Seconds()
	speed := float64(downloaded) / elapsed / (1024 * 1024) // MB/s
	remaining := (float64(total) - float64(downloaded)) / (speed * 1024 * 1024)

	// 使用固定宽度的格式化字符串，确保每次输出的长度一致，完全覆盖上一行
	fmt.Printf("\rProgress: %0.1f%% (%0.2f/%0.2f MB) - Speed: %0.2f MB/s - Remaining: %0.1f sec                                                                 ",
		percent, float64(downloaded)/(1024*1024), float64(total)/(1024*1024), speed, remaining)
}

// gunzipFile 解压缩 gzip 文件
func gunzipFile(gzPath, tsvPath string) error {
	gzFile, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := gzFile.Close(); err != nil {
			log.Printf("Failed to close gzip file: %v", err)
		}
	}()

	gzReader, err := gzip.NewReader(gzFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := gzReader.Close(); err != nil {
			log.Printf("Failed to close gzip reader: %v", err)
		}
	}()

	tsvFile, err := os.Create(tsvPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := tsvFile.Close(); err != nil {
			log.Printf("Failed to close TSV file: %v", err)
		}
	}()

	if _, err := io.Copy(tsvFile, gzReader); err != nil {
		return err
	}

	return nil
}

// GetBinaryDatabasePath 获取二进制数据库文件路径
func GetBinaryDatabasePath() string {
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Printf("Failed to get executable directory: %v", err)
		exeDir = "."
	}
	return filepath.Join(exeDir, BinaryDatabaseFile)
}

// getTSVPath 获取 TSV 文件路径
func getTSVPath() string {
	return TempTSVFile
}

// loadFromTSV 从 TSV 文件加载 ASN 数据
func (asns *ASNs) loadFromTSV(filePath string) error {
	asns.asns = []*ASN{} // 清空现有内容
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close file: %v", err)
		}
	}()

	var asnCount int
	var start = time.Now()
	log.Println("Loading ASN database from TSV file")

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || line[0] == '#' {
			continue
		}

		columns := strings.Split(line, "\t")
		if len(columns) < 5 {
			continue
		}

		// iptoasn.com TSV 格式: first_ip\tlast_ip\tasn\tcountry\tdescription
		firstIP := ParseIP(columns[0])
		lastIP := ParseIP(columns[1])
		asnNumber := columns[2]
		country := columns[3]
		description := columns[4]

		if firstIP == nil || lastIP == nil || asnNumber == "" {
			continue
		}

		// 确保 ASN 编号以 "AS" 前缀开头
		if len(asnNumber) > 0 && !strings.HasPrefix(strings.ToUpper(asnNumber), "AS") {
			asnNumber = "AS" + asnNumber
		}

		asn := &ASN{
			FirstIP:     *firstIP,
			LastIP:      *lastIP,
			Number:      asnNumber,
			Country:     country,
			Description: description,
		}
		asns.asns = append(asns.asns, asn)
		asnCount++
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	log.Printf("Loaded %d ASN entries", asnCount)
	log.Printf("Loading time: %v", time.Since(start))

	if len(asns.asns) > 0 {
		start = time.Now()
		sort.Slice(asns.asns, func(i, j int) bool {
			return asns.asns[i].FirstIP.LessThan(asns.asns[j].FirstIP)
		})
		log.Printf("Sorting completed in %v", time.Since(start))
	}

	return nil
}

// loadFromBinary 从二进制文件加载 ASN 数据
func (asns *ASNs) loadFromBinary(filePath string) error {
	asns.asns = []*ASN{}
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close file: %v", err)
		}
	}()

	var count int32
	if err := binary.Read(file, binary.LittleEndian, &count); err != nil {
		return err
	}

	asns.asns = make([]*ASN, 0, count)
	start := time.Now()

	for i := 0; i < int(count); i++ {
		// 读取 ASN 编号长度
		var numberLen int32
		if err := binary.Read(file, binary.LittleEndian, &numberLen); err != nil {
			return err
		}

		numberBytes := make([]byte, numberLen)
		if _, err := file.Read(numberBytes); err != nil {
			return err
		}

		// 读取国家代码长度
		var countryLen int32
		if err := binary.Read(file, binary.LittleEndian, &countryLen); err != nil {
			return err
		}

		countryBytes := make([]byte, countryLen)
		if _, err := file.Read(countryBytes); err != nil {
			return err
		}

		// 读取描述信息长度
		var descLen int32
		if err := binary.Read(file, binary.LittleEndian, &descLen); err != nil {
			return err
		}

		descBytes := make([]byte, descLen)
		if _, err := file.Read(descBytes); err != nil {
			return err
		}

		// 读取 FirstIP (16字节 IPv6)
		var firstIPBytes [16]byte
		if _, err := file.Read(firstIPBytes[:]); err != nil {
			return err
		}

		// 读取 LastIP (16字节 IPv6)
		var lastIPBytes [16]byte
		if _, err := file.Read(lastIPBytes[:]); err != nil {
			return err
		}

		asn := &ASN{
			Number:      string(numberBytes),
			Country:     string(countryBytes),
			Description: string(descBytes),
			FirstIP:     Ip{IP: net.IP(firstIPBytes[:])},
			LastIP:      Ip{IP: net.IP(lastIPBytes[:])},
		}

		asns.asns = append(asns.asns, asn)
	}

	log.Printf("Loaded %d ASN entries from binary in %v", count, time.Since(start))
	return nil
}

// saveToBinary 将 ASN 数据保存到二进制文件
func (asns *ASNs) saveToBinary(filePath string) error {
	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close file: %v", err)
		}
	}()

	var count int32 = int32(len(asns.asns))
	if err := binary.Write(file, binary.LittleEndian, count); err != nil {
		return err
	}

	start := time.Now()

	for _, asn := range asns.asns {
		// 写入 ASN 编号
		var numberLen int32 = int32(len(asn.Number))
		if err := binary.Write(file, binary.LittleEndian, numberLen); err != nil {
			return err
		}
		if _, err := file.Write([]byte(asn.Number)); err != nil {
			return err
		}

		// 写入国家代码
		var countryLen int32 = int32(len(asn.Country))
		if err := binary.Write(file, binary.LittleEndian, countryLen); err != nil {
			return err
		}
		if _, err := file.Write([]byte(asn.Country)); err != nil {
			return err
		}

		// 写入描述信息
		var descLen int32 = int32(len(asn.Description))
		if err := binary.Write(file, binary.LittleEndian, descLen); err != nil {
			return err
		}
		if _, err := file.Write([]byte(asn.Description)); err != nil {
			return err
		}

		// 写入 FirstIP (转换为 IPv6 格式，16字节)
		firstIPv6 := asn.FirstIP.ToIPv6()
		var firstIPBytes [16]byte
		copy(firstIPBytes[:], firstIPv6)
		if _, err := file.Write(firstIPBytes[:]); err != nil {
			return err
		}

		// 写入 LastIP (转换为 IPv6 格式，16字节)
		lastIPv6 := asn.LastIP.ToIPv6()
		var lastIPBytes [16]byte
		copy(lastIPBytes[:], lastIPv6)
		if _, err := file.Write(lastIPBytes[:]); err != nil {
			return err
		}
	}

	log.Printf("Binary database saved in %v", time.Since(start))
	return nil
}

// LookupByIP 查找与 IP 对应的 ASN
func (asns *ASNs) LookupByIP(ipStr string) (*ASN, error) {
	// 首先检查缓存
	if value, ok := asns.cache.Load(ipStr); ok {
		return value.(*ASN), nil
	}

	ip := ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address format: %v", ipStr)
	}

	asn := asns.lookupByIP(*ip)
	if asn != nil {
		// 缓存查找结果
		asns.cache.Store(ipStr, asn)
	}

	return asn, nil
}

func (asns *ASNs) lookupByIP(ip Ip) *ASN {
	if len(asns.asns) == 0 {
		return nil
	}

	left := 0
	right := len(asns.asns) - 1

	for left <= right {
		mid := left + (right-left)/2
		current := asns.asns[mid]

		if current.ContainsIP(ip) {
			return current
		} else if current.FirstIP.GreaterThan(ip) {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return nil
}

// GetAllASNs 获取所有 ASN 信息
func (asns *ASNs) GetAllASNs() []*ASN {
	return asns.asns
}

// Size 获取 ASN 的数量
func (asns *ASNs) Size() int {
	return len(asns.asns)
}

// cleanupTempFiles 清理临时文件
func cleanupTempFiles() {
	filesToRemove := []string{TempTSVFile, TempGZFile}

	for _, fileName := range filesToRemove {
		if _, err := os.Stat(fileName); err == nil {
			if err := os.Remove(fileName); err != nil {
				log.Printf("Failed to remove %s: %v", fileName, err)
			} else {
				log.Printf("Removed temporary file: %s", fileName)
			}
		}
	}
}
