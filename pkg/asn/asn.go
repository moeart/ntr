package asn

import "net"

// ASN 表示一个自治系统
type ASN struct {
	FirstIP     Ip
	LastIP      Ip
	Number      string
	Country     string
	Description string
}

// NewASN 创建新的 ASN 实例
func NewASN(firstIP, lastIP string, number, country, description string) *ASN {
	if number == "" {
		return nil
	}

	// 确保 Number 字段以 "AS" 前缀开头
	if len(number) > 0 && number[:1] != "A" {
		number = "AS" + number
	}

	return &ASN{
		FirstIP:     Ip{net.ParseIP(firstIP)},
		LastIP:      Ip{net.ParseIP(lastIP)},
		Number:      number,
		Country:     country,
		Description: description,
	}
}

// FromSingleIP 创建临时 ASN 实例
func FromSingleIP(ipStr string, number, country, description string) *ASN {
	return &ASN{
		FirstIP:     Ip{net.ParseIP(ipStr)},
		LastIP:      Ip{net.ParseIP(ipStr)},
		Number:      number,
		Country:     country,
		Description: description,
	}
}

// CompareTo 比较两个 ASN
func (a ASN) CompareTo(other ASN) int {
	return a.FirstIP.CompareTo(other.FirstIP)
}

// ContainsIP 检查是否包含指定的 IP
func (a ASN) ContainsIP(ip Ip) bool {
	return a.FirstIP.LessThanOrEqual(ip) && a.LastIP.GreaterThanOrEqual(ip)
}

// LessThan 检查当前 ASN 是否小于另一个 ASN
func (a ASN) LessThan(other ASN) bool {
	return a.CompareTo(other) < 0
}

// GreaterThan 检查当前 ASN 是否大于另一个 ASN
func (a ASN) GreaterThan(other ASN) bool {
	return a.CompareTo(other) > 0
}

// LessThanOrEqual 检查当前 ASN 是否小于或等于另一个 ASN
func (a ASN) LessThanOrEqual(other ASN) bool {
	return a.CompareTo(other) <= 0
}

// GreaterThanOrEqual 检查当前 ASN 是否大于或等于另一个 ASN
func (a ASN) GreaterThanOrEqual(other ASN) bool {
	return a.CompareTo(other) >= 0
}
