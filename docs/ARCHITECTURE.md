# 🏗️ Architecture Documentation

## Overview

Share My Clipboard is a distributed peer-to-peer application built with a modular architecture that emphasizes separation of concerns, real-time synchronization, and network resilience.

---

## 📐 Architecture Principles

### 1. **Layered Architecture**
The application follows a clean layered approach:
- **Presentation Layer** — GUI and user interactions
- **Application Layer** — Business logic and orchestration
- **Network Layer** — P2P communication and discovery
- **Data Layer** — Clipboard and file management

### 2. **Event-Driven Communication**
Components communicate via callbacks and channels, enabling loose coupling and high responsiveness.

### 3. **Concurrency Model**
Go's goroutines and channels are used extensively for:
- Non-blocking UI updates
- Parallel file transfers
- Background network monitoring

---

## 🔄 Data Flow Diagrams

### 1. Clipboard Synchronization Flow

```
┌─────────────┐
│   User      │
│  Copies     │
│   Text      │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────────┐
│            Clipboard Manager            │
│  ┌───────────────────────────────────┐  │
│  │  Watch OS Clipboard Changes       │  │
│  │  (polling every 100ms)            │  │
│  └───────────────┬───────────────────┘  │
│                  │                      │
│                  ▼                      │
│  ┌───────────────────────────────────┐  │
│  │  Detect Change → Generate Event   │  │
│  │  ClipboardContent{Type, Data}     │  │
│  └───────────────┬───────────────────┘  │
└──────────────────┼──────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────┐
│            Network Manager               │
│  ┌────────────────────────────────────┐  │
│  │  BroadcastClipboard(content)       │  │
│  │  → Send to all connected peers     │  │
│  └────────────────┬───────────────────┘  │
└─────────────────┬─┼──────────────────────┘
                  │ │
         ┌────────┘ └────────┐
         ▼                   ▼
┌─────────────────┐   ┌─────────────────┐
│   Peer A        │   │   Peer B        │
│  (TCP Socket)   │   │  (TCP Socket)   │
└────────┬────────┘   └────────┬────────┘
         │                     │
         ▼                     ▼
   ┌──────────────┐      ┌──────────────┐
   │ Receive Data │      │ Receive Data │
   │ Update Local │      │ Update Local │
   │  Clipboard   │      │  Clipboard   │
   └──────────────┘      └──────────────┘
```

### 2. File Transfer Flow (Chunked)

```
┌──────────────┐
│  User clicks │
│  "Send File" │
└──────┬───────┘
       │
       ▼
┌─────────────────────────────────────────┐
│          IPC Client                     │
│  SendFiles(filePaths) → IPC Server      │
└───────────────┬─────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────┐
│          IPC Server Handler             │
│  1. Read file from disk                 │
│  2. Compute SHA256 checksum             │
│  3. Split into 512KB chunks             │
└───────────────┬─────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────┐
│      Network Manager (Chunked)          │
│                                         │
│  For each connected peer:               │
│  ┌───────────────────────────────────┐  │
│  │ 1. Send FileChunkStart            │  │
│  │    {FileName, TotalSize, Chunks}  │  │
│  │                                   │  │
│  │ 2. Send FileChunkData (loop)      │  │
│  │    {ChunkIndex, Data[512KB]}      │  │
│  │                                   │  │
│  │ 3. Send FileChunkComplete         │  │
│  │    {FileID, Checksum}             │  │
│  └───────────────────────────────────┘  │
└───────────────┬─────────────────────────┘
                │
                ▼
         ┌──────────────┐
         │  Peer Device │
         └──────┬───────┘
                │
                ▼
┌─────────────────────────────────────────┐
│      Peer: Chunk Assembly               │
│  1. Receive FileChunkStart              │
│  2. Create empty chunks map             │
│  3. Receive FileChunkData (N times)     │
│     → Store in chunks[index]            │
│  4. Receive FileChunkComplete           │
│  5. Reassemble: chunk[0] + ... + [N]    │
│  6. Verify checksum                     │
│  7. Write to disk if valid              │
└─────────────────────────────────────────┘
```

