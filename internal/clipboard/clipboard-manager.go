package clipboard

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.design/x/clipboard"
)

type Manager struct {
	watchChan   chan ClipboardContent
	stopChan    chan struct{}
	lastHash    string
	isWatching  bool
	downloadDir string
}

type ClipboardContent struct {
	Type     ContentType
	Text     string
	FilePath string
	FileData []byte
	FileName string
}

type ContentType int

const (
	ContentTypeText ContentType = iota
	ContentTypeImage
	ContentTypeFile
)

func NewManager(downloadDir string) *Manager {
	// Initialize clipboard
	err := clipboard.Init()
	if err != nil {
		fmt.Printf("Failed to initialize clipboard: %v\n", err)
		return nil
	}

	// Ensure download directory exists
	os.MkdirAll(downloadDir, 0755)

	m := &Manager{
		watchChan:   make(chan ClipboardContent, 10),
		stopChan:    make(chan struct{}),
		downloadDir: downloadDir,
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

	go func() {
		<-m.stopChan
		cancel()
	}()

	textCh := clipboard.Watch(ctx, clipboard.FmtText)
	imageCh := clipboard.Watch(ctx, clipboard.FmtImage)

	for {
		select {
		case <-m.stopChan:
			return

		case data := <-textCh:
			if data == nil {
				return
			}

			content := string(data)
			hash := computeHash(content)

			if hash != m.lastHash {
				m.lastHash = hash

				// ИСПРАВЛЕНО: Попытка обработать как путь к файлу
				content = strings.TrimSpace(content)

				if len(content) >= 2 && content[0] == '"' && content[len(content)-1] == '"' {
					content = content[1 : len(content)-1]
				}

				// Проверяем, является ли это путём к файлу
				if m.looksLikeFilePath(content) {
					// Пытаемся прочитать файл
					if fileInfo, err := os.Stat(content); err == nil && !fileInfo.IsDir() {
						// Это валидный файл - читаем его
						fileData, err := os.ReadFile(content)
						if err == nil && len(fileData) > 0 {
							clipContent := ClipboardContent{
								Type:     ContentTypeFile,
								FilePath: content,
								FileName: filepath.Base(content),
								FileData: fileData,
							}

							select {
							case m.watchChan <- clipContent:
								fmt.Printf("[CLIPBOARD] Detected file copy: %s (%d bytes)\n",
									clipContent.FileName, len(fileData))
							case <-time.After(500 * time.Millisecond):
							}
							continue
						}
					}
				}

				// Обычный текст
				clipContent := ClipboardContent{
					Type: ContentTypeText,
					Text: content,
				}

				select {
				case m.watchChan <- clipContent:
				case <-time.After(500 * time.Millisecond):
				}
			}

		case data := <-imageCh:
			if data == nil {
				continue
			}

			hash := computeHash(string(data))

			if hash != m.lastHash {
				m.lastHash = hash

				fileName := fmt.Sprintf("clipboard_image_%d.png", time.Now().Unix())
				filePath := filepath.Join(m.downloadDir, fileName)

				err := os.WriteFile(filePath, data, 0644)
				if err != nil {
					fmt.Printf("Failed to save clipboard image: %v\n", err)
					continue
				}

				clipContent := ClipboardContent{
					Type:     ContentTypeImage,
					FilePath: filePath,
					FileName: fileName,
					FileData: data,
				}

				select {
				case m.watchChan <- clipContent:
					fmt.Printf("[CLIPBOARD] Detected image copy: %s (%d bytes)\n",
						fileName, len(data))
				case <-time.After(500 * time.Millisecond):
				}
			}
		}
	}
}

func (m *Manager) looksLikeFilePath(text string) bool {
	text = strings.TrimSpace(text)

	// Не должно быть переносов строк (только один путь)
	if strings.Contains(text, "\n") {
		return false
	}

	// Слишком длинный текст - скорее всего не путь
	if len(text) > 500 {
		return false
	}

	// Проверяем паттерны путей
	// Windows: C:\path\file.ext или \\server\share\file.ext
	// Linux/Mac: /path/file.ext или ~/path/file.ext

	isWindowsPath := len(text) >= 3 &&
		((text[1] == ':' && (text[2] == '\\' || text[2] == '/')) ||
			strings.HasPrefix(text, "\\\\"))

	isUnixPath := strings.HasPrefix(text, "/") || strings.HasPrefix(text, "~/")

	if !isWindowsPath && !isUnixPath {
		return false
	}

	// Проверяем наличие расширения файла
	ext := filepath.Ext(text)
	if ext == "" {
		return false
	}

	return true
}

func (m *Manager) handleFilePath(path string) {
	// Проверяем существование файла
	fileInfo, err := os.Stat(path)
	if err != nil {
		// Not a valid file path, treat as text
		clipContent := ClipboardContent{
			Type: ContentTypeText,
			Text: path,
		}

		select {
		case m.watchChan <- clipContent:
		case <-time.After(500 * time.Millisecond):
		}
		return
	}

	if fileInfo.IsDir() {
		// Directory - treat as text
		clipContent := ClipboardContent{
			Type: ContentTypeText,
			Text: path,
		}

		select {
		case m.watchChan <- clipContent:
		case <-time.After(500 * time.Millisecond):
		}
		return
	}

	// Valid file - read and send
	fileData, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Failed to read file %s: %v\n", path, err)
		return
	}

	clipContent := ClipboardContent{
		Type:     ContentTypeFile,
		FilePath: path,
		FileName: filepath.Base(path),
		FileData: fileData,
	}

	select {
	case m.watchChan <- clipContent:
	case <-time.After(500 * time.Millisecond):
	}
}

func (m *Manager) isFilePath(text string) bool {
	text = strings.TrimSpace(text)

	// Check if it looks like a file path
	// Windows: C:\path\to\file or \\server\share
	// Unix: /path/to/file or ~/path/to/file
	if strings.HasPrefix(text, "/") ||
		strings.HasPrefix(text, "~") ||
		(len(text) >= 3 && text[1] == ':' && (text[2] == '\\' || text[2] == '/')) ||
		strings.HasPrefix(text, "\\\\") {

		// Must not contain newlines (single path only)
		if strings.Contains(text, "\n") {
			return false
		}

		return true
	}

	return false
}

func (m *Manager) Watch() <-chan ClipboardContent {
	return m.watchChan
}

func (m *Manager) SetClipboard(content ClipboardContent) error {
	// Update last hash to prevent echo
	switch content.Type {
	case ContentTypeText:
		m.lastHash = computeHash(content.Text)
		clipboard.Write(clipboard.FmtText, []byte(content.Text))

	case ContentTypeImage, ContentTypeFile:
		if len(content.FileData) > 0 {
			// Save file to download directory
			savePath := filepath.Join(m.downloadDir, content.FileName)

			err := os.WriteFile(savePath, content.FileData, 0644)
			if err != nil {
				return fmt.Errorf("failed to save file: %w", err)
			}

			// For images, also write to clipboard as image
			if content.Type == ContentTypeImage {
				m.lastHash = computeHash(string(content.FileData))
				clipboard.Write(clipboard.FmtImage, content.FileData)
			} else {
				// For other files, write the file path to clipboard
				m.lastHash = computeHash(savePath)
				clipboard.Write(clipboard.FmtText, []byte(savePath))
			}

			fmt.Printf("File saved to: %s\n", savePath)
		}
	}

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

// ComputeFileChecksum calculates MD5 checksum of file data
func ComputeFileChecksum(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// IsImageFile checks if file is an image based on extension
func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp"}

	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}
