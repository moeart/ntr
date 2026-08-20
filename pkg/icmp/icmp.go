package icmp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	xicmp "golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	ProtocolICMP     = 1  // Internet Control Message
	ProtocolIPv6ICMP = 58 // ICMP for IPv6
)

// ICMPReturn contains the info for a returned ICMP.
type ICMPReturn struct {
	Success bool
	Addr    string
	Elapsed time.Duration
}

// SendDiscoverICMP sends an ICMP probe to discover a hop at the given TTL.
func SendDiscoverICMP(localAddr string, dst net.Addr, ttl, id int, timeout time.Duration, seq int) (ICMPReturn, error) {
	return SendICMP(localAddr, dst, "", ttl, id, timeout, seq)
}

// SendDiscoverICMPv6 sends an ICMPv6 probe to discover a hop at the given hop limit.
func SendDiscoverICMPv6(localAddr string, dst net.Addr, ttl, id int, timeout time.Duration, seq int) (ICMPReturn, error) {
	return SendICMPv6(localAddr, dst, "", ttl, id, timeout, seq)
}

// SendICMP sends an IPv4 ICMP echo probe with a specific TTL.
func SendICMP(localAddr string, dst net.Addr, target string, ttl, id int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	if timeout <= 0 {
		return hop, fmt.Errorf("ICMP timeout must be greater than zero")
	}

	start := time.Now()
	conn, err := xicmp.ListenPacket("ip4:icmp", localAddr)
	if err != nil {
		return hop, fmt.Errorf("failed to listen on %q: %w", localAddr, err)
	}
	defer conn.Close()

	if err := conn.IPv4PacketConn().SetTTL(ttl); err != nil {
		return hop, fmt.Errorf("failed to set IPv4 TTL %d: %w", ttl, err)
	}
	deadline := start.Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return hop, fmt.Errorf("failed to set ICMP deadline: %w", err)
	}

	body := probeBody(seq)
	message := xicmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &xicmp.Echo{ID: id, Seq: seq, Data: body},
	}
	wire, err := message.Marshal(nil)
	if err != nil {
		return hop, fmt.Errorf("failed to marshal ICMP request: %w", err)
	}
	if _, err := conn.WriteTo(wire, dst); err != nil {
		return hop, fmt.Errorf("failed to send ICMP request: %w", err)
	}

	peer, _, err := listenForSpecific4(conn, deadline, target, body, id, seq, wire)
	if err != nil {
		return hop, err
	}
	return ICMPReturn{Success: true, Addr: peer, Elapsed: time.Since(start)}, nil
}

// SendICMPv6 sends an IPv6 ICMP echo probe with a specific hop limit.
func SendICMPv6(localAddr string, dst net.Addr, target string, ttl, id int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	if timeout <= 0 {
		return hop, fmt.Errorf("ICMP timeout must be greater than zero")
	}

	start := time.Now()
	conn, err := xicmp.ListenPacket("ip6:ipv6-icmp", localAddr)
	if err != nil {
		return hop, fmt.Errorf("failed to listen on %q: %w", localAddr, err)
	}
	defer conn.Close()

	if err := conn.IPv6PacketConn().SetHopLimit(ttl); err != nil {
		return hop, fmt.Errorf("failed to set IPv6 hop limit %d: %w", ttl, err)
	}
	deadline := start.Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return hop, fmt.Errorf("failed to set ICMP deadline: %w", err)
	}

	body := probeBody(seq)
	message := xicmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &xicmp.Echo{ID: id, Seq: seq, Data: body},
	}
	wire, err := message.Marshal(nil)
	if err != nil {
		return hop, fmt.Errorf("failed to marshal ICMPv6 request: %w", err)
	}
	if _, err := conn.WriteTo(wire, dst); err != nil {
		return hop, fmt.Errorf("failed to send ICMPv6 request: %w", err)
	}

	peer, _, err := listenForSpecific6(conn, deadline, target, body, id, seq, wire)
	if err != nil {
		return hop, err
	}
	return ICMPReturn{Success: true, Addr: peer, Elapsed: time.Since(start)}, nil
}

