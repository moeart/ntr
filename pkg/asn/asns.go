package asn

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
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
	// IPToASNURL - Official source for IP to ASN data
	IPToASNURL = "https://iptoasn.com/data/ip2asn-combined.tsv.gz"
	// BinaryDatabaseFile - Compiled binary database filename
	BinaryDatabaseFile = "data/ipasn.bin"
	// TempTSVFile - Temporary storage for downloaded and decompressed TSV file
	TempTSVFile = "ip2asn-combined.tsv"
	// TempGZFile - Temporary storage for downloaded gzip file
	TempGZFile = "ip2asn-combined.tsv.gz"
	// BinaryFormatVersion - Binary file format version
	BinaryFormatVersion = 2
)

// Binary file format structure:
// Header (16 bytes):
//   - Magic number: 4 bytes "IPAS"
//   - Version: 4 bytes uint32
//   - Record count: 8 bytes uint64
// Index section: (16 + 8) * record count
//   - Each index: Start IP(16 bytes) + Offset(8 bytes)
// Data section:
//   - Each record: End IP(16 bytes) + Country code length(1 byte) + ASN length(1 byte) + Description length(2 bytes) + Data

// ASNs - Stores all ASN information and provides query functionality
type ASNs struct {
	// Memory mapped file
	mappedData  []byte
	indexOffset int64    // Index start position
	dataOffset  int64    // Data start position
	recordCount int64    // Number of records
	file        *os.File // File handle (kept open for memory mapping)
	initialized atomic.Bool

	// Cache
	cache sync.Map
}

// NewASNs - Create new ASNs instance and load ASN database
func NewASNs() (*ASNs, error) {
	asns := &ASNs{}

	filePath := GetBinaryDatabasePath()
	err := asns.loadFromBinaryMMAP(filePath)
	if err == nil {
		return asns, nil
	}

	// Return empty ASNs so program can run normally, just won't find ASN data
	asns.initialized.Store(true)
	return asns, nil
}

// UpdateASNDatabase - Download latest ASN database from specified URL
func UpdateASNDatabase(downloadURL string) error {
	// If no URL provided, use default
	if downloadURL == "" {
		downloadURL = IPToASNURL
	}

	fmt.Println("Starting ASN database update")

	// Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Download gzip file
	fmt.Printf("Downloading ip2asn-combined.tsv.gz from %s...\n", downloadURL)
	if err := downloadFileWithProgress(TempGZFile, downloadURL); err != nil {
		return fmt.Errorf("failed to download database: %v", err)
	}

	// Decompress gzip file
	fmt.Println("Decompressing downloaded file...")
	if err := gunzipFile(TempGZFile, TempTSVFile); err != nil {
		cleanupTempFiles()
		return fmt.Errorf("failed to decompress database: %v", err)
	}

	fmt.Println("Compiling database to binary...")
	// First collect all ASN records
	records, err := collectASNRecords(TempTSVFile)
	if err != nil {
		cleanupTempFiles()
		return fmt.Errorf("failed to collect ASN records: %v", err)
	}

	// Save as optimized format
	if err := saveOptimizedBinary(GetBinaryDatabasePath(), records); err != nil {
		cleanupTempFiles()
		return fmt.Errorf("failed to compile binary database: %v", err)
	}

	cleanupTempFiles()
	fmt.Printf("Database updated successfully, %d records compiled", len(records))
	return nil
}

// collectASNRecords - Collect ASN records from TSV file
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

		// Ensure ASN number starts with "AS" prefix
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

	// Sort by start IP
	sort.Slice(records, func(i, j int) bool {
		return records[i].FirstIP.LessThan(records[j].FirstIP)
	})

	return records, nil
}

