package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Section represents an INI section in insertion order.
type Section struct {
	Name   string            `json:"name"`
	Keys   []string          `json:"keys"`
	Values map[string]string `json:"values"`
}

// INIFile represents a parsed Python configparser-compatible INI file.
type INIFile struct {
	Sections []*Section
	byName   map[string]*Section
}

// NewINIFile creates an empty INIFile.
func NewINIFile() *INIFile {
	return &INIFile{
		byName: make(map[string]*Section),
	}
}

// ParseINI parses Python configparser-compatible INI text (supporting multi-line indented values).
func ParseINI(content string) *INIFile {
	doc := NewINIFile()
	scanner := bufio.NewScanner(strings.NewReader(content))
	// Allow long lines (e.g. encrypted tokens or pasted service account JSON)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var currentSec *Section
	var currentKey string

	for scanner.Scan() {
		rawLine := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(rawLine)

		// Empty line or comment
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}

		// Continuation line (indented value in Python configparser)
		if (strings.HasPrefix(rawLine, " ") || strings.HasPrefix(rawLine, "\t")) && currentSec != nil && currentKey != "" {
			prev := currentSec.Values[currentKey]
			if prev == "" {
				currentSec.Values[currentKey] = trimmed
			} else {
				currentSec.Values[currentKey] = prev + "\n" + trimmed
			}
			continue
		}

		// Section header [name]
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			secName := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			currentSec = doc.EnsureSection(secName)
			currentKey = ""
			continue
		}

		// Key = Value or Key : Value
		if currentSec != nil {
			idx := strings.IndexAny(rawLine, "=:")
			if idx > 0 {
				key := strings.TrimSpace(rawLine[:idx])
				val := strings.TrimSpace(rawLine[idx+1:])
				currentSec.Set(key, val)
				currentKey = key
			}
		}
	}

	return doc
}

// LoadINI reads and parses an INI file from disk. If the file does not exist, returns an empty INIFile and nil error.
func LoadINI(path string) (*INIFile, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewINIFile(), false, nil
		}
		return nil, false, err
	}
	return ParseINI(string(data)), true, nil
}

// EnsureSection returns an existing section or creates a new one.
func (f *INIFile) EnsureSection(name string) *Section {
	if s, ok := f.byName[name]; ok {
		return s
	}
	s := &Section{
		Name:   name,
		Values: make(map[string]string),
	}
	f.Sections = append(f.Sections, s)
	f.byName[name] = s
	return s
}

// GetSection retrieves a section by exact name.
func (f *INIFile) GetSection(name string) *Section {
	return f.byName[name]
}

// DeleteSection removes a section by exact name.
func (f *INIFile) DeleteSection(name string) bool {
	if _, ok := f.byName[name]; !ok {
		return false
	}
	delete(f.byName, name)
	out := make([]*Section, 0, len(f.Sections)-1)
	for _, s := range f.Sections {
		if s.Name != name {
			out = append(out, s)
		}
	}
	f.Sections = out
	return true
}

// Set sets or updates a key on a section while preserving key order.
func (s *Section) Set(key, value string) {
	if _, exists := s.Values[key]; !exists {
		s.Keys = append(s.Keys, key)
	}
	s.Values[key] = value
}

// Delete removes a key from a section.
func (s *Section) Delete(key string) {
	if _, exists := s.Values[key]; !exists {
		return
	}
	delete(s.Values, key)
	out := make([]string, 0, len(s.Keys)-1)
	for _, k := range s.Keys {
		if k != key {
			out = append(out, k)
		}
	}
	s.Keys = out
}

// Get returns a key's value or fallback.
func (s *Section) Get(key, fallback string) string {
	if s == nil {
		return fallback
	}
	if v, ok := s.Values[key]; ok {
		return v
	}
	return fallback
}

// GetBool parses a Python configparser boolean value.
func (s *Section) GetBool(key string, fallback bool) bool {
	if s == nil {
		return fallback
	}
	v, ok := s.Values[key]
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "on", "1":
		return true
	case "false", "no", "off", "0":
		return false
	default:
		return fallback
	}
}

