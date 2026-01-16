package asn

import (
	"math/big"
	"net"
)

// Ip 继承自 net.IP 并添加了比较方法
type Ip struct {
	net.IP
}

// ParseIP 解析 IP 地址字符串为 Ip 类型
func ParseIP(ipStr string) *Ip {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil
	}
	return &Ip{IP: ip}
}

// ToIPv6 将 IP 转换为 IPv6 表示形式
func (i Ip) ToIPv6() net.IP {
	if i.To4() != nil {
		// IPv4 地址转换为 IPv4-mapped IPv6 地址
		ipv6 := make(net.IP, net.IPv6len)
		copy(ipv6[12:], i.To4())
		ipv6[10] = 0xff
		ipv6[11] = 0xff
		return ipv6
	}
	return i.IP
}

// CompareTo 比较两个 IP 地址
func (i Ip) CompareTo(other Ip) int {
	// 首先将两个 IP 都转换为 IPv6 表示形式
	ip1 := i.ToIPv6()
	ip2 := other.ToIPv6()

	// 使用 big.Int 进行比较
	bigIp1 := &big.Int{}
	bigIp1.SetBytes(ip1)

	bigIp2 := &big.Int{}
	bigIp2.SetBytes(ip2)

	return bigIp1.Cmp(bigIp2)
}

// LessThan 检查当前 IP 是否小于另一个 IP
func (i Ip) LessThan(other Ip) bool {
	return i.CompareTo(other) < 0
}

// GreaterThan 检查当前 IP 是否大于另一个 IP
func (i Ip) GreaterThan(other Ip) bool {
	return i.CompareTo(other) > 0
}

// LessThanOrEqual 检查当前 IP 是否小于或等于另一个 IP
func (i Ip) LessThanOrEqual(other Ip) bool {
	return i.CompareTo(other) <= 0
}

// GreaterThanOrEqual 检查当前 IP 是否大于或等于另一个 IP
func (i Ip) GreaterThanOrEqual(other Ip) bool {
	return i.CompareTo(other) >= 0
}