func probeBody(seq int) []byte {
	body := make([]byte, 5)
	binary.LittleEndian.PutUint32(body[:4], uint32(seq))
	body[4] = 'x'
	return body
}

func listenForSpecific6(conn *xicmp.PacketConn, deadline time.Time, neededPeer string, neededBody []byte, id, needSeq int, sent []byte) (string, []byte, error) {
	_ = sent
	if err := conn.SetReadDeadline(deadline); err != nil {
		return "", nil, err
	}

	for {
		buffer := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(buffer)
		if err != nil {
			return "", nil, err
		}
		if n <= 0 || n > len(buffer) {
			continue
		}
		if neededPeer != "" && peer.String() != neededPeer {
			continue
		}

		message, err := xicmp.ParseMessage(ProtocolIPv6ICMP, buffer[:n])
		if err != nil {
			continue
		}

		if typ, ok := message.Type.(ipv6.ICMPType); ok && typ == ipv6.ICMPTypeTimeExceeded {
			timeExceeded, ok := message.Body.(*xicmp.TimeExceeded)
			if !ok || len(timeExceeded.Data) < 40 {
				continue
			}
			inner, err := xicmp.ParseMessage(ProtocolIPv6ICMP, timeExceeded.Data[40:])
			if err != nil {
				continue
			}
			echo, ok := inner.Body.(*xicmp.Echo)
			if ok && echo.ID == id && echo.Seq == needSeq {
				return peer.String(), nil, nil
			}
			continue
		}

		if typ, ok := message.Type.(ipv6.ICMPType); ok && typ == ipv6.ICMPTypeEchoReply {
			echo, ok := message.Body.(*xicmp.Echo)
			if !ok || echo.ID != id || echo.Seq != needSeq || !bytes.Equal(echo.Data, neededBody) {
				continue
			}
			return peer.String(), echo.Data, nil
		}
	}
}

// listenForSpecific4 listens for a reply from a specific destination with a specific body.
func listenForSpecific4(conn *xicmp.PacketConn, deadline time.Time, neededPeer string, neededBody []byte, pid, needSeq int, sent []byte) (string, []byte, error) {
	if err := conn.SetReadDeadline(deadline); err != nil {
		return "", nil, err
	}

	for {
		buffer := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(buffer)
		if err != nil {
			return "", nil, err
		}
		if n <= 0 || n > len(buffer) {
			continue
		}
		if neededPeer != "" && peer.String() != neededPeer {
			continue
		}

		message, err := xicmp.ParseMessage(ProtocolICMP, buffer[:n])
		if err != nil {
			continue
		}

		if typ, ok := message.Type.(ipv4.ICMPType); ok && typ == ipv4.ICMPTypeTimeExceeded {
			timeExceeded, ok := message.Body.(*xicmp.TimeExceeded)
			if !ok || len(sent) < 4 {
				continue
			}
			index := bytes.Index(timeExceeded.Data, sent[:4])
			if index < 0 {
				continue
			}
			inner, err := xicmp.ParseMessage(ProtocolICMP, timeExceeded.Data[index:])
			if err != nil {
				continue
			}
			echo, ok := inner.Body.(*xicmp.Echo)
			if ok && echo.ID == pid && echo.Seq == needSeq {
				return peer.String(), nil, nil
			}
			continue
		}

		if typ, ok := message.Type.(ipv4.ICMPType); ok && typ == ipv4.ICMPTypeEchoReply {
			echo, ok := message.Body.(*xicmp.Echo)
			if !ok || echo.ID != pid || echo.Seq != needSeq || !bytes.Equal(echo.Data, neededBody) {
				continue
			}
			return peer.String(), echo.Data, nil
		}
	}
}
