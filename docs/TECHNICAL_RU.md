# 🔬 Технические детали реализации Share My Clipboard

## Введение

Этот документ содержит подробное объяснение технологий и архитектурных решений, использованных в проекте Share My Clipboard. Он создан для образовательных целей и поможет понять, как работают различные компоненты системы.

---

## 📚 Оглавление

1. [Технологический стек](#️-технологический-стек)
2. [Архитектура приложения](#-архитектура-приложения)
3. [Сетевая коммуникация](#-сетевая-коммуникация)
4. [Работа с буфером обмена](#-работа-с-буфером-обмена)
5. [Передача файлов](#-передача-файлов)
6. [IPC и интеграция с Windows](#ipc-и-интеграция-с-windows)
7. [Многопоточность и синхронизация](#-многопоточность-и-синхронизация)
8. [Обработка ошибок](#-обработка-ошибок)

---

## 🛠️ Технологический стек

### Язык программирования: Go (Golang)

**Почему Go?**
- **Встроенная поддержка многопоточности** через goroutines и channels
- **Быстрая компиляция** в нативные бинарные файлы
- **Простота работы с сетью** благодаря стандартной библиотеке `net`
- **Кроссплатформенность** (хотя сейчас проект только для Windows)
- **Сборка мусора** автоматически управляет памятью

### Основные библиотеки

#### 1. **Fyne** — GUI фреймворк
```go
import "fyne.io/fyne/v2"
```

**Что это?**  
Fyne — это современный кроссплатформенный фреймворк для создания графических интерфейсов на Go.

**Почему Fyne?**
- Нативный вид на каждой платформе
- Не требует установки дополнительных зависимостей
- Поддержка тёмной темы из коробки
- Простой декларативный API

**Как используется в проекте:**
```go
a := app.NewWithID("com.krasnov.clipboard")
a.Settings().SetTheme(theme.DarkTheme())
w := a.NewWindow("Share My Clipboard")
w.Resize(fyne.NewSize(440, 530))
```

- `app.NewWithID` создаёт приложение с уникальным ID (для хранения настроек)
- `theme.DarkTheme()` применяет тёмную тему
- `NewWindow` создаёт главное окно приложения

#### 2. **golang.design/x/clipboard** — работа с буфером обмена
```go
import "golang.design/x/clipboard"
```

**Что это делает?**  
Предоставляет кроссплатформенный доступ к системному буферу обмена.

**Как работает:**
- **Чтение:** `clipboard.Read(clipboard.FmtText)` — получить текст из буфера
- **Запись:** `clipboard.Write(clipboard.FmtText, data)` — записать текст
- **Мониторинг:** постоянный опрос (polling) каждые 100 мс для обнаружения изменений

**Пример из проекта:**
```go
func (m *Manager) Watch() chan ClipboardContent {
    go func() {
        ticker := time.NewTicker(100 * time.Millisecond)
        for range ticker.C {
            data := clipboard.Read(clipboard.FmtText)
            if len(data) > 0 && !bytes.Equal(data, m.lastContent) {
                m.channel <- ClipboardContent{
                    Type: ContentTypeText,
                    Text: string(data),
                }
                m.lastContent = data
            }
        }
    }()
    return m.channel
}
```

**Как это работает:**
1. `time.NewTicker` создаёт таймер, который срабатывает каждые 100 мс
2. `clipboard.Read` читает текущее содержимое буфера
3. `bytes.Equal` проверяет, изменилось ли содержимое
4. Если изменилось — отправляет событие в канал

#### 3. **golang.org/x/sys/windows** — Windows API
```go
import "golang.org/x/sys/windows/registry"
```

**Что это?**  
Низкоуровневый доступ к системным вызовам Windows.

**Где используется:**
- **Регистрация контекстного меню** через реестр Windows
- **Вызов WinAPI MessageBox** без консоли

**Пример регистрации в реестре:**
```go
func Register() error {
    key, _, err := registry.CreateKey(
        registry.CURRENT_USER,
        `Software\Classes\*\shell\ShareMyClipboard`,
        registry.ALL_ACCESS,
    )
    if err != nil {
        return err
    }
    defer key.Close()
    
    key.SetStringValue("", "Send to Connected Devices")
    key.SetStringValue("Icon", exePath)
    return nil
}
```

**Что происходит:**
1. `CreateKey` создаёт ключ в реестре (HKEY_CURRENT_USER)
2. Путь `Software\Classes\*\shell\...` — это место, где Windows ищет контекстные меню для файлов
3. `SetStringValue("", ...)` устанавливает текст пункта меню
4. `SetStringValue("Icon", ...)` устанавливает иконку

---

## 🏗️ Архитектура приложения

### Модульная структура

```
internal/
├── app/           # Оркестрация всех компонентов
├── clipboard/     # Управление буфером обмена
├── network/       # Сетевая коммуникация
├── ipc/           # Межпроцессное взаимодействие
├── contextmenu/   # Интеграция с Windows Shell
└── ui/            # UI компоненты
```

### Паттерны проектирования

#### 1. **Observer (Наблюдатель)**

**Проблема:** Нужно уведомлять несколько компонентов об изменениях буфера обмена.

**Решение:** Clipboard Manager отправляет события через канал, на который подписываются другие компоненты.

```go
// Clipboard Manager (Subject)
type Manager struct {
    channel chan ClipboardContent
}

func (m *Manager) Watch() chan ClipboardContent {
    return m.channel // Подписка на события
}

// Network Manager (Observer)
go func() {
    for clipContent := range clipboardMgr.Watch() {
        connMgr.BroadcastClipboard(clipContent.Text)
    }
}()
```

**Почему это удобно:**
- Clipboard Manager не знает, кто использует его данные
- Можно легко добавить новых подписчиков
- Асинхронная обработка через goroutines

#### 2. **Strategy (Стратегия)**

**Проблема:** Разные типы контента (текст, файл, изображение) требуют разной обработки.

**Решение:** Определяем тип контента и применяем соответствующую стратегию.

```go
type ClipboardContent struct {
    Type     ContentType // text, file, image
    Text     string
    FileName string
    FileData []byte
}

// Обработка в зависимости от типа
switch clipContent.Type {
case ContentTypeText:
    handleText(clipContent.Text)
case ContentTypeFile:
    handleFile(clipContent.FileName, clipContent.FileData)
case ContentTypeImage:
    handleImage(clipContent.FileData)
}
```

#### 3. **Singleton (Одиночка)**

**Проблема:** IPC сервер должен быть один на всё приложение.

**Решение:** Используем `sync.Once` для создания единственного экземпляра.

```go
var (
    ipcServerInstance *IPCServer
    ipcServerOnce     sync.Once
)

func NewIPCServer() (*IPCServer, error) {
    ipcServerOnce.Do(func() {
        listener, _ := net.Listen("tcp", "127.0.0.1:54323")
        ipcServerInstance = &IPCServer{listener: listener}
    })
    return ipcServerInstance, nil
}
```

**Что делает `sync.Once`:**
- Гарантирует, что функция выполнится только один раз
- Потокобезопасно (можно вызывать из разных goroutines)

---

## 🌐 Сетевая коммуникация

### 1. Обнаружение устройств (UDP Broadcast)

**Принцип работы:**
1. Приложение отправляет UDP-пакет на **broadcast адрес** (например, 192.168.1.255)
2. Все устройства в сети получают этот пакет
3. Устройства с запущенным приложением отвечают своими данными

```go
func Scan(hostname string) {
    // Отправка broadcast
    conn, _ := net.DialUDP("udp", nil, &net.UDPAddr{
        IP:   net.IPv4bcast, // 255.255.255.255
        Port: 9999,
    })
    message := fmt.Sprintf("DISCOVER:%s", hostname)
    conn.Write([]byte(message))
    
    // Приём ответов
    buffer := make([]byte, 1024)
    conn.SetReadDeadline(time.Now().Add(2 * time.Second))
    n, addr, _ := conn.ReadFromUDP(buffer)
    
    // Парсинг ответа
    response := string(buffer[:n])
    // "RESPONSE:OtherPC:192.168.0.105"
}
```

**Почему UDP?**
- Не требует установки соединения (быстрее)
- Broadcast работает только с UDP
- Потеря пакетов не критична для discovery

### 2. P2P Соединения (TCP)

**Почему TCP для передачи данных?**
- Гарантирует доставку пакетов в правильном порядке
- Автоматическая проверка целостности
- Поддержка больших объёмов данных

**Установка соединения:**
```go
func Connect(peerIP string) error {
    // 1. Отправка запроса на подключение
    conn, _ := net.Dial("tcp", fmt.Sprintf("%s:5000", peerIP))
    request := ConnectionRequest{
        FromName: "MyPC",
        FromIP:   "192.168.0.105",
    }
    json.NewEncoder(conn).Encode(request)
    
    // 2. Ожидание ответа
    var response ConnectionResponse
    json.NewDecoder(conn).Decode(&response)
    
    if response.Accept {
        // 3. Сохранение соединения
        connections[peerIP] = conn
        
        // 4. Запуск обработчика сообщений
        go handleMessages(conn)
    }
    return nil
}
```

**Persistent Connection:**
- Соединение держится открытым всё время
- Новое сообщение = просто запись в сокет
- Избегаем overhead на переподключение

### 3. Протокол сообщений (JSON)

**Формат:**
```json
{
  "type": "clipboard_text",
  "data": {
    "from_ip": "192.168.0.105",
    "content": "Hello, World!"
  }
}
```

**Почему JSON?**
- Человекочитаемый формат (легко отлаживать)
- Стандартная библиотека Go имеет отличную поддержку
- Гибкость структуры данных

**Альтернативы:**
- **Protocol Buffers** — быстрее, но сложнее
- **MessagePack** — компактнее, но менее читаемый
- **Простые байты** — максимальная скорость, но без структуры

---

## 📋 Работа с буфером обмена

### Polling vs Event-based

**Polling (используется в проекте):**
```go
ticker := time.NewTicker(100 * time.Millisecond)
for range ticker.C {
    currentData := clipboard.Read(clipboard.FmtText)
    if !bytes.Equal(currentData, lastData) {
        // Буфер изменился!
        lastData = currentData
    }
}
```

**Плюсы:**
- Простота реализации
- Работает на всех платформах

**Минусы:**
- Задержка до 100 мс перед обнаружением изменения
- Постоянное использование CPU (минимальное)

**Event-based (альтернатива):**
- Windows: `AddClipboardFormatListener` API
- Linux: X11 события
- macOS: NSPasteboard change count

**Почему polling:**
- `golang.design/clipboard` не поддерживает events
- Универсальность для будущей кроссплатформенности

---

## 📦 Передача файлов

### Chunked Transfer (Чанкированная передача)

**Проблема:** Файлы размером 1 GB нельзя загрузить в память целиком.

**Решение:** Разбиваем файл на куски (chunks) по 512 KB.

```go
const chunkSize = 512 * 1024 // 512 KB

func SendFile(fileName string, fileData []byte) {
    totalChunks := (len(fileData) + chunkSize - 1) / chunkSize
    
    // 1. Отправка метаданных
    sendMessage(FileChunkStart{
        FileName:    fileName,
        TotalSize:   len(fileData),
        TotalChunks: totalChunks,
        Checksum:    sha256.Sum256(fileData),
    })
    
    // 2. Отправка чанков
    for i := 0; i < totalChunks; i++ {
        start := i * chunkSize
        end := min(start+chunkSize, len(fileData))
        
        sendMessage(FileChunkData{
            ChunkIndex: i,
            Data:       fileData[start:end],
        })
    }
    
    // 3. Завершение
    sendMessage(FileChunkComplete{
        FileID: generateID(),
    })
}
```

**Получение на другой стороне:**
```go
type FileTransfer struct {
    FileName    string
    TotalChunks int
    Chunks      map[int][]byte // индекс → данные
}

func OnFileChunkData(chunk FileChunkData) {
    transfer.Chunks[chunk.ChunkIndex] = chunk.Data
    
    if len(transfer.Chunks) == transfer.TotalChunks {
        // Все чанки получены — собираем файл
        fileData := assembleChunks(transfer.Chunks)
        saveFile(transfer.FileName, fileData)
    }
}
```

**Почему 512 KB?**
- Баланс между количеством сетевых операций и размером буфера
- Меньше → больше overhead на передачу
- Больше → больше задержка при ошибке

### Проверка целостности (Checksum)

**SHA-256 хеш:**
```go
import "crypto/sha256"

func ComputeFileChecksum(data []byte) string {
    hash := sha256.Sum256(data)
    return fmt.Sprintf("%x", hash)
}
```

**Проверка на получателе:**
```go
if actualChecksum != expectedChecksum {
    return errors.New("file corrupted during transfer")
}
```

**Почему SHA-256?**
- Криптографически стойкий (невозможно подделать)
- Быстрый (около 300 MB/s на современном CPU)
- Стандарт в индустрии

---

## 🔄 IPC и интеграция с Windows

### Inter-Process Communication

**Проблема:** Контекстное меню запускает новый процесс, но GUI уже работает.

**Решение:** Второй процесс подключается к первому через TCP на localhost.

```go
// Процесс 1 (GUI) — запускает IPC сервер
func main() {
    ipcServer, _ := ipc.NewIPCServer() // Слушает 127.0.0.1:54323
    app.Run()
}

// Процесс 2 (контекстное меню) — отправляет команду
func main() {
    if len(os.Args) > 1 && os.Args[1] == "--send" {
        client := ipc.NewIPCClient()
        client.SendFiles([]string{os.Args[2]})
        os.Exit(0)
    }
}
```

**Формат IPC сообщения:**
```json
{
  "type": "send_files",
  "data": {
    "file_paths": ["C:\\Users\\...\\file.txt"]
  }
}
```

### Регистрация контекстного меню

**Реестр Windows:**
```
HKEY_CURRENT_USER\Software\Classes\*\shell\ShareMyClipboard
  (Default) = "Send to Connected Devices"
  Icon = "C:\path\to\app.exe"
  
  \command
    (Default) = "C:\path\to\app.exe" --send "%1"
```

**Что делает Windows:**
1. Пользователь нажимает ПКМ на файле
2. Windows читает `*\shell\ShareMyClipboard`
3. Показывает пункт меню "Send to Connected Devices"
4. При клике запускает: `app.exe --send "C:\path\to\selected\file.txt"`

---

## 🧵 Многопоточность и синхронизация

### Goroutines

**Что это?**  
Легковесные потоки, управляемые Go runtime (не OS threads).

**Использование в проекте:**
```go
// 1. Мониторинг буфера обмена
go func() {
    for range time.NewTicker(100 * time.Millisecond).C {
        checkClipboard()
    }
}()

// 2. Приём сетевых сообщений
go func() {
    for {
        message := readFromNetwork()
        handleMessage(message)
    }
}()

// 3. Передача файлов
go func() {
    for _, peer := range connectedPeers {
        sendFile(peer, fileData)
    }
}()
```

**Почему не OS threads?**
- Goroutines легче (~2 KB стека vs ~1 MB для thread)
- Можно запустить миллионы goroutines
- Автоматическое управление через Go scheduler

### Channels (Каналы)

**Что это?**  
Механизм передачи данных между goroutines (как очередь).

```go
ch := make(chan ClipboardContent, 10) // буфер на 10 элементов

// Отправитель
go func() {
    ch <- ClipboardContent{Text: "Hello"}
}()

// Получатель
go func() {
    content := <-ch
    fmt.Println(content.Text)
}()
```

**Буферизированный vs небуферизированный:**
- **Небуферизированный:** `make(chan T)` — отправитель блокируется до получения
- **Буферизированный:** `make(chan T, N)` — может отправить N элементов без блокировки

### Mutex (Взаимное исключение)

**Проблема:** Несколько goroutines одновременно изменяют данные → race condition.

```go
type DeviceStore struct {
    Devices   []Device
    DevicesMu sync.RWMutex // Read-Write Mutex
}

func (s *DeviceStore) AddDevice(d Device) {
    s.DevicesMu.Lock()         // Эксклюзивная блокировка
    defer s.DevicesMu.Unlock()
    s.Devices = append(s.Devices, d)
}

func (s *DeviceStore) GetDevices() []Device {
    s.DevicesMu.RLock()         // Shared блокировка (чтение)
    defer s.DevicesMu.RUnlock()
    return s.Devices
}
```

**RWMutex vs Mutex:**
- **Mutex:** только один может читать/писать
- **RWMutex:** много читателей ИЛИ один писатель

### WaitGroup (Ожидание завершения)

**Проблема:** Нужно дождаться завершения всех goroutines.

```go
var wg sync.WaitGroup

for _, peer := range peers {
    wg.Add(1) // Увеличиваем счётчик
    go func(p Peer) {
        defer wg.Done() // Уменьшаем счётчик при выходе
        sendFile(p, fileData)
    }(peer)
}

wg.Wait() // Блокируется до wg.Done() от всех
fmt.Println("Все файлы отправлены!")
```

---

## ⚠️ Обработка ошибок

### Паттерн: Error Wrapping

```go
func ReadFile(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read %s: %w", path, err)
    }
    return data, nil
}
```

**`%w` формат:**
- Оборачивает исходную ошибку
- Позволяет проверять тип: `errors.Is(err, os.ErrNotExist)`

### Graceful Degradation

**Принцип:** Приложение продолжает работать даже при ошибках.

```go
// Если IPC сервер не запустился — просто логируем
ipcServer, err := ipc.NewIPCServer()
if err != nil {
    log.Printf("Warning: IPC server failed: %v", err)
    // Приложение продолжает работу без контекстного меню
}
```

---

## 🎓 Ключевые концепции для изучения

### 1. **Concurrency vs Parallelism**
- **Concurrency:** несколько задач *выполняются* (не обязательно одновременно)
- **Parallelism:** несколько задач *одновременно выполняются* на разных ядрах

### 2. **Blocking vs Non-Blocking I/O**
- **Blocking:** операция останавливает поток до завершения
- **Non-Blocking:** операция возвращается сразу, результат позже

### 3. **TCP vs UDP**
- **TCP:** надёжный, упорядоченный, медленнее
- **UDP:** ненадёжный, быстрый, для broadcast

### 4. **JSON Encoding/Decoding**
```go
// Кодирование (Go → JSON)
json.NewEncoder(conn).Encode(message)

// Декодирование (JSON → Go)
var msg Message
json.NewDecoder(conn).Decode(&msg)
```

---

## 📚 Рекомендуемые ресурсы для изучения

1. **Книги:**
   - "The Go Programming Language" — Donovan & Kernighan
   - "Concurrency in Go" — Katherine Cox-Buday
   - "Network Programming with Go" — Jan Newmarch

2. **Онлайн курсы:**
   - [Tour of Go](https://go.dev/tour/) — официальный туториал
   - [Go by Example](https://gobyexample.com/) — практические примеры

3. **Документация:**
   - [Go Standard Library](https://pkg.go.dev/std)
   - [Fyne Docs](https://developer.fyne.io/)

---

**Удачи в изучении! Если возникнут вопросы — открывай issue на GitHub! 🚀**
