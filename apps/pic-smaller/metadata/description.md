# Pic Smaller

**Pic Smaller** is a self-hosted, open-source batch image optimizer that processes files locally in the browser using Web Workers and WebAssembly codecs so images never leave your device.

## Key Features

- **PNG Compression with ImageQuant + OxiPNG:** Combines lossy color palette quantization (`libimagequant`, 2-256 colors and dithering control) with multi-level **OxiPNG** optimization (standard and Extreme Mode) for compact PNG output while keeping transparency.
- **Multi-Format Support:** Optimizes **JPEG** (MozJPEG), **PNG** (ImageQuant + OxiPNG), **WebP**, **AVIF**, **SVG** (SVGO), **GIF**, and decodes **HEIC/HEIF** inputs.
- **Interactive Comparison & Editing:** Split-screen before-and-after preview slider, batch resizing, ratio/pixel cropping, clipboard paste, folder import, and single-file or ZIP archive export.
- **Lightweight Container:** Runs Next.js standalone as an unprivileged `node` user on internal port `3000` (mapped to host port `3002` by default).

## Optional Configuration

- **Container Name (`CONTAINER_NAME`):** Custom Docker container name (defaults to `pic-smaller`).
