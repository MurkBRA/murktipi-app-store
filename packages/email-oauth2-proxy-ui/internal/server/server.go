package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"email-oauth2-proxy-ui/internal/auth"
	"email-oauth2-proxy-ui/internal/certs"
	"email-oauth2-proxy-ui/internal/config"
	"email-oauth2-proxy-ui/internal/imapauth"
	"email-oauth2-proxy-ui/internal/logs"
	"email-oauth2-proxy-ui/internal/reload"
)

// Options configures the HTTP server.
type Options struct {
	ConfigDir      string
	ConfigFile     string
	CredStoreFile  string
	LogFile        string
	ProxyHost      string
	ProxyContainer string
	DockerSocket   string
	ComposePath    string
	ComposeService string
	AdminUser      string
	AdminPassword  string
	WebFS          fs.FS
}

// Server serves the Web UI and JSON API.
type Server struct {
	opts  Options
	auth  *auth.Manager
	oauth *imapauth.Manager
	mux   *http.ServeMux
}

// New creates a configured Server instance.
func New(opts Options) *Server {
	if opts.ConfigFile == "" {
		opts.ConfigFile = filepath.Join(opts.ConfigDir, "emailproxy.config")
	}
	if opts.CredStoreFile == "" {
		opts.CredStoreFile = filepath.Join(opts.ConfigDir, "credstore.config")
	}
	if opts.LogFile == "" {
		opts.LogFile = filepath.Join(opts.ConfigDir, "emailproxy.log")
	}
	if opts.ProxyHost == "" {
		opts.ProxyHost = "email-oauth2-proxy"
	}
	if opts.ProxyContainer == "" {
		opts.ProxyContainer = opts.ProxyHost
	}
	if opts.DockerSocket == "" {
		opts.DockerSocket = "/var/run/docker.sock"
	}
	if opts.ComposePath == "" {
		opts.ComposePath = "/compose"
	}
	if opts.ComposeService == "" {
		opts.ComposeService = "email-oauth2-proxy"
	}

	s := &Server{
		opts:  opts,
		auth:  auth.NewManager(opts.AdminUser, opts.AdminPassword),
		oauth: imapauth.NewManager(opts.ProxyHost, opts.ConfigFile, opts.LogFile),
		mux:   http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) pendingRestartHint() reload.Result {
	if reload.CanRestart(s.opts.DockerSocket, s.opts.ProxyContainer) {
		return reload.Result{
			Supported: true,
			Reloaded:  false,
			Message:   "Alteração salva! Clique em 'Reiniciar Proxy' para aplicar no Email OAuth2 Proxy.",
		}
	}
	return reload.Result{
		Supported: false,
		Reloaded:  false,
		Message:   "Alteração salva! Reinicie o container do Email OAuth2 Proxy no Runtipi para aplicar.",
	}
}

// Handler returns the root http.Handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// StartMonitor starts background passive monitoring of emailproxy.log for external client OAuth requests.
func (s *Server) StartMonitor(ctx context.Context) {
	s.oauth.StartMonitor(ctx)
}

func (s *Server) registerRoutes() {
	// Auth endpoints (public)
	s.mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	// Automatic OAuth callback endpoint (matches active session by state/active flow)
	s.mux.HandleFunc("GET /oauth/callback", s.handleAutomaticOAuthCallback)

	// Protected API endpoints
	req := s.auth.Require
	s.mux.HandleFunc("GET /api/overview", req(s.handleOverview))
	s.mux.HandleFunc("GET /api/templates", req(s.handleTemplates))
	s.mux.HandleFunc("POST /api/templates/apply", req(s.handleApplyTemplate))

	s.mux.HandleFunc("POST /api/servers", req(s.handleUpsertServer))
	s.mux.HandleFunc("DELETE /api/servers/{section}", req(s.handleDeleteServer))

	s.mux.HandleFunc("POST /api/accounts", req(s.handleUpsertAccount))
	s.mux.HandleFunc("DELETE /api/accounts/{account}", req(s.handleDeleteAccount))
	s.mux.HandleFunc("DELETE /api/tokens/{account}", req(s.handleDeleteToken))

	s.mux.HandleFunc("POST /api/globals", req(s.handleSetGlobals))

	s.mux.HandleFunc("GET /api/config/raw", req(s.handleGetRawConfig))
	s.mux.HandleFunc("PUT /api/config/raw", req(s.handlePutRawConfig))
	s.mux.HandleFunc("POST /api/proxy/reload", req(s.handleReloadProxy))
	s.mux.HandleFunc("POST /api/proxy/toggle-debug", req(s.handleToggleDebug))
	s.mux.HandleFunc("POST /api/ports/check", req(s.handlePortsCheck))
	s.mux.HandleFunc("POST /api/ports/apply", req(s.handlePortsApply))

	s.mux.HandleFunc("POST /api/oauth/start", req(s.handleOAuthStart))
	s.mux.HandleFunc("GET /api/oauth/session/{id}", req(s.handleOAuthGet))
	s.mux.HandleFunc("POST /api/oauth/session/{id}/callback", req(s.handleOAuthCallback))
	s.mux.HandleFunc("POST /api/oauth/session/{id}/cancel", req(s.handleOAuthCancel))
	s.mux.HandleFunc("GET /api/oauth/pending", req(s.handleOAuthPendingList))
	s.mux.HandleFunc("POST /api/oauth/pending/{id}/callback", req(s.handleOAuthPendingCallback))
	s.mux.HandleFunc("DELETE /api/oauth/pending/{id}", req(s.handleOAuthPendingDismiss))

	s.mux.HandleFunc("GET /api/certs", req(s.handleGetCerts))
	s.mux.HandleFunc("POST /api/certs/generate", req(s.handleGenerateCerts))
	s.mux.HandleFunc("POST /api/certs/apply-tls", req(s.handleApplyTLS))
	s.mux.HandleFunc("GET /api/certs/ca.crt", req(s.handleDownloadCA))

	s.mux.HandleFunc("GET /api/logs", req(s.handleGetLogs))
	s.mux.HandleFunc("GET /api/logs/stream", req(s.handleStreamLogs))
	s.mux.HandleFunc("POST /api/logs/clear", req(s.handleClearLogs))
	s.mux.HandleFunc("GET /api/logs/download", req(s.handleDownloadLogs))

	// Static SPA frontend
	if s.opts.WebFS != nil {
		fileServer := http.FileServer(http.FS(s.opts.WebFS))
		s.mux.Handle("GET /", fileServer)
	}
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":       s.auth.Enabled(),
		"authenticated": s.auth.IsAuthenticated(r),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.auth.ValidateCredentials(body.Username, body.Password) {
		writeErr(w, http.StatusUnauthorized, "Usuário ou senha inválidos")
		return
	}
	s.auth.SetSession(w, r, body.Username)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.ClearSession(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	cfgINI, cfgExists, err := config.LoadINI(s.opts.ConfigFile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("Erro ao ler config: %v", err))
		return
	}
	cacheINI, cacheExists, err := config.LoadINI(s.opts.CredStoreFile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("Erro ao ler credstore: %v", err))
		return
	}

	servers := config.ExtractServers(cfgINI)
	accounts := config.ExtractAccounts(cfgINI, cacheINI, true)
	for i := range accounts {
		accounts[i].MatchedServers = config.MatchServersForAccount(accounts[i], servers)
	}
	globals := config.ExtractGlobals(cfgINI)
	probes := imapauth.ProbeServers(s.opts.ProxyHost, servers)
	certStatus := certs.Inspect(s.opts.ConfigDir)
	debugMode, debugKnown := reload.DetectDebugState(s.opts.LogFile)
	dockerPorts := reload.InspectDockerPortsOverview(s.opts.DockerSocket, s.opts.ProxyContainer, s.opts.ComposePath, s.opts.ComposeService, servers)

	writeJSON(w, http.StatusOK, map[string]any{
		"proxy_host":             s.opts.ProxyHost,
		"config_dir":             s.opts.ConfigDir,
		"config_exists":          cfgExists,
		"credstore_exists":       cacheExists,
		"auth_enabled":           s.auth.Enabled(),
		"restart_available":      reload.CanRestart(s.opts.DockerSocket, s.opts.ProxyContainer),
		"debug_mode":             debugMode,
		"debug_known":            debugKnown,
		"debug_toggle_available": reload.CanToggleDebug(s.opts.DockerSocket, s.opts.ProxyContainer),
		"docker_ports":           dockerPorts,
		"pending_auths":          s.oauth.ListPending(),
		"servers":                servers,
		"probes":                 probes,
		"accounts":               accounts,
		"globals":                globals,
		"certs":                  certStatus,
	})
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"templates": config.ProviderTemplates(),
	})
}

