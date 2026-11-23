package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CACertFile    = "ca-cert.pem"
	CAKeyFile     = "ca-key.pem"
	ProxyCertFile = "proxy-cert.pem"
	ProxyKeyFile  = "proxy-key.pem"
)

// CertStatus describes the current state of certificates inside /config.
type CertStatus struct {
	HasCA              bool     `json:"has_ca"`
	HasServerCert      bool     `json:"has_server_cert"`
	CASubject          string   `json:"ca_subject,omitempty"`
	CANotAfter         string   `json:"ca_not_after,omitempty"`
	CAFingerprint      string   `json:"ca_fingerprint,omitempty"`
	ServerSubject      string   `json:"server_subject,omitempty"`
	ServerNotAfter     string   `json:"server_not_after,omitempty"`
	ServerFingerprint  string   `json:"server_fingerprint,omitempty"`
	ServerIPs          []string `json:"server_ips,omitempty"`
	ServerDNS          []string `json:"server_dns,omitempty"`
	ContainerCertPath  string   `json:"container_cert_path"`
	ContainerKeyPath   string   `json:"container_key_path"`
}

// GenerateRequest configures local CA + server certificate creation.
type GenerateRequest struct {
	Hosts      []string `json:"hosts"`       // IPs or DNS names (e.g. server IP or mail.local)
	CommonName string   `json:"common_name"` // Defaults to first host
	Days       int      `json:"days"`        // Defaults to 3650
	ReuseCA    bool     `json:"reuse_ca"`    // Keep existing CA if present so clients don't need to reinstall CA
}

// Inspect reads existing CA and server certificates from configDir.
func Inspect(configDir string) CertStatus {
	st := CertStatus{
		ContainerCertPath: "/config/" + ProxyCertFile,
		ContainerKeyPath:  "/config/" + ProxyKeyFile,
	}

	if caCert, err := parseCertFile(filepath.Join(configDir, CACertFile)); err == nil {
		st.HasCA = true
		st.CASubject = caCert.Subject.CommonName
		st.CANotAfter = caCert.NotAfter.UTC().Format(time.RFC3339)
		st.CAFingerprint = formatSHA256(caCert.Raw)
	}

	if srvCert, err := parseCertFile(filepath.Join(configDir, ProxyCertFile)); err == nil {
		if _, keyErr := os.Stat(filepath.Join(configDir, ProxyKeyFile)); keyErr == nil {
			st.HasServerCert = true
		}
		st.ServerSubject = srvCert.Subject.CommonName
		st.ServerNotAfter = srvCert.NotAfter.UTC().Format(time.RFC3339)
		st.ServerFingerprint = formatSHA256(srvCert.Raw)
		for _, ip := range srvCert.IPAddresses {
			st.ServerIPs = append(st.ServerIPs, ip.String())
		}
		st.ServerDNS = append(st.ServerDNS, srvCert.DNSNames...)
	}

	return st
}