---

## 🌐 Network Architecture

### Device Discovery (UDP Broadcast)

```go
// Pseudo-code
func DiscoverDevices() {
    1. Broadcast UDP packet on port 9999
       Message: {Type: "DISCOVER", HostName: "MyPC"}
    
    2. Listen for responses on same port
       Response: {Type: "RESPONSE", HostName: "OtherPC", IP: "192.168.0.X"}
    
    3. Add discovered devices to DeviceStore
}
```

### P2P Connection (TCP)

```go
// Pseudo-code
func EstablishConnection(peerIP string) {
    1. Send ConnectionRequest
       {FromName: "MyPC", FromIP: "192.168.0.105"}
    
    2. Peer shows acceptance dialog
    
    3. Receive ConnectionResponse
       {Accept: true/false}
    
    4. If accepted:
       - Open persistent TCP socket
       - Start message handler goroutine
       - Register callbacks for data events
}
```

### Message Protocol

All messages are JSON-encoded with a simple envelope:

```json
{
  "type": "clipboard_text | file_chunk_start | file_chunk_data | ...",
  "data": { ... }
}
```

**Message Types:**
- `connection_request` — Request to connect
- `connection_response` — Accept/decline connection
- `clipboard_text` — Text clipboard content
- `file_chunk_start` — Begin file transfer
- `file_chunk_data` — File data chunk (512KB)
- `file_chunk_complete` — End file transfer
- `disconnect` — Graceful disconnect

---

## 🔐 Security Considerations

### Current Implementation
- **Local network only** — No external connections
- **No authentication** — Relies on physical network security
- **No encryption** — Data transmitted in plaintext (acceptable on private networks)

### Future Enhancements (Roadmap)
- Optional password protection for connections
- TLS encryption for transfers
- Certificate-based trust model

---

## 📦 Key Design Patterns

### 1. **Observer Pattern**
Clipboard changes and network events are broadcast to registered listeners.

```go
type ClipboardManager struct {
    listeners []chan ClipboardContent
}

func (m *ClipboardManager) Watch() chan ClipboardContent {
    ch := make(chan ClipboardContent, 10)
    m.listeners = append(m.listeners, ch)
    return ch
}
```

### 2. **Strategy Pattern**
Different content types (text, file, image) have separate handling strategies.

```go
type ContentHandler interface {
    Handle(content ClipboardContent) error
}

type TextHandler struct{}
type FileHandler struct{}
type ImageHandler struct{}
```

### 3. **Singleton Pattern**
IPC server and network manager are singletons to ensure single instance.

```go
var (
    ipcServerInstance *IPCServer
    ipcServerOnce     sync.Once
)

func GetIPCServer() *IPCServer {
    ipcServerOnce.Do(func() {
        ipcServerInstance, _ = NewIPCServer()
    })
    return ipcServerInstance
}
```

---

## 🚀 Performance Optimizations

### 1. **Chunked File Transfer**
Large files are split into 512KB chunks for:
- Reduced memory footprint
- Parallel processing potential
- Better error recovery

### 2. **Connection Pooling**
Persistent TCP connections avoid reconnection overhead.

### 3. **Buffered Channels**
Prevents blocking on high-frequency clipboard changes:
```go
clipboardChan := make(chan ClipboardContent, 100)
```

### 4. **Goroutine Pooling**
File transfers use dedicated goroutines to avoid blocking main thread.

---

## 📚 References

- [Fyne GUI Framework](https://fyne.io/)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [TCP/IP Network Programming](https://beej.us/guide/bgnet/)
- [Clipboard API Design](https://developer.mozilla.org/en-US/docs/Web/API/Clipboard_API)
