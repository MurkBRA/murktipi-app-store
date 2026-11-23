# Email OAuth2 Proxy UI

Painel web leve em **Go** (zero dependências externas) para gerenciar uma instalação existente do **[Email OAuth2 Proxy](https://github.com/simonrob/email-oauth2-proxy)** no seu **Runtipi**, sem depender de registro privado no GHCR: na subida, o container baixa o código-fonte diretamente deste repositório e compila o binário estático offline em segundos utilizando `golang:1.24-alpine` (mantendo cache do binário em `${APP_DATA_DIR}/data/bin`).

---

## Principais Recursos

- **Assistente OAuth 1-Clique & Monitor Externo:** dispara o login IMAP/TLS direto no proxy e detecta pedidos OAuth pendentes enviados por clientes externos (MailCraft, Thunderbird, impressoras, etc.), com rota automática `/oauth/callback`.
- **Dashboard & Probes em Tempo Real:** testa as portas IMAP/SMTP/POP configuradas e mostra o status dos tokens salvos no `credstore.config`.
- **Templates & Fluxos Avançados:** presets prontos para **Outlook / Office 365**, **Gmail**, **Yahoo**, **AOL** e **Fastmail** (Mozilla Thunderbird, FairEmail, GNOME Online Accounts, Mailspring) e suporte a Authorization Code, Device Flow, Client Credentials (CCG), ROPCG, Google Service Account e JWT.
- **Gerador de Certificado TLS & Download do `ca-cert.crt`:** gera CA local X.509 v3 + certificado com SANs, aplica nos servidores com 1 clique e permite baixar o `ca-cert.crt`.
- **Gerenciador de Portas Docker & Compose:** verifica conflitos reais no host e no Docker via `/var/run/docker.sock`, suportando atualização direta no container (Modo B) ou sincronização no `docker-compose.yml` do `user-config` (Modo A).
- **Editor do `emailproxy.config`, Debug ao Vivo (`SIGUSR1`) & Streaming de Logs:** edição estruturada ou raw com backup automático `.bak` e visualização de `/config/emailproxy.log`.

---

## Pré-requisitos e Configuração no Instalador

Este app foi projetado para se conectar ao container **`email-oauth2-proxy`** já instalado no seu Runtipi:

| Variável | Obrigatória | Padrão | Descrição |
| :--- | :---: | :---: | :--- |
| `PROXY_CONFIG_DIR` | Sim | — | Caminho absoluto no servidor até a pasta `data/config` do `email-oauth2-proxy` (ex: `/opt/runtipi/app-data/migrated/email-oauth2-proxy/data/config`). |
| `PROXY_COMPOSE_DIR` | Não | `/tmp/email-oauth2-proxy-compose` | Caminho opcional até `user-config/.../email-oauth2-proxy` para sincronização de portas no YAML (Modo A). |
| `PROXY_HOST` | Sim | `email-oauth2-proxy` | Hostname do container proxy na rede `tipi_main_network`. |
| `PROXY_CONTAINER` | Não | `email-oauth2-proxy` | Nome exato do container Docker do proxy para reinício e `SIGUSR1`. |
| `ADMIN_USER` | Não | `admin` | Usuário de acesso ao painel (quando `ADMIN_PASSWORD` estiver definida). |
| `ADMIN_PASSWORD` | Não | *(vazio)* | Senha do painel Web UI (deixe vazio para acesso sem login). |

> **Dica:** Para descobrir rapidamente o caminho exato da pasta `data/config` do seu `email-oauth2-proxy` no servidor Linux, execute:
> `docker inspect email-oauth2-proxy --format '{{range .Mounts}}{{if eq .Destination "/config"}}{{.Source}}{{end}}{{end}}'`
