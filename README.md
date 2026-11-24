# MurkTipi App Store 🏪

[![CI & Anti-Regression Tests](https://github.com/murkbra/murktipi-app-store/actions/workflows/ci.yml/badge.svg)](https://github.com/murkbra/murktipi-app-store/actions/workflows/ci.yml)
[![Tests Passing](https://img.shields.io/badge/tests-58%20passed-brightgreen.svg)](https://github.com/murkbra/murktipi-app-store)
[![Runtipi Schema](https://img.shields.io/badge/runtipi%20schema-v2-blue.svg)](https://runtipi.io)

Repositório customizado de aplicativos (App Store de Usuário) para o **[Runtipi](https://runtipi.io)**, mantido por **[@murkbra](https://github.com/murkbra)**.

---

## 🚀 Como Adicionar esta Loja no seu Runtipi

1. Abra o painel do seu **Runtipi**.
2. Navegue até **Settings** (Configurações) > **App Stores**.
3. Clique em **Add Custom App Store**.
4. Insira a URL do repositório:
   ```text
   https://github.com/murkbra/murktipi-app-store
   ```
5. Clique em salvar. Os novos aplicativos aparecerão imediatamente na sua loja do Runtipi!

---

## 📦 Aplicativos Disponíveis

### 1. 🌀 [Spiral Mining Pool](apps/spiral-pool)
Servidor de pool de mineração solo e multi-moedas com Stratum V1/V2, dashboard administrativo e métricas Prometheus integradas.

- **Modos de Operação:** `single` (moeda dedicada) ou `multi` (múltiplas moedas simultâneas).
- **Algoritmos & Criptomoedas Suportadas:**
  - **SHA-256d:** Bitcoin (BTC), DigiByte (DGB), Litecoin Cash, Namecoin (NMC), Myriadcoin (XMY), Fractal Bitcoin (FBTC), eCash (XEC).
  - **Scrypt:** Litecoin (LTC), Dogecoin (DOGE), PepeCoin (PEP), Catcoin (CAT).
- **Merge Mining:** Suporte a mineração combinada (ex: LTC+DOGE ou BTC+NMC) sem perda de hashrate.
- **Portas Stratum Dinâmicas:** Configurável diretamente no instalador visual do Runtipi.
- **Segurança:** Suporte a Stratum V2 com criptografia ponta a ponta (Noise Protocol).

### 2. 🛡️ [Technitium DNS Server](apps/technitium)
Servidor DNS autoritativo e recursivo de nível corporativo com bloqueio de anúncios e rastreadores em toda a rede (estilo Pi-hole / AdGuard Home) e suporte a protocolos modernos.

- **Configuração Clara & Direta:**
  - `DNS_SERVER_DOMAIN`: Nome de domínio padrão (FQDN) do servidor.
  - `DNS_SERVER_RECURSION`: Política de recursão DNS (Allow, AllowOnlyForPrivateNetworks, Deny, etc.).
  - `DNS_SERVER_ADMIN_PASSWORD`: Senha do painel web administrativo.
- **Portas Suportadas:** Porta 53 (TCP e UDP) para consultas DNS de alto desempenho e porta 5380 para interface gráfica.
- **Resolução & Criptografia:** Suporte a DoH (DNS-over-HTTPS), DoT (DNS-over-TLS) e DoQ (DNS-over-QUIC) configuráveis pela interface.

### 3. 🤖 [Telegram Bot API](apps/telegram-bot-api)
Servidor local oficial da Telegram Bot API empacotado pelo projeto **aiogram** (`aiogram/telegram-bot-api`), removendo limites da API na nuvem e permitindo acesso a arquivos locais.

- **Credenciais Obrigatórias (`my.telegram.org`):**
  - `TELEGRAM_API_ID` e `TELEGRAM_API_HASH` (da conta cliente do Telegram, não o token do bot).
- **Variáveis Pré-configuradas no Instalador:**
  - `TELEGRAM_LOCAL`: `"1"` (modo local ativado).
  - `TELEGRAM_HTTP_PORT`: `2627` (porta HTTP interna mapeada via `${APP_PORT}:2627`).
  - `TELEGRAM_STAT`: `"1"` (endpoint de estatísticas ativo).
  - `TELEGRAM_VERBOSITY`: `0` (nível de logs padrão).
- **Persistência:** Dados armazenados automaticamente em `${APP_DATA_DIR}/data`.

### 4. 🔐 [Email OAuth2 Proxy UI](apps/email-oauth2-proxy-ui)
Painel web leve em Go ([`packages/email-oauth2-proxy-ui`](packages/email-oauth2-proxy-ui)) com imagem multi-arch (`amd64`/`arm64`) versionada e publicada via GitHub Actions no GHCR (`ghcr.io/murkbra/email-oauth2-proxy-ui`) para gerenciar uma instalação existente do **Email OAuth2 Proxy** no Runtipi.

- **Integração Automática via `ROOT_FOLDER_HOST`:** Monta automaticamente `${ROOT_FOLDER_HOST}/app-data/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy/data/config` em `/config` e `${ROOT_FOLDER_HOST}/user-config/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy` em `/compose`, além de `/var/run/docker.sock` e conexão na rede `tipi_main_network` (`PROXY_HOST`).
- **Funcionalidades:** Assistente OAuth 1-clique, monitor de pedidos OAuth externos, probes IMAP/SMTP/POP, geração de CA/certificados TLS, gerenciador de portas Docker/Compose (Modos A e B), editor do `emailproxy.config`, Debug ao vivo (`SIGUSR1`) e streaming de logs.
- **Autenticação Opcional:** Login por sessão configurável via `ADMIN_USER` e `ADMIN_PASSWORD`.

### 5. 🌐 [Browserless](apps/browserless)
Servidor de navegadores headless (**Browserless v2** `v2.64.3`) para automação via Puppeteer, Playwright e CDP na porta `9222` (`${APP_PORT}:3000`).

- **Escolha da Imagem Base (`BROWSERLESS_IMAGE`):** Seleção no instalador entre `chrome` *(padrão)*, `chromium`, `firefox`, `webkit` e `multi` (`ghcr.io/browserless/<imagem>:v2.64.3`, com tag versionada para updates pelo Runtipi).
- **Variáveis e `shm_size` Opcionais:** `SHM_SIZE`, `MAX_CONCURRENT_SESSIONS`, `CONNECTION_TIMEOUT`, `PREBOOT_CHROME`, `CHROME_REFRESH_TIME`, `ENABLE_CORS` e `DEMOING` sem valor forçado (se deixadas em branco, não são injetadas no container / `shm_size` mantém o padrão nativo `64m` do Docker).

### 6. 🚢 [Dockhand](apps/dockhand)
Painel moderno de gerenciamento Docker (`fnsys/dockhand:v1.0.59`) com orquestração visual de Compose stacks, GitOps, terminal/logs interativos, scanner de vulnerabilidades e backend **PostgreSQL 16** dedicado (`postgres:16-alpine` com healthcheck).

- **Porta Ajustada:** Porta padrão do host definida em `3180` (`${APP_PORT}:3000`), evitando conflito com serviços na porta `3000`.
- **Banco com Senha Aleatória:** `POSTGRES_USER` obrigatório definido pelo usuário, `POSTGRES_DB` (`dockhand`), e `POSTGRES_PASSWORD` gerado automaticamente pelo Runtipi em hexadecimal (`type: random`) para uso seguro na `DATABASE_URL` (sem `PUID`/`PGID` e com `CONTAINER_NAME` / `POSTGRES_CONTAINER_NAME` opcionais).

---

## 🧪 Política de Testes & Anti-Regressão

Este repositório possui uma suíte rigorosa de testes automatizados e uma política estrita: **qualquer alteração, nova funcionalidade ou correção de bug DEVE vir acompanhada de novos testes anti-regressão.**

### Executando os Testes Localmente

1. Instale as dependências de desenvolvimento:
   ```bash
   pip install -r requirements-dev.txt
   ```

2. Execute o validador rápido da loja:
   ```bash
   python scripts/validate_store.py
   ```

3. Execute a suíte completa de testes via `pytest`:
   ```bash
   pytest -v
   ```

### Estrutura dos Testes

- `tests/test_schema_validation.py`: Valida o formato dos arquivos `config.json`, tipos de campos, obrigatoriedade de campos, logo 1:1 e descrições.
- `tests/test_compose_validation.py`: Valida a sintaxe YAML do `docker-compose.yml`, o padrão Runtipi v2 (`x-runtipi.schema_version: 2`), serviço principal (`is_main: true`) e mapeamento correto de todas as variáveis de ambiente.
- `tests/test_spiral_pool.py`: Testes específicos da Spiral Pool (modos single/multi, flags de moedas, dependência saudável do Postgres, portas Stratum).
- `tests/test_technitium.py`: Testes específicos do Technitium (validação das 3 variáveis de ambiente essenciais + `CONTAINER_NAME` opcional, portas DNS UDP/TCP 53, console 5380 e formato `logo.jpg`).
- `tests/test_telegram_bot_api.py`: Testes específicos do Telegram Bot API (obrigatoriedade de API ID/Hash, exclusão de `TZ` do formulário, preservação dos padrões originais + `CONTAINER_NAME` opcional, mapeamento `${APP_PORT}:2627` e `logo.jpg`).
- `tests/test_email_oauth2_proxy_ui.py`: Testes específicos do Email OAuth2 Proxy UI (integração `ROOT_FOLDER_HOST`, `PROXY_STORE_ID`, imagem GHCR, Dockerfile Go 1.27 Alpine e `logo.jpg`).
- `tests/test_browserless.py`: Testes específicos do Browserless (versão `v2.64.3` fixada, seleção de imagem com `chrome` padrão, variáveis/shm_size opcionais sem default forçado, `CONTAINER_NAME` opcional e `logo.jpg`).
- `tests/test_dockhand.py`: Testes específicos do Dockhand (`fnsys/dockhand:v1.0.59`, Postgres 16 com healthcheck, porta ajustada `3180 -> 3000`, ausência de `PUID`/`PGID`, `POSTGRES_USER` definido pelo usuário, `POSTGRES_PASSWORD` `random`/`hex` e `CONTAINER_NAME` configurável).
- `tests/test_regression.py`: **Suíte Anti-Regressão** (`REG-001` a `REG-031`) documentando cada caso de borda para garantir que bugs anteriores nunca voltem a ocorrer.
- `.github/workflows/ci.yml`: Automação contínua no GitHub Actions disparada a cada push e pull request.

---

## 📄 Licença

Distribuído sob licença MIT. Consulte `LICENSE` para mais detalhes.
