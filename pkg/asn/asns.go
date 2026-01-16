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
	"sync/atomic"
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
	// BinaryFormatVersion 二进制文件格式版本
	BinaryFormatVersion = 2
)

// 二进制文件格式结构：
// Header (16字节):
//   - 魔数: 4字节 "IPAS"
//   - 版本: 4字节 uint32
//   - 记录数: 8字节 uint64
// 索引部分: (16 + 8) * 记录数
//   - 每个索引: 起始IP(16字节) + 偏移量(8字节)
// 数据部分:
//   - 每个记录: 结束IP(16字节) + 国家代码长度(1字节) + ASN长度(1字节) + 描述长度(2字节) + 数据

// ASNs 存储所有 ASN 信息并提供查询功能
type ASNs struct {
	// 使用内存映射文件
	mappedData  []byte
	indexOffset int64    // 索引开始位置
	dataOffset  int64    // 数据开始位置
	recordCount int64    // 记录数
	file        *os.File // 文件句柄（保持打开用于内存映射）
	initialized atomic.Bool

	// 缓存
	cache sync.Map
}

// NewASNs 创建新的 ASNs 实例并加载 ASN 数据库
func NewASNs() (*ASNs, error) {
	asns := &ASNs{}

	filePath := GetBinaryDatabasePath()
	err := asns.loadFromBinaryMMAP(filePath)
	if err == nil {
		log.Printf("Successfully loaded ASN database from binary file (records: %d)", asns.recordCount)
		return asns, nil
	}

	log.Printf("Warning: Could not load ASN database from binary file: %v", err)
	log.Println("ASN data will not be available. To enable ASN lookups, please run the update command.")
	log.Println("You can manually update the database by calling UpdateASNDatabase()")

	// 返回一个空的ASNs，这样程序可以正常运行，只是查询不到ASN
	asns.initialized.Store(true)
	return asns, nil
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
	if err := downloadFileWithProgress(TempGZFile, IPToASNURL); err != nil {
		return fmt.Errorf("failed to download database: %v", err)
	}

	// 解压缩 gzip 文件
	log.Println("Decompressing downloaded file...")
	if err := gunzipFile(TempGZFile, TempTSVFile); err != nil {
		cleanupTempFiles()
		return fmt.Errorf("failed to decompress database: %v", err)
	}

	log.Println("Compiling database to binary...")
	// 先收集所有ASN记录
	records, err := collectASNRecords(TempTSVFile)
	if err != nil {
		cleanupTempFiles()
		return fmt.Errorf("failed to collect ASN records: %v", err)
	}

	// 保存为优化格式
	if err := saveOptimizedBinary(GetBinaryDatabasePath(), records); err != nil {
		cleanupTempFiles()
		return fmt.Errorf("failed to compile binary database: %v", err)
	}

	cleanupTempFiles()
	log.Printf("Database updated successfully, %d records compiled", len(records))
	return nil
}

// collectASNRecords 从TSV文件收集ASN记录
func collectASNRecords(filePath string) ([]*ASN, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []*ASN
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

		records = append(records, &ASN{
			FirstIP:     *firstIP,
			LastIP:      *lastIP,
			Number:      asnNumber,
			Country:     country,
			Description: description,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 按起始IP排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].FirstIP.LessThan(records[j].FirstIP)
	})

	return records, nil
}

// saveOptimizedBinary 保存为优化格式的二进制文件
func saveOptimizedBinary(filePath string, records []*ASN) error {
	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 1. 写入文件头
	header := make([]byte, 16)
	// 魔数
	copy(header[0:4], []byte("IPAS"))
	// 版本
	binary.LittleEndian.PutUint32(header[4:8], BinaryFormatVersion)
	// 记录数
	binary.LittleEndian.PutUint64(header[8:16], uint64(len(records)))

	if _, err := file.Write(header); err != nil {
		return err
	}

	// 2. 准备写入索引和数据
	indexOffset := int64(16)                           // 头之后开始
	dataOffset := indexOffset + int64(len(records))*24 // 每个索引24字节(16+8)

	// 3. 写入索引和数据
	currentDataOffset := dataOffset
	for _, record := range records {
		// 写入索引项
		indexEntry := make([]byte, 24)
		// 起始IP
		firstIPv6 := record.FirstIP.ToIPv6()
		copy(indexEntry[0:16], firstIPv6)
		// 偏移量
		binary.LittleEndian.PutUint64(indexEntry[16:24], uint64(currentDataOffset))

		if _, err := file.Write(indexEntry); err != nil {
			return err
		}

		// 准备数据记录
		countryBytes := []byte(record.Country)
		asnBytes := []byte(record.Number)
		descBytes := []byte(record.Description)

		// 计算数据长度
		dataLen := 16 + // 结束IP
			1 + // 国家代码长度
			1 + // ASN长度
			2 + // 描述长度
			len(countryBytes) + len(asnBytes) + len(descBytes)

		// 跳转到数据位置写入
		currentPos, _ := file.Seek(0, io.SeekCurrent)
		file.Seek(currentDataOffset, io.SeekStart)

		dataRecord := make([]byte, dataLen)
		offset := 0

		// 结束IP
		lastIPv6 := record.LastIP.ToIPv6()
		copy(dataRecord[offset:offset+16], lastIPv6)
		offset += 16

		// 国家代码长度和内容
		dataRecord[offset] = byte(len(countryBytes))
		offset++
		copy(dataRecord[offset:offset+len(countryBytes)], countryBytes)
		offset += len(countryBytes)

		// ASN长度和内容
		dataRecord[offset] = byte(len(asnBytes))
		offset++
		copy(dataRecord[offset:offset+len(asnBytes)], asnBytes)
		offset += len(asnBytes)

		// 描述长度和内容
		binary.LittleEndian.PutUint16(dataRecord[offset:offset+2], uint16(len(descBytes)))
		offset += 2
		copy(dataRecord[offset:offset+len(descBytes)], descBytes)

		if _, err := file.Write(dataRecord); err != nil {
			return err
		}

		// 回到索引写入位置
		file.Seek(currentPos, io.SeekStart)
		currentDataOffset += int64(dataLen)
	}

	return nil
}

