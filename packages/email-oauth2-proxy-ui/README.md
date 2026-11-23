# Email OAuth2 Proxy — WebGUI (Sidecar em Go)

Painel web leve em Go (zero dependências externas) criado para rodar como **sidecar** ao lado do [simonrob/email-oauth2-proxy](https://github.com/simonrob/email-oauth2-proxy) no **Runtipi** ou Docker Compose — **sem precisar de `docker build`**, iniciando direto via `command` na imagem oficial `golang:1.24-alpine`.

## Recursos

- **Assistente OAuth 1-Clique + Monitor de Pedidos Externos:** dispara o login IMAP/TLS direto no container do proxy **e** monitora passivamente pedidos OAuth vindos de clientes externos (MailCraft, Thunderbird, impressoras, etc.), exibindo alerta no Dashboard com popup, envio automático ao colar e rota automática `/oauth/callback`.
- **Dashboard em Tempo Real:** testa todas as portas configuradas (IMAP/SMTP/POP, com ou sem TLS), exibe modal com portas disponíveis por conta (`IMAP-2993`, `POP-2995`, `SMTP-2465`) e status dos tokens salvos no `credstore.config`.
- **Templates, Presets & Fluxos Avançados:** configuração rápida para **Outlook / Office 365**, **Gmail**, **Yahoo**, **AOL** e **Fastmail** com presets verificados (**Mozilla Thunderbird**, **FairEmail**, **GNOME Online Accounts** e **Mailspring**) + modal completo de edição de contas suportando **Authorization Code**, **Device Flow**, **O365 Client Credentials (CCG)**, **Password (ROPCG)**, **Google Service Account (JSON)** e **Certificados JWT**.
- **Gerador de Certificado TLS + Download do `ca-cert.crt`:** gera CA local X.509 v3 + certificado do servidor com SAN (IPs/DNS, compatível com `rustls` / MailCraft), aplica em todos os servidores com 1 clique e libera o download do `ca-cert.crt` pronto para instalar no Windows.
- **Gerenciador de Portas Docker & Compose (Modos A e B):** verifica disponibilidade real de portas no Docker e no host antes de abrir qualquer porta, e permite ao usuário escolher na hora entre atualizar apenas o `docker-compose.yml` (Modo A), abrir direto no container via `/var/run/docker.sock` com rollback automático (Modo B) ou ambos (A + B).
- **Editor do `emailproxy.config`, Debug ao Vivo & Logs Estilo Terminal:** edição estruturada ou raw (com backup automático `.bak`), toggle de **Debug Mode em tempo real (`SIGUSR1`)**, reinício do container proxy via Docker socket e streaming formatado de `/config/emailproxy.log`.
- **Autenticação opcional:** tela de login protegida por cookie assinado via `ADMIN_USER` e `ADMIN_PASSWORD`.

---

## Como subir no Runtipi (Sem `docker build`)

1. Copie todos os arquivos deste projeto (`web/`, `internal/`, `main.go`, `go.mod`, etc.) para uma pasta fixa fora do `app-data` do Runtipi:
   ```text
   /home/Docker/email-oauth2-proxy-ui
   ```

2. Copie o conteúdo de [`docker-compose.runtipi.yml`](docker-compose.runtipi.yml) para o override do Runtipi em:
   ```text
   <pasta-do-runtipi>/user-config/migrated/email-oauth2-proxy/docker-compose.yml
   ```
   *(Troque `ADMIN_PASSWORD` por uma senha segura.)*

3. Reinicie o app **Email OAuth2 Proxy** no painel do Runtipi.
   - Como o projeto não tem dependências externas, o `command` (`go build -buildvcs=false -o /tmp/eop-ui . && exec /tmp/eop-ui`) compila offline em ~1–2 segundos ao subir e executa apenas o binário Go como PID 1.

4. Acesse o painel no navegador:
   ```text
   http://IP-DO-SERVIDOR:8090
   ```

### Alternativa: `docker compose` separado (também sem `docker build`)
Ajuste o caminho da pasta `data/config`, o `PROXY_HOST` e o nome da rede Docker (`docker network ls`) em [`docker-compose.yml`](docker-compose.yml) e rode:
```bash
docker compose up -d
```

---

## Variáveis de Ambiente

| Variável | Padrão | Descrição |
|---|---|---|
| `TZ` | `America/Sao_Paulo` | Fuso horário (suportado nativamente via `time/tzdata` embutido) |
| `LISTEN_ADDR` | `:8090` | Endereço/porta HTTP do painel |
| `CONFIG_DIR` | `/config` | Pasta compartilhada com `emailproxy.config`, `credstore.config`, `emailproxy.log` e certificados |
| `PROXY_HOST` | `email-oauth2-proxy` | Hostname/container do `email-oauth2-proxy` na rede Docker |
| `PROXY_CONTAINER` | *(mesmo que `PROXY_HOST`)* | Nome exato do container do proxy (usado com `/var/run/docker.sock`) |
| `DOCKER_SOCKET` | `/var/run/docker.sock` | Caminho do socket Docker para gerenciar portas, reiniciar proxy e alternar Debug Mode |
| `COMPOSE_DIR` / `COMPOSE_FILE` | `/compose` | Pasta ou arquivo `docker-compose.yml` opcional para sincronizar `ports:` (Modo A) |
| `COMPOSE_SERVICE` | `email-oauth2-proxy` | Nome do serviço dentro do YAML ao atualizar `ports:` |
| `ADMIN_USER` | `admin` | Usuário de login do painel |
| `ADMIN_PASSWORD` | *(vazio)* | Senha do painel (se vazia, login fica desativado) |
