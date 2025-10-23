package contextmenu

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	menuName    = "ShareMyClipboard"
	menuText    = "Send to Connected Devices"
	iconDefault = ""
)

// Register adds context menu entry to Windows Explorer
func Register() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Normalize path
	exePath = filepath.Clean(exePath)

	// Register for single files
	if err := registerForFiles(exePath); err != nil {
		return fmt.Errorf("failed to register for files: %w", err)
	}

	// Register for multiple files (Directory background)
	if err := registerForMultipleFiles(exePath); err != nil {
		return fmt.Errorf("failed to register for multiple files: %w", err)
	}

	fmt.Println("[ContextMenu] Successfully registered in Windows Explorer")
	return nil
}

// Unregister removes context menu entry
func Unregister() error {
	// Remove single file entry
	registry.DeleteKey(registry.CURRENT_USER,
		`Software\Classes\*\shell\`+menuName)

	// Remove multiple files entry
	registry.DeleteKey(registry.CURRENT_USER,
		`Software\Classes\Directory\Background\shell\`+menuName)

	fmt.Println("[ContextMenu] Successfully unregistered")
	return nil
}

// registerForFiles registers context menu for single/multiple selected files
func registerForFiles(exePath string) error {
	// Create main key
	key, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Classes\*\shell\`+menuName,
		registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer key.Close()

	// Set menu text
	if err := key.SetStringValue("", menuText); err != nil {
		return err
	}

	// Set icon (optional - uses app icon)
	if err := key.SetStringValue("Icon", exePath); err != nil {
		return err
	}

	// Create command subkey
	cmdKey, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Classes\*\shell\`+menuName+`\command`,
		registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer cmdKey.Close()

	cmdLine := fmt.Sprintf(`"%s" --send "%%1"`, exePath)

	if err := cmdKey.SetStringValue("", cmdLine); err != nil {
		return err
	}

	return nil
}

// registerForMultipleFiles registers for directory background (multiple files)
func registerForMultipleFiles(exePath string) error {
	// Create main key
	key, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Classes\Directory\Background\shell\`+menuName,
		registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer key.Close()

	// Set menu text
	if err := key.SetStringValue("", menuText); err != nil {
		return err
	}

	// Set icon
	if err := key.SetStringValue("Icon", exePath); err != nil {
		return err
	}

	// Create command subkey
	cmdKey, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Classes\Directory\Background\shell\`+menuName+`\command`,
		registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer cmdKey.Close()

	// Set command - %V gives current directory path
	cmdLine := fmt.Sprintf(`"%s" --send-from-dir "%%V"`, exePath)

	if err := cmdKey.SetStringValue("", cmdLine); err != nil {
		return err
	}

	return nil
}

// IsRegistered checks if context menu is already registered
func IsRegistered() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Classes\*\shell\`+menuName,
		registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	key.Close()
	return true
}
