# 📋 Share My Clipboard

> **Seamless clipboard and file sharing across Windows devices on your local network** 🚀

[![Windows](https://img.shields.io/badge/platform-Windows-0078D6?style=flat-square&logo=windows)](https://www.microsoft.com/windows)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)

Share My Clipboard is a powerful, user-friendly desktop application that enables instant clipboard synchronization and file transfers between Windows computers on the same network — no cloud, no internet required! 🌐

---

## ✨ Key Features

### 🔄 **Real-Time Clipboard Sync**
- **Instant text sharing** — Copy on one device, paste on another immediately
- **Automatic synchronization** — No manual triggers needed
- **Bidirectional support** — Works seamlessly between any connected devices

### 📁 **Advanced File Sharing**
- **Drag & drop files** — Copy files to clipboard and they're instantly shared
- **Right-click context menu** — "Send to Connected Devices" integration
- **Multiple file support** — Send several files at once
- **Large file transfers** — Handles files up to 1GB+ with chunked streaming
- **Smart file detection** — Automatically identifies images, documents, and executables
- **Fast transfers** — 512KB chunks for optimal network utilization (10x faster than traditional methods)

### 🖼️ **Image Support**
- **Screenshots** — Instantly share screenshots between devices
- **Image clipboard** — Copy images in any app and they appear on connected devices
- **Auto-save** — Received files are saved to `Downloads/ShareMyClipboard`

### 🔌 **Zero-Configuration Networking**
- **Automatic device discovery** — Finds devices on your network automatically
- **Peer-to-peer connections** — Direct device-to-device communication
- **No server required** — Works entirely on your local network
- **Connection requests** — Accept/decline connections with friendly device names

### 🎨 **Modern GUI**
- **Dark theme** — Easy on the eyes
- **Real-time notifications** — See when content is received
- **Device management** — Visual interface showing all available devices
- **Connection status** — Know exactly which devices are connected

### 🛠️ **Windows Integration**
- **Context menu** — Right-click any file → "Send to Connected Devices"
- **Background operation** — Runs silently without console windows
- **System tray support** — Minimizes to tray for non-intrusive operation
- **Auto-start option** — Launch on Windows startup

### 🔒 **Privacy & Security**
- **Local network only** — Data never leaves your network
- **No cloud storage** — Your files stay on your devices
- **No telemetry** — Zero data collection
- **Direct P2P** — No intermediary servers

---

## 🚀 Quick Start

### Installation

1. **Download** the latest release from [Releases](https://github.com/Krasnovvvvv/share-my-clipboard/releases)
2. **Extract** `share-my-clipboard.exe` to any folder
3. **Run** the executable — GUI will open automatically

### First Use

1. **Launch** the application on all devices you want to connect
2. **Wait** for device discovery (2-3 seconds)
3. **Click "Connect"** on any discovered device
4. **Accept** the connection request on the other device
5. **Done!** Start copying and pasting 🎉

---

## 📖 Usage

### Sharing Text
1. Copy text on any connected device
2. It automatically appears in clipboard on all other devices
3. Paste anywhere!

### Sharing Files via Clipboard
1. Right-click file(s) → "Copy as path"
2. File(s) automatically sent to all connected devices
3. Files appear in `Downloads/ShareMyClipboard` on receiving devices

### Sharing Files via Context Menu
1. Right-click file(s) → "Send to Connected Devices"
2. Files instantly sent to all connected devices
3. Receive confirmation notification

### Sharing Screenshots
1. Take a screenshot (e.g., Win+Shift+S)
2. Screenshot automatically sent to connected devices
3. Paste on any device to use

---

## 🏗️ Architecture

Share My Clipboard is built using a modular, scalable architecture. For detailed technical documentation, see [ARCHITECTURE.md](docs/ARCHITECTURE.md)

### High-Level Overview

```
┌─────────────────────────────────────────────────────────┐
│                     User Interface (Fyne)                │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐ │
│  │  Device List │  │ Notification │  │ Context Menu   │ │
│  │  Management  │  │   System     │  │  Integration   │ │
│  └─────────────┘  └──────────────┘  └────────────────┘ │
└─────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────▼─────────────────────────────┐
│                  Application Core                      │
│  ┌──────────────┐  ┌────────────────┐  ┌───────────┐ │
│  │   Clipboard  │  │    Network     │  │    IPC    │ │
│  │   Manager    │  │    Manager     │  │  Server   │ │
│  └──────────────┘  └────────────────┘  └───────────┘ │
└────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────▼─────────────────────────────┐
│                 Network Layer (TCP)                    │
│  ┌──────────────┐  ┌────────────────┐  ┌───────────┐ │
│  │   Discovery  │  │   P2P Connect  │  │  Chunked  │ │
│  │  (Broadcast) │  │   Management   │  │  Transfer │ │
│  └──────────────┘  └────────────────┘  └───────────┘ │
└────────────────────────────────────────────────────────┘
```

### Core Components

- **GUI Layer** (Fyne) — Cross-platform desktop UI
- **Clipboard Manager** — Monitors and syncs clipboard content
- **Network Manager** — P2P discovery and connections
- **IPC Server** — Inter-process communication for context menu
- **Context Menu Integration** — Windows Shell Extension
- **File Transfer Engine** — Chunked streaming with checksums

---

## 🔧 Building from Source

### Prerequisites
- Go 1.21 or higher
- Windows 10/11
- Git

### Build Steps

```bash
# Clone repository
git clone https://github.com/Krasnovvvvv/share-my-clipboard.git
cd share-my-clipboard

# Install dependencies
go mod download

# Build GUI application (no console window)
go build -ldflags="-H=windowsgui" -o share-my-clipboard.exe

# Run
./share-my-clipboard.exe
```

### Development Build (with console for debugging)

```bash
go build -o share-my-clipboard-debug.exe
./share-my-clipboard-debug.exe
```

---

## 📚 Documentation

- [**Architecture Guide**](docs/ARCHITECTURE.md) — Deep dive into technical design
- [**Технические детали (RU)**](docs/TECHNICAL_RU.md) — Подробное объяснение технологий
- [**API Documentation**](docs/API.md) — Internal API reference
- [**Contributing Guide**](CONTRIBUTING.md) — How to contribute

---

## 🤝 Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details

---

## 📝 License

This project is licensed under the MIT License - see [LICENSE](LICENSE) file for details

---

## 🌟 Show Your Support

If you like this project, please give it a ⭐ on GitHub!

---

## 📬 Contact

- **GitHub Issues**: [Report bugs or request features](https://github.com/Krasnovvvvv/share-my-clipboard/issues)
- **Discussions**: [Join the community](https://github.com/Krasnovvvvv/share-my-clipboard/discussions)

---

## 🙏 Acknowledgments

Built with amazing open-source projects:
- [Fyne](https://fyne.io/) — Cross-platform GUI toolkit
- [golang-design/clipboard](https://github.com/golang-design/clipboard) — Universal clipboard package
- [schollz/peerdiscovery](https://github.com/schollz/peerdiscovery) — Zero-config peer discovery
