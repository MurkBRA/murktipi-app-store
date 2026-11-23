package config

// CredentialPreset describes a known, consolidated public OAuth2 client (such as Mozilla Thunderbird,
// FairEmail, GNOME Online Accounts, or Mailspring) or a blank custom credential template for a provider.
type CredentialPreset struct {
	ID                    string `json:"id"`
	Label                 string `json:"label"`
	SourceNote            string `json:"source_note"`
	ClientID              string `json:"client_id"`
	ClientSecret          string `json:"client_secret"`
	RedirectURI           string `json:"redirect_uri"`
	RedirectListenAddress string `json:"redirect_listen_address"`
	UsePKCE               bool   `json:"use_pkce"`
	OAuth2Flow            string `json:"oauth2_flow"`
	PermissionURLOverride string `json:"permission_url_override,omitempty"`
}

// ProviderTemplate defines a complete ready-to-apply template for an email provider.
type ProviderTemplate struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	DefaultDomains []string           `json:"default_domains"`
	IMAPPort       int                `json:"imap_port"`
	IMAPHost       string             `json:"imap_host"`
	IMAPRemotePort int                `json:"imap_remote_port"`
	SMTPPort       int                `json:"smtp_port"`
	SMTPHost       string             `json:"smtp_host"`
	SMTPRemotePort int                `json:"smtp_remote_port"`
	SMTPStartTLS   bool               `json:"smtp_starttls"`
	PermissionURL  string             `json:"permission_url"`
	TokenURL       string             `json:"token_url"`
	OAuth2Scope    string             `json:"oauth2_scope"`
	Presets        []CredentialPreset `json:"presets"`
}

