package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Krasnovvvvv/share-my-clipboard/internal/app"
	"github.com/Krasnovvvvv/share-my-clipboard/internal/contextmenu"
	"github.com/Krasnovvvvv/share-my-clipboard/internal/ipc"
)

func main() {
	// Define flags
	registerMenu := flag.Bool("register-menu", false, "Register context menu in Windows Explorer")
	unregisterMenu := flag.Bool("unregister-menu", false, "Unregister context menu from Windows Explorer")
	sendFiles := flag.String("send", "", "Send file to connected devices (used by context menu)")

	flag.Parse()

	// Handle context menu registration
	if *registerMenu {
		if err := contextmenu.Register(); err != nil {
			fmt.Printf("Failed to register context menu: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Context menu registered successfully!")
		fmt.Println("Right-click on any file → 'Send to Connected Devices'")
		os.Exit(0)
	}

	if *unregisterMenu {
		if err := contextmenu.Unregister(); err != nil {
			fmt.Printf("Failed to unregister context menu: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Context menu unregistered successfully!")
		os.Exit(0)
	}

	// Handle file sending from context menu
	if *sendFiles != "" {
		// Collect all file paths from arguments
		filePaths := []string{*sendFiles}
		filePaths = append(filePaths, flag.Args()...)

		if err := sendFilesToRunningApp(filePaths); err != nil {
			fmt.Printf("Failed to send files: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully sent %d file(s) to connected devices\n", len(filePaths))
		os.Exit(0)
	}

	// Check if another instance is already running
	if ipc.IsRunning() {
		fmt.Println("Application is already running.")
		os.Exit(1)
	}

	// Auto-register context menu on first run
	if !contextmenu.IsRegistered() {
		fmt.Println("First run detected - registering context menu...")
		if err := contextmenu.Register(); err != nil {
			fmt.Printf("Warning: Failed to register context menu: %v\n", err)
		} else {
			fmt.Println("✓ Context menu registered!")
		}
	}

	// Start normal GUI application
	app.Run()
}

// sendFilesToRunningApp sends files to already running application instance
func sendFilesToRunningApp(filePaths []string) error {
	client := ipc.NewIPCClient()

	// Filter out invalid paths and prepare files
	validPaths := make([]string, 0, len(filePaths))
	for _, path := range filePaths {
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			validPaths = append(validPaths, path)
			fmt.Printf("Queued: %s\n", path)
		}
	}

	if len(validPaths) == 0 {
		return fmt.Errorf("no valid files to send")
	}

	return client.SendFiles(validPaths)
}