// saveOptimizedBinary - Save ASN records to optimized binary format
func saveOptimizedBinary(filePath string, records []*ASN) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 1. Write file header
	header := make([]byte, 16)
	// Magic number
	copy(header[0:4], []byte("IPAS"))
	// Version
	binary.LittleEndian.PutUint32(header[4:8], BinaryFormatVersion)
	// Record count
	binary.LittleEndian.PutUint64(header[8:16], uint64(len(records)))

	if _, err := file.Write(header); err != nil {
		return err
	}

	// 2. Prepare to write index and data
	indexOffset := int64(16)                           // Start after header
	dataOffset := indexOffset + int64(len(records))*24 // Each index is 24 bytes (16+8)

	// 3. Write index and data
	currentDataOffset := dataOffset
	for _, record := range records {
		// Write index entry
		indexEntry := make([]byte, 24)
		// Start IP
		firstIPv6 := record.FirstIP.ToIPv6()
		copy(indexEntry[0:16], firstIPv6)
		// Offset
		binary.LittleEndian.PutUint64(indexEntry[16:24], uint64(currentDataOffset))

		if _, err := file.Write(indexEntry); err != nil {
			return err
		}

		// Prepare data record
		countryBytes := []byte(record.Country)
		asnBytes := []byte(record.Number)
		descBytes := []byte(record.Description)

		// Calculate data length
		dataLen := 16 + // End IP
			1 + // Country code length
			1 + // ASN length
			2 + // Description length
			len(countryBytes) + len(asnBytes) + len(descBytes)

		// Jump to data position
		currentPos, _ := file.Seek(0, io.SeekCurrent)
		file.Seek(currentDataOffset, io.SeekStart)

		dataRecord := make([]byte, dataLen)
		offset := 0

		// End IP
		lastIPv6 := record.LastIP.ToIPv6()
		copy(dataRecord[offset:offset+16], lastIPv6)
		offset += 16

		// Country code length and content
		dataRecord[offset] = byte(len(countryBytes))
		offset++
		copy(dataRecord[offset:offset+len(countryBytes)], countryBytes)
		offset += len(countryBytes)

		// ASN length and content
		dataRecord[offset] = byte(len(asnBytes))
		offset++
		copy(dataRecord[offset:offset+len(asnBytes)], asnBytes)
		offset += len(asnBytes)

		// Description length and content
		binary.LittleEndian.PutUint16(dataRecord[offset:offset+2], uint16(len(descBytes)))
		offset += 2
		copy(dataRecord[offset:offset+len(descBytes)], descBytes)

		if _, err := file.Write(dataRecord); err != nil {
			return err
		}

		// Return to index writing position
		file.Seek(currentPos, io.SeekStart)
		currentDataOffset += int64(dataLen)
	}

	return nil
}

// loadFromBinaryMMAP - Load binary file using memory mapping
func (asns *ASNs) loadFromBinaryMMAP(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read file header
	header := make([]byte, 16)
	if _, err := file.Read(header); err != nil {
		return err
	}

	// Verify magic number
	if string(header[0:4]) != "IPAS" {
		return fmt.Errorf("invalid binary format")
	}

	// Check version
	version := binary.LittleEndian.Uint32(header[4:8])
	if version != BinaryFormatVersion {
		return fmt.Errorf("unsupported binary format version: %d", version)
	}

	// Get record count
	recordCount := binary.LittleEndian.Uint64(header[8:16])
	asns.recordCount = int64(recordCount)

	// Calculate index and data offsets
	asns.indexOffset = 16
	asns.dataOffset = asns.indexOffset + int64(recordCount)*24

	// Reopen file for memory mapping (keep open)
	asns.file, err = os.Open(filePath)
	if err != nil {
		return err
	}

	// Get file size
	_, err = asns.file.Stat()
	if err != nil {
		asns.file.Close()
		return err
	}

	// Use memory mapping on Unix systems, regular read on Windows
	if canUseMMAP() {
		// Use syscall.Mmap for memory mapping (using regular read for simplicity)
		// In production, syscall package can be used for true memory mapping
		data, err := io.ReadAll(asns.file)
		if err != nil {
			asns.file.Close()
			return err
		}
		asns.mappedData = data
	} else {
		// For systems that don't support memory mapping, read entire file
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

// canUseMMAP - Check if memory mapping is available
func canUseMMAP() bool {
	// More complex check logic can be added here
	// Currently returns true but uses regular read as fallback
	return true
}

// getIndexEntry - Get index entry
func (asns *ASNs) getIndexEntry(idx int64) (firstIP Ip, offset uint64) {
	if idx < 0 || idx >= asns.recordCount {
		return Ip{}, 0
	}

	indexPos := asns.indexOffset + idx*24
	// Read start IP
	firstIPBytes := asns.mappedData[indexPos : indexPos+16]
	firstIP = Ip{IP: net.IP(firstIPBytes)}

	// Read offset
	offset = binary.LittleEndian.Uint64(asns.mappedData[indexPos+16 : indexPos+24])

	return firstIP, offset
}

// getDataRecord - Get data record
func (asns *ASNs) getDataRecord(offset uint64) *ASN {
	if offset >= uint64(len(asns.mappedData)) {
		return nil
	}

	data := asns.mappedData[offset:]

	// Read end IP
	lastIP := Ip{IP: net.IP(data[0:16])}
	data = data[16:]

	// Read country code
	countryLen := int(data[0])
	data = data[1:]
	country := string(data[:countryLen])
	data = data[countryLen:]

	// Read ASN
	asnLen := int(data[0])
	data = data[1:]
	asnNumber := string(data[:asnLen])
	data = data[asnLen:]

	// Read description
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

// LookupByIP - Lookup ASN for given IP address
func (asns *ASNs) LookupByIP(ipStr string) (*ASN, error) {
	if !asns.initialized.Load() || asns.recordCount == 0 {
		return nil, fmt.Errorf("ASN database not initialized")
	}

	// First check cache
	if value, ok := asns.cache.Load(ipStr); ok {
		return value.(*ASN), nil
	}

	ip := ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address format: %v", ipStr)
	}

	asn := asns.lookupByIPBinary(*ip)
	if asn != nil {
		// Cache lookup result
		asns.cache.Store(ipStr, asn)
	}

	return asn, nil
}

// lookupByIPBinary - Use binary search to locate IP
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
			// Check if it's the last record or next record's start IP is greater than current IP
			if mid == asns.recordCount-1 {
				// This is the last record, check if in range
				return asns.checkRecord(offset, ip)
			}

			nextFirstIP, _ := asns.getIndexEntry(mid + 1)
			if nextFirstIP.GreaterThan(ip) {
				// Next record's start IP is greater than current IP, check current record
				return asns.checkRecord(offset, ip)
			} else {
				// Continue searching right
				left = mid + 1
			}
		} else {
			// Continue searching left
			right = mid - 1
		}
	}

	return nil
}