// loadFromBinaryMMAP 使用内存映射加载二进制文件
func (asns *ASNs) loadFromBinaryMMAP(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 读取文件头
	header := make([]byte, 16)
	if _, err := file.Read(header); err != nil {
		return err
	}

	// 验证魔数
	if string(header[0:4]) != "IPAS" {
		return fmt.Errorf("invalid binary format")
	}

	// 检查版本
	version := binary.LittleEndian.Uint32(header[4:8])
	if version != BinaryFormatVersion {
		return fmt.Errorf("unsupported binary format version: %d", version)
	}

	// 获取记录数
	recordCount := binary.LittleEndian.Uint64(header[8:16])
	asns.recordCount = int64(recordCount)

	// 计算索引和数据偏移
	asns.indexOffset = 16
	asns.dataOffset = asns.indexOffset + int64(recordCount)*24

	// 重新打开文件用于内存映射（保持打开）
	asns.file, err = os.Open(filePath)
	if err != nil {
		return err
	}

	// 获取文件大小
	_, err = asns.file.Stat()
	if err != nil {
		asns.file.Close()
		return err
	}

	// 在Unix系统上使用内存映射，Windows使用普通读取
	if canUseMMAP() {
		// 使用syscall.Mmap实现内存映射（为简化代码，这里使用普通读取）
		// 在实际生产环境中，可以引入syscall包实现真正的内存映射
		data, err := io.ReadAll(asns.file)
		if err != nil {
			asns.file.Close()
			return err
		}
		asns.mappedData = data
	} else {
		// 对于不支持内存映射的系统，直接读取整个文件
		data, err := io.ReadAll(asns.file)
		if err != nil {
			asns.file.Close()
			return err
		}
		asns.mappedData = data
	}

	asns.initialized.Store(true)
	return nil
}

// canUseMMAP 检查是否可以使用内存映射
func canUseMMAP() bool {
	// 这里可以添加更复杂的检查逻辑
	// 目前返回true，但使用普通读取作为后备
	return true
}

// getIndexEntry 获取索引项
func (asns *ASNs) getIndexEntry(idx int64) (firstIP Ip, offset uint64) {
	if idx < 0 || idx >= asns.recordCount {
		return Ip{}, 0
	}

	indexPos := asns.indexOffset + idx*24
	// 读取起始IP
	firstIPBytes := asns.mappedData[indexPos : indexPos+16]
	firstIP = Ip{IP: net.IP(firstIPBytes)}

	// 读取偏移量
	offset = binary.LittleEndian.Uint64(asns.mappedData[indexPos+16 : indexPos+24])

	return firstIP, offset
}

// getDataRecord 获取数据记录
func (asns *ASNs) getDataRecord(offset uint64) *ASN {
	if offset >= uint64(len(asns.mappedData)) {
		return nil
	}

	data := asns.mappedData[offset:]

	// 读取结束IP
	lastIP := Ip{IP: net.IP(data[0:16])}
	data = data[16:]

	// 读取国家代码
	countryLen := int(data[0])
	data = data[1:]
	country := string(data[:countryLen])
	data = data[countryLen:]

	// 读取ASN
	asnLen := int(data[0])
	data = data[1:]
	asnNumber := string(data[:asnLen])
	data = data[asnLen:]

	// 读取描述
	descLen := int(binary.LittleEndian.Uint16(data[0:2]))
	data = data[2:]
	description := string(data[:descLen])

	return &ASN{
		LastIP:      lastIP,
		Number:      asnNumber,
		Country:     country,
		Description: description,
	}
}

