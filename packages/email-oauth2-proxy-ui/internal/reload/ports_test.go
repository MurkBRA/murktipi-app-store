package reload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateComposePortsYAMLPreservesCommentsAndProtectedPort(t *testing.T) {
	input := `# Override Runtipi
services:
  email-oauth2-proxy:
    container_name: email-oauth2-proxy
    ports:
      - "${APP_PORT}:80" # Keep Runtipi callback port
      - "1993:1993" # Outlook IMAP
      - "1587:1587" # Outlook SMTP
    environment:
      - TZ=${TZ:-America/Sao_Paulo}

  email-oauth2-proxy-ui:
    image: golang:1.24-alpine
    ports:
      - "8090:8090"
`

	specs := []PortSpec{
		{Port: 1993, Section: "IMAP-1993", ServerAddress: "outlook.office365.com"},
		{Port: 1587, Section: "SMTP-1587", ServerAddress: "smtp-mail.outlook.com"},
		{Port: 4993, Section: "IMAP-4993", ServerAddress: "imap.aol.com"},
	}

	out, err := UpdateComposePortsYAML(input, "email-oauth2-proxy", specs)
	if err != nil {
		t.Fatalf("UpdateComposePortsYAML failed: %v", err)
	}

	if !strings.Contains(out, `- "${APP_PORT}:80" # Keep Runtipi callback port`) {
		t.Fatalf("expected protected port 80 line preserved, got:\n%s", out)
	}
	if !strings.Contains(out, `- "4993:4993" # IMAP-4993 (imap.aol.com)`) {
		t.Fatalf("expected new port 4993 added, got:\n%s", out)
	}
	if !strings.Contains(out, `- TZ=${TZ:-America/Sao_Paulo}`) {
		t.Fatalf("expected env var preserved, got:\n%s", out)
	}
	if !strings.Contains(out, `- "8090:8090"`) {
		t.Fatalf("expected UI service ports untouched, got:\n%s", out)
	}
}

func TestUpdateComposePortsYAMLInsertsMissingPortsBlock(t *testing.T) {
	input := `services:
  email-oauth2-proxy:
    container_name: email-oauth2-proxy
    environment:
      - TZ=America/Sao_Paulo

  email-oauth2-proxy-ui:
    image: golang:1.24-alpine
`

	out, err := UpdateComposePortsYAML(input, "email-oauth2-proxy", []PortSpec{
		{Port: 2993, Section: "IMAP-2993", ServerAddress: "imap.gmail.com"},
	})
	if err != nil {
		t.Fatalf("UpdateComposePortsYAML failed: %v", err)
	}

	if !strings.Contains(out, "    ports:\n      - \"2993:2993\" # IMAP-2993 (imap.gmail.com)") {
		t.Fatalf("expected inserted ports block, got:\n%s", out)
	}
	if !strings.Contains(out, "  email-oauth2-proxy-ui:") {
		t.Fatalf("expected second service preserved, got:\n%s", out)
	}
}

func TestResolveComposeFileFromDirectoryAndWriteBackup(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	orig := "services:\n  email-oauth2-proxy:\n    container_name: email-oauth2-proxy\n"
	if err := os.WriteFile(composeFile, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, writable := ResolveComposeFile(dir)
	if resolved != composeFile || !writable {
		t.Fatalf("expected %s writable=true, got %q writable=%v", composeFile, resolved, writable)
	}

	updated, err := UpdateComposePortsYAML(orig, "email-oauth2-proxy", []PortSpec{{Port: 1993, Section: "IMAP-1993"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteComposeFile(resolved, updated); err != nil {
		t.Fatal(err)
	}

	bak, err := os.ReadFile(composeFile + ".bak")
	if err != nil || string(bak) != orig {
		t.Fatalf("expected .bak to match original content")
	}
}
