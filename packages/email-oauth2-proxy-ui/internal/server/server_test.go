package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerAuthTemplatesAndCerts(t *testing.T) {
	dir := t.TempDir()
	composeDir := t.TempDir()
	composePath := filepath.Join(composeDir, "docker-compose.yml")
	initialCompose := `services:
  email-oauth2-proxy:
    container_name: email-oauth2-proxy
    ports:
      - "${APP_PORT}:80"
    environment:
      - TZ=America/Sao_Paulo
`
	if err := os.WriteFile(composePath, []byte(initialCompose), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(Options{
		ConfigDir:     dir,
		ProxyHost:     "127.0.0.1",
		ComposePath:   composeDir,
		AdminUser:     "admin",
		AdminPassword: "secret-password",
	})

	// 1. Unauthenticated API call should return 401
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/overview", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without login, got %d", rec.Code)
	}

	// 2. Login and obtain session cookie
	loginBody := `{"username":"admin","password":"secret-password"}`
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie")
	}
	authCookie := cookies[0]

	doReq := func(method, path string, payload any) *httptest.ResponseRecorder {
		var body bytes.Buffer
		if payload != nil {
			_ = json.NewEncoder(&body).Encode(payload)
		}
		req := httptest.NewRequest(method, path, &body)
		req.AddCookie(authCookie)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		return rr
	}

	// 3. Generate TLS cert for 192.168.4.19
	rec = doReq(http.MethodPost, "/api/certs/generate", map[string]any{
		"hosts":                []string{"192.168.4.19"},
		"days":                 365,
		"apply_to_all_servers": false,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("generate certs failed: %d %s", rec.Code, rec.Body.String())
	}

	// 4. Download ca-cert.crt
	rec = doReq(http.MethodGet, "/api/certs/ca.crt", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "BEGIN CERTIFICATE") {
		t.Fatalf("download CA failed: %d", rec.Code)
	}

	// 5. Apply Thunderbird preset for Outlook + Gmail + Yahoo with TLS enabled
	for _, item := range []struct {
		provider string
		account  string
	}{
		{"outlook", "@outlook.com"},
		{"gmail", "@gmail.com"},
		{"yahoo", "@yahoo.com"},
	} {
		rec = doReq(http.MethodPost, "/api/templates/apply", map[string]any{
			"provider_id":      item.provider,
			"preset_id":        "thunderbird",
			"account_target":   item.account,
			"create_servers":   true,
			"enable_tls":       true,
			"enable_catch_all": true,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("apply template %s failed: %d %s", item.provider, rec.Code, rec.Body.String())
		}
	}

	// 6. Verify overview reports 6 servers (all with local TLS) + 3 catch-all accounts
	rec = doReq(http.MethodGet, "/api/overview", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview failed: %d %s", rec.Code, rec.Body.String())
	}
	var ov struct {
		Servers []struct {
			Section  string `json:"section"`
			LocalTLS bool   `json:"local_tls"`
		} `json:"servers"`
		Accounts []struct {
			Account        string `json:"account"`
			IsCatchAll     bool   `json:"is_catch_all"`
			MatchedServers []struct {
				Section string `json:"section"`
			} `json:"matched_servers"`
		} `json:"accounts"`
		Globals struct {
			AllowCatchAll bool `json:"allow_catch_all_accounts"`
		} `json:"globals"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ov); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if len(ov.Servers) != 6 {
		t.Fatalf("expected 6 servers, got %d", len(ov.Servers))
	}
	for _, s := range ov.Servers {
		if !s.LocalTLS {
			t.Fatalf("expected LocalTLS=true on server %s", s.Section)
		}
	}
	if len(ov.Accounts) != 3 || !ov.Globals.AllowCatchAll {
		t.Fatalf("unexpected accounts/globals: %+v", ov)
	}
	for _, a := range ov.Accounts {
		if len(a.MatchedServers) != 2 {
			t.Fatalf("expected 2 matched servers (IMAP+SMTP) for %s, got %+v", a.Account, a.MatchedServers)
		}
	}

	// 7. Verify /oauth/callback returns HTML page when no session is active
	cbRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(cbRec, httptest.NewRequest(http.MethodGet, "/oauth/callback?code=abc", nil))
	if cbRec.Code != http.StatusBadRequest || !strings.Contains(cbRec.Body.String(), "eop_oauth_callback") {
		t.Fatalf("unexpected /oauth/callback response: %d %s", cbRec.Code, cbRec.Body.String())
	}

	// 8. Verify clearing tokens on a catch-all account keeps the account listed as "pending"
	credPath := filepath.Join(dir, "credstore.config")
	credData := "[teste@outlook.com]\naccess_token = enc1\nrefresh_token = enc2\naccess_token_expiry = 1893456000\n"
	if err := os.WriteFile(credPath, []byte(credData), 0o644); err != nil {
		t.Fatal(err)
	}
	rec = doReq(http.MethodDelete, "/api/tokens/teste@outlook.com", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete token failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doReq(http.MethodGet, "/api/overview", nil)
	var ov2 struct {
		Accounts []struct {
			Account         string `json:"account"`
			TokenStatus     string `json:"token_status"`
			HasRefreshToken bool   `json:"has_refresh_token"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ov2); err != nil {
		t.Fatal(err)
	}
	foundPending := false
	for _, a := range ov2.Accounts {
		if a.Account == "teste@outlook.com" {
			foundPending = true
			if a.TokenStatus != "pending" || a.HasRefreshToken {
				t.Fatalf("expected teste@outlook.com to remain pending with no refresh token, got %+v", a)
			}
		}
	}
	if !foundPending {
		t.Fatalf("expected catch-all account teste@outlook.com to remain visible after clearing token")
	}

	// 9. Verify advanced account flows (O365 CCG + JWT cert, Google Service Account JSON) & debug detection
	rec = doReq(http.MethodPost, "/api/accounts", map[string]any{
		"account":              "corp@empresa.com",
		"oauth2_flow":          "client_credentials",
		"permission_url":       "https://should-be-stripped",
		"token_url":            "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
		"oauth2_scope":         "https://outlook.office365.com/.default",
		"client_id":            "ccg-client-id",
		"jwt_certificate_path": "/config/jwt-cert.pem",
		"jwt_key_path":         "/config/jwt-key.pem",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert CCG account failed: %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(http.MethodPost, "/api/accounts", map[string]any{
		"account":       "svc@workspace.org",
		"oauth2_flow":   "service_account",
		"token_url":     "https://oauth2.googleapis.com/token",
		"oauth2_scope":  "https://mail.google.com/",
		"client_id":     "key",
		"client_secret": "{\n  \"type\": \"service_account\",\n  \"client_email\": \"sa@proj.iam.gserviceaccount.com\"\n}",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert service_account failed: %d %s", rec.Code, rec.Body.String())
	}

	// Write sample log with debug mode toggled to False and check overview reflects it
	logPath := filepath.Join(dir, "emailproxy.log")
	_ = os.WriteFile(logPath, []byte("2026-11-23 12:00:00: Initialising Email OAuth 2.0 Proxy in debug mode\n2026-11-23 12:01:00: Setting debug mode: False\n"), 0o644)

	rec = doReq(http.MethodGet, "/api/overview", nil)
	var ov3 struct {
		DebugMode  bool `json:"debug_mode"`
		DebugKnown bool `json:"debug_known"`
		Accounts   []struct {
			Account            string `json:"account"`
			OAuth2Flow         string `json:"oauth2_flow"`
			PermissionURL      string `json:"permission_url"`
			JWTCertificatePath string `json:"jwt_certificate_path"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ov3); err != nil {
		t.Fatal(err)
	}
	if !ov3.DebugKnown || ov3.DebugMode {
		t.Fatalf("expected debug_known=true debug_mode=false, got known=%v mode=%v", ov3.DebugKnown, ov3.DebugMode)
	}
	foundCCG := false
	for _, a := range ov3.Accounts {
		if a.Account == "corp@empresa.com" {
			foundCCG = true
			if a.OAuth2Flow != "client_credentials" || a.PermissionURL != "" || a.JWTCertificatePath != "/config/jwt-cert.pem" {
				t.Fatalf("unexpected CCG account saved: %+v", a)
			}
		}
	}
	if !foundCCG {
		t.Fatalf("expected corp@empresa.com in overview")
	}

	// 10. Verify port check + Mode A (compose_only) sync via /api/ports/apply and /api/templates/apply
	rec = doReq(http.MethodPost, "/api/ports/check", map[string]any{
		"ports": []int{1993, 4993},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("ports check failed: %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(http.MethodPost, "/api/ports/apply", map[string]any{
		"mode": "compose_only",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("ports apply compose_only failed: %d %s", rec.Code, rec.Body.String())
	}

	// Apply AOL template with port_mode="compose_only" and confirm 4993 & 4465 appear in compose file while :80 stays preserved
	rec = doReq(http.MethodPost, "/api/templates/apply", map[string]any{
		"provider_id":      "aol",
		"preset_id":        "thunderbird",
		"account_target":   "@aol.com",
		"create_servers":   true,
		"enable_tls":       true,
		"enable_catch_all": true,
		"port_mode":        "compose_only",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("apply AOL template with compose_only failed: %d %s", rec.Code, rec.Body.String())
	}

	composeBytes, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	composeStr := string(composeBytes)
	for _, want := range []string{`${APP_PORT}:80`, `"1993:1993"`, `"4993:4993"`, `"4465:4465"`} {
		if !strings.Contains(composeStr, want) {
			t.Fatalf("expected %s in updated compose file, got:\n%s", want, composeStr)
		}
	}
}
