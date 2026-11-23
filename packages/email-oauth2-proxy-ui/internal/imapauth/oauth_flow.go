package imapauth

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"email-oauth2-proxy-ui/internal/config"
	"email-oauth2-proxy-ui/internal/logs"
)

// AuthSession represents an active or finished OAuth 2.0 interactive login wizard session.
type AuthSession struct {
	ID             string               `json:"id"`
	Email          string               `json:"email"`
	Port           int                  `json:"port"`
	UseTLS         bool                 `json:"use_tls"`
	CallbackPort   int                  `json:"callback_port"`
	RedirectURI    string               `json:"redirect_uri,omitempty"`
	Status         string               `json:"status"` // "connecting", "awaiting_user", "exchanging", "success", "error", "cancelled"
	AuthURL        string               `json:"auth_url,omitempty"`
	OAuthState     string               `json:"-"`
	DeviceCode     string               `json:"device_code,omitempty"`
	Message        string               `json:"message,omitempty"`
	IMAPResponse   string               `json:"imap_response,omitempty"`
	MatchedServers []config.ServerEntry `json:"matched_servers,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`

	cancel context.CancelFunc
}

// PendingAuth represents an OAuth authorization request triggered passively by an external email client.
type PendingAuth struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	AuthURL      string    `json:"auth_url"`
	OAuthState   string    `json:"-"`
	DeviceCode   string    `json:"device_code,omitempty"`
	RedirectURI  string    `json:"redirect_uri,omitempty"`
	CallbackPort int       `json:"callback_port"`
	DetectedAt   time.Time `json:"detected_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Manager coordinates OAuth wizard sessions and passive external client requests against email-oauth2-proxy.
type Manager struct {
	proxyHost    string
	configPath   string
	logPath      string
	callbackHost string
	mu           sync.RWMutex
	sessions     map[string]*AuthSession
	pending      map[string]*PendingAuth // keyed by lowercase email
}

// NewManager creates an OAuth flow Manager.
func NewManager(proxyHost, configPath, logPath string) *Manager {
	return &Manager{
		proxyHost:    proxyHost,
		configPath:   configPath,
		logPath:      logPath,
		callbackHost: proxyHost,
		sessions:     make(map[string]*AuthSession),
		pending:      make(map[string]*PendingAuth),
	}
}

// StartRequest specifies inputs for starting a 1-click OAuth wizard session.
type StartRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Port     int    `json:"port,omitempty"`
}

var (
	visitURLRe        = regexp.MustCompile(`Please visit the following URL to authenticate account ([^:]+):\s*(https?://\S+)`)
	deviceCodeRe      = regexp.MustCompile(`Visit\s+(https?://\S+)\s+and use code\s+([A-Za-z0-9-]+)`)
	authReqReceivedRe = regexp.MustCompile(`Authorisation request received for\s+(\S+@\S+)`)
	authDoneRe        = regexp.MustCompile(`(?:Successfully authenticated|OK AUTHENTICATE completed|Authorisation result error for account|No-GUI authorisation request failed or timed out for account)\b`)
)