func (s *Server) handleApplyTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProviderID     string `json:"provider_id"`
		PresetID       string `json:"preset_id"`
		AccountTarget  string `json:"account_target"` // e.g. "@outlook.com" or "user@outlook.com"
		ClientID       string `json:"client_id"`
		ClientSecret   string `json:"client_secret"`
		RedirectURI    string `json:"redirect_uri"`
		CreateServers  bool   `json:"create_servers"`
		EnableTLS      bool   `json:"enable_tls"`
		EnableCatchAll bool   `json:"enable_catch_all"`
		PortMode       string `json:"port_mode"` // "", "none", "docker_only", "compose_and_docker", "compose_only"
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	tpl, preset := config.FindTemplate(body.ProviderID, body.PresetID)
	if tpl == nil {
		writeErr(w, http.StatusBadRequest, "Provedor desconhecido")
		return
	}

	accountName := strings.TrimSpace(body.AccountTarget)
	if accountName == "" && len(tpl.DefaultDomains) > 0 {
		accountName = tpl.DefaultDomains[0]
	}
	if !strings.Contains(accountName, "@") {
		writeErr(w, http.StatusBadRequest, "Conta ou domínio deve conter '@' (ex.: @outlook.com ou nome@outlook.com)")
		return
	}

	ini, _, err := config.LoadINI(s.opts.ConfigFile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	certStatus := certs.Inspect(s.opts.ConfigDir)
	useTLS := body.EnableTLS && certStatus.HasServerCert

	if body.CreateServers {
		if body.PortMode != "" && body.PortMode != "none" {
			if errMsg := s.verifyCandidatePortsAvailable(config.ExtractServers(ini), []int{tpl.IMAPPort, tpl.SMTPPort}); errMsg != "" {
				writeErr(w, http.StatusConflict, errMsg)
				return
			}
		}

		certPath, keyPath := "", ""
		if useTLS {
			certPath = certStatus.ContainerCertPath
			keyPath = certStatus.ContainerKeyPath
		}
		config.UpsertServer(ini, config.ServerEntry{
			Protocol:             "IMAP",
			LocalPort:            tpl.IMAPPort,
			LocalAddress:         "0.0.0.0",
			ServerAddress:        tpl.IMAPHost,
			ServerPort:           tpl.IMAPRemotePort,
			LocalCertificatePath: certPath,
			LocalKeyPath:         keyPath,
		})
		config.UpsertServer(ini, config.ServerEntry{
			Protocol:             "SMTP",
			LocalPort:            tpl.SMTPPort,
			LocalAddress:         "0.0.0.0",
			ServerAddress:        tpl.SMTPHost,
			ServerPort:           tpl.SMTPRemotePort,
			ServerStartTLS:       tpl.SMTPStartTLS,
			LocalCertificatePath: certPath,
			LocalKeyPath:         keyPath,
		})
	}

	permURL := tpl.PermissionURL
	clientID := strings.TrimSpace(body.ClientID)
	clientSecret := strings.TrimSpace(body.ClientSecret)
	redirectURI := strings.TrimSpace(body.RedirectURI)
	redirectListen := "http://0.0.0.0:80"
	usePKCE := false
	oauthFlow := ""

	if preset != nil {
		if preset.PermissionURLOverride != "" {
			permURL = preset.PermissionURLOverride
		}
		if clientID == "" {
			clientID = preset.ClientID
		}
		if clientSecret == "" {
			clientSecret = preset.ClientSecret
		}
		if redirectURI == "" {
			redirectURI = preset.RedirectURI
		}
		redirectListen = preset.RedirectListenAddress
		usePKCE = preset.UsePKCE
		oauthFlow = preset.OAuth2Flow
	}

	if clientID == "" {
		writeErr(w, http.StatusBadRequest, "Informe o client_id ou selecione o preset automático Mozilla Thunderbird")
		return
	}

	config.UpsertAccount(ini, config.AccountEntry{
		Account:               accountName,
		PermissionURL:         permURL,
		TokenURL:              tpl.TokenURL,
		OAuth2Scope:           tpl.OAuth2Scope,
		OAuth2Flow:            oauthFlow,
		RedirectURI:           redirectURI,
		RedirectListenAddress: redirectListen,
		ClientID:              clientID,
		ClientSecret:          clientSecret,
		UsePKCE:               usePKCE,
	}, false)

	globals := config.ExtractGlobals(ini)
	globals.DeleteAccountTokenOnPasswordError = false
	if body.EnableCatchAll || strings.HasPrefix(accountName, "@") {
		globals.AllowCatchAllAccounts = true
	}
	config.SetGlobals(ini, globals)

	if err := config.WriteFileWithBackup(s.opts.ConfigFile, ini.Render()); err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("Erro ao salvar config: %v", err))
		return
	}

	var portResult *reload.ApplyPortsResult
	reloadRes := s.pendingRestartHint()
	if body.CreateServers && body.PortMode != "" && body.PortMode != "none" {
		res := s.applyPortsForConfig(ini, body.PortMode)
		portResult = &res
		if res.OK {
			reloadRes = reload.Result{
				Supported: true,
				Reloaded:  res.DockerRecreated,
				Message:   res.Message,
			}
		} else {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":       res.Message,
				"port_result": res,
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"account":     accountName,
		"reload":      reloadRes,
		"port_result": portResult,
	})
}

