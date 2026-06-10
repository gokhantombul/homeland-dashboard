# 🏠 Homelab Dashboard

A lightweight, **single-binary** Proxmox monitoring dashboard written in Go with full **Progressive Web App (PWA)** support. Installable on iOS and Android just like a native app.

> **🇹🇷 Türkçe döküman** bu dosyanın alt yarısında yer almaktadır.

---

## Table of Contents

- [Features](#features)
- [Screenshots](#screenshots)
- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Notification Channels](#notification-channels)
- [PWA Installation](#pwa-installation)
- [Building from Source](#building-from-source)
- [Docker](#docker)
- [Project Structure](#project-structure)
- [API Reference](#api-reference)
- [License](#license)

---

## Features

| Category | Details |
|---|---|
| **Backend** | Pure Go standard library (`net/http`, `html/template`, `embed`) |
| **Proxmox** | Live VM & LXC data via Proxmox API with token auth |
| **Notifications** | Webhook, Telegram Bot, WhatsApp, SMS — configurable via env vars |
| **PWA** | Installable on iOS (Safari) & Android (Chrome) as a standalone app |
| **UI** | Premium dark-mode, mobile-first, Tailwind CSS |
| **Single binary** | All assets embedded — deploy with one file |
| **Demo mode** | Works without Proxmox (shows sample data) |
| **Auto-refresh** | Live updates every 30 seconds without page reload |
| **Offline** | Service Worker caches the shell and shows an offline page |

---

## Requirements

- **Go 1.21+** (for building)
- A **Proxmox VE** server (optional — demo mode works without one)
- Network access from the dashboard host to your Proxmox API port (`8006`)

---

## Quick Start

```bash
# Clone
git clone https://github.com/gokhantombul/homeland-dashboard.git
cd homeland-dashboard

# Build (produces a single binary, ~8 MB)
go build -o homelab-dashboard .

# Run with demo data (no Proxmox required)
./homelab-dashboard

# Open
open http://localhost:8080
```

---

## Configuration

All configuration is done via **environment variables** — no config files needed.

| Variable | Default | Description |
|---|---|---|
| `PROXMOX_URL` | *(empty)* | Proxmox host, e.g. `https://192.168.1.100:8006` |
| `PROXMOX_TOKEN` | *(empty)* | API token: `USER@REALM!TOKENID=SECRET` |
| `PORT` | `8080` | HTTP listen port |
| `NOTIFICATION_SERVICE_URL` | *(empty)* | Generic webhook URL for alerts |
| `TELEGRAM_BOT_TOKEN` | *(empty)* | Telegram bot token |
| `TELEGRAM_CHAT_ID` | *(empty)* | Telegram chat / channel ID |
| `WHATSAPP_WEBHOOK_URL` | *(empty)* | WhatsApp Business webhook URL |
| `SMS_WEBHOOK_URL` | *(empty)* | SMS gateway webhook URL |

### Proxmox API Token

1. In the Proxmox web UI go to **Datacenter → Permissions → API Tokens**.
2. Create a token for your user (e.g. `root@pam!dashboard`).
3. Grant at least **PVEAuditor** role on `/` (read-only is fine).
4. Set the env var: `PROXMOX_TOKEN=root@pam!dashboard=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`

### Example .env

```bash
export PROXMOX_URL="https://192.168.1.100:8006"
export PROXMOX_TOKEN="root@pam!dashboard=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
export PORT="8080"
export TELEGRAM_BOT_TOKEN="123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ"
export TELEGRAM_CHAT_ID="-100123456789"
export NOTIFICATION_SERVICE_URL="https://hooks.example.com/homelab"
```

---

## Notification Channels

The dashboard supports four notification channels, enabled by setting the corresponding environment variable.

### Webhook (generic)

Set `NOTIFICATION_SERVICE_URL`. The server will POST:

```json
{
  "service": "ubuntu-server",
  "status": "stopped",
  "message": "Service 'ubuntu-server' — status: stopped",
  "time": "2025-06-10T14:30:00Z"
}
```

Works with **n8n**, **Home Assistant webhooks**, **Zapier**, **Make**, etc.

### Telegram

1. Create a bot via [@BotFather](https://t.me/botfather) → get `TELEGRAM_BOT_TOKEN`.
2. Add the bot to your group/channel → get `TELEGRAM_CHAT_ID`.
3. Set both env vars. Alerts are sent as Markdown-formatted messages.

### WhatsApp

Set `WHATSAPP_WEBHOOK_URL` to a WhatsApp Business API webhook endpoint  
(e.g. [Twilio WhatsApp](https://www.twilio.com/whatsapp), [360dialog](https://360dialog.com/), or a self-hosted gateway).  
The same JSON payload as the generic webhook is POST-ed.

### SMS

Set `SMS_WEBHOOK_URL` to any HTTP SMS gateway endpoint  
(e.g. [Twilio SMS](https://www.twilio.com/sms), [Vonage](https://www.vonage.com/), [BulkSMS](https://www.bulksms.com/)).  
Same JSON payload.

### Routing alerts from the dashboard

Each VM/LXC card has a **"Send Alert"** button. Clicking it opens a modal where you can:

- Tick which channels to use (only configured channels appear)
- Type a custom message (or leave blank for the auto-generated one)
- Hit **Send Notification**

The response shows per-channel success/failure.

---

## PWA Installation

### Android (Chrome / Edge)

1. Open `http://<your-host>:8080` in Chrome.
2. Tap the **"Install App"** banner that appears in the header, or use the browser menu → **"Add to Home Screen"**.
3. The app opens in fullscreen, standalone mode.

### iOS (Safari)

1. Open the URL in Safari.
2. Tap the **Share** button (rectangle with arrow).
3. Tap **"Add to Home Screen"**.
4. The app icon appears on your home screen.

### Desktop (Chrome / Edge)

Click the install icon in the address bar or the **"Install App"** button in the dashboard header.

---

## Building from Source

```bash
# Standard build
go build -o homelab-dashboard .

# Optimized / smaller binary
CGO_ENABLED=0 go build -ldflags="-s -w" -o homelab-dashboard .

# Cross-compile for Raspberry Pi (arm64)
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o homelab-dashboard-arm64 .

# Cross-compile for arm (32-bit)
GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -o homelab-dashboard-armv7 .
```

---

## Docker

```dockerfile
# Dockerfile (multi-stage)
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o homelab-dashboard .

FROM scratch
COPY --from=builder /app/homelab-dashboard /homelab-dashboard
EXPOSE 8080
ENTRYPOINT ["/homelab-dashboard"]
```

```bash
docker build -t homelab-dashboard .
docker run -p 8080:8080 \
  -e PROXMOX_URL="https://192.168.1.100:8006" \
  -e PROXMOX_TOKEN="root@pam!dashboard=..." \
  -e TELEGRAM_BOT_TOKEN="..." \
  -e TELEGRAM_CHAT_ID="..." \
  homelab-dashboard
```

---

## Project Structure

```
homeland-dashboard/
├── main.go                 # Single Go source file — server, handlers, icon generator
├── go.mod                  # Module definition (standard library only, no deps)
├── templates/
│   └── index.html          # Server-side rendered HTML (Go template)
├── static/
│   ├── manifest.json       # PWA manifest
│   ├── sw.js               # Service Worker (offline support, caching)
│   └── icon.svg            # Vector app icon
└── README.md               # This file
```

Everything in `templates/` and `static/` is embedded into the binary via `//go:embed` — nothing needs to be deployed alongside the binary.

---

## API Reference

### `GET /`

Returns the rendered HTML dashboard page.

### `GET /api/resources`

Returns live Proxmox resource data as JSON.

**Response example:**

```json
[
  {
    "id": "qemu/100",
    "name": "ubuntu-server",
    "type": "qemu",
    "status": "running",
    "node": "pve-01",
    "cpu": 0.127,
    "mem": 2684354560,
    "maxmem": 4294967296,
    "disk": 10737418240,
    "maxdisk": 32212254720,
    "uptime": 349200,
    "vmid": 100,
    "tags": "production"
  }
]
```

### `POST /api/notify`

Send a notification to one or more channels.

**Request body:**

```json
{
  "service": "ubuntu-server",
  "status": "stopped",
  "message": "Custom message (optional)",
  "channels": ["telegram", "webhook"]
}
```

**Response:**

```json
{
  "success": true,
  "results": {
    "telegram": "ok",
    "webhook": "ok"
  }
}
```

On partial failure:

```json
{
  "success": false,
  "results": {
    "telegram": "ok",
    "sms": "error: SMS_WEBHOOK_URL not set"
  }
}
```

### `GET /icons/icon-192.png` · `GET /icons/icon-512.png`

Dynamically generated PNG icons (embedded in the binary, no external files).

---

## License

MIT © 2025 Gökhan Tombul

---
---

# 🏠 Homelab Dashboard — Türkçe Dokümantasyon

Proxmox altyapınızı izlemek için hafif, **tek çalıştırılabilir dosya** olarak dağıtılabilen, Go ile yazılmış, tam **PWA (İlerleyen Web Uygulaması)** destekli bir gösterge paneli. iOS ve Android'e yerel uygulama gibi kurulabilir.

---

## İçindekiler

- [Özellikler](#özellikler)
- [Gereksinimler](#gereksinimler)
- [Hızlı Başlangıç](#hızlı-başlangıç)
- [Yapılandırma](#yapılandırma)
- [Bildirim Kanalları](#bildirim-kanalları)
- [PWA Kurulumu](#pwa-kurulumu)
- [Kaynaktan Derleme](#kaynaktan-derleme)
- [Docker](#docker-1)
- [API Referansı](#api-referansı)

---

## Özellikler

| Kategori | Detay |
|---|---|
| **Arka Uç** | Saf Go standart kütüphanesi (`net/http`, `html/template`, `embed`) |
| **Proxmox** | API token kimlik doğrulamasıyla canlı VM & LXC verisi |
| **Bildirimler** | Webhook, Telegram Bot, WhatsApp, SMS — ortam değişkenleriyle yapılandırılabilir |
| **PWA** | iOS (Safari) ve Android (Chrome) üzerinde bağımsız uygulama olarak kurulabilir |
| **Arayüz** | Premium koyu tema, önce mobil, Tailwind CSS |
| **Tek binary** | Tüm dosyalar derlenmiş içine gömülü — tek dosya dağıtımı |
| **Demo mod** | Proxmox olmadan çalışır (örnek verilerle) |
| **Otomatik yenileme** | Sayfa yenilemesiz 30 saniyede bir canlı güncelleme |
| **Çevrimdışı** | Service Worker kabuk önbelleğini yapar, çevrimdışı sayfa gösterir |

---

## Gereksinimler

- **Go 1.21+** (derleme için)
- **Proxmox VE** sunucusu (isteğe bağlı — demo mod Proxmox olmadan çalışır)
- Dashboard sunucusundan Proxmox API portuna (`8006`) ağ erişimi

---

## Hızlı Başlangıç

```bash
# Depoyu klonla
git clone https://github.com/gokhantombul/homeland-dashboard.git
cd homeland-dashboard

# Derle (tek binary, ~8 MB)
go build -o homelab-dashboard .

# Demo verisiyle çalıştır (Proxmox gerekmez)
./homelab-dashboard

# Tarayıcıda aç
open http://localhost:8080
```

---

## Yapılandırma

Tüm yapılandırma **ortam değişkenleri** ile yapılır — ek dosya gerekmez.

| Değişken | Varsayılan | Açıklama |
|---|---|---|
| `PROXMOX_URL` | *(boş)* | Proxmox sunucu adresi, örn. `https://192.168.1.100:8006` |
| `PROXMOX_TOKEN` | *(boş)* | API token: `USER@REALM!TOKENID=SECRET` |
| `PORT` | `8080` | HTTP dinleme portu |
| `NOTIFICATION_SERVICE_URL` | *(boş)* | Genel webhook URL'si |
| `TELEGRAM_BOT_TOKEN` | *(boş)* | Telegram bot token'ı |
| `TELEGRAM_CHAT_ID` | *(boş)* | Telegram sohbet / kanal ID'si |
| `WHATSAPP_WEBHOOK_URL` | *(boş)* | WhatsApp Business webhook URL'si |
| `SMS_WEBHOOK_URL` | *(boş)* | SMS gateway webhook URL'si |

### Proxmox API Token Oluşturma

1. Proxmox web arayüzünde **Datacenter → Permissions → API Tokens** menüsüne gidin.
2. Kullanıcınız için bir token oluşturun (örn. `root@pam!dashboard`).
3. `/` yolunda en az **PVEAuditor** rolü verin (salt okunur yeterli).
4. Ortam değişkenini ayarlayın:
   ```bash
   export PROXMOX_TOKEN="root@pam!dashboard=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
   ```

### Örnek .env Dosyası

```bash
export PROXMOX_URL="https://192.168.1.100:8006"
export PROXMOX_TOKEN="root@pam!dashboard=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
export PORT="8080"
export TELEGRAM_BOT_TOKEN="123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ"
export TELEGRAM_CHAT_ID="-100123456789"
export NOTIFICATION_SERVICE_URL="https://hooks.example.com/homelab"
```

---

## Bildirim Kanalları

Dashboard dört bildirim kanalını destekler; ilgili ortam değişkeni ayarlandığında kanal etkinleşir.

### Webhook (Genel)

`NOTIFICATION_SERVICE_URL` değişkenini ayarlayın. Sunucu aşağıdaki JSON'u POST eder:

```json
{
  "service": "ubuntu-server",
  "status": "stopped",
  "message": "Service 'ubuntu-server' — status: stopped",
  "time": "2025-06-10T14:30:00Z"
}
```

**n8n**, **Home Assistant webhook'ları**, **Zapier**, **Make** vb. ile çalışır.

### Telegram

1. [@BotFather](https://t.me/botfather) üzerinden bir bot oluşturun → `TELEGRAM_BOT_TOKEN` alın.
2. Botu grubunuza/kanalınıza ekleyin → `TELEGRAM_CHAT_ID` alın.
3. Her iki değişkeni ayarlayın. Bildirimler Markdown formatında gönderilir.

### WhatsApp

`WHATSAPP_WEBHOOK_URL` değişkenini bir WhatsApp Business API webhook adresiyle ayarlayın  
(ör. [Twilio WhatsApp](https://www.twilio.com/whatsapp), [360dialog](https://360dialog.com/) veya kendi altyapınız).  
Aynı JSON yükü POST edilir.

### SMS

`SMS_WEBHOOK_URL` değişkenini herhangi bir HTTP SMS gateway adresiyle ayarlayın  
(ör. [Twilio SMS](https://www.twilio.com/sms), [Vonage](https://www.vonage.com/), [NetGSM](https://www.netgsm.com.tr/)).  
Aynı JSON yükü kullanılır.

### Dashboard'dan Bildirim Gönderme

Her VM/LXC kartında **"Send Alert"** butonu bulunur. Tıklandığında açılan modal'da:

- Hangi kanalların kullanılacağını seçin (yalnızca yapılandırılmış kanallar görünür)
- İsteğe bağlı özel mesaj yazın
- **Send Notification** butonuna tıklayın

Yanıtta kanal bazında başarı/hata durumu gösterilir.

---

## PWA Kurulumu

### Android (Chrome / Edge)

1. Chrome'da `http://<sunucu>:8080` adresini açın.
2. Başlıktaki **"Install App"** butonuna ya da tarayıcı menüsünden **"Ana Ekrana Ekle"** seçeneğine dokunun.
3. Uygulama tam ekran, bağımsız modda açılır.

### iOS (Safari)

1. URL'yi Safari'de açın.
2. **Paylaş** butonuna dokunun (ok içeren dikdörtgen).
3. **"Ana Ekrana Ekle"** seçeneğine dokunun.
4. Uygulama simgesi ana ekranınızda görünür.

### Masaüstü (Chrome / Edge)

Adres çubuğundaki kurulum ikonuna veya dashboard başlığındaki **"Install App"** butonuna tıklayın.

---

## Kaynaktan Derleme

```bash
# Standart derleme
go build -o homelab-dashboard .

# Optimize edilmiş / küçük binary
CGO_ENABLED=0 go build -ldflags="-s -w" -o homelab-dashboard .

# Raspberry Pi için çapraz derleme (arm64)
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o homelab-dashboard-arm64 .

# arm (32-bit) için çapraz derleme
GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -o homelab-dashboard-armv7 .
```

---

## Docker

```bash
docker build -t homelab-dashboard .
docker run -p 8080:8080 \
  -e PROXMOX_URL="https://192.168.1.100:8006" \
  -e PROXMOX_TOKEN="root@pam!dashboard=..." \
  -e TELEGRAM_BOT_TOKEN="..." \
  -e TELEGRAM_CHAT_ID="..." \
  homelab-dashboard
```

---

## API Referansı

### `GET /`

Render edilmiş HTML dashboard sayfasını döner.

### `GET /api/resources`

Proxmox kaynak verilerini JSON olarak döner.

### `POST /api/notify`

Bir veya birden fazla kanala bildirim gönderir.

**İstek gövdesi:**

```json
{
  "service": "ubuntu-server",
  "status": "stopped",
  "message": "İsteğe bağlı özel mesaj",
  "channels": ["telegram", "webhook"]
}
```

**Yanıt:**

```json
{
  "success": true,
  "results": {
    "telegram": "ok",
    "webhook": "ok"
  }
}
```

---

## Lisans

MIT © 2025 Gökhan Tombul
