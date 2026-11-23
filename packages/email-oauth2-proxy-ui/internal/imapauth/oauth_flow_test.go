package imapauth

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"email-oauth2-proxy-ui/internal/certs"
)

func TestOAuthFlowEndToEndWithMockTLSProxy(t *testing.T) {
	dir := t.TempDir()

	// Generate real TLS certs for 127.0.0.1
	if _, err := certs.Generate(dir, certs.GenerateRequest{Hosts: []string{"127.0.0.1"}}); err != nil {
		t.Fatalf("generate certs: %v", err)
	}
	tlsCert, err := tls.LoadX509KeyPair(
		filepath.Join(dir, certs.ProxyCertFile),
		filepath.Join(dir, certs.ProxyKeyFile),
	)
	if err != nil {
		t.Fatalf("load keypair: %v", err)
	}

	callbackReceived := make(chan string, 1)

	// Mock local HTTP callback server (mimics email-oauth2-proxy --local-server-auth on port 80)
	cbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackReceived <- r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Email OAuth 2.0 Proxy successfully authenticated account test@outlook.com.</body></html>"))
	}))
	defer cbSrv.Close()
	cbURL, _ := url.Parse(cbSrv.URL)
	cbPort, _ := strconv.Atoi(cbURL.Port())

	logPath := filepath.Join(dir, "emailproxy.log")
	if err := os.WriteFile(logPath, []byte("2026-11-19 12:00:00,000: Initialised Email OAuth 2.0 Proxy\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mock TLS IMAP server (mimics email-oauth2-proxy IMAP listener)
	imapListener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{tlsCert}})
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}
	defer imapListener.Close()
	imapPort := imapListener.Addr().(*net.TCPAddr).Port

	go func() {
		conn, err := imapListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = fmt.Fprint(conn, "* OK Mock IMAP Proxy Ready\r\n")
		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if !strings.HasPrefix(line, "A001 AUTHENTICATE PLAIN ") {
			_, _ = fmt.Fprint(conn, "A001 BAD unexpected command\r\n")
			return
		}

		// Append OAuth prompt URL to emailproxy.log just like emailproxy.py does
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
		_, _ = fmt.Fprintln(f, "2026-11-19 12:00:01,000: Please visit the following URL to authenticate account test@outlook.com: https://login.microsoftonline.com/common/oauth2/v2.0/authorize?client_id=9e5f94bc&state=xyz")
		_ = f.Close()

		// Wait for callback forwarded by our UI backend
		select {
		case q := <-callbackReceived:
			if strings.Contains(q, "code=MOCK_AUTH_CODE") {
				_, _ = fmt.Fprint(conn, "A001 OK AUTHENTICATE completed.\r\n")
			} else {
				_, _ = fmt.Fprint(conn, "A001 NO missing code\r\n")
			}
		case <-time.After(5 * time.Second):
			_, _ = fmt.Fprint(conn, "A001 NO timeout\r\n")
		}
	}()

	// Write emailproxy.config pointing at mock IMAP + POP + SMTP ports and mock callback port
	cfgPath := filepath.Join(dir, "emailproxy.config")
	cfgContent := fmt.Sprintf(`[IMAP-%d]
server_address = outlook.office365.com
server_port = 993
local_address = 0.0.0.0
local_certificate_path = /config/proxy-cert.pem
local_key_path = /config/proxy-key.pem

[POP-1995]
server_address = outlook.office365.com
server_port = 995
local_address = 0.0.0.0
local_certificate_path = /config/proxy-cert.pem
local_key_path = /config/proxy-key.pem

[SMTP-1587]
server_address = smtp-mail.outlook.com
server_port = 587
server_starttls = True
local_address = 0.0.0.0
local_certificate_path = /config/proxy-cert.pem
local_key_path = /config/proxy-key.pem

[@outlook.com]
permission_url = https://login.microsoftonline.com/common/oauth2/v2.0/authorize
token_url = https://login.microsoftonline.com/common/oauth2/v2.0/token
oauth2_scope = https://outlook.office.com/IMAP.AccessAsUser.All offline_access
redirect_uri = https://localhost
redirect_listen_address = http://127.0.0.1:%d
client_id = 9e5f94bc-e8a4-4e73-b8be-63364c29d753
`, imapPort, cbPort)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager("127.0.0.1", cfgPath, logPath)
	sess, err := mgr.Start(StartRequest{
		Email:    "test@outlook.com",
		Password: "MyLocalPassword!@#",
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if len(sess.MatchedServers) != 3 {
		t.Fatalf("expected 3 matched servers (IMAP, POP, SMTP), got %d: %+v", len(sess.MatchedServers), sess.MatchedServers)
	}

	// Poll until auth_url is captured from log
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		cur, _ := mgr.Get(sess.ID)
		if cur.AuthURL != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	cur, _ := mgr.Get(sess.ID)
	if !strings.HasPrefix(cur.AuthURL, "https://login.microsoftonline.com/") {
		t.Fatalf("expected captured auth_url, got: %+v", cur)
	}

	// Test automatic callback routing by OAuth state (same path GET /oauth/callback uses)
	if _, _, err := mgr.SubmitCallbackAuto("code=MOCK_AUTH_CODE&state=xyz"); err != nil {
		t.Fatalf("SubmitCallbackAuto failed: %v", err)
	}

	// Wait for IMAP login completion
	deadline = time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		cur, _ = mgr.Get(sess.ID)
		if cur.Status == "success" || cur.Status == "error" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if cur.Status != "success" {
		t.Fatalf("expected status=success, got: %+v", cur)
	}
}

func TestPassiveMonitorDetectsExternalClientAuth(t *testing.T) {
	dir := t.TempDir()
	cbGot := make(chan string, 1)
	cbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cbGot <- r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer cbSrv.Close()
	cbURL, _ := url.Parse(cbSrv.URL)
	cbPort, _ := strconv.Atoi(cbURL.Port())

	logPath := filepath.Join(dir, "emailproxy.log")
	if err := os.WriteFile(logPath, []byte("2026-11-23 12:00:00,000: Initialised Email OAuth 2.0 Proxy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "emailproxy.config")
	cfg := fmt.Sprintf("[@gmail.com]\ntoken_url = https://oauth2.googleapis.com/token\noauth2_scope = https://mail.google.com/\nclient_id = cid\nredirect_uri = http://localhost:1999\nredirect_listen_address = http://127.0.0.1:%d\n", cbPort)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager("127.0.0.1", cfgPath, logPath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.StartMonitor(ctx)
	time.Sleep(150 * time.Millisecond)

	// Simulate an external client triggering an OAuth prompt in emailproxy.log
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprintln(f, "2026-11-23 12:00:02,000: Please visit the following URL to authenticate account ext@gmail.com: https://accounts.google.com/o/oauth2/auth?client_id=cid&state=extstate99")
	_ = f.Close()

	var pending []PendingAuth
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		pending = mgr.ListPending()
		if len(pending) == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(pending) != 1 || pending[0].Email != "ext@gmail.com" {
		t.Fatalf("expected 1 pending external auth for ext@gmail.com, got %+v", pending)
	}

	// Verify automatic callback routing resolves passive pending item by state
	if _, _, err := mgr.SubmitCallbackAuto("code=EXT_CODE&state=extstate99"); err != nil {
		t.Fatalf("SubmitCallbackAuto for pending failed: %v", err)
	}
	select {
	case q := <-cbGot:
		if !strings.Contains(q, "code=EXT_CODE") {
			t.Fatalf("unexpected callback query: %s", q)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for callback")
	}
	if len(mgr.ListPending()) != 0 {
		t.Fatalf("expected pending list cleared after callback")
	}
}