// Start initiates an IMAP login against email-oauth2-proxy and tails the log for the OAuth authorization URL.
func (m *Manager) Start(req StartRequest) (*AuthSession, error) {
	email := strings.TrimSpace(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("informe um endereço de e-mail válido")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("informe uma senha local para criptografar o token desta conta")
	}

	ini, exists, err := config.LoadINI(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler emailproxy.config: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("arquivo emailproxy.config ainda não existe")
	}

	servers := config.ExtractServers(ini)
	targetSrv, err := pickIMAPServer(servers, email, req.Port)
	if err != nil {
		return nil, err
	}

	callbackPort := resolveCallbackPort(ini, email)
	accSec := config.ResolveAccountSection(ini, email)
	redirectURI := ""
	accEntry := config.AccountEntry{Account: email}
	if accSec != nil {
		redirectURI = accSec.Get("redirect_uri", "")
		accEntry.TokenURL = accSec.Get("token_url", "")
		accEntry.PermissionURL = accSec.Get("permission_url", "")
		accEntry.OAuth2Scope = accSec.Get("oauth2_scope", "")
	}
	matchedServers := config.MatchServersForAccount(accEntry, servers)
	if len(matchedServers) == 0 {
		matchedServers = []config.ServerEntry{targetSrv}
	}

	// Record current log offset before triggering login so we only capture lines for this session
	var logOffset int64
	if info, err := os.Stat(m.logPath); err == nil {
		logOffset = info.Size()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	id := randomID()
	now := time.Now().UTC()

	sess := &AuthSession{
		ID:             id,
		Email:          email,
		Port:           targetSrv.LocalPort,
		UseTLS:         targetSrv.LocalTLS,
		CallbackPort:   callbackPort,
		RedirectURI:    redirectURI,
		Status:         "connecting",
		Message:        fmt.Sprintf("Conectando via IMAP em %s:%d...", m.proxyHost, targetSrv.LocalPort),
		MatchedServers: matchedServers,
		CreatedAt:      now,
		UpdatedAt:      now,
		cancel:         cancel,
	}

	m.mu.Lock()
	m.cleanupOldLocked()
	m.sessions[id] = sess
	m.mu.Unlock()

	go m.watchLogs(ctx, id, email, logOffset)
	go m.runIMAPLogin(ctx, id, email, req.Password, targetSrv)

	return m.cloneSession(sess), nil
}

// Get returns a snapshot of an AuthSession by ID.
func (m *Manager) Get(id string) (*AuthSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, false
	}
	return m.cloneSession(s), true
}

// Cancel aborts an in-flight OAuth session.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		if s.Status == "connecting" || s.Status == "awaiting_user" || s.Status == "exchanging" {
			s.Status = "cancelled"
			s.Message = "Autenticação cancelada pelo usuário."
			s.UpdatedAt = time.Now().UTC()
		}
		if s.cancel != nil {
			s.cancel()
		}
	}
	m.mu.Unlock()
	return ok
}