// ProviderTemplates returns verified presets extracted from open-source email clients
// officially referenced in simonrob/email-oauth2-proxy's documentation.
func ProviderTemplates() []ProviderTemplate {
	return []ProviderTemplate{
		{
			ID:             "outlook",
			Name:           "Outlook / Hotmail / Office 365",
			Description:    "Contas pessoais (@outlook.com, @hotmail.com, @live.com) e Microsoft 365.",
			DefaultDomains: []string{"@outlook.com", "@hotmail.com", "@live.com"},
			IMAPPort:       1993,
			IMAPHost:       "outlook.office365.com",
			IMAPRemotePort: 993,
			SMTPPort:       1587,
			SMTPHost:       "smtp-mail.outlook.com",
			SMTPRemotePort: 587,
			SMTPStartTLS:   true,
			PermissionURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL:       "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			OAuth2Scope:    "https://outlook.office.com/IMAP.AccessAsUser.All https://outlook.office.com/POP.AccessAsUser.All https://outlook.office.com/SMTP.Send offline_access",
			Presets: []CredentialPreset{
				{
					ID:                    "thunderbird",
					Label:                 "Mozilla Thunderbird (Recomendado / Automático)",
					SourceNote:            "Client ID público oficial do Mozilla Thunderbird (OAuth2Providers.sys.mjs). Sem client_secret; redirect_uri https://localhost.",
					ClientID:              "9e5f94bc-e8a4-4e73-b8be-63364c29d753",
					ClientSecret:          "",
					RedirectURI:           "https://localhost",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               false,
					OAuth2Flow:            "",
				},
				{
					ID:                    "fairemail",
					Label:                 "FairEmail (Backup Open-Source)",
					SourceNote:            "Credenciais públicas do cliente open-source FairEmail (M66B/FairEmail providers.xml). Redireciona para https://oauth.faircode.eu/?code=... — basta copiar a URL e colar no Passo B.",
					ClientID:              "3514cf2c-e7a3-45a2-80d4-6a3c3498eca0",
					ClientSecret:          "r=o4@SlzU:MA6MlXM6mPG[44i4gYPq[@",
					RedirectURI:           "https://oauth.faircode.eu/",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               false,
					OAuth2Flow:            "",
				},
				{
					ID:                    "mailspring",
					Label:                 "Mailspring (Backup Open-Source + PKCE)",
					SourceNote:            "Client ID público do Mailspring para Office 365/Outlook (Foundry376/Mailspring). Usa PKCE e redirect http://localhost:12141/desktop.",
					ClientID:              "8787a430-6eee-41e1-b914-681d90d35625",
					ClientSecret:          "",
					RedirectURI:           "http://localhost:12141/desktop",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
				{
					ID:                    "custom_auth_code",
					Label:                 "Custom (Meu app no Microsoft Entra / Azure)",
					SourceNote:            "Use seu próprio Application (client) ID registrado no portal Azure.",
					ClientID:              "",
					ClientSecret:          "",
					RedirectURI:           "http://localhost:1999",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               false,
					OAuth2Flow:            "",
				},
				{
					ID:                    "custom_device",
					Label:                 "Custom — Device Flow (Entra / Azure sem redirect)",
					SourceNote:            "Fluxo de código de dispositivo (microsoft.com/devicelogin). Exige app com 'Allow public client flows' ativado no Azure.",
					ClientID:              "",
					ClientSecret:          "",
					RedirectURI:           "",
					RedirectListenAddress: "",
					UsePKCE:               false,
					OAuth2Flow:            "device",
					PermissionURLOverride: "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode",
				},
			},
		},
		{
			ID:             "gmail",
			Name:           "Gmail / Google Workspace",
			Description:    "Contas @gmail.com pessoais ou Google Workspace com domínio próprio.",
			DefaultDomains: []string{"@gmail.com"},
			IMAPPort:       2993,
			IMAPHost:       "imap.gmail.com",
			IMAPRemotePort: 993,
			SMTPPort:       2465,
			SMTPHost:       "smtp.gmail.com",
			SMTPRemotePort: 465,
			SMTPStartTLS:   false,
			PermissionURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL:       "https://oauth2.googleapis.com/token",
			OAuth2Scope:    "https://mail.google.com/",
			Presets: []CredentialPreset{
				{
					ID:                    "thunderbird",
					Label:                 "Mozilla Thunderbird (Recomendado / Automático)",
					SourceNote:            "Credenciais públicas oficiais do Mozilla Thunderbird (OAuth2Providers.sys.mjs). Evita expiração de 7 dias do modo Testing do Google Cloud.",
					ClientID:              "406964657835-aq8lmia8j95dhl1a2bvharmfk3t1hgqj.apps.googleusercontent.com",
					ClientSecret:          "kSmqreRr0qwBWJgbf5Y-PjSU",
					RedirectURI:           "http://localhost:1999",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
				{
					ID:                    "gnome_online_accounts",
					Label:                 "GNOME Online Accounts (Backup Open-Source)",
					SourceNote:            "Credenciais públicas do projeto GNOME Online Accounts (gitlab.gnome.org/GNOME/gnome-online-accounts). Desktop app verificado pelo Google.",
					ClientID:              "44438659992-7kgjeitenc16ssihbtdjbgguch7ju55s.apps.googleusercontent.com",
					ClientSecret:          "-gMLuQyDiI0XrQS_vx_mhuYF",
					RedirectURI:           "http://localhost:1999",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
				{
					ID:                    "custom_google",
					Label:                 "Custom (Meu app no Google Cloud Console)",
					SourceNote:            "Use Client ID e Client Secret de um Desktop App criado no Google Cloud Console.",
					ClientID:              "",
					ClientSecret:          "",
					RedirectURI:           "http://localhost:1999",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
			},
		},
		{
			ID:             "yahoo",
			Name:           "Yahoo Mail",
			Description:    "Contas @yahoo.com e @yahoo.com.br (Yahoo não aceita novos registros OAuth de e-mail).",
			DefaultDomains: []string{"@yahoo.com", "@yahoo.com.br"},
			IMAPPort:       3993,
			IMAPHost:       "imap.mail.yahoo.com",
			IMAPRemotePort: 993,
			SMTPPort:       3465,
			SMTPHost:       "smtp.mail.yahoo.com",
			SMTPRemotePort: 465,
			SMTPStartTLS:   false,
			PermissionURL:  "https://api.login.yahoo.com/oauth2/request_auth",
			TokenURL:       "https://api.login.yahoo.com/oauth2/get_token",
			OAuth2Scope:    "mail-w",
			Presets: []CredentialPreset{
				{
					ID:                    "thunderbird",
					Label:                 "Mozilla Thunderbird (Recomendado / PKCE)",
					SourceNote:            "Client ID público do Mozilla Thunderbird para Yahoo (OAuth2Providers.sys.mjs). Usa PKCE sem client_secret e redirect https://127.0.0.1.",
					ClientID:              "dj0yJmk9WVZUaWRNUUZSQTBNJmQ9WVdrOVNqbHJUMGhtTkU4bWNHbzlNQT09JnM9Y29uc3VtZXJzZWNyZXQmc3Y9MCZ4PTgz",
					ClientSecret:          "",
					RedirectURI:           "https://127.0.0.1",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
				{
					ID:                    "fairemail_pkce",
					Label:                 "FairEmail — PKCE (Backup Open-Source)",
					SourceNote:            "Client ID público do FairEmail para Yahoo (yahoo2 em M66B/FairEmail). Redireciona para https://oauth.faircode.eu/?code=....",
					ClientID:              "dj0yJmk9RnRORGNuNTlnaXlUJmQ9WVdrOVVrMXViV2RDU1dZbWNHbzlNQT09JnM9Y29uc3VtZXJzZWNyZXQmc3Y9MCZ4PTBm",
					ClientSecret:          "",
					RedirectURI:           "https://oauth.faircode.eu/",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
				{
					ID:                    "fairemail_secret",
					Label:                 "FairEmail — Client Secret (Backup 2)",
					SourceNote:            "Credencial clássica do FairEmail para Yahoo com client_secret (M66B/FairEmail). Redireciona para https://oauth.faircode.eu/?code=....",
					ClientID:              "dj0yJmk9SWtNZm1vUFc2THJDJnM9Y29uc3VtZXJzZWNyZXQmc3Y9MCZ4PWRi",
					ClientSecret:          "261a91fbbdb2852c4e80fdd3d1db4a8ebfa4c42b",
					RedirectURI:           "https://oauth.faircode.eu/",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               false,
					OAuth2Flow:            "",
				},
			},
		},
		{
			ID:             "aol",
			Name:           "AOL Mail",
			Description:    "Contas @aol.com (mesmo modelo do Yahoo, requer cliente já aprovado).",
			DefaultDomains: []string{"@aol.com"},
			IMAPPort:       4993,
			IMAPHost:       "imap.aol.com",
			IMAPRemotePort: 993,
			SMTPPort:       4465,
			SMTPHost:       "smtp.aol.com",
			SMTPRemotePort: 465,
			SMTPStartTLS:   false,
			PermissionURL:  "https://api.login.aol.com/oauth2/request_auth",
			TokenURL:       "https://api.login.aol.com/oauth2/get_token",
			OAuth2Scope:    "mail-w",
			Presets: []CredentialPreset{
				{
					ID:                    "thunderbird",
					Label:                 "Mozilla Thunderbird (Recomendado / PKCE)",
					SourceNote:            "Client ID público do Mozilla Thunderbird para AOL (OAuth2Providers.sys.mjs).",
					ClientID:              "dj0yJmk9MGNoTTQ0SjhIN1dSJmQ9WVdrOVdIVnFkVVp2UkZNbWNHbzlNQT09JnM9Y29uc3VtZXJzZWNyZXQmc3Y9MCZ4PTNm",
					ClientSecret:          "",
					RedirectURI:           "https://127.0.0.1",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
				{
					ID:                    "fairemail",
					Label:                 "FairEmail (Backup Open-Source)",
					SourceNote:            "Client ID público do FairEmail para AOL (M66B/FairEmail). Redireciona para https://oauth.faircode.eu/?code=....",
					ClientID:              "dj0yJmk9SFVJYWVtS21RMjYyJmQ9WVdrOVJVcENRMXBTT1drbWNHbzlNQT09JnM9Y29uc3VtZXJzZWNyZXQmc3Y9MCZ4PTJh",
					ClientSecret:          "",
					RedirectURI:           "https://oauth.faircode.eu/",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
			},
		},
		{
			ID:             "fastmail",
			Name:           "Fastmail",
			Description:    "Contas Fastmail com OAuth 2.0 + PKCE.",
			DefaultDomains: []string{"@fastmail.com"},
			IMAPPort:       5993,
			IMAPHost:       "imap.fastmail.com",
			IMAPRemotePort: 993,
			SMTPPort:       5465,
			SMTPHost:       "smtp.fastmail.com",
			SMTPRemotePort: 465,
			SMTPStartTLS:   false,
			PermissionURL:  "https://api.fastmail.com/oauth/authorize",
			TokenURL:       "https://api.fastmail.com/oauth/refresh",
			OAuth2Scope:    "https://www.fastmail.com/dev/protocol-imap https://www.fastmail.com/dev/protocol-pop https://www.fastmail.com/dev/protocol-smtp",
			Presets: []CredentialPreset{
				{
					ID:                    "thunderbird",
					Label:                 "Mozilla Thunderbird (Recomendado / PKCE)",
					SourceNote:            "Client ID público do Mozilla Thunderbird para Fastmail (OAuth2Providers.sys.mjs).",
					ClientID:              "35f141ae",
					ClientSecret:          "",
					RedirectURI:           "http://127.0.0.1/fastmail",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
				{
					ID:                    "fairemail",
					Label:                 "FairEmail (Backup Open-Source + PKCE)",
					SourceNote:            "Client ID público do FairEmail para Fastmail (M66B/FairEmail). Redireciona para https://oauth.faircode.eu/?code=....",
					ClientID:              "353941ae",
					ClientSecret:          "",
					RedirectURI:           "https://oauth.faircode.eu/",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
				{
					ID:                    "custom_fastmail",
					Label:                 "Custom (Meu Client ID Fastmail)",
					SourceNote:            "Registre um cliente OAuth em fastmail.com/dev.",
					ClientID:              "",
					ClientSecret:          "",
					RedirectURI:           "http://localhost:1999",
					RedirectListenAddress: "http://0.0.0.0:80",
					UsePKCE:               true,
					OAuth2Flow:            "",
				},
			},
		},
	}
}

// FindTemplate looks up a provider template and preset by ID.
func FindTemplate(providerID, presetID string) (*ProviderTemplate, *CredentialPreset) {
	for _, tpl := range ProviderTemplates() {
		if tpl.ID == providerID {
			t := tpl
			if presetID == "" && len(t.Presets) > 0 {
				p := t.Presets[0]
				return &t, &p
			}
			for _, pr := range t.Presets {
				if pr.ID == presetID {
					p := pr
					return &t, &p
				}
			}
			return &t, nil
		}
	}
	return nil, nil
}
