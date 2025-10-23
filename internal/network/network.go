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

const (
	connectionPort    = 54322
	heartbeatInterval = 5 * time.Second
	connectionTimeout = 15 * time.Second
	FileChunkSize     = 512 * 1024 // 512KB chunks
)

// ---------- MESSAGE TYPES ----------
type MessageType string

const (
	MsgTypeRequest      MessageType = "request"
	MsgTypeResponse     MessageType = "response"
	MsgTypeHeartbeat    MessageType = "heartbeat"
	MsgTypeHeartbeatAck MessageType = "heartbeat_ack"
	MsgTypeClipboard    MessageType = "clipboard"
	MsgTypeDisconnect   MessageType = "disconnect"
	MsgTypeShutdown     MessageType = "shutdown"

	MsgTypeFileChunkStart    MessageType = "file_chunk_start"
	MsgTypeFileChunkData     MessageType = "file_chunk_data"
	MsgTypeFileChunkComplete MessageType = "file_chunk_complete"
)

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

// ---------- MESSAGE STRUCTURES ----------
type Message struct {
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ConnectionRequest struct {
	FromName string `json:"from_name"`
	FromIP   string `json:"from_ip"`
	FromMAC  string `json:"from_mac"`
	ToIP     string `json:"to_ip"`
}

type ConnectionResponse struct {
	FromIP  string `json:"from_ip"`
	FromMAC string `json:"from_mac"`
	ToIP    string `json:"to_ip"`
	Accept  bool   `json:"accept"`
}

type HeartbeatMessage struct {
	FromIP    string    `json:"from_ip"`
	Timestamp time.Time `json:"timestamp"`
}

type ClipboardData struct {
	FromIP    string `json:"from_ip"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

type DisconnectMessage struct {
	FromIP string `json:"from_ip"`
	Reason string `json:"reason"`
}

type FileChunkStart struct {
	FileID      string `json:"file_id"`
	FileName    string `json:"file_name"`
	TotalSize   int64  `json:"total_size"`
	TotalChunks int    `json:"total_chunks"`
	Checksum    string `json:"checksum"`
	FromIP      string `json:"from_ip"`
}

type FileChunkData struct {
	FileID     string `json:"file_id"`
	ChunkIndex int    `json:"chunk_index"`
	Data       []byte `json:"data"`
}

type FileChunkComplete struct {
	FileID   string `json:"file_id"`
	Checksum string `json:"checksum"`
}

// ---------- CONNECTION STATE ----------
type ConnectionState struct {
	conn          net.Conn
	ip            string
	name          string
	isHub         bool
	lastHeartbeat time.Time
	readChan      chan Message
	writeChan     chan Message
	closeChan     chan struct{}
	mu            sync.RWMutex
}

// ---------- CONNECTION MANAGER ----------
type ConnectionManager struct {
	connections map[string]*ConnectionState
	listener    net.Listener
	LocalIP     string
	hostname    string
	mu          sync.RWMutex

	OnRequest           func(req ConnectionRequest)
	OnResult            func(resp ConnectionResponse)
	OnDisconnect        func(ip string, reason string)
	OnClipboard         func(data ClipboardData)
	OnFileChunkStart    func(start FileChunkStart)
	OnFileChunkData     func(chunk FileChunkData)
	OnFileChunkComplete func(complete FileChunkComplete)
	onConnEstablished   func(ip string)
}

func NewConnectionManager(hostname string) *ConnectionManager {
	c := &ConnectionManager{
		connections: make(map[string]*ConnectionState),
		hostname:    hostname,
	}
	c.LocalIP = getPreferredLocalIP()
	go c.listenTCP()
	return c
}

// ---------- DISCOVERY ----------
func (s *DeviceStore) Scan(hostname string) bool {
	discoveries, _ := peerdiscovery.Discover(peerdiscovery.Settings{
		Limit:     -1,
		Payload:   []byte(hostname),
		Port:      fmt.Sprintf("%d", connectionPort),
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

		mac := getMACForIP(ip)
		seen[ip] = true

		found := false
		for i := range s.Devices {
			if s.Devices[i].IP == ip {
				if s.Devices[i].Name != name || s.Devices[i].MAC != mac {
					s.Devices[i].Name = name
					s.Devices[i].MAC = mac
					changed = true
				}
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
	if start >= len(s.Devices) {
		return []Device{}
	}

	return s.Devices[start:end]
}

// ---------- TCP LISTENER ----------
func (c *ConnectionManager) listenTCP() {
	var err error
	c.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", connectionPort))
	if err != nil {
		fmt.Printf("listenTCP error: %v\n", err)
		return
	}

	fmt.Printf("TCP listener started on %s\n", c.listener.Addr())

	for {
		conn, err := c.listener.Accept()
		if err != nil {
			continue
		}

		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetKeepAlive(true)
			tcpConn.SetKeepAlivePeriod(30 * time.Second)
		}

		go c.handleIncomingConnection(conn)
	}
}

func (c *ConnectionManager) handleIncomingConnection(conn net.Conn) {
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{})

	var msg Message
	if err := json.Unmarshal(buf[:n], &msg); err != nil {
		conn.Close()
		return
	}

	switch msg.Type {
	case MsgTypeRequest:
		var req ConnectionRequest
		json.Unmarshal(msg.Data, &req)
		if c.OnRequest != nil {
			c.OnRequest(req)
		}
		conn.Close()

	case MsgTypeResponse:
		var resp ConnectionResponse
		json.Unmarshal(msg.Data, &resp)
		if c.OnResult != nil {
			c.OnResult(resp)
		}
		conn.Close()

	default:
		remoteIP := strings.Split(conn.RemoteAddr().String(), ":")[0]
		fmt.Printf("[DEBUG] Accepting persistent connection from %s\n", remoteIP)
		c.establishConnection(remoteIP, "", conn, false)
	}
}

// ---------- CONNECTION ESTABLISHMENT ----------
func (c *ConnectionManager) SendRequest(req ConnectionRequest) error {
	msg := Message{Type: MsgTypeRequest}
	msg.Data, _ = json.Marshal(req)
	return c.sendOneTimeMessage(req.ToIP, msg)
}

func (c *ConnectionManager) SendResponse(resp ConnectionResponse) error {
	msg := Message{Type: MsgTypeResponse}
	msg.Data, _ = json.Marshal(resp)
	return c.sendOneTimeMessage(resp.ToIP, msg)
}

func (c *ConnectionManager) sendOneTimeMessage(ip string, msg Message) error {
	conn, err := c.dialTCP(ip)
	if err != nil {
		return fmt.Errorf("sendOneTimeMessage dial error: %w", err)
	}
	defer conn.Close()

	data, _ := json.Marshal(msg)
	_, err = conn.Write(data)
	return err
}

func (c *ConnectionManager) Connect(ip, name string) error {
	time.Sleep(100 * time.Millisecond)

	conn, err := c.dialTCP(ip)
	if err != nil {
		return fmt.Errorf("connect dial error: %w", err)
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	fmt.Printf("[DEBUG] Initiating persistent connection to %s\n", ip)
	return c.establishConnection(ip, name, conn, true)
}

func (c *ConnectionManager) establishConnection(ip, name string, conn net.Conn, isHub bool) error {
	c.mu.Lock()

	if _, exists := c.connections[ip]; exists {
		c.mu.Unlock()
		conn.Close()
		fmt.Printf("[DEBUG] Already connected to %s, closing duplicate\n", ip)
		return fmt.Errorf("already connected to %s", ip)
	}

	state := &ConnectionState{
		conn:          conn,
		ip:            ip,
		name:          name,
		isHub:         isHub,
		lastHeartbeat: time.Now(),
		readChan:      make(chan Message, 100),
		writeChan:     make(chan Message, 100),
		closeChan:     make(chan struct{}),
	}

	c.connections[ip] = state
	c.mu.Unlock()

	fmt.Printf("[DEBUG] Connection established with %s (isHub=%v)\n", ip, isHub)

	go c.readLoop(state)
	go c.writeLoop(state)
	go c.heartbeatLoop(state)

	if c.onConnEstablished != nil {
		c.onConnEstablished(ip)
	}

	return nil
}

// ---------- CONNECTION LOOPS ----------
func (c *ConnectionManager) readLoop(state *ConnectionState) {
	defer c.handleConnectionClose(state)

	// Use JSON decoder for proper streaming
	dec := json.NewDecoder(state.conn)

	for {
		select {
		case <-state.closeChan:
			return
		default:
		}

		state.conn.SetReadDeadline(time.Now().Add(connectionTimeout))

		var msg Message
		if err := dec.Decode(&msg); err != nil {
			fmt.Printf("[DEBUG] Read/decode error from %s: %v\n", state.ip, err)
			return
		}

		c.handleMessage(state, msg)
	}
}

func (c *ConnectionManager) writeLoop(state *ConnectionState) {
	// Use JSON encoder for proper streaming
	enc := json.NewEncoder(state.conn)

	for {
		select {
		case <-state.closeChan:
			return
		case msg := <-state.writeChan:
			state.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := enc.Encode(&msg); err != nil {
				fmt.Printf("[DEBUG] Write error to %s: %v\n", state.ip, err)
				return
			}
		}
	}
}

func (c *ConnectionManager) heartbeatLoop(state *ConnectionState) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-state.closeChan:
			return
		case <-ticker.C:
			hb := HeartbeatMessage{
				FromIP:    c.LocalIP,
				Timestamp: time.Now(),
			}
			msg := Message{Type: MsgTypeHeartbeat}
			msg.Data, _ = json.Marshal(hb)

			select {
			case state.writeChan <- msg:
			case <-time.After(1 * time.Second):
				return
			}

			state.mu.RLock()
			lastHB := state.lastHeartbeat
			state.mu.RUnlock()

			if time.Since(lastHB) > connectionTimeout {
				fmt.Printf("[DEBUG] Connection to %s timed out\n", state.ip)
				return
			}
		}
	}
}

func (c *ConnectionManager) handleMessage(state *ConnectionState, msg Message) {
	switch msg.Type {
	case MsgTypeHeartbeat:
		ack := Message{Type: MsgTypeHeartbeatAck}
		select {
		case state.writeChan <- ack:
		default:
		}

	case MsgTypeHeartbeatAck:
		state.mu.Lock()
		state.lastHeartbeat = time.Now()
		state.mu.Unlock()

	case MsgTypeClipboard:
		var clipData ClipboardData
		if err := json.Unmarshal(msg.Data, &clipData); err == nil {
			if c.OnClipboard != nil {
				c.OnClipboard(clipData)
			}
		}

	case MsgTypeFileChunkStart:
		var start FileChunkStart
		if err := json.Unmarshal(msg.Data, &start); err == nil {
			if c.OnFileChunkStart != nil {
				c.OnFileChunkStart(start)
			}
		}

	case MsgTypeFileChunkData:
		var chunk FileChunkData
		if err := json.Unmarshal(msg.Data, &chunk); err == nil {
			if c.OnFileChunkData != nil {
				c.OnFileChunkData(chunk)
			}
		}

	case MsgTypeFileChunkComplete:
		var complete FileChunkComplete
		if err := json.Unmarshal(msg.Data, &complete); err == nil {
			if c.OnFileChunkComplete != nil {
				c.OnFileChunkComplete(complete)
			}
		}

	case MsgTypeDisconnect:
		var discMsg DisconnectMessage
		if err := json.Unmarshal(msg.Data, &discMsg); err == nil {
			if c.OnDisconnect != nil {
				c.OnDisconnect(discMsg.FromIP, discMsg.Reason)
			}
		}

	case MsgTypeShutdown:
		if c.OnDisconnect != nil {
			c.OnDisconnect(state.ip, "Hub shutdown")
		}
	}
}

func (c *ConnectionManager) handleConnectionClose(state *ConnectionState) {
	state.conn.Close()

	c.mu.Lock()
	delete(c.connections, state.ip)
	c.mu.Unlock()

	fmt.Printf("[DEBUG] Connection closed with %s\n", state.ip)

	if c.OnDisconnect != nil {
		c.OnDisconnect(state.ip, "Connection closed")
	}
}

// ---------- DISCONNECTION ----------
func (c *ConnectionManager) Disconnect(ip string) error {
	c.mu.Lock()
	state, exists := c.connections[ip]
	c.mu.Unlock()

	if !exists {
		return fmt.Errorf("not connected to %s", ip)
	}

	discMsg := DisconnectMessage{
		FromIP: c.LocalIP,
		Reason: "User disconnected",
	}
	msg := Message{Type: MsgTypeDisconnect}
	msg.Data, _ = json.Marshal(discMsg)

	select {
	case state.writeChan <- msg:
		time.Sleep(100 * time.Millisecond)
	case <-time.After(1 * time.Second):
	}

	close(state.closeChan)
	return nil
}

func (c *ConnectionManager) DisconnectAll() {
	c.mu.RLock()
	ips := make([]string, 0, len(c.connections))
	for ip := range c.connections {
		ips = append(ips, ip)
	}
	c.mu.RUnlock()

	for _, ip := range ips {
		c.Disconnect(ip)
	}
}

func (c *ConnectionManager) ShutdownAsHub() {
	msg := Message{Type: MsgTypeShutdown}

	c.mu.RLock()
	for _, state := range c.connections {
		select {
		case state.writeChan <- msg:
		case <-time.After(500 * time.Millisecond):
		}
	}
	c.mu.RUnlock()

	time.Sleep(200 * time.Millisecond)
	c.DisconnectAll()
}

// ---------- CLIPBOARD BROADCAST ----------
func (c *ConnectionManager) BroadcastClipboard(content string) {
	clipData := ClipboardData{
		FromIP:    c.LocalIP,
		Content:   content,
		Timestamp: time.Now().Unix(),
	}

	msg := Message{Type: MsgTypeClipboard}
	msg.Data, _ = json.Marshal(clipData)

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, state := range c.connections {
		select {
		case state.writeChan <- msg:
		case <-time.After(500 * time.Millisecond):
			fmt.Printf("Failed to send clipboard to %s\n", state.ip)
		}
	}
}

// ---------- FILE TRANSFER WITH CHUNKING ----------
func (c *ConnectionManager) BroadcastFileClipboard(fileName string, fileData []byte, checksum string) {
	fileID := fmt.Sprintf("%s_%d", fileName, time.Now().Unix())
	fileSize := int64(len(fileData))
	totalChunks := (len(fileData) + FileChunkSize - 1) / FileChunkSize

	fmt.Printf("[NET] Broadcasting file %s in %d chunks (%d KB)\n", fileName, totalChunks, fileSize/1024)

	c.mu.RLock()
	connections := make([]*ConnectionState, 0, len(c.connections))
	for _, state := range c.connections {
		connections = append(connections, state)
	}
	c.mu.RUnlock()

	//Use WaitGroup to ensure all sends complete
	var wg sync.WaitGroup
	for _, state := range connections {
		wg.Add(1)
		go func(st *ConnectionState) {
			defer wg.Done()
			c.sendFileToConnection(st, fileID, fileName, fileData, fileSize, totalChunks, checksum)
		}(state)
	}
	wg.Wait()
	fmt.Printf("[NET] All file transfers initiated\n")
}

func (c *ConnectionManager) sendFileToConnection(state *ConnectionState, fileID, fileName string,
	fileData []byte, fileSize int64, totalChunks int, checksum string) {

	// 1. Send start message
	start := FileChunkStart{
		FileID:      fileID,
		FileName:    fileName,
		TotalSize:   fileSize,
		TotalChunks: totalChunks,
		Checksum:    checksum,
		FromIP:      c.LocalIP,
	}

	msg := Message{Type: MsgTypeFileChunkStart}
	msg.Data, _ = json.Marshal(start)

	select {
	case state.writeChan <- msg:
	case <-time.After(5 * time.Second):
		fmt.Printf("[NET] Failed to send file start to %s\n", state.ip)
		return
	}

	fmt.Printf("[NET] Sending file %s to %s in %d chunks\n", fileName, state.ip, totalChunks)

	// 2. Send chunks
	for i := 0; i < totalChunks; i++ {
		startIdx := i * FileChunkSize
		endIdx := startIdx + FileChunkSize
		if endIdx > len(fileData) {
			endIdx = len(fileData)
		}

		chunkData := FileChunkData{
			FileID:     fileID,
			ChunkIndex: i,
			Data:       fileData[startIdx:endIdx],
		}

		msg := Message{Type: MsgTypeFileChunkData}
		msg.Data, _ = json.Marshal(chunkData)

		select {
		case state.writeChan <- msg:
			// No delay for maximum speed
		case <-time.After(10 * time.Second):
			fmt.Printf("[NET] Failed to send chunk %d/%d to %s\n", i+1, totalChunks, state.ip)
			return
		}

		if (i+1)%10 == 0 || i == totalChunks-1 {
			fmt.Printf("[NET] Sent chunk %d/%d to %s\n", i+1, totalChunks, state.ip)
		}
	}

	// 3. Send complete message
	complete := FileChunkComplete{
		FileID:   fileID,
		Checksum: checksum,
	}

	msg = Message{Type: MsgTypeFileChunkComplete}
	msg.Data, _ = json.Marshal(complete)

	select {
	case state.writeChan <- msg:
		fmt.Printf("[NET] File %s sent successfully to %s\n", fileName, state.ip)
	case <-time.After(5 * time.Second):
		fmt.Printf("[NET] Failed to send file complete to %s\n", state.ip)
	}
}

// ---------- STATE QUERIES ----------
func (c *ConnectionManager) IsConnected(ip string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.connections[ip]
	return exists
}

func (c *ConnectionManager) GetConnectedIPs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ips := make([]string, 0, len(c.connections))
	for ip := range c.connections {
		ips = append(ips, ip)
	}
	return ips
}

func (c *ConnectionManager) SetOnConnEstablished(callback func(string)) {
	c.onConnEstablished = callback
}

// ---------- NETWORK UTILITIES ----------
func (c *ConnectionManager) dialTCP(toIP string) (net.Conn, error) {
	localIP := c.LocalIP
	laddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", localIP))
	raddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", toIP, connectionPort))

	dialer := net.Dialer{
		LocalAddr: laddr,
		Timeout:   5 * time.Second,
	}

	return dialer.Dial("tcp", raddr.String())
}

func getPreferredLocalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	var ethernetIP, wifiIP, otherIP string

	for _, iface := range interfaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}

		name := strings.ToLower(iface.Name)

		if strings.Contains(name, "vbox") ||
			strings.Contains(name, "virtual") ||
			strings.Contains(name, "vm") ||
			strings.Contains(name, "docker") ||
			strings.Contains(name, "veth") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}

			ip := ipnet.IP.String()
			if isIgnoredIP(ip) {
				continue
			}

			if strings.Contains(name, "eth") || strings.Contains(name, "en") {
				if ethernetIP == "" {
					ethernetIP = ip
				}
			} else if strings.Contains(name, "wlan") || strings.Contains(name, "wi") {
				if wifiIP == "" {
					wifiIP = ip
				}
			} else {
				if otherIP == "" {
					otherIP = ip
				}
			}
		}
	}

	if ethernetIP != "" {
		fmt.Printf("Selected Ethernet IP: %s\n", ethernetIP)
		return ethernetIP
	}
	if wifiIP != "" {
		fmt.Printf("Selected Wi-Fi IP: %s\n", wifiIP)
		return wifiIP
	}
	if otherIP != "" {
		fmt.Printf("Selected other IP: %s\n", otherIP)
		return otherIP
	}

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		fmt.Printf("Selected fallback IP: %s\n", localAddr.IP.String())
		return localAddr.IP.String()
	}

	return "127.0.0.1"
}

func isIgnoredIP(ip string) bool {
	return ip == "127.0.0.1" ||
		strings.HasPrefix(ip, "192.168.56.") ||
		strings.HasPrefix(ip, "169.254.") ||
		strings.HasPrefix(ip, "172.17.") ||
		strings.HasPrefix(ip, "172.18.")
}

func getMACForIP(ip string) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if ok && ipnet.IP.String() == ip {
				return iface.HardwareAddr.String()
			}
		}
	}

	return ""
}

func (s *DeviceStore) FindNameByIP(ip string) string {
	s.DevicesMu.RLock()
	defer s.DevicesMu.RUnlock()
	for _, d := range s.Devices {
		if d.IP == ip {
			return d.Name
		}
	}
	return ""
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
			// Device disappeared, will be handled by heartbeat timeout
		}
	}
}