func (s *Server) handleUpsertServer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		config.ServerEntry
		PortMode string `json:"port_mode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	proto := strings.ToUpper(strings.TrimSpace(body.Protocol))
	if proto != "IMAP" && proto != "SMTP" && proto != "POP" {
		writeErr(w, http.StatusBadRequest, "Protocolo deve ser IMAP, SMTP ou POP")
		return
	}
	if body.LocalPort < 1024 || body.LocalPort > 65535 {
		writeErr(w, http.StatusBadRequest, "Porta local deve estar entre 1024 e 65535")
		return
	}
	if strings.TrimSpace(body.ServerAddress) == "" || body.ServerPort <= 0 {
		writeErr(w, http.StatusBadRequest, "Informe o endereço e a porta do servidor remoto")
		return
	}

	ini, _, err := config.LoadINI(s.opts.ConfigFile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if body.PortMode != "" && body.PortMode != "none" {
		if errMsg := s.verifyCandidatePortsAvailable(config.ExtractServers(ini), []int{body.LocalPort}); errMsg != "" {
			writeErr(w, http.StatusConflict, errMsg)
			return
		}
	}

	// If renaming an existing section, remove old one
	if body.Section != "" && body.Section != fmt.Sprintf("%s-%d", proto, body.LocalPort) {
		ini.DeleteSection(body.Section)
	}

	secName := config.UpsertServer(ini, body.ServerEntry)
	if err := config.WriteFileWithBackup(s.opts.ConfigFile, ini.Render()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	var portResult *reload.ApplyPortsResult
	reloadRes := s.pendingRestartHint()
	if body.PortMode != "" && body.PortMode != "none" {
		res := s.applyPortsForConfig(ini, body.PortMode)
		portResult = &res
		if res.OK {
			reloadRes = reload.Result{
				Supported: true,
				Reloaded:  res.DockerRecreated,
				Message:   res.Message,
			}
		} else {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":       res.Message,
				"port_result": res,
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"section":     secName,
		"reload":      reloadRes,
		"port_result": portResult,
	})
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	section := r.PathValue("section")
	ini, exists, err := config.LoadINI(s.opts.ConfigFile)
	if err != nil || !exists {
		writeErr(w, http.StatusNotFound, "Arquivo de configuração não encontrado")
		return
	}
	if !ini.DeleteSection(section) {
		writeErr(w, http.StatusNotFound, "Servidor não encontrado")
		return
	}
	if err := config.WriteFileWithBackup(s.opts.ConfigFile, ini.Render()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reload": s.pendingRestartHint()})
}

func (s *Server) handleUpsertAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		config.AccountEntry
		OriginalAccount    string `json:"original_account"`
		KeepExistingSecret bool   `json:"keep_existing_secret"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !strings.Contains(body.Account, "@") {
		writeErr(w, http.StatusBadRequest, "Informe um e-mail ou catch-all (ex.: @outlook.com)")
		return
	}
	if strings.TrimSpace(body.TokenURL) == "" || strings.TrimSpace(body.ClientID) == "" {
		writeErr(w, http.StatusBadRequest, "token_url e client_id são obrigatórios")
		return
	}
	flow := strings.TrimSpace(body.OAuth2Flow)
	if strings.TrimSpace(body.OAuth2Scope) == "" && !(flow == "password" && strings.TrimSpace(body.OAuth2Resource) != "") {
		writeErr(w, http.StatusBadRequest, "oauth2_scope é obrigatório (ou oauth2_resource no fluxo password)")
		return
	}
	// Non-interactive flows must not specify permission_url (emailproxy.py uses its absence to select the flow)
	if flow == "client_credentials" || flow == "password" || flow == "service_account" {
		body.PermissionURL = ""
		body.RedirectURI = ""
		body.RedirectListenAddress = ""
	}
	if flow == "device" {
		body.RedirectURI = ""
		body.RedirectListenAddress = ""
	}

	ini, _, err := config.LoadINI(s.opts.ConfigFile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if body.OriginalAccount != "" && body.OriginalAccount != body.Account {
		if old := ini.GetSection(body.OriginalAccount); old != nil && body.KeepExistingSecret && body.ClientSecret == "" {
			body.ClientSecret = old.Get("client_secret", "")
		}
		ini.DeleteSection(body.OriginalAccount)
	}

	secName := config.UpsertAccount(ini, body.AccountEntry, body.KeepExistingSecret)
	if strings.HasPrefix(secName, "@") {
		g := config.ExtractGlobals(ini)
		g.AllowCatchAllAccounts = true
		config.SetGlobals(ini, g)
	}

	if err := config.WriteFileWithBackup(s.opts.ConfigFile, ini.Render()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "account": secName, "reload": s.pendingRestartHint()})
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	clearToken := r.URL.Query().Get("clear_token") == "true"

	ini, exists, err := config.LoadINI(s.opts.ConfigFile)
	if err == nil && exists {
		if ini.DeleteSection(account) {
			if err := config.WriteFileWithBackup(s.opts.ConfigFile, ini.Render()); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

	if clearToken {
		if cacheINI, cExists, err := config.LoadINI(s.opts.CredStoreFile); err == nil && cExists {
			if cacheINI.DeleteSection(account) {
				_ = config.WriteFileWithBackup(s.opts.CredStoreFile, cacheINI.Render())
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reload": s.pendingRestartHint()})
}

func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	removed := false
	tokenKeys := []string{"access_token", "access_token_expiry", "refresh_token", "token_salt", "token_iterations", "last_activity"}

	// Clear only cached token keys in credstore.config while keeping the account section,
	// so catch-all accounts stay listed on the Dashboard as "Pendente" with the "Autenticar" button.
	if cacheINI, exists, err := config.LoadINI(s.opts.CredStoreFile); err == nil && exists {
		if sec := cacheINI.GetSection(account); sec != nil {
			changed := false
			for _, k := range tokenKeys {
				if sec.Get(k, "") != "" {
					sec.Delete(k)
					changed = true
				}
			}
			if changed {
				if err := config.WriteFileWithBackup(s.opts.CredStoreFile, cacheINI.Render()); err != nil {
					writeErr(w, http.StatusInternalServerError, err.Error())
					return
				}
				removed = true
			}
		}
	}

	// Also clean inline tokens if stored directly in emailproxy.config
	if cfgINI, exists, err := config.LoadINI(s.opts.ConfigFile); err == nil && exists {
		if sec := cfgINI.GetSection(account); sec != nil {
			changed := false
			for _, k := range tokenKeys {
				if sec.Get(k, "") != "" {
					sec.Delete(k)
					changed = true
				}
			}
			if changed {
				_ = config.WriteFileWithBackup(s.opts.ConfigFile, cfgINI.Render())
				removed = true
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removed": removed, "reload": s.pendingRestartHint()})
}

func (s *Server) handleSetGlobals(w http.ResponseWriter, r *http.Request) {
	var body config.GlobalOptions
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ini, _, err := config.LoadINI(s.opts.ConfigFile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	config.SetGlobals(ini, body)
	if err := config.WriteFileWithBackup(s.opts.ConfigFile, ini.Render()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reload": s.pendingRestartHint()})
}

func (s *Server) handleGetRawConfig(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(s.opts.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{"exists": false, "content": ""})
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"exists": true, "content": string(data)})
}

func (s *Server) handlePutRawConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := config.WriteFileWithBackup(s.opts.ConfigFile, body.Content); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reload": s.pendingRestartHint()})
}

func (s *Server) handleReloadProxy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, reload.RestartProxy(s.opts.DockerSocket, s.opts.ProxyContainer))
}

func (s *Server) handleToggleDebug(w http.ResponseWriter, r *http.Request) {
	res := reload.ToggleDebug(s.opts.DockerSocket, s.opts.ProxyContainer)
	if res.Reloaded {
		// Brief wait for emailproxy.py to write "Setting debug mode: True/False" into the log
		time.Sleep(300 * time.Millisecond)
	}
	debugMode, debugKnown := reload.DetectDebugState(s.opts.LogFile)
	writeJSON(w, http.StatusOK, map[string]any{
		"supported":   res.Supported,
		"reloaded":    res.Reloaded,
		"message":     res.Message,
		"debug_mode":  debugMode,
		"debug_known": debugKnown,
	})
}

func (s *Server) handleOAuthPendingList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"pending": s.oauth.ListPending(),
	})
}