// SubmitCallback parses the post-login URL pasted by the user (e.g. https://localhost/?code=...&state=...)
// and forwards its query parameters over plain HTTP directly to email-oauth2-proxy's local auth listener.
func (m *Manager) SubmitCallback(id, pastedURL string) (string, error) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.RUnlock()
		return "", fmt.Errorf("sessão não encontrada")
	}
	cbPort := s.CallbackPort
	m.mu.RUnlock()

	rawQuery, err := extractQueryString(pastedURL)
	if err != nil {
		return "", err
	}

	m.updateSession(id, func(s *AuthSession) {
		if s.Status == "awaiting_user" || s.Status == "connecting" {
			s.Status = "exchanging"
			s.Message = "Enviando código OAuth para o proxy e concluindo login..."
		}
	})

	targetURL := fmt.Sprintf("http://%s:%d/?%s", m.callbackHost, cbPort, rawQuery)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(targetURL)
	if err != nil {
		return "", fmt.Errorf("falha ao enviar callback para %s:%d: %w", m.callbackHost, cbPort, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyStr := strings.TrimSpace(string(bodyBytes))
	if resp.StatusCode >= 400 {
		return bodyStr, fmt.Errorf("proxy respondeu HTTP %d ao callback", resp.StatusCode)
	}
	return bodyStr, nil
}

// StartMonitor passively watches emailproxy.log in the background to capture OAuth requests
// triggered by external email clients (MailCraft, Thunderbird, printers, scripts, etc.).
func (m *Manager) StartMonitor(ctx context.Context) {
	var startOffset int64
	if info, err := os.Stat(m.logPath); err == nil {
		startOffset = info.Size()
	}

	var lastRequestedEmail string
	logs.Stream(ctx, m.logPath, startOffset, func(line string) {
		if reqM := authReqReceivedRe.FindStringSubmatch(line); reqM != nil {
			lastRequestedEmail = strings.TrimSpace(reqM[1])
		}

		if mMatch := visitURLRe.FindStringSubmatch(line); mMatch != nil {
			email := strings.TrimSpace(mMatch[1])
			authURL := strings.TrimSpace(mMatch[2])
			m.recordExternalPending(email, authURL, "")
			return
		}

		if dMatch := deviceCodeRe.FindStringSubmatch(line); dMatch != nil {
			authURL := strings.TrimSpace(dMatch[1])
			code := strings.TrimSpace(dMatch[2])
			email := lastRequestedEmail
			if email == "" {
				email = "conta@externa"
			}
			m.recordExternalPending(email, authURL, code)
			return
		}

		if authDoneRe.MatchString(line) {
			lowLine := strings.ToLower(line)
			m.mu.Lock()
			for k, p := range m.pending {
				if strings.Contains(lowLine, strings.ToLower(p.Email)) {
					delete(m.pending, k)
				}
			}
			m.mu.Unlock()
		}
	})
}

func (m *Manager) recordExternalPending(email, authURL, deviceCode string) {
	lowEmail := strings.ToLower(email)

	// Skip if an active wizard session in our UI already owns this email
	m.mu.RLock()
	for _, s := range m.sessions {
		if strings.EqualFold(s.Email, email) && (s.Status == "connecting" || s.Status == "awaiting_user" || s.Status == "exchanging") {
			m.mu.RUnlock()
			return
		}
	}
	m.mu.RUnlock()

	cbPort := 80
	redirectURI := ""
	if ini, exists, err := config.LoadINI(m.configPath); err == nil && exists {
		cbPort = resolveCallbackPort(ini, email)
		if sec := config.ResolveAccountSection(ini, email); sec != nil {
			redirectURI = sec.Get("redirect_uri", "")
		}
	}

	oauthState := ""
	if u, err := url.Parse(authURL); err == nil {
		oauthState = u.Query().Get("state")
	}

	now := time.Now().UTC()
	m.mu.Lock()
	m.pending[lowEmail] = &PendingAuth{
		ID:           randomID(),
		Email:        email,
		AuthURL:      authURL,
		OAuthState:   oauthState,
		DeviceCode:   deviceCode,
		RedirectURI:  redirectURI,
		CallbackPort: cbPort,
		DetectedAt:   now,
		ExpiresAt:    now.Add(5 * time.Minute),
	}
	m.mu.Unlock()
}

// ListPending returns active, non-expired external client OAuth requests.
func (m *Manager) ListPending() []PendingAuth {
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]PendingAuth, 0, len(m.pending))
	for k, p := range m.pending {
		if now.After(p.ExpiresAt) {
			delete(m.pending, k)
			continue
		}
		out = append(out, *p)
	}
	return out
}

// DismissPending removes a pending external OAuth item by ID.
func (m *Manager) DismissPending(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, p := range m.pending {
		if p.ID == id {
			delete(m.pending, k)
			return true
		}
	}
	return false
}

