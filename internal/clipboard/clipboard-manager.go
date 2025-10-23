package clipboard

import (
	"context"
	"fmt"
	"time"

	"golang.design/x/clipboard"
)

type Manager struct {
	watchChan  chan string
	stopChan   chan struct{}
	lastHash   string
	isWatching bool
}

func NewManager() *Manager {
	// Initialize clipboard
	err := clipboard.Init()
	if err != nil {
		fmt.Printf("Failed to initialize clipboard: %v\n", err)
		return nil
	}

	m := &Manager{
		watchChan: make(chan string, 10),
		stopChan:  make(chan struct{}),
	}

	// Start watching clipboard changes
	go m.watchClipboard()

	return m
}

func (m *Manager) watchClipboard() {
	m.isWatching = true
	defer func() {
		m.isWatching = false
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a context-aware watcher
	go func() {
		<-m.stopChan
		cancel()
	}()

	ch := clipboard.Watch(ctx, clipboard.FmtText)

	for {
		select {
		case <-m.stopChan:
			return

		case data := <-ch:
			if data == nil {
				return
			}

			content := string(data)

			// Compute simple hash to detect changes
			hash := computeHash(content)

			if hash != m.lastHash {
				m.lastHash = hash

				// Send to channel (non-blocking)
				select {
				case m.watchChan <- content:
				case <-time.After(500 * time.Millisecond):
					// Skip if channel is full
				}
			}
		}
	}
}

func (m *Manager) Watch() <-chan string {
	return m.watchChan
}

func (m *Manager) SetClipboard(content string) error {
	// Update last hash to prevent echo
	m.lastHash = computeHash(content)

	// Write to clipboard
	clipboard.Write(clipboard.FmtText, []byte(content))

	return nil
}

func (m *Manager) GetClipboard() (string, error) {
	data := clipboard.Read(clipboard.FmtText)
	if data == nil {
		return "", fmt.Errorf("failed to read clipboard")
	}

	return string(data), nil
}

func (m *Manager) Stop() {
	if m.isWatching {
		close(m.stopChan)
	}
}

// Simple hash function to detect clipboard changes
func computeHash(s string) string {
	if len(s) > 100 {
		// For long strings, use first 50 + last 50 + length
		return s[:50] + s[len(s)-50:] + fmt.Sprintf("_%d", len(s))
	}
	return s
}