// Render serializes the INIFile back to Python configparser-compatible text.
func (f *INIFile) Render() string {
	var b strings.Builder
	for i, s := range f.Sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("[")
		b.WriteString(s.Name)
		b.WriteString("]\n")
		for _, k := range s.Keys {
			v := s.Values[k]
			if strings.Contains(v, "\n") {
				lines := strings.Split(v, "\n")
				b.WriteString(k)
				b.WriteString(" = ")
				b.WriteString(lines[0])
				b.WriteString("\n")
				for _, line := range lines[1:] {
					b.WriteString("\t")
					b.WriteString(line)
					b.WriteString("\n")
				}
			} else {
				b.WriteString(k)
				b.WriteString(" = ")
				b.WriteString(v)
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

// WriteFileWithBackup writes content atomically and creates a timestamped .bak backup if the file existed.
func WriteFileWithBackup(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}

	if existing, err := os.ReadFile(path); err == nil && len(existing) > 0 {
		bakPath := path + ".bak"
		_ = os.WriteFile(bakPath, existing, 0o600)
	}

	tmpPath := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// Domain types derived from emailproxy.config and credstore.config

var serverSectionRe = regexp.MustCompile(`^(IMAP|POP|SMTP)-(\d+)$`)

// ServerEntry represents an [IMAP-1993] / [SMTP-1587] / [POP-1995] section.
type ServerEntry struct {
	Section              string `json:"section"`
	Protocol             string `json:"protocol"`
	LocalPort            int    `json:"local_port"`
	LocalAddress         string `json:"local_address"`
	ServerAddress        string `json:"server_address"`
	ServerPort           int    `json:"server_port"`
	ServerStartTLS       bool   `json:"server_starttls"`
	LocalStartTLS        bool   `json:"local_starttls"`
	LocalCertificatePath string `json:"local_certificate_path"`
	LocalKeyPath         string `json:"local_key_path"`
	LocalTLS             bool   `json:"local_tls"`
}

// AccountEntry represents an account or catch-all section (contains '@').
type AccountEntry struct {
	Account               string        `json:"account"`
	IsCatchAll            bool          `json:"is_catch_all"`
	PermissionURL         string        `json:"permission_url"`
	TokenURL              string        `json:"token_url"`
	OAuth2Scope           string        `json:"oauth2_scope"`
	OAuth2Resource        string        `json:"oauth2_resource,omitempty"`
	OAuth2Flow            string        `json:"oauth2_flow"`
	RedirectURI           string        `json:"redirect_uri"`
	RedirectListenAddress string        `json:"redirect_listen_address"`
	ClientID              string        `json:"client_id"`
	ClientSecret          string        `json:"client_secret"`
	HasClientSecret       bool          `json:"has_client_secret"`
	UsePKCE               bool          `json:"use_pkce"`
	JWTCertificatePath    string        `json:"jwt_certificate_path,omitempty"`
	JWTKeyPath            string        `json:"jwt_key_path,omitempty"`
	// Token status (from credstore.config or inline in emailproxy.config)
	HasAccessToken    bool          `json:"has_access_token"`
	HasRefreshToken   bool          `json:"has_refresh_token"`
	AccessTokenExpiry int64         `json:"access_token_expiry"`
	LastActivity      int64         `json:"last_activity"`
	TokenStatus       string        `json:"token_status"` // "authenticated", "expired_access", "pending", "catch_all"
	FromCacheOnly     bool          `json:"from_cache_only"`
	MatchedServers    []ServerEntry `json:"matched_servers,omitempty"`
}

// GlobalOptions represents the [emailproxy] section.
type GlobalOptions struct {
	DeleteAccountTokenOnPasswordError    bool `json:"delete_account_token_on_password_error"`
	EncryptClientSecretOnFirstUse        bool `json:"encrypt_client_secret_on_first_use"`
	UseLoginPasswordAsClientCredsSecret  bool `json:"use_login_password_as_client_credentials_secret"`
	AllowCatchAllAccounts                bool `json:"allow_catch_all_accounts"`
}

// ParsedProxyConfig is the structured view of emailproxy.config + credstore.config.
type ParsedProxyConfig struct {
	ConfigExists    bool           `json:"config_exists"`
	CredStoreExists bool           `json:"credstore_exists"`
	Servers         []ServerEntry  `json:"servers"`
	Accounts        []AccountEntry `json:"accounts"`
	Globals         GlobalOptions  `json:"globals"`
}

// ExtractServers parses all [IMAP-*], [POP-*], [SMTP-*] sections.
func ExtractServers(ini *INIFile) []ServerEntry {
	servers := make([]ServerEntry, 0)
	for _, s := range ini.Sections {
		m := serverSectionRe.FindStringSubmatch(s.Name)
		if m == nil {
			continue
		}
		localPort, _ := strconv.Atoi(m[2])
		remotePort, _ := strconv.Atoi(s.Get("server_port", "0"))
		certPath := s.Get("local_certificate_path", "")
		keyPath := s.Get("local_key_path", "")
		localStartTLS := s.GetBool("local_starttls", false)
		servers = append(servers, ServerEntry{
			Section:              s.Name,
			Protocol:             m[1],
			LocalPort:            localPort,
			LocalAddress:         s.Get("local_address", "::"),
			ServerAddress:        s.Get("server_address", ""),
			ServerPort:           remotePort,
			ServerStartTLS:       s.GetBool("server_starttls", false),
			LocalStartTLS:        localStartTLS,
			LocalCertificatePath: certPath,
			LocalKeyPath:         keyPath,
			LocalTLS:             certPath != "" && keyPath != "" && !localStartTLS,
		})
	}
	return servers
}

// ExtractAccounts merges account sections from emailproxy.config and token status from credstore.config.
// If maskSecrets is true, client_secret is redacted to empty string while HasClientSecret remains accurate.
func ExtractAccounts(cfgINI, cacheINI *INIFile, maskSecrets bool) []AccountEntry {
	accounts := make([]AccountEntry, 0)
	seen := make(map[string]bool)

	buildEntry := func(name string, cfgSec, cacheSec *Section, cacheOnly bool) AccountEntry {
		getVal := func(k string) string {
			if cfgSec != nil {
				if v := cfgSec.Get(k, ""); v != "" {
					return v
				}
			}
			if cacheSec != nil {
				return cacheSec.Get(k, "")
			}
			return ""
		}

		// Token fields may live in cacheSec (credstore.config) or inline in cfgSec
		getTokenVal := func(k string) string {
			if cacheSec != nil {
				if v := cacheSec.Get(k, ""); v != "" {
					return v
				}
			}
			if cfgSec != nil {
				return cfgSec.Get(k, "")
			}
			return ""
		}

		accessToken := getTokenVal("access_token")
		refreshToken := getTokenVal("refresh_token")
		expiry, _ := strconv.ParseInt(getTokenVal("access_token_expiry"), 10, 64)
		lastAct, _ := strconv.ParseInt(getTokenVal("last_activity"), 10, 64)

		isCatchAll := strings.HasPrefix(name, "@")
		status := "pending"
		if isCatchAll {
			status = "catch_all"
		} else if refreshToken != "" || accessToken != "" {
			if expiry > 0 && time.Now().Unix() > expiry && refreshToken == "" {
				status = "expired_access"
			} else {
				status = "authenticated"
			}
		}

		secret := ""
		if cfgSec != nil {
			secret = cfgSec.Get("client_secret", "")
		}
		hasSecret := secret != "" || getTokenVal("client_secret_encrypted") != ""
		if maskSecrets {
			secret = ""
		}

		return AccountEntry{
			Account:               name,
			IsCatchAll:            isCatchAll,
			PermissionURL:         getVal("permission_url"),
			TokenURL:              getVal("token_url"),
			OAuth2Scope:           getVal("oauth2_scope"),
			OAuth2Resource:        getVal("oauth2_resource"),
			OAuth2Flow:            getVal("oauth2_flow"),
			RedirectURI:           getVal("redirect_uri"),
			RedirectListenAddress: getVal("redirect_listen_address"),
			ClientID:              getVal("client_id"),
			ClientSecret:          secret,
			HasClientSecret:       hasSecret,
			UsePKCE:               cfgSec != nil && cfgSec.GetBool("use_pkce", false),
			JWTCertificatePath:    getVal("jwt_certificate_path"),
			JWTKeyPath:            getVal("jwt_key_path"),
			HasAccessToken:        accessToken != "",
			HasRefreshToken:       refreshToken != "",
			AccessTokenExpiry:     expiry,
			LastActivity:          lastAct,
			TokenStatus:           status,
			FromCacheOnly:         cacheOnly,
		}
	}

	if cfgINI != nil {
		for _, s := range cfgINI.Sections {
			if !strings.Contains(s.Name, "@") {
				continue
			}
			var cacheSec *Section
			if cacheINI != nil {
				cacheSec = cacheINI.GetSection(s.Name)
			}
			accounts = append(accounts, buildEntry(s.Name, s, cacheSec, false))
			seen[s.Name] = true
		}
	}

	if cacheINI != nil {
		for _, s := range cacheINI.Sections {
			if !strings.Contains(s.Name, "@") || seen[s.Name] {
				continue
			}
			// Inherit domain catch-all config if available
			var domainSec *Section
			if cfgINI != nil {
				if idx := strings.LastIndex(s.Name, "@"); idx >= 0 {
					domainSec = cfgINI.GetSection(s.Name[idx:])
					if domainSec == nil {
						domainSec = cfgINI.GetSection("@")
					}
				}
			}
			accounts = append(accounts, buildEntry(s.Name, domainSec, s, true))
		}
	}

	return accounts
}

// ExtractGlobals parses the [emailproxy] section with upstream defaults.
func ExtractGlobals(ini *INIFile) GlobalOptions {
	sec := ini.GetSection("emailproxy")
	return GlobalOptions{
		DeleteAccountTokenOnPasswordError:   sec.GetBool("delete_account_token_on_password_error", true),
		EncryptClientSecretOnFirstUse:       sec.GetBool("encrypt_client_secret_on_first_use", false),
		UseLoginPasswordAsClientCredsSecret: sec.GetBool("use_login_password_as_client_credentials_secret", false),
		AllowCatchAllAccounts:               sec.GetBool("allow_catch_all_accounts", false),
	}
}

// MatchServersForAccount returns all configured proxy servers (IMAP, POP, SMTP) that match an account's provider.
func MatchServersForAccount(acc AccountEntry, servers []ServerEntry) []ServerEntry {
	provider := detectAccountProvider(acc)
	if provider == "" {
		return nil
	}
	matched := make([]ServerEntry, 0)
	for _, s := range servers {
		if serverMatchesProvider(s, provider) {
			matched = append(matched, s)
		}
	}
	return matched
}

func detectAccountProvider(acc AccountEntry) string {
	urls := strings.ToLower(acc.TokenURL + " " + acc.PermissionURL + " " + acc.OAuth2Scope)
	switch {
	case strings.Contains(urls, "microsoftonline") || strings.Contains(urls, "outlook.office") || strings.Contains(urls, "login.live.com"):
		return "outlook"
	case strings.Contains(urls, "googleapis.com") || strings.Contains(urls, "accounts.google.com") || strings.Contains(urls, "mail.google.com"):
		return "gmail"
	case strings.Contains(urls, "login.yahoo.com"):
		return "yahoo"
	case strings.Contains(urls, "login.aol.com"):
		return "aol"
	case strings.Contains(urls, "fastmail.com"):
		return "fastmail"
	}

	// Fallback to email domain if URLs were not present
	domain := ""
	if idx := strings.LastIndex(acc.Account, "@"); idx >= 0 {
		domain = strings.ToLower(acc.Account[idx+1:])
	}
	switch {
	case strings.Contains(domain, "outlook") || strings.Contains(domain, "hotmail") || strings.Contains(domain, "live.") || strings.Contains(domain, "msn."):
		return "outlook"
	case strings.Contains(domain, "gmail") || strings.Contains(domain, "googlemail"):
		return "gmail"
	case strings.Contains(domain, "yahoo") || strings.Contains(domain, "ymail"):
		return "yahoo"
	case strings.Contains(domain, "aol"):
		return "aol"
	case strings.Contains(domain, "fastmail"):
		return "fastmail"
	}
	return ""
}

func serverMatchesProvider(s ServerEntry, provider string) bool {
	host := strings.ToLower(s.ServerAddress)
	switch provider {
	case "outlook":
		return strings.Contains(host, "office365") || strings.Contains(host, "outlook") || strings.Contains(host, "hotmail") || strings.Contains(host, "live.com")
	case "gmail":
		return strings.Contains(host, "gmail") || strings.Contains(host, "googlemail") || strings.Contains(host, "google.com")
	case "yahoo":
		return strings.Contains(host, "yahoo") || strings.Contains(host, "ymail")
	case "aol":
		return strings.Contains(host, "aol")
	case "fastmail":
		return strings.Contains(host, "fastmail")
	default:
		return false
	}
}

// ResolveAccountSection finds the exact account section or matching domain/@ catch-all section.
func ResolveAccountSection(ini *INIFile, email string) *Section {
	if s := ini.GetSection(email); s != nil {
		return s
	}
	if idx := strings.LastIndex(email, "@"); idx >= 0 {
		domainKey := strings.ToLower(email[idx:])
		for _, s := range ini.Sections {
			if strings.ToLower(s.Name) == domainKey {
				return s
			}
		}
		if s := ini.GetSection("@"); s != nil {
			return s
		}
	}
	return nil
}

// UpsertServer creates or updates a server section on ini.
func UpsertServer(ini *INIFile, entry ServerEntry) string {
	proto := strings.ToUpper(strings.TrimSpace(entry.Protocol))
	secName := fmt.Sprintf("%s-%d", proto, entry.LocalPort)
	s := ini.EnsureSection(secName)
	s.Set("server_address", strings.TrimSpace(entry.ServerAddress))
	s.Set("server_port", strconv.Itoa(entry.ServerPort))
	if entry.ServerStartTLS {
		s.Set("server_starttls", "True")
	} else {
		s.Delete("server_starttls")
	}
	localAddr := strings.TrimSpace(entry.LocalAddress)
	if localAddr == "" {
		localAddr = "0.0.0.0"
	}
	s.Set("local_address", localAddr)
	if entry.LocalStartTLS {
		s.Set("local_starttls", "True")
	} else {
		s.Delete("local_starttls")
	}
	if strings.TrimSpace(entry.LocalCertificatePath) != "" && strings.TrimSpace(entry.LocalKeyPath) != "" {
		s.Set("local_certificate_path", strings.TrimSpace(entry.LocalCertificatePath))
		s.Set("local_key_path", strings.TrimSpace(entry.LocalKeyPath))
	} else {
		s.Delete("local_certificate_path")
		s.Delete("local_key_path")
	}
	return secName
}

// UpsertAccount creates or updates an account or catch-all section on ini.
func UpsertAccount(ini *INIFile, entry AccountEntry, keepExistingSecret bool) string {
	secName := strings.TrimSpace(entry.Account)
	s := ini.EnsureSection(secName)

	setOrDelete := func(key, val string) {
		val = strings.TrimSpace(val)
		if val == "" {
			s.Delete(key)
		} else {
			s.Set(key, val)
		}
	}

	setOrDelete("permission_url", entry.PermissionURL)
	setOrDelete("token_url", entry.TokenURL)
	setOrDelete("oauth2_scope", entry.OAuth2Scope)
	setOrDelete("oauth2_resource", entry.OAuth2Resource)
	setOrDelete("oauth2_flow", entry.OAuth2Flow)
	setOrDelete("redirect_uri", entry.RedirectURI)
	setOrDelete("redirect_listen_address", entry.RedirectListenAddress)
	setOrDelete("client_id", entry.ClientID)
	setOrDelete("jwt_certificate_path", entry.JWTCertificatePath)
	setOrDelete("jwt_key_path", entry.JWTKeyPath)

	if strings.TrimSpace(entry.ClientSecret) != "" {
		s.Set("client_secret", strings.TrimSpace(entry.ClientSecret))
	} else if !keepExistingSecret {
		s.Delete("client_secret")
	}

	if entry.UsePKCE {
		s.Set("use_pkce", "True")
	} else {
		s.Delete("use_pkce")
	}

	return secName
}

// SetGlobals writes the [emailproxy] section.
func SetGlobals(ini *INIFile, g GlobalOptions) {
	s := ini.EnsureSection("emailproxy")
	boolStr := func(v bool) string {
		if v {
			return "True"
		}
		return "False"
	}
	s.Set("delete_account_token_on_password_error", boolStr(g.DeleteAccountTokenOnPasswordError))
	s.Set("encrypt_client_secret_on_first_use", boolStr(g.EncryptClientSecretOnFirstUse))
	s.Set("use_login_password_as_client_credentials_secret", boolStr(g.UseLoginPasswordAsClientCredsSecret))
	s.Set("allow_catch_all_accounts", boolStr(g.AllowCatchAllAccounts))
}
