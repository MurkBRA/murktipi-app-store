# Browserless (Headless Browser Server)

Servidor **Browserless v2** (`v2.64.3`) para automação de navegadores headless via **Puppeteer**, **Playwright** e **Chrome DevTools Protocol (CDP)**, com escolha direta da imagem base no instalador do Runtipi.

---

## Imagens Base Disponíveis (`ghcr.io/browserless/*:v2.64.3`)

Durante a instalação você pode selecionar qual imagem pública do Browserless deseja rodar (com **`chrome`** pré-selecionado por padrão):

- **`chrome`** *(padrão)* — Google Chrome oficial.
- **`chromium`** — Chromium multi-arquitetura.
- **`firefox`** — Mozilla Firefox.
- **`webkit`** — WebKit (engine Safari).
- **`multi`** — Pacote combinado (Chromium + Firefox + WebKit).

> **Nota:** As variantes `edge` e `sdk` do repositório Browserless não possuem imagem pública acessível anonimamente no GHCR (`401/403`), por isso foram listadas apenas as 5 imagens públicas multi-arch (`amd64`/`arm64`).

---

## Variáveis Opcionais e `shm_size`

Todas as variáveis de ambiente (exceto `TZ`, injetado automaticamente pelo Runtipi) e `SHM_SIZE` são **opcionais** e não possuem valor padrão forçado:

- **Se você deixar em branco:**
  - As variáveis de ambiente (`MAX_CONCURRENT_SESSIONS`, `CONNECTION_TIMEOUT`, `PREBOOT_CHROME`, `CHROME_REFRESH_TIME`, `ENABLE_CORS`, `DEMOING`) **não são adicionadas ao container** (pass-through YAML sem `=`).
  - O campo `SHM_SIZE` assume `64m`, que é exatamente o tamanho padrão nativo de `/dev/shm` no Docker quando `shm_size` não é definido.
- **Sugestões de preenchimento:**
  - `SHM_SIZE`: `2gb`
  - `MAX_CONCURRENT_SESSIONS`: `10`
  - `CONNECTION_TIMEOUT`: `60000`
  - `PREBOOT_CHROME`: `true`
  - `CHROME_REFRESH_TIME`: `3600000`
  - `ENABLE_CORS`: `true`
  - `DEMOING`: `false`

---

## Conexão e Portas

- **Porta externa padrão:** `9222` (mapeada via `${APP_PORT}:3000`)
- **Endpoint interno na rede `tipi_main_network`:** `http://browserless:3000` / `ws://browserless:3000`
- **Endpoint externo / host:** `http://<IP-DO-RUNTIPI>:9222` / `ws://<IP-DO-RUNTIPI>:9222`
