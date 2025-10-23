package ipc

/*
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdlib.h>
*/
import "C"

import (
	"path/filepath"
)

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

const (
	ipcPort    = 54323
	ipcTimeout = 5 * time.Second
)

type IPCServer struct {
	listener net.Listener
	handlers map[string]func(data []byte) error
	mu       sync.RWMutex
	running  bool
}

type IPCMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type SendFilesRequest struct {
	FilePaths []string `json:"file_paths"`
}

// NewIPCServer creates IPC server for inter-process communication
func NewIPCServer() (*IPCServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", ipcPort))
	if err != nil {
		return nil, fmt.Errorf("failed to start IPC server: %w", err)
	}

	server := &IPCServer{
		listener: listener,
		handlers: make(map[string]func(data []byte) error),
		running:  true,
	}

	fmt.Printf("[IPC] Server started on port %d\n", ipcPort)
	go server.acceptConnections()

	return server, nil
}

// RegisterHandler registers handler for specific message type
func (s *IPCServer) RegisterHandler(msgType string, handler func(data []byte) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[msgType] = handler
}

func (s *IPCServer) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				fmt.Printf("[IPC] Accept error: %v\n", err)
			}
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *IPCServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(ipcTimeout))

	decoder := json.NewDecoder(conn)
	var msg IPCMessage

	if err := decoder.Decode(&msg); err != nil {
		fmt.Printf("[IPC] Failed to decode message: %v\n", err)
		return
	}

	s.mu.RLock()
	handler, exists := s.handlers[msg.Type]
	s.mu.RUnlock()

	if !exists {
		fmt.Printf("[IPC] Unknown message type: %s\n", msg.Type)
		sendResponse(conn, false, "unknown message type")
		return
	}

	if err := handler(msg.Data); err != nil {
		fmt.Printf("[IPC] Handler error: %v\n", err)
		sendResponse(conn, false, err.Error())
		return
	}

	sendResponse(conn, true, "success")
}

func sendResponse(conn net.Conn, success bool, message string) {
	response := map[string]interface{}{
		"success": success,
		"message": message,
	}
	json.NewEncoder(conn).Encode(response)
}

// Stop stops IPC server
func (s *IPCServer) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	fmt.Println("[IPC] Server stopped")
}

// IPCClient sends messages to running application
type IPCClient struct{}

// NewIPCClient creates new IPC client
func NewIPCClient() *IPCClient {
	return &IPCClient{}
}

// SendFiles sends file paths to running GUI application
func (c *IPCClient) SendFiles(filePaths []string) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ipcPort), 3*time.Second)
	if err != nil {
		showUserMessage("Share My Clipboard is not running.\nLaunch the application to send the files.")
		return fmt.Errorf("application is not running")
	}
	defer conn.Close()

	request := SendFilesRequest{FilePaths: filePaths}
	msg := IPCMessage{Type: "send_files"}
	msg.Data, _ = json.Marshal(request)

	conn.SetDeadline(time.Now().Add(ipcTimeout))

	if err := json.NewEncoder(conn).Encode(&msg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if success, ok := response["success"].(bool); !ok || !success {
		return fmt.Errorf("request failed: %v", response["message"])
	}

	return nil
}

// IsRunning checks if GUI application is already running
func IsRunning() bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ipcPort), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// CheckSingleInstance checks if another instance is running
func CheckSingleInstance() error {
	if IsRunning() {
		return fmt.Errorf("application is already running")
	}
	return nil
}

// GetLockFile returns path to lock file
func GetLockFile() string {
	tmpDir := os.TempDir()
	return fmt.Sprintf("%s/share-my-clipboard.lock", tmpDir)
}

func showUserMessage(msg string) {
	// Log to file instead of showing MessageBox
	logFile := filepath.Join(os.TempDir(), "share-my-clipboard.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
}
