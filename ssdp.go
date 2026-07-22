package main

import (
	"net"
	"strings"
	"time"
)

func discoverRokus(timeout time.Duration) ([]string, error) {
	ssdp, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	msg := []byte("M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"ST: roku:ecp\r\n" +
		"MX: 3\r\n\r\n")
	if _, err := conn.WriteToUDP(msg, ssdp); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	_ = conn.SetReadDeadline(deadline)

	seen := map[string]bool{}
	var locs []string
	buf := make([]byte, 2048)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			if len(locs) > 0 {
				break
			}
			return nil, err
		}
		loc := parseSSDPLocation(string(buf[:n]))
		if loc == "" || seen[loc] {
			continue
		}
		seen[loc] = true
		locs = append(locs, loc)
	}
	return locs, nil
}

func parseSSDPLocation(resp string) string {
	for _, line := range strings.Split(resp, "\r\n") {
		if len(line) >= 9 && strings.EqualFold(line[:9], "LOCATION:") {
			return strings.TrimSpace(line[9:])
		}
	}
	return ""
}