// Generate creates (or reuses) a local Root CA and signs a server certificate with SANs compatible with rustls/MailCraft.
func Generate(configDir string, req GenerateRequest) (CertStatus, error) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return CertStatus{}, fmt.Errorf("create config dir: %w", err)
	}

	days := req.Days
	if days <= 0 {
		days = 3650
	}
	if days > 36500 {
		days = 36500
	}

	ipSet := make(map[string]net.IP)
	dnsSet := make(map[string]bool)

	// Always include loopback and container hostname defaults
	ipSet["127.0.0.1"] = net.ParseIP("127.0.0.1")
	dnsSet["localhost"] = true
	dnsSet["email-oauth2-proxy"] = true

	var firstHost string
	for _, raw := range req.Hosts {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\n' || r == ';'
		}) {
			h := strings.TrimSpace(part)
			if h == "" {
				continue
			}
			if firstHost == "" {
				firstHost = h
			}
			if ip := net.ParseIP(h); ip != nil {
				ipSet[ip.String()] = ip
			} else {
				dnsSet[h] = true
			}
		}
	}

	cn := strings.TrimSpace(req.CommonName)
	if cn == "" {
		if firstHost != "" {
			cn = firstHost
		} else {
			cn = "email-oauth2-proxy"
		}
	}

	caCertPath := filepath.Join(configDir, CACertFile)
	caKeyPath := filepath.Join(configDir, CAKeyFile)

	var caCert *x509.Certificate
	var caKey *rsa.PrivateKey

	if req.ReuseCA {
		c, err1 := parseCertFile(caCertPath)
		k, err2 := parseRSAKeyFile(caKeyPath)
		if err1 == nil && err2 == nil {
			caCert = c
			caKey = k
		}
	}

	now := time.Now().UTC()
	if caCert == nil || caKey == nil {
		var err error
		caKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return CertStatus{}, fmt.Errorf("generate CA key: %w", err)
		}

		caSerial, err := randomSerial()
		if err != nil {
			return CertStatus{}, err
		}

		caTmpl := &x509.Certificate{
			SerialNumber: caSerial,
			Subject: pkix.Name{
				CommonName:   "Email OAuth2 Proxy Local CA",
				Organization: []string{"Email OAuth2 Proxy"},
			},
			NotBefore:             now.Add(-5 * time.Minute),
			NotAfter:              now.AddDate(0, 0, days),
			KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
			BasicConstraintsValid: true,
			IsCA:                  true,
			MaxPathLenZero:        true,
		}

		caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
		if err != nil {
			return CertStatus{}, fmt.Errorf("create CA cert: %w", err)
		}
		caCert, err = x509.ParseCertificate(caDER)
		if err != nil {
			return CertStatus{}, fmt.Errorf("parse CA cert: %w", err)
		}

		if err := writePEM(caCertPath, "CERTIFICATE", caDER, 0o644); err != nil {
			return CertStatus{}, err
		}
		if err := writePEM(caKeyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caKey), 0o600); err != nil {
			return CertStatus{}, err
		}
	}

	// Generate leaf server certificate signed by the local CA
	srvKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return CertStatus{}, fmt.Errorf("generate server key: %w", err)
	}

	srvSerial, err := randomSerial()
	if err != nil {
		return CertStatus{}, err
	}

	ips := make([]net.IP, 0, len(ipSet))
	for _, ip := range ipSet {
		ips = append(ips, ip)
	}
	dnsNames := make([]string, 0, len(dnsSet))
	for d := range dnsSet {
		dnsNames = append(dnsNames, d)
	}

	srvTmpl := &x509.Certificate{
		SerialNumber: srvSerial,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"Email OAuth2 Proxy"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(0, 0, days),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		IPAddresses:           ips,
		DNSNames:              dnsNames,
	}

	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		return CertStatus{}, fmt.Errorf("sign server cert: %w", err)
	}

	if err := writePEM(filepath.Join(configDir, ProxyCertFile), "CERTIFICATE", srvDER, 0o644); err != nil {
		return CertStatus{}, err
	}
	if err := writePEM(filepath.Join(configDir, ProxyKeyFile), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(srvKey), 0o600); err != nil {
		return CertStatus{}, err
	}

	return Inspect(configDir), nil
}

// ReadCACert returns the raw PEM bytes of ca-cert.pem.
func ReadCACert(configDir string) ([]byte, error) {
	return os.ReadFile(filepath.Join(configDir, CACertFile))
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}
	return n, nil
}

func writePEM(path, blockType string, der []byte, perm os.FileMode) error {
	data := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	return os.WriteFile(path, data, perm)
}

func parseCertFile(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM in %s", path)
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseRSAKeyFile(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid key PEM in %s", path)
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	anyKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := anyKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key in %s is not RSA", path)
	}
	return rsaKey, nil
}

func formatSHA256(raw []byte) string {
	sum := sha256.Sum256(raw)
	hexStr := strings.ToUpper(hex.EncodeToString(sum[:]))
	parts := make([]string, 0, len(hexStr)/2)
	for i := 0; i < len(hexStr); i += 2 {
		parts = append(parts, hexStr[i:i+2])
	}
	return strings.Join(parts, ":")
}
