package certs

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndVerifyChainForIP(t *testing.T) {
	dir := t.TempDir()

	st, err := Generate(dir, GenerateRequest{
		Hosts: []string{"192.168.4.19", "mail.homelab.local"},
		Days:  365,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !st.HasCA || !st.HasServerCert {
		t.Fatalf("expected CA and server cert, got %+v", st)
	}

	// Verify server cert validates cleanly against the generated CA for IP 192.168.4.19 (same check rustls/MailCraft runs)
	caPEM, err := ReadCACert(dir)
	if err != nil {
		t.Fatalf("ReadCACert: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatalf("failed to append CA PEM")
	}

	srvPEM, err := os.ReadFile(filepath.Join(dir, ProxyCertFile))
	if err != nil {
		t.Fatalf("read server cert: %v", err)
	}
	block, _ := pem.Decode(srvPEM)
	srvCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse server cert: %v", err)
	}

	if _, err := srvCert.Verify(x509.VerifyOptions{
		Roots:     roots,
		DNSName:   "192.168.4.19",
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("certificate chain failed verification for IP 192.168.4.19: %v", err)
	}

	// Test ReuseCA preserves CA fingerprint
	origFP := st.CAFingerprint
	st2, err := Generate(dir, GenerateRequest{
		Hosts:   []string{"192.168.4.20"},
		ReuseCA: true,
	})
	if err != nil {
		t.Fatalf("second Generate with ReuseCA failed: %v", err)
	}
	if st2.CAFingerprint != origFP {
		t.Fatalf("expected ReuseCA to preserve CA fingerprint: %s vs %s", origFP, st2.CAFingerprint)
	}
}
