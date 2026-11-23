package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseINIAndMultiLineValues(t *testing.T) {
	sample := `
# Comment line
[Email OAuth 2.0 Proxy configuration file]
documentation = First line
	second line indented
	third line

[IMAP-1993]
server_address = outlook.office365.com
server_port = 993
local_address = 0.0.0.0
local_certificate_path = /config/proxy-cert.pem
local_key_path = /config/proxy-key.pem

[@outlook.com]
permission_url = https://login.microsoftonline.com/common/oauth2/v2.0/authorize
token_url = https://login.microsoftonline.com/common/oauth2/v2.0/token
oauth2_scope = https://outlook.office.com/IMAP.AccessAsUser.All offline_access
redirect_uri = https://localhost
redirect_listen_address = http://0.0.0.0:80
client_id = 9e5f94bc-e8a4-4e73-b8be-63364c29d753

[emailproxy]
delete_account_token_on_password_error = False
allow_catch_all_accounts = True
`

	ini := ParseINI(sample)

	doc := ini.GetSection("Email OAuth 2.0 Proxy configuration file")
	if doc == nil {
		t.Fatalf("expected documentation section")
	}
	if !strings.Contains(doc.Get("documentation", ""), "second line indented") {
		t.Fatalf("expected multiline continuation, got: %q", doc.Get("documentation", ""))
	}

	servers := ExtractServers(ini)
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Protocol != "IMAP" || servers[0].LocalPort != 1993 || !servers[0].LocalTLS {
		t.Fatalf("unexpected server entry: %+v", servers[0])
	}

	cacheINI := ParseINI(`
[user@outlook.com]
access_token = encrypted-access
refresh_token = encrypted-refresh
access_token_expiry = 1893456000
last_activity = 1795000000
`)

	accounts := ExtractAccounts(ini, cacheINI, true)
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts (1 catch-all + 1 cached), got %d", len(accounts))
	}
	if !accounts[0].IsCatchAll || accounts[0].TokenStatus != "catch_all" {
		t.Fatalf("unexpected catch-all account: %+v", accounts[0])
	}
	if accounts[1].Account != "user@outlook.com" || accounts[1].TokenStatus != "authenticated" || !accounts[1].FromCacheOnly {
		t.Fatalf("unexpected cached account: %+v", accounts[1])
	}

	globals := ExtractGlobals(ini)
	if globals.DeleteAccountTokenOnPasswordError || !globals.AllowCatchAllAccounts {
		t.Fatalf("unexpected globals: %+v", globals)
	}

	// Round-trip render
	rendered := ini.Render()
	reparsed := ParseINI(rendered)
	if reparsed.GetSection("@outlook.com").Get("client_id", "") != "9e5f94bc-e8a4-4e73-b8be-63364c29d753" {
		t.Fatalf("round-trip lost client_id")
	}
}

func TestWriteFileWithBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "emailproxy.config")

	if err := WriteFileWithBackup(path, "[emailproxy]\nallow_catch_all_accounts = False\n"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteFileWithBackup(path, "[emailproxy]\nallow_catch_all_accounts = True\n"); err != nil {
		t.Fatalf("second write: %v", err)
	}

	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
	if !strings.Contains(string(bak), "False") {
		t.Fatalf("backup did not contain previous contents: %s", string(bak))
	}
}
