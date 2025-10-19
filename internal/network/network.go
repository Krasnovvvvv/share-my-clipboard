package network

import (
	"reflect"
	"sync"

	"github.com/schollz/peerdiscovery"
)

type Device struct {
	Name string
	IP   string
}

type DeviceStore struct {
	Devices   []Device
	DevicesMu sync.RWMutex
}

func (s *DeviceStore) Scan(hostname string) bool {
	discoveries, _ := peerdiscovery.Discover(peerdiscovery.Settings{
		Limit:     -1,
		Payload:   []byte(hostname),
		Port:      "8877",
		TimeLimit: 2,
	})
	found := []Device{}
	for _, d := range discoveries {
		found = append(found, Device{
			Name: string(d.Payload),
			IP:   d.Address,
		})
	}
	s.DevicesMu.Lock()
	changed := !reflect.DeepEqual(s.Devices, found)
	s.Devices = found
	s.DevicesMu.Unlock()
	return changed
}

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
