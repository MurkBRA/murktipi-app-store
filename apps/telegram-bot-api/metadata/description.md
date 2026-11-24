# Telegram Bot API (Local Server)

A self-hosted **Telegram Bot API** HTTP server packaged by **aiogram** and built on TDLib.

## Why run a local Bot API server?

- **Large File Support**: Upload files up to **2000 MB** instead of the 50 MB cloud Bot API limit.
- **Direct Local File Downloads**: Download files up to **2000 MB** without the 20 MB cloud limit and read downloaded files directly via local paths.
- **Dedicated Performance**: Serve high-throughput bots using local HTTP webhooks and custom ports or bind addresses.

## Requirements

- Obtain your Telegram client **API ID** (`TELEGRAM_API_ID`) and **API Hash** (`TELEGRAM_API_HASH`) at [my.telegram.org](https://my.telegram.org) (these belong to your Telegram client account, not `@BotFather`).
- Point your bot client or automation platform (for example n8n, aiogram, or python-telegram-bot) to `http://telegram-bot-api:2627` or your host port.
