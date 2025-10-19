package network

import (
	"sync"

	"github.com/schollz/peerdiscovery"
)

// Device представляет найденное устройство в сети.
type Device struct {
	Name        string
	IP          string
	IsConnected bool
}

// DeviceStore управляет списком видимых устройств.
type DeviceStore struct {
	Devices   []Device
	DevicesMu sync.RWMutex
}

// Scan выполняет сетевое сканирование и аккуратно обновляет список устройств,
// не удаляя ранее найденные, если они временно не ответили.
func (s *DeviceStore) Scan(hostname string) bool {
	discoveries, _ := peerdiscovery.Discover(peerdiscovery.Settings{
		Limit:     -1,
		Payload:   []byte(hostname),
		Port:      "8877",
		TimeLimit: 2,
	})

	s.DevicesMu.Lock()
	defer s.DevicesMu.Unlock()

	existing := make(map[string]*Device)
	for i := range s.Devices {
		existing[s.Devices[i].IP] = &s.Devices[i]
	}

	changed := false

	for _, d := range discoveries {
		ip := d.Address
		name := string(d.Payload)

		if dev, ok := existing[ip]; ok {
			// обновляем имя если изменилось
			if dev.Name != name {
				dev.Name = name
				changed = true
			}
			// помечаем, что устройство активно
			delete(existing, ip)
		} else {
			// добавляем новое устройство
			s.Devices = append(s.Devices, Device{
				Name: name,
				IP:   ip,
			})
			changed = true
		}
	}

	// Устройства, не ответившие в этом цикле, не удаляются —
	// просто считаем, что они временно недоступны.
	// Это стабилизирует список и исключает "мигание" UI.

	return changed
}

// GetPage возвращает порцию устройств для отображения.
func (s *DeviceStore) GetPage(page, pageSize int) []Device {
	s.DevicesMu.RLock()
	defer s.DevicesMu.RUnlock()

	start := page * pageSize
	end := start + pageSize
	if end > len(s.Devices) {
		end = len(s.Devices)
	}
	return s.Devices[start:end]
}

// ConnectionManager управляет состояниями подключений.
type ConnectionManager struct {
	active map[string]bool
	mu     sync.RWMutex
}

// NewConnectionManager создаёт новый менеджер подключений.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{active: make(map[string]bool)}
}

// Connect помечает устройство как подключённое.
func (c *ConnectionManager) Connect(ip string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active[ip] = true
}

// Disconnect помечает устройство как отключённое.
func (c *ConnectionManager) Disconnect(ip string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.active, ip)
}

// IsConnected проверяет, находится ли устройство в подключённом состоянии.
func (c *ConnectionManager) IsConnected(ip string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active[ip]
}
