package imapauth

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"email-oauth2-proxy-ui/internal/config"
)

// PortProbeResult holds live connectivity status for a configured proxy server port.
type PortProbeResult struct {
	Section       string `json:"section"`
	Protocol      string `json:"protocol"`
	LocalPort     int    `json:"local_port"`
	ServerAddress string `json:"server_address"`
	ServerPort    int    `json:"server_port"`
	LocalTLS      bool   `json:"local_tls"`
	LocalStartTLS bool   `json:"local_starttls"`
	Online        bool   `json:"online"`
	LatencyMs     int64  `json:"latency_ms"`
	Banner        string `json:"banner,omitempty"`
	Error         string `json:"error,omitempty"`
}

// ProbeServers checks all configured servers concurrently against proxyHost.
func ProbeServers(proxyHost string, servers []config.ServerEntry) []PortProbeResult {
	results := make([]PortProbeResult, len(servers))
	var wg sync.WaitGroup

	for i, srv := range servers {
		wg.Add(1)
		go func(idx int, s config.ServerEntry) {
			defer wg.Done()
			results[idx] = probeOne(proxyHost, s)
		}(i, srv)
	}
	wg.Wait()
	return results
}

func probeOne(proxyHost string, s config.ServerEntry) PortProbeResult {
	res := PortProbeResult{
		Section:       s.Section,
		Protocol:      s.Protocol,
		LocalPort:     s.LocalPort,
		ServerAddress: s.ServerAddress,
		ServerPort:    s.ServerPort,
		LocalTLS:      s.LocalTLS,
		LocalStartTLS: s.LocalStartTLS,
	}

	addr := net.JoinHostPort(proxyHost, fmt.Sprintf("%d", s.LocalPort))
	start := time.Now()

	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: 2 * time.Second}

	if s.LocalTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true, // local sidecar check
		})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.Online = false
		res.Error = err.Error()
		return res
	}
	defer conn.Close()

	res.Online = true
	_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	reader := bufio.NewReader(conn)
	line, readErr := reader.ReadString('\n')
	if readErr == nil {
		res.Banner = strings.TrimSpace(line)
	}
	return res
}