// SubmitPendingCallback forwards a pasted browser callback URL for a pending external client request.
func (m *Manager) SubmitPendingCallback(id, pastedURL string) (string, error) {
	m.mu.RLock()
	var target *PendingAuth
	var targetKey string
	for k, p := range m.pending {
		if p.ID == id {
			cp := *p
			target = &cp
			targetKey = k
			break
		}
	}
	m.mu.RUnlock()

	if target == nil {
		return "", fmt.Errorf("pedido pendente não encontrado ou já expirado")
	}

	rawQuery, err := extractQueryString(pastedURL)
	if err != nil {
		return "", err
	}

	targetURL := fmt.Sprintf("http://%s:%d/?%s", m.callbackHost, target.CallbackPort, rawQuery)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(targetURL)
	if err != nil {
		return "", fmt.Errorf("falha ao enviar callback para %s:%d: %w", m.callbackHost, target.CallbackPort, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyStr := strings.TrimSpace(string(bodyBytes))
	if resp.StatusCode >= 400 {
		return bodyStr, fmt.Errorf("proxy respondeu HTTP %d ao callback", resp.StatusCode)
	}

	m.mu.Lock()
	delete(m.pending, targetKey)
	m.mu.Unlock()
	return bodyStr, nil
}

// SubmitCallbackAuto locates the matching active OAuth session (or passive external pending request)
// by OAuth state parameter or most recent active flow, and forwards the callback query string to the proxy.
func (m *Manager) SubmitCallbackAuto(rawQuery string) (*AuthSession, string, error) {
	if strings.TrimSpace(rawQuery) == "" {
		return nil, "", fmt.Errorf("callback sem parâmetros (?code=...&state=...)")
	}
	parsedQ, _ := url.ParseQuery(rawQuery)
	incomingState := parsedQ.Get("state")

	m.mu.RLock()
	var matchedID string
	var latestActive *AuthSession
	for id, s := range m.sessions {
		if s.Status != "awaiting_user" && s.Status != "connecting" {
			continue
		}
		if incomingState != "" && s.OAuthState != "" && s.OAuthState == incomingState {
			matchedID = id
			break
		}
		if latestActive == nil || s.UpdatedAt.After(latestActive.UpdatedAt) {
			latestActive = s
		}
	}
	if matchedID == "" && latestActive != nil {
		matchedID = latestActive.ID
	}

	var matchedPendingID string
	if matchedID == "" {
		var latestPending *PendingAuth
		now := time.Now().UTC()
		for _, p := range m.pending {
			if now.After(p.ExpiresAt) {
				continue
			}
			if incomingState != "" && p.OAuthState != "" && p.OAuthState == incomingState {
				matchedPendingID = p.ID
				break
			}
			if latestPending == nil || p.DetectedAt.After(latestPending.DetectedAt) {
				latestPending = p
			}
		}
		if matchedPendingID == "" && latestPending != nil {
			matchedPendingID = latestPending.ID
		}
	}
	m.mu.RUnlock()

	if matchedID != "" {
		respBody, err := m.SubmitCallback(matchedID, "?"+rawQuery)
		sess, _ := m.Get(matchedID)
		return sess, respBody, err
	}

	if matchedPendingID != "" {
		respBody, err := m.SubmitPendingCallback(matchedPendingID, "?"+rawQuery)
		return nil, respBody, err
	}

	return nil, "", fmt.Errorf("nenhuma sessão OAuth ativa aguardando callback no momento")
}

func (m *Manager) watchLogs(ctx context.Context, id, email string, startOffset int64) {
	lowerEmail := strings.ToLower(email)
	logs.Stream(ctx, m.logPath, startOffset, func(line string) {
		if mMatch := visitURLRe.FindStringSubmatch(line); mMatch != nil {
			if strings.EqualFold(strings.TrimSpace(mMatch[1]), email) {
				authURL := strings.TrimSpace(mMatch[2])
				oauthState := ""
				if u, err := url.Parse(authURL); err == nil {
					oauthState = u.Query().Get("state")
				}
				m.updateSession(id, func(s *AuthSession) {
					if s.Status == "connecting" || s.Status == "awaiting_user" {
						s.Status = "awaiting_user"
						s.AuthURL = authURL
						s.OAuthState = oauthState
						s.Message = "Link de autorização capturado! Abra o login no provedor."
					}
				})
				return
			}
		}

		if dMatch := deviceCodeRe.FindStringSubmatch(line); dMatch != nil {
			authURL := strings.TrimSpace(dMatch[1])
			code := strings.TrimSpace(dMatch[2])
			m.updateSession(id, func(s *AuthSession) {
				if s.Status == "connecting" || s.Status == "awaiting_user" {
					s.Status = "awaiting_user"
					s.AuthURL = authURL
					s.DeviceCode = code
					s.Message = fmt.Sprintf("Fluxo Device Flow: abra %s e informe o código %s.", authURL, code)
				}
			})
			return
		}

		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, lowerEmail) && strings.Contains(lowerLine, "unable to start local server") {
			m.updateSession(id, func(s *AuthSession) {
				if s.Status == "connecting" {
					s.Message = "Aviso nos logs: o proxy não conseguiu iniciar o servidor local de callback (porta já em uso ou outra conta autenticando)."
				}
			})
		}
	})
}

func (m *Manager) runIMAPLogin(ctx context.Context, id, email, password string, srv config.ServerEntry) {
	defer func() {
		m.mu.RLock()
		s, ok := m.sessions[id]
		var cancel context.CancelFunc
		if ok {
			cancel = s.cancel
		}
		m.mu.RUnlock()
		if cancel != nil {
			cancel()
		}
	}()

	addr := net.JoinHostPort(m.proxyHost, strconv.Itoa(srv.LocalPort))
	dialer := &net.Dialer{Timeout: 8 * time.Second}

	var conn net.Conn
	var err error
	if srv.LocalTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
		})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		m.updateSession(id, func(s *AuthSession) {
			if s.Status != "cancelled" {
				s.Status = "error"
				s.Message = fmt.Sprintf("Não foi possível conectar ao proxy em %s: %v. Verifique se o container do Email OAuth2 Proxy foi reiniciado.", addr, err)
			}
		})
		return
	}
	defer conn.Close()

	// Abort socket promptly when context is cancelled
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Minute))
	reader := bufio.NewReader(conn)

	// Read IMAP server greeting (* OK ...)
	greeting, err := reader.ReadString('\n')
	if err != nil {
		m.updateSession(id, func(s *AuthSession) {
			if s.Status != "cancelled" {
				s.Status = "error"
				s.Message = fmt.Sprintf("Conexão fechada antes da saudação IMAP (verifique se TLS local no config bate com o proxy em execução): %v", err)
			}
		})
		return
	}
	greeting = strings.TrimSpace(greeting)

	// Send AUTHENTICATE PLAIN <base64(\x00email\x00password)> — handles any characters cleanly and is censored in proxy logs
	saslRaw := "\x00" + email + "\x00" + password
	saslB64 := base64.StdEncoding.EncodeToString([]byte(saslRaw))
	cmd := fmt.Sprintf("A001 AUTHENTICATE PLAIN %s\r\n", saslB64)
	if _, err := io.WriteString(conn, cmd); err != nil {
		m.updateSession(id, func(s *AuthSession) {
			if s.Status != "cancelled" {
				s.Status = "error"
				s.Message = fmt.Sprintf("Erro ao enviar comando IMAP: %v", err)
			}
		})
		return
	}

	m.updateSession(id, func(s *AuthSession) {
		if s.Status == "connecting" {
			s.Message = fmt.Sprintf("Conectado (%s). Aguardando link OAuth nos logs do proxy...", truncate(greeting, 60))
		}
	})

	// Read until tagged response "A001 OK/NO/BAD ..."
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			m.updateSession(id, func(s *AuthSession) {
				if s.Status != "cancelled" && s.Status != "success" {
					s.Status = "error"
					s.Message = fmt.Sprintf("Conexão IMAP encerrada durante autenticação: %v", err)
				}
			})
			return
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "A001 ") {
			continue
		}

		_, _ = io.WriteString(conn, "A002 LOGOUT\r\n")
		if strings.HasPrefix(strings.ToUpper(trimmed), "A001 OK") {
			m.updateSession(id, func(s *AuthSession) {
				s.Status = "success"
				s.IMAPResponse = trimmed
				if s.AuthURL == "" && s.DeviceCode == "" {
					s.Message = "Conta autenticada com sucesso! Token já estava válido e funcional."
				} else {
					s.Message = "Autenticação OAuth 2.0 concluída com sucesso! Tokens salvos no credstore.config."
				}
			})
		} else {
			m.updateSession(id, func(s *AuthSession) {
				if s.Status != "cancelled" {
					s.Status = "error"
					s.IMAPResponse = trimmed
					s.Message = fmt.Sprintf("Login recusado pelo proxy/provedor: %s", trimmed)
				}
			})
		}
		return
	}
}

