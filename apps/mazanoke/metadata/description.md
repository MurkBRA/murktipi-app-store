# MAZANOKE

**MAZANOKE** is a self-hosted, offline-capable image optimizer and converter that processes images locally in your browser without telemetry or cloud uploads.

## Key Features

- **On-Device Image Optimization:** Adjust quality, target a maximum file size, or limit width and height locally in your browser.
- **Broad Format Support:** Compress and convert between **JPG**, **PNG**, **WebP**, and **ICO**, with input support for **HEIC**, **AVIF**, **TIFF**, **GIF**, and **SVG**.
- **Privacy & EXIF Removal:** Automatically strips sensitive EXIF metadata such as camera details, dates, and GPS location.
- **PWA & Clipboard Workflow:** Supports clipboard image paste and installation as an offline Progressive Web App.

## Optional Configuration

- **Container Name (`CONTAINER_NAME`):** Custom Docker container name (defaults to `mazanoke`).
- **Basic Authentication (`USERNAME` & `PASSWORD`):** Provide both variables to protect access with Nginx HTTP Basic Authentication; leave either blank to allow direct access without a login prompt.