func (s *Server) handleOAuthPendingCallback(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	proxyResp, err := s.oauth.SubmitPendingCallback(id, body.URL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"proxy_response": proxyResp,
		"pending":        s.oauth.ListPending(),
	})
}

func (s *Server) handleOAuthPendingDismiss(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.oauth.DismissPending(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"pending": s.oauth.ListPending(),
	})
}

func (s *Server) handlePortsCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ports []int `json:"ports"`
	}
	_ = decodeJSON(r, &body)

	ini, _, err := config.LoadINI(s.opts.ConfigFile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	servers := config.ExtractServers(ini)

	candidates := body.Ports
	if len(candidates) == 0 {
		for _, srv := range servers {
			candidates = append(candidates, srv.LocalPort)
		}
	}

	statuses, probeErr := reload.CheckPortsWithHostProbe(s.opts.DockerSocket, s.opts.ProxyContainer, servers, candidates)
	overview := reload.InspectDockerPortsOverview(s.opts.DockerSocket, s.opts.ProxyContainer, s.opts.ComposePath, s.opts.ComposeService, servers)
	overview.Statuses = statuses

	resp := map[string]any{
		"ok":           probeErr == nil,
		"statuses":     statuses,
		"docker_ports": overview,
	}
	if probeErr != nil {
		resp["warning"] = probeErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePortsApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode  string            `json:"mode"` // "docker_only", "compose_and_docker", "compose_only"
		Ports []int             `json:"ports"`
		Specs []reload.PortSpec `json:"specs"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	ini, _, err := config.LoadINI(s.opts.ConfigFile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	servers := config.ExtractServers(ini)
	srvByPort := make(map[int]config.ServerEntry, len(servers))
	for _, srv := range servers {
		srvByPort[srv.LocalPort] = srv
	}

	specs := body.Specs
	if len(specs) == 0 {
		if len(body.Ports) == 0 {
			specs = reload.SpecsFromServers(servers)
		} else {
			for _, p := range body.Ports {
				if srv, ok := srvByPort[p]; ok {
					specs = append(specs, reload.PortSpec{
						Port:          p,
						Section:       srv.Section,
						ServerAddress: srv.ServerAddress,
					})
				} else {
					specs = append(specs, reload.PortSpec{
						Port:    p,
						Section: fmt.Sprintf("PORT-%d", p),
					})
				}
			}
		}
	}

	res := reload.ApplyPorts(s.opts.DockerSocket, s.opts.ProxyContainer, s.opts.ComposePath, s.opts.ComposeService, body.Mode, specs)
	overview := reload.InspectDockerPortsOverview(s.opts.DockerSocket, s.opts.ProxyContainer, s.opts.ComposePath, s.opts.ComposeService, servers)
	if !res.OK {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":        res.Message,
			"result":       res,
			"docker_ports": overview,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"result":       res,
		"docker_ports": overview,
	})
}

func (s *Server) verifyCandidatePortsAvailable(existingServers []config.ServerEntry, ports []int) string {
	statuses, _ := reload.CheckPortsWithHostProbe(s.opts.DockerSocket, s.opts.ProxyContainer, existingServers, ports)
	portSet := make(map[int]bool, len(ports))
	for _, p := range ports {
		portSet[p] = true
	}
	for _, st := range statuses {
		if !portSet[st.Port] {
			continue
		}
		if st.Status == "conflict_docker" || st.Status == "conflict_host" {
			return fmt.Sprintf("Porta %d indisponível (%s) — escolha outra porta ou salve apenas no emailproxy.config.", st.Port, st.Message)
		}
	}
	return ""
}

func (s *Server) applyPortsForConfig(ini *config.INIFile, mode string) reload.ApplyPortsResult {
	servers := config.ExtractServers(ini)
	ov := reload.InspectDockerPortsOverview(s.opts.DockerSocket, s.opts.ProxyContainer, s.opts.ComposePath, s.opts.ComposeService, servers)

	specs := reload.SpecsFromServers(servers)
	// Preserve any non-80 extra ports that were already published on the proxy container
	protSet := make(map[int]bool, len(ov.ProtectedPorts))
	for _, p := range ov.ProtectedPorts {
		protSet[p] = true
	}
	inSpecs := make(map[int]bool, len(specs))
	for _, sp := range specs {
		inSpecs[sp.Port] = true
	}
	for _, p := range ov.PublishedPorts {
		if !protSet[p] && !inSpecs[p] {
			specs = append(specs, reload.PortSpec{Port: p, Section: fmt.Sprintf("PORT-%d", p)})
		}
	}

	return reload.ApplyPorts(s.opts.DockerSocket, s.opts.ProxyContainer, s.opts.ComposePath, s.opts.ComposeService, mode, specs)
}

func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	var body imapauth.StartRequest
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	sess, err := s.oauth.Start(body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleOAuthGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, ok := s.oauth.Get(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "Sessão de autenticação não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	proxyResp, err := s.oauth.SubmitCallback(id, body.URL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	sess, _ := s.oauth.Get(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"proxy_response": proxyResp,
		"session":        sess,
	})
}

func (s *Server) handleOAuthCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.oauth.Cancel(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAutomaticOAuthCallback(w http.ResponseWriter, r *http.Request) {
	sess, _, err := s.oauth.SubmitCallbackAuto(r.URL.RawQuery)
	ok := err == nil
	title := "Autenticação recebida!"
	msg := "Código OAuth enviado ao Email OAuth2 Proxy. Esta janela será fechada automaticamente."
	sessionID := ""
	if sess != nil {
		sessionID = sess.ID
	}
	if err != nil {
		title = "Não foi possível concluir o callback"
		msg = err.Error()
	}

	payloadJSON, _ := json.Marshal(map[string]any{
		"type":       "eop_oauth_callback",
		"ok":         ok,
		"session_id": sessionID,
		"message":    msg,
	})

	statusColor := "#22c55e"
	statusCode := http.StatusOK
	if !ok {
		statusColor = "#ef4444"
		statusCode = http.StatusBadRequest
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>%s</title>
<style>
  body { margin: 0; font-family: system-ui, -apple-system, sans-serif; background: #070708; color: #f1f1f4; display: grid; place-items: center; min-height: 100vh; padding: 20px; }
  .box { max-width: 440px; width: 100%%; background: #131317; border: 1px solid %s; border-radius: 12px; padding: 24px; text-align: center; }
  h2 { margin: 0 0 10px; color: %s; font-size: 18px; }
  p { margin: 0 0 16px; color: #8e8e9a; font-size: 13.5px; line-height: 1.5; }
  button { background: #3b82f6; color: #fff; border: 0; border-radius: 8px; padding: 8px 16px; font-weight: 600; cursor: pointer; }
</style>
</head>
<body>
  <div class="box">
    <h2>%s</h2>
    <p>%s</p>
    <button onclick="window.close()">Fechar janela</button>
  </div>
  <script>
    (function() {
      var payload = %s;
      try {
        if (window.opener && !window.opener.closed) {
          window.opener.postMessage(payload, window.location.origin);
        }
      } catch (e) {}
      if (payload.ok) {
        setTimeout(function() { window.close(); }, 900);
      }
    })();
  </script>
</body>
</html>`,
		html.EscapeString(title),
		statusColor,
		statusColor,
		html.EscapeString(title),
		html.EscapeString(msg),
		string(payloadJSON),
	)
}