// checkRecord - Check if record contains IP
func (asns *ASNs) checkRecord(offset uint64, ip Ip) *ASN {
	record := asns.getDataRecord(offset)
	if record == nil {
		return nil
	}

	if record.LastIP.GreaterThanOrEqual(ip) {
		// We need to create a complete ASN record containing start IP
		// Note: We can't get start IP here because index and data are separate
		// In practical use, this may not be a problem because start IP is usually not needed
		return record
	}

	return nil
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

// gunzipFile - Decompress gzip file
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

	// Use buffered writing to improve decompression speed
	buf := make([]byte, 32*1024)
	_, err = io.CopyBuffer(tsvFile, gzReader, buf)
	return err
}

// GetBinaryDatabasePath - Get binary database file path
func GetBinaryDatabasePath() string {
	// First check current directory
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

	// If none exist, return default path
	return filepath.Join(currentDir, BinaryDatabaseFile)
}

// GetAllASNs - Get all ASN information
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

// Size - Get number of ASNs
func (asns *ASNs) Size() int {
	return int(asns.recordCount)
}

// cleanupTempFiles - Cleanup temporary files
func cleanupTempFiles() {
	filesToRemove := []string{TempTSVFile, TempGZFile}

	for _, fileName := range filesToRemove {
		if _, err := os.Stat(fileName); err == nil {
			if err := os.Remove(fileName); err != nil {
				fmt.Printf("Failed to remove %s: %v", fileName, err)
			}
		}
	}
}

// Close - Close ASN database and release resources
func (asns *ASNs) Close() error {
	asns.initialized.Store(false)
	asns.mappedData = nil
	asns.recordCount = 0

	if asns.file != nil {
		return asns.file.Close()
	}
	return nil
}

// IsInitialized - Check if database is initialized
func (asns *ASNs) IsInitialized() bool {
	return asns.initialized.Load() && asns.recordCount > 0
}

// ipToUint128 - Convert IP to uint128 (for comparison)
func ipToUint128(ip net.IP) (uint64, uint64) {
	ipv6 := ip.To16()
	if ipv6 == nil {
		return 0, 0
	}
	high := binary.BigEndian.Uint64(ipv6[:8])
	low := binary.BigEndian.Uint64(ipv6[8:])
	return high, low
}

// Simple memory pool to reduce GC pressure
var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32)
	},
}

// getBuffer - Get buffer from pool
func getBuffer() []byte {
	return bufferPool.Get().([]byte)
}

// putBuffer - Put buffer back into pool
func putBuffer(buf []byte) {
	bufferPool.Put(buf)
}
