package network

import (
	"sync"

	"github.com/schollz/peerdiscovery"
)

// Device — структура устройства
type Device struct {
	Name        string
	IP          string
	IsConnected bool
}

// DeviceStore управляет списком устройств
type DeviceStore struct {
	Devices   []Device
	DevicesMu sync.RWMutex
}

// Scan обновляет список устройств, добавляет новые и удаляет те, которые пропали
func (s *DeviceStore) Scan(hostname string) bool {
	discoveries, _ := peerdiscovery.Discover(peerdiscovery.Settings{
		Limit:     -1,
		Payload:   []byte(hostname),
		Port:      "8877",
		TimeLimit: 2,
	})

	s.DevicesMu.Lock()
	defer s.DevicesMu.Unlock()

	seen := make(map[string]bool)
	changed := false

	for _, d := range discoveries {
		ip := d.Address
		name := string(d.Payload)
		seen[ip] = true

		found := false
		for i := range s.Devices {
			if s.Devices[i].IP == ip {
				found = true
				if s.Devices[i].Name != name {
					s.Devices[i].Name = name
					changed = true
				}
				break
			}
		}
		if !found {
			s.Devices = append(s.Devices, Device{Name: name, IP: ip})
			changed = true
		}
	}

	// удаляем устаревшие устройства
	filtered := s.Devices[:0]
	for _, dev := range s.Devices {
		if seen[dev.IP] {
			filtered = append(filtered, dev)
		} else {
			changed = true
		}
	}
	s.Devices = filtered

	return changed
}

// GetPage возвращает текущую страницу списка
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

// ConnectionManager управляет активными соединениями и обменом запросами
type ConnectionManager struct {
	active    map[string]bool
	mu        sync.RWMutex
	OnRequest func(req ConnectionRequest)
	OnResult  func(resp ConnectionResponse)
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		active: make(map[string]bool),
	}
}

// Connect / Disconnect / IsConnected
func (c *ConnectionManager) Connect(ip string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active[ip] = true
}
func (c *ConnectionManager) Disconnect(ip string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.active, ip)
}
func (c *ConnectionManager) IsConnected(ip string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active[ip]
}

// ConnectionRequest — запрос на подключение
type ConnectionRequest struct {
	FromName string
	FromIP   string
	ToIP     string
}

// ConnectionResponse — ответ на запрос
type ConnectionResponse struct {
	FromIP string
	ToIP   string
	Accept bool
}

// HandleRequest и HandleResponse пока работают локально (эмуляция сети)
func (c *ConnectionManager) SendRequest(req ConnectionRequest) {
	if c.OnRequest != nil {
		go c.OnRequest(req)
	}
}
func (c *ConnectionManager) SendResponse(resp ConnectionResponse) {
	if c.OnResult != nil {
		go c.OnResult(resp)
	}
}