func (s *Server) handleGetCerts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, certs.Inspect(s.opts.ConfigDir))
}

func (s *Server) handleGenerateCerts(w http.ResponseWriter, r *http.Request) {
	var body struct {
		certs.GenerateRequest
		ApplyToAllServers bool `json:"apply_to_all_servers"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(body.Hosts) == 0 {
		writeErr(w, http.StatusBadRequest, "Informe pelo menos um IP ou hostname do servidor")
		return
	}
	status, err := certs.Generate(s.opts.ConfigDir, body.GenerateRequest)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	var reloadRes *reload.Result
	if body.ApplyToAllServers {
		if ini, exists, err := config.LoadINI(s.opts.ConfigFile); err == nil && exists {
			for _, srv := range config.ExtractServers(ini) {
				srv.LocalCertificatePath = status.ContainerCertPath
				srv.LocalKeyPath = status.ContainerKeyPath
				config.UpsertServer(ini, srv)
			}
			_ = config.WriteFileWithBackup(s.opts.ConfigFile, ini.Render())
			res := s.pendingRestartHint()
			reloadRes = &res
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"certs":  status,
		"reload": reloadRes,
	})
}

func (s *Server) handleApplyTLS(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enable bool `json:"enable"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	st := certs.Inspect(s.opts.ConfigDir)
	if body.Enable && !st.HasServerCert {
		writeErr(w, http.StatusBadRequest, "Gere primeiro o certificado TLS antes de ativar nos servidores")
		return
	}
	ini, exists, err := config.LoadINI(s.opts.ConfigFile)
	if err != nil || !exists {
		writeErr(w, http.StatusNotFound, "emailproxy.config não encontrado")
		return
	}
	for _, srv := range config.ExtractServers(ini) {
		if body.Enable {
			srv.LocalCertificatePath = st.ContainerCertPath
			srv.LocalKeyPath = st.ContainerKeyPath
		} else {
			srv.LocalCertificatePath = ""
			srv.LocalKeyPath = ""
			srv.LocalStartTLS = false
		}
		config.UpsertServer(ini, srv)
	}
	if err := config.WriteFileWithBackup(s.opts.ConfigFile, ini.Render()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reload": s.pendingRestartHint()})
}

func (s *Server) handleDownloadCA(w http.ResponseWriter, r *http.Request) {
	data, err := certs.ReadCACert(s.opts.ConfigDir)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Certificado CA (ca-cert.pem) ainda não foi gerado")
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="ca-cert.crt"`)
	_, _ = w.Write(data)
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	maxLines := 300
	if q := r.URL.Query().Get("lines"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 5000 {
			maxLines = n
		}
	}
	lines, size, err := logs.ReadTail(s.opts.LogFile, maxLines)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lines": lines,
		"size":  size,
	})
}

func (s *Server) handleStreamLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "Streaming não suportado")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	initial, size, _ := logs.ReadTail(s.opts.LogFile, 150)
	for _, line := range initial {
		payload, _ := json.Marshal(line)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
	}
	flusher.Flush()

	logs.Stream(r.Context(), s.opts.LogFile, size, func(line string) {
		payload, _ := json.Marshal(line)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	})
}

func (s *Server) handleClearLogs(w http.ResponseWriter, r *http.Request) {
	if err := logs.Clear(s.opts.LogFile); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDownloadLogs(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open(s.opts.LogFile)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Arquivo de log ainda não existe")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="emailproxy.log"`)
	_, _ = io.Copy(w, f)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 2*1024*1024))
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}
