# Telegram Bot API (Local Server)

Servidor local da **Telegram Bot API** empacotado pela equipe do **aiogram** (`aiogram/telegram-bot-api`), permitindo executar sua própria instância dedicada da API de Bots do Telegram diretamente no seu Runtipi.

---

## Principais Vantagens do Servidor Local

- **Upload de arquivos até 2000 MB:** Remove o limite padrão de 50 MB da API oficial na nuvem do Telegram.
- **Download de arquivos até 2000 MB:** Remove o limite padrão de 20 MB para baixar arquivos enviados aos seus bots.
- **Modo Local (`TELEGRAM_LOCAL=1`):** Permite enviar arquivos via caminhos locais e usar URLs/IPs locais para webhooks HTTP.
- **Menor Latência e Controle Total:** Ideal para bots de mídia, automações (n8n, Typebot, Dify, scripts Python/Node.js) e fluxos na mesma rede Docker (`tipi_main_network`).

---

## Credenciais Obrigatórias (`my.telegram.org`)

Antes de instalar, obtenha o **`API ID`** e o **`API Hash`** da sua conta pessoal/cliente do Telegram:

1. Acesse [https://my.telegram.org](https://my.telegram.org) e faça login com o seu número de telefone.
2. Entre em **API development tools** e crie uma aplicação.
3. Preencha **`TELEGRAM_API_ID`** e **`TELEGRAM_API_HASH`** no formulário de instalação do Runtipi.

> **Importante:** `TELEGRAM_API_ID` e `TELEGRAM_API_HASH` pertencem à **conta do cliente Telegram** (de `my.telegram.org`), **não** são o token do bot gerado no `@BotFather`.

---

## Variáveis de Ambiente Configuráveis

Caso não deseje alterar nada além das credenciais obrigatórias, todas as opções avançadas já vêm preenchidas com os valores padrão:

| Variável | Obrigatória | Padrão | Descrição |
| :--- | :---: | :---: | :--- |
| `TELEGRAM_API_ID` | Sim | — | API ID da conta Telegram (`my.telegram.org`). |
| `TELEGRAM_API_HASH` | Sim | — | API Hash da conta Telegram (`my.telegram.org`). |
| `TELEGRAM_LOCAL` | Não | `1` | Habilita o modo local (`--local`) para arquivos e webhooks locais. |
| `TELEGRAM_HTTP_PORT` | Não | `2627` | Porta HTTP interna do processo `telegram-bot-api`. |
| `TELEGRAM_STAT` | Não | `1` | Habilita o endpoint interno de estatísticas (`--http-stat-port=8082`). |
| `TELEGRAM_VERBOSITY` | Não | `0` | Nível de detalhamento dos logs (`0` = mínimo). |

---

## Como Conectar seus Bots

- **Acesso pela rede interna do Runtipi (`tipi_main_network`):**
  `http://telegram-bot-api:2627`
- **Acesso externo / host:**
  `http://<IP-DO-SEU-RUNTIPI>:<APP_PORT>` (padrão `2627`)
- **Persistência de dados:**
  Os dados do servidor são armazenados em `${APP_DATA_DIR}/data` mapeado para `/var/lib/telegram-bot-api`.
