package network

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/schollz/peerdiscovery"
)

const connectionPort = 54322

// ---------- DEVICE MODEL ----------
type Device struct {
	Name        string
	IP          string
	MAC         string
	IsConnected bool
}

type DeviceStore struct {
	Devices   []Device
	DevicesMu sync.RWMutex
}

// ---------- DISCOVERY ----------
func (s *DeviceStore) Scan(hostname string) bool {
	discoveries, _ := peerdiscovery.Discover(peerdiscovery.Settings{
		Limit:     -1,
		Payload:   []byte(hostname),
		Port:      "54322",
		TimeLimit: 3 * time.Second,
	})

	s.DevicesMu.Lock()
	defer s.DevicesMu.Unlock()

	seen := make(map[string]bool)
	changed := false

	for _, d := range discoveries {
		ip := d.Address
		name := string(d.Payload)
		if isIgnoredIP(ip) {
			continue
		}
		mac := getMAC(ip)
		seen[ip] = true
		found := false

		for i := range s.Devices {
			if s.Devices[i].MAC == mac {
				s.Devices[i].IP = ip
				s.Devices[i].Name = name
				found = true
				break
			}
		}
		if !found {
			s.Devices = append(s.Devices, Device{Name: name, IP: ip, MAC: mac})
			changed = true
		}
	}

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

func (s *DeviceStore) GetPage(page, size int) []Device {
	s.DevicesMu.RLock()
	defer s.DevicesMu.RUnlock()
	start := page * size
	end := start + size
	if end > len(s.Devices) {
		end = len(s.Devices)
	}
	return s.Devices[start:end]
}

func isIgnoredIP(ip string) bool {
	return ip == "127.0.0.1" ||
		strings.HasPrefix(ip, "192.168.56.") ||
		strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "169.254.")
}

// ---------- CONNECTION MANAGER ----------
type ConnectionManager struct {
	connections map[string]net.Conn
	OnRequest   func(req ConnectionRequest)
	OnResult    func(resp ConnectionResponse)
	mu          sync.RWMutex
	localIP     string
}

type ConnectionRequest struct {
	Type     string
	FromName string
	FromIP   string
	FromMAC  string
	ToIP     string
	ToMAC    string
}

type ConnectionResponse struct {
	Type    string
	FromIP  string
	FromMAC string
	ToIP    string
	Accept  bool
}

func NewConnectionManager() *ConnectionManager {
	c := &ConnectionManager{connections: map[string]net.Conn{}}
	c.localIP = getLocalIP()
	go c.listenTCP()
	return c
}

// ---------- TCP LISTENER ----------
func (c *ConnectionManager) listenTCP() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", c.localIP, connectionPort))
	if err != nil {
		fmt.Println("listenTCP error:", err)
		return
	}
	fmt.Println("TCP listener started on", listener.Addr())

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go c.handleConn(conn)
	}
}

func (c *ConnectionManager) handleConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	var base map[string]interface{}
	if json.Unmarshal(buf[:n], &base) != nil {
		return
	}

	switch base["Type"] {
	case "request":
		var r ConnectionRequest
		json.Unmarshal(buf[:n], &r)
		if c.OnRequest != nil {
			c.OnRequest(r)
		}
	case "response":
		var r ConnectionResponse
		json.Unmarshal(buf[:n], &r)
		if c.OnResult != nil {
			c.OnResult(r)
		}
	}
}

// ---------- CONNECTION SENDERS ----------
func (c *ConnectionManager) SendRequest(req ConnectionRequest) {
	req.Type = "request"
	b, _ := json.Marshal(req)
	conn, err := dialTCP(req.ToIP)
	if err != nil {
		fmt.Println("SendRequest error:", err)
		return
	}
	defer conn.Close()
	conn.Write(b)
}

func (c *ConnectionManager) SendResponse(resp ConnectionResponse) {
	resp.Type = "response"
	b, _ := json.Marshal(resp)
	conn, err := dialTCP(resp.ToIP)
	if err != nil {
		fmt.Println("SendResponse error:", err)
		return
	}
	defer conn.Close()
	conn.Write(b)
}

// Принудительная привязка Dial к активному физическому IP
func dialTCP(toIP string) (net.Conn, error) {
	localIP := getLocalIP()
	laddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", localIP))
	raddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", toIP, connectionPort))
	dialer := net.Dialer{LocalAddr: laddr, Timeout: 3 * time.Second}
	return dialer.Dial("tcp", raddr.String())
}

// ---------- STATE CONTROL ----------
func (c *ConnectionManager) Connect(ip string, conn net.Conn) {
	c.mu.Lock()
	c.connections[ip] = conn
	c.mu.Unlock()
}

func (c *ConnectionManager) Disconnect(ip string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, ok := c.connections[ip]
	if ok && conn != nil {
		_ = conn.Close()
	}
	delete(c.connections, ip)
	fmt.Println("Disconnected from", ip)
}

func (c *ConnectionManager) IsConnected(ip string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.connections[ip]
	return ok
}

// ---------- IP DETECTION ----------
func getLocalIP() string {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		name := strings.ToLower(iface.Name)
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		if strings.Contains(name, "vbox") || strings.Contains(name, "virtual") || strings.Contains(name, "vm") {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				if isIgnoredIP(ip) {
					continue
				}
				return ip
			}
		}
	}
	// Подстраховка: получить IP активного маршрута
	conn, err := net.Dial("udp", "192.168.0.1:80")
	if err == nil {
		defer conn.Close()
		return conn.LocalAddr().(*net.UDPAddr).IP.String()
	}
	return "127.0.0.1"
}

func getMAC(ip string) string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.HardwareAddr != nil && len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String()
		}
	}
	return ""
}

// ---------- LEGACY COMPATIBILITY ----------
func (c *ConnectionManager) LegacyConnect(ip string) {
	c.mu.Lock()
	if _, exists := c.connections[ip]; !exists {
		c.connections[ip] = nil
	}
	c.mu.Unlock()
}

func (c *ConnectionManager) CheckDisconnects(ds *DeviceStore, update func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for ip := range c.connections {
		found := false
		ds.DevicesMu.RLock()
		for _, d := range ds.Devices {
			if d.IP == ip {
				found = true
				break
			}
		}
		ds.DevicesMu.RUnlock()
		if !found {
			delete(c.connections, ip)
			update()
		}
	}
}