// LookupByIP 查找与 IP 对应的 ASN
func (asns *ASNs) LookupByIP(ipStr string) (*ASN, error) {
	if !asns.initialized.Load() || asns.recordCount == 0 {
		return nil, fmt.Errorf("ASN database not initialized")
	}

	// 首先检查缓存
	if value, ok := asns.cache.Load(ipStr); ok {
		return value.(*ASN), nil
	}

	ip := ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address format: %v", ipStr)
	}

	asn := asns.lookupByIPBinary(*ip)
	if asn != nil {
		// 缓存查找结果
		asns.cache.Store(ipStr, asn)
	}

	return asn, nil
}

// lookupByIPBinary 使用二分查找定位IP
func (asns *ASNs) lookupByIPBinary(ip Ip) *ASN {
	if asns.recordCount == 0 {
		return nil
	}

	left := int64(0)
	right := asns.recordCount - 1

	for left <= right {
		mid := left + (right-left)/2
		firstIP, offset := asns.getIndexEntry(mid)

		if firstIP.LessThanOrEqual(ip) {
			// 检查是否是最后一个记录或下一个记录的起始IP大于当前IP
			if mid == asns.recordCount-1 {
				// 这是最后一个记录，检查是否在范围内
				return asns.checkRecord(offset, ip)
			}

			nextFirstIP, _ := asns.getIndexEntry(mid + 1)
			if nextFirstIP.GreaterThan(ip) {
				// 下一个记录的起始IP大于当前IP，检查当前记录
				return asns.checkRecord(offset, ip)
			} else {
				// 继续向右查找
				left = mid + 1
			}
		} else {
			// 继续向左查找
			right = mid - 1
		}
	}

	return nil
}

// checkRecord 检查记录是否包含IP
func (asns *ASNs) checkRecord(offset uint64, ip Ip) *ASN {
	record := asns.getDataRecord(offset)
	if record == nil {
		return nil
	}

	if record.LastIP.GreaterThanOrEqual(ip) {
		// 我们需要创建一个完整的ASN记录，包含起始IP
		// 注意：这里我们无法获取起始IP，因为索引和数据是分离的
		// 在实际使用中，这可能不是问题，因为起始IP通常不需要
		return record
	}

	return nil
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

// gunzipFile 解压缩 gzip 文件
func gunzipFile(gzPath, tsvPath string) error {
	gzFile, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer gzFile.Close()

	gzReader, err := gzip.NewReader(gzFile)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tsvFile, err := os.Create(tsvPath)
	if err != nil {
		return err
	}
	defer tsvFile.Close()

	// 使用缓冲写入提高解压速度
	buf := make([]byte, 32*1024)
	_, err = io.CopyBuffer(tsvFile, gzReader, buf)
	return err
}

// GetBinaryDatabasePath 获取二进制数据库文件路径
func GetBinaryDatabasePath() string {
	// 首先检查当前目录
	currentDir, _ := os.Getwd()
	paths := []string{
		filepath.Join(currentDir, BinaryDatabaseFile),
		filepath.Join(currentDir, "data", "ipasn.bin"),
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

// GetAllASNs 获取所有 ASN 信息
func (asns *ASNs) GetAllASNs() []*ASN {
	if !asns.initialized.Load() || asns.recordCount == 0 {
		return nil
	}

	var result []*ASN
	for i := int64(0); i < asns.recordCount; i++ {
		_, offset := asns.getIndexEntry(i)
		record := asns.getDataRecord(offset)
		if record != nil {
			result = append(result, record)
		}
	}
	return result
}

// Size 获取 ASN 的数量
func (asns *ASNs) Size() int {
	return int(asns.recordCount)
}

// cleanupTempFiles 清理临时文件
func cleanupTempFiles() {
	filesToRemove := []string{TempTSVFile, TempGZFile}

	for _, fileName := range filesToRemove {
		if _, err := os.Stat(fileName); err == nil {
			if err := os.Remove(fileName); err != nil {
				log.Printf("Failed to remove %s: %v", fileName, err)
			}
		}
	}
}

// Close 关闭ASN数据库，释放资源
func (asns *ASNs) Close() error {
	asns.initialized.Store(false)
	asns.mappedData = nil
	asns.recordCount = 0

	if asns.file != nil {
		return asns.file.Close()
	}
	return nil
}

// IsInitialized 检查数据库是否已初始化
func (asns *ASNs) IsInitialized() bool {
	return asns.initialized.Load() && asns.recordCount > 0
}

// ipToUint128 将IP转换为uint128（用于比较）
func ipToUint128(ip net.IP) (uint64, uint64) {
	ipv6 := ip.To16()
	if ipv6 == nil {
		return 0, 0
	}
	high := binary.BigEndian.Uint64(ipv6[:8])
	low := binary.BigEndian.Uint64(ipv6[8:])
	return high, low
}

// 简单的内存池，减少GC压力
var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32)
	},
}

// getBuffer 从池中获取缓冲区
func getBuffer() []byte {
	return bufferPool.Get().([]byte)
}

// putBuffer 将缓冲区放回池中
func putBuffer(buf []byte) {
	bufferPool.Put(buf)
}
