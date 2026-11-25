# ImgCompress

**ImgCompress** is a self-hosted web application for compressing, converting, resizing, and optimizing images locally without uploading files to external third-party services.

## Key Features

- **70+ Supported Formats:** Compress and convert between modern and legacy image formats including JPEG, PNG, WebP, AVIF, JPEG XL, HEIC, GIF, SVG, TIFF, BMP, PDF, and camera RAW files.
- **Batch Processing & Fine-Grained Control:** Process single images or multi-file batches with adjustable compression targets, quality sliders, resizing options, and EXIF metadata stripping.
- **AI-Powered Enhancements:** Built-in support for AI upscaling and automated background removal.
- **Lightweight Non-Root Container:** Runs unprivileged (`nonroot` user) and listens on internal port `5000` (exposed on host port `3001` by default).

## Optional Configuration

All application settings are optional and fall back to upstream defaults when left unset:

- **Container Name (`CONTAINER_NAME`):** Custom Docker container name (defaults to `imgcompress`).
- **Max Upload Size (`MAX_UPLOAD_SIZE_MB`):** Maximum upload size limit in megabytes (defaults to `40960` MB when unset).
- **Web Workers (`WEB_WORKERS`):** Worker concurrency setting, either `auto` or a positive integer.
- **UI & Background Toggles:** Independently disable the sidebar mascot (`DISABLE_LOGO`), automatic storage management (`DISABLE_STORAGE_MANAGEMENT`), or GitHub update checks (`DISABLE_UPDATE_CHECK`).
- **Notifications (`NOTIFY_URL` & `PUBLIC_BASE_URL`):** Optional Apprise notification URL and public base URL for job completion alerts.