func (m *Manager) updateSession(id string, fn func(*AuthSession)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		fn(s)
		s.UpdatedAt = time.Now().UTC()
	}
}

func (m *Manager) cloneSession(s *AuthSession) *AuthSession {
	cp := *s
	cp.cancel = nil
	return &cp
}

func (m *Manager) cleanupOldLocked() {
	cutoff := time.Now().UTC().Add(-30 * time.Minute)
	for id, s := range m.sessions {
		if s.UpdatedAt.Before(cutoff) && s.Status != "connecting" && s.Status != "awaiting_user" && s.Status != "exchanging" {
			delete(m.sessions, id)
		}
	}
}

func pickIMAPServer(servers []config.ServerEntry, email string, requestedPort int) (config.ServerEntry, error) {
	imapServers := make([]config.ServerEntry, 0)
	for _, s := range servers {
		if strings.EqualFold(s.Protocol, "IMAP") {
			imapServers = append(imapServers, s)
			if requestedPort > 0 && s.LocalPort == requestedPort {
				return s, nil
			}
		}
	}
	if requestedPort > 0 {
		return config.ServerEntry{}, fmt.Errorf("servidor IMAP na porta %d não encontrado no emailproxy.config", requestedPort)
	}
	if len(imapServers) == 0 {
		return config.ServerEntry{}, fmt.Errorf("nenhum servidor [IMAP-*] configurado no emailproxy.config")
	}

	// Auto-match by email domain
	domain := ""
	if idx := strings.LastIndex(email, "@"); idx >= 0 {
		domain = strings.ToLower(email[idx+1:])
	}

	matchHost := func(substr string) (config.ServerEntry, bool) {
		for _, s := range imapServers {
			if strings.Contains(strings.ToLower(s.ServerAddress), substr) {
				return s, true
			}
		}
		return config.ServerEntry{}, false
	}

	switch {
	case strings.Contains(domain, "gmail") || strings.Contains(domain, "googlemail"):
		if s, ok := matchHost("gmail"); ok {
			return s, nil
		}
	case strings.Contains(domain, "outlook") || strings.Contains(domain, "hotmail") || strings.Contains(domain, "live.") || strings.Contains(domain, "msn."):
		if s, ok := matchHost("office365"); ok {
			return s, nil
		}
		if s, ok := matchHost("outlook"); ok {
			return s, nil
		}
	case strings.Contains(domain, "yahoo") || strings.Contains(domain, "ymail"):
		if s, ok := matchHost("yahoo"); ok {
			return s, nil
		}
	case strings.Contains(domain, "aol"):
		if s, ok := matchHost("aol"); ok {
			return s, nil
		}
	case strings.Contains(domain, "fastmail"):
		if s, ok := matchHost("fastmail"); ok {
			return s, nil
		}
	}

	return imapServers[0], nil
}

