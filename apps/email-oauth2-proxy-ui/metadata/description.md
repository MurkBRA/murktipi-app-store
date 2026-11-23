# Email OAuth2 Proxy UI

Painel web leve em **Go** (imagem oficial compilada via GitHub Actions em `ghcr.io/murkbra/email-oauth2-proxy-ui:latest`, multi-arch `amd64`/`arm64`) para gerenciar uma instalação existente do **[Email OAuth2 Proxy](https://github.com/simonrob/email-oauth2-proxy)** no seu **Runtipi**.

---

## Principais Recursos

- **Assistente OAuth 1-Clique & Monitor Externo:** dispara o login IMAP/TLS direto no proxy e detecta pedidos OAuth pendentes enviados por clientes externos (MailCraft, Thunderbird, impressoras, etc.), com rota automática `/oauth/callback`.
- **Dashboard & Probes em Tempo Real:** testa as portas IMAP/SMTP/POP configuradas e mostra o status dos tokens salvos no `credstore.config`.
- **Templates & Fluxos Avançados:** presets prontos para **Outlook / Office 365**, **Gmail**, **Yahoo**, **AOL** e **Fastmail** (Mozilla Thunderbird, FairEmail, GNOME Online Accounts, Mailspring) e suporte a Authorization Code, Device Flow, Client Credentials (CCG), ROPCG, Google Service Account e JWT.
- **Gerador de Certificado TLS & Download do `ca-cert.crt`:** gera CA local X.509 v3 + certificado com SANs, aplica nos servidores com 1 clique e permite baixar o `ca-cert.crt`.
- **Gerenciador de Portas Docker & Compose:** verifica conflitos reais no host e no Docker via `/var/run/docker.sock`, suportando edição direta no `docker-compose.yml` do `user-config` do `email-oauth2-proxy` (Modo A) e/ou atualização direta no container (Modo B).
- **Editor do `emailproxy.config`, Debug ao Vivo (`SIGUSR1`) & Streaming de Logs:** edição estruturada ou raw com backup automático `.bak` e visualização de `/config/emailproxy.log`.

---

## Integração Automática com o Runtipi (`ROOT_FOLDER_HOST`)

No Runtipi v4, a variável interna **`ROOT_FOLDER_HOST`** é injetada automaticamente no `app.env` de todos os aplicativos e aponta para a pasta raiz do Runtipi (onde ficam `app-data/` e `user-config/`). O container monta automaticamente:

- **`/config`** ← `${ROOT_FOLDER_HOST}/app-data/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy/data/config`
- **`/compose`** ← `${ROOT_FOLDER_HOST}/user-config/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy`
- **`/var/run/docker.sock`** ← Socket Docker do host

| Variável | Obrigatória | Padrão | Descrição |
| :--- | :---: | :---: | :--- |
| `PROXY_STORE_ID` | Sim | `migrated` | Subpasta da loja onde o `email-oauth2-proxy` está instalado (`migrated` para apps migrados ou `runtipi` para instalação nova da loja oficial). |
| `PROXY_HOST` | Sim | `email-oauth2-proxy` | Hostname do container proxy na rede `tipi_main_network`. |
| `PROXY_CONTAINER` | Não | `email-oauth2-proxy` | Nome exato do container Docker do proxy para reinício e `SIGUSR1`. |
| `ADMIN_USER` | Não | `admin` | Usuário de acesso ao painel (quando `ADMIN_PASSWORD` estiver definida). |
| `ADMIN_PASSWORD` | Não | *(vazio)* | Senha do painel Web UI (deixe vazio para acesso sem login). |