func resolveCallbackPort(ini *config.INIFile, email string) int {
	sec := config.ResolveAccountSection(ini, email)
	if sec == nil {
		return 80
	}
	listenAddr := sec.Get("redirect_listen_address", "")
	if listenAddr == "" {
		listenAddr = sec.Get("redirect_uri", "")
	}
	if listenAddr == "" {
		return 80
	}
	u, err := url.Parse(listenAddr)
	if err != nil {
		return 80
	}
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			return n
		}
	}
	return 80
}

func extractQueryString(pasted string) (string, error) {
	s := strings.TrimSpace(pasted)
	if s == "" {
		return "", fmt.Errorf("cole a URL retornada pelo navegador")
	}
	// Accept full URL (https://localhost/?code=...&state=...), or query-only (?code=... / code=...)
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", fmt.Errorf("URL inválida: %w", err)
		}
		if u.RawQuery == "" {
			return "", fmt.Errorf("a URL colada não contém parâmetros (?code=...&state=...)")
		}
		return u.RawQuery, nil
	}
	s = strings.TrimPrefix(s, "?")
	if !strings.Contains(s, "code=") && !strings.Contains(s, "error=") {
		return "", fmt.Errorf("URL inválida: esperado algo como https://localhost/?code=...&state=...")
	}
	return s, nil
}

func randomID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
