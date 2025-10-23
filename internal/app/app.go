package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Krasnovvvvv/share-my-clipboard/internal/clipboard"
	"github.com/Krasnovvvvv/share-my-clipboard/internal/ipc"
	"github.com/Krasnovvvvv/share-my-clipboard/internal/network"
	"github.com/Krasnovvvvv/share-my-clipboard/internal/ui"
)

// File transfer state for chunk assembly
type FileTransferState struct {
	FileName    string
	TotalSize   int64
	TotalChunks int
	Checksum    string
	FromIP      string
	Chunks      map[int][]byte
	mu          sync.RWMutex
}

func Run() {
	a := app.NewWithID("share-my-clipboard")
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Share My Clipboard")
	w.Resize(fyne.NewSize(440, 530))
	w.SetIcon(ui.ResourceMainiconPng)

	page := 0
	const pageSize = 3

	ds := &network.DeviceStore{}

	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}

	connMgr := network.NewConnectionManager(hostName)

	// Create downloads directory and clipboard manager
	homeDir, _ := os.UserHomeDir()
	downloadDir := filepath.Join(homeDir, "Downloads", "ShareMyClipboard")
	os.MkdirAll(downloadDir, 0755)
	clipboardMgr := clipboard.NewManager(downloadDir)

	// Start IPC server for context menu integration
	ipcServer, err := ipc.NewIPCServer()
	if err != nil {
		fmt.Printf("Warning: Failed to start IPC server: %v\n", err)
	} else {
		// Register handler for file sending from context menu
		ipcServer.RegisterHandler("send_files", func(data []byte) error {
			var req ipc.SendFilesRequest
			if err := json.Unmarshal(data, &req); err != nil {
				return fmt.Errorf("failed to unmarshal request: %w", err)
			}

			if len(connMgr.GetConnectedIPs()) == 0 {
				fyne.Do(func() {
					ui.NotifyError("There are no connected devices to send the file!")
				})
				return errors.New("no connected devices")
			}

			fmt.Printf("[IPC] Received request to send %d file(s)\n", len(req.FilePaths))

			// Process each file
			for _, filePath := range req.FilePaths {
				// Read file
				fileData, err := os.ReadFile(filePath)
				if err != nil {
					fmt.Printf("[IPC] Failed to read %s: %v\n", filePath, err)
					continue
				}

				// Calculate checksum
				checksum := clipboard.ComputeFileChecksum(fileData)
				fileName := filepath.Base(filePath)

				// Broadcast to all connected devices
				connMgr.BroadcastFileClipboard(fileName, fileData, checksum)

				fmt.Printf("[IPC] Sent %s (%d bytes) to connected devices\n",
					fileName, len(fileData))

				// Show notification
				fyne.Do(func() {
					ui.NotifyInfo(fmt.Sprintf("Sending %s to connected devices...", fileName))
				})
			}

			return nil
		})
	}

	// UI elements
	cardsBox := container.NewVBox()
	pageLabel := widget.NewLabel("")
	updateTrigger := make(chan struct{}, 1)

	triggerUpdate := func() {
		select {
		case updateTrigger <- struct{}{}:
		default:
		}
	}

	var updatePage func()
	updatePage = func() {
		devs := ds.GetPage(page, pageSize)
		cardsBox.Objects = nil

		for _, d := range devs {
			isConn := connMgr.IsConnected(d.IP)
			devCopy := d

			card := container.NewCenter(ui.MakeDeviceCard(
				d.Name, d.IP, isConn,
				func(ip string) {
					req := network.ConnectionRequest{
						FromName: hostName,
						FromIP:   connMgr.LocalIP,
						FromMAC:  "",
						ToIP:     ip,
					}
					if err := connMgr.SendRequest(req); err != nil {
						fyne.Do(func() {
							ui.NotifyError(fmt.Sprintf("Failed to send request: %v", err))
						})
						return
					}
					fyne.Do(func() {
						ui.NotifyInfo(fmt.Sprintf("Connection request sent to %s", devCopy.Name))
					})
				},
				func(ip string) {
					deviceName := ds.FindNameByIP(ip)
					if deviceName == "" {
						deviceName = ip
					}
					if err := connMgr.Disconnect(ip); err != nil {
						fyne.Do(func() {
							ui.NotifyError(fmt.Sprintf("Failed to disconnect: %v", err))
						})
						return
					}
					fyne.Do(func() {
						ui.NotifyInfo(fmt.Sprintf("Disconnected from %s", deviceName))
					})
					triggerUpdate()
				},
			))
			cardsBox.Add(card)
		}

		total := len(ds.Devices)
		totalPages := (total + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		pageLabel.SetText(fmt.Sprintf("Page %d of %d", page+1, totalPages))
		cardsBox.Refresh()
	}

	// Connection request handler
	connMgr.OnRequest = func(req network.ConnectionRequest) {
		fyne.Do(func() {
			ui.ConfirmConnection(w, req.FromName, func(approved bool) {
				resp := network.ConnectionResponse{
					FromIP: connMgr.LocalIP,
					ToIP:   req.FromIP,
					Accept: approved,
				}
				if err := connMgr.SendResponse(resp); err != nil {
					ui.NotifyError(fmt.Sprintf("Failed to send response: %v", err))
					return
				}
				if approved {
					ui.NotifySuccess("Connection accepted", fmt.Sprintf("Waiting for %s to connect", req.FromName))
				} else {
					ui.NotifyInfo(fmt.Sprintf("Connection request from %s declined", req.FromName))
				}
				triggerUpdate()
			})
		})
	}

	// Connection response handler
	connMgr.OnResult = func(resp network.ConnectionResponse) {
		deviceName := ds.FindNameByIP(resp.FromIP)
		if deviceName == "" {
			deviceName = resp.FromIP
		}
		fyne.Do(func() {
			if resp.Accept {
				if err := connMgr.Connect(resp.FromIP, ""); err != nil {
					ui.NotifyError(fmt.Sprintf("Failed to connect: %v", err))
					triggerUpdate()
					return
				}
				ui.NotifySuccess("Connected", fmt.Sprintf("Connected with %s", deviceName))
			} else {
				ui.NotifyInfo(fmt.Sprintf("%s declined connection", deviceName))
			}
			triggerUpdate()
		})
	}

	connMgr.SetOnConnEstablished(func(ip string) {
		fyne.Do(func() {
			fmt.Printf("[APP] Connection established with %s\n", ip)
			triggerUpdate()
		})
	})

	connMgr.OnDisconnect = func(ip string, reason string) {
		deviceName := ds.FindNameByIP(ip)
		if deviceName == "" {
			deviceName = ip
		}
		fyne.Do(func() {
			if reason == "Hub shutdown" {
				ui.NotifyInfo("Hub disconnected - all connections closed")
			} else {
				ui.NotifyInfo(fmt.Sprintf("Disconnected from %s: %s", deviceName, reason))
			}
			triggerUpdate()
		})
	}

	// Clipboard data handler (text)
	connMgr.OnClipboard = func(data network.ClipboardData) {
		if clipboardMgr == nil {
			return
		}
		deviceName := ds.FindNameByIP(data.FromIP)
		if deviceName == "" {
			deviceName = data.FromIP
		}
		clipContent := clipboard.ClipboardContent{
			Type: clipboard.ContentTypeText,
			Text: data.Content,
		}
		if err := clipboardMgr.SetClipboard(clipContent); err != nil {
			fmt.Printf("Failed to set clipboard: %v\n", err)
		} else {
			fyne.Do(func() {
				ui.NotifyInfo(fmt.Sprintf("Clipboard updated from %s", deviceName))
			})
		}
	}

	// Chunked file transfer state for receiver
	activeTransfers := make(map[string]*FileTransferState)
	var transfersMu sync.RWMutex

	// File chunk start handler
	connMgr.OnFileChunkStart = func(start network.FileChunkStart) {
		deviceName := ds.FindNameByIP(start.FromIP)
		if deviceName == "" {
			deviceName = start.FromIP
		}
		fmt.Printf("[APP] File transfer started: %s (%d bytes, %d chunks)\n",
			start.FileName, start.TotalSize, start.TotalChunks)
		transfersMu.Lock()
		activeTransfers[start.FileID] = &FileTransferState{
			FileName:    start.FileName,
			TotalSize:   start.TotalSize,
			TotalChunks: start.TotalChunks,
			Checksum:    start.Checksum,
			FromIP:      start.FromIP,
			Chunks:      make(map[int][]byte),
		}
		transfersMu.Unlock()
		fyne.Do(func() {
			ui.NotifyInfo(fmt.Sprintf("Receiving %s from %s...", start.FileName, deviceName))
		})
	}

	// File chunk data handler
	connMgr.OnFileChunkData = func(chunk network.FileChunkData) {
		transfersMu.RLock()
		transfer, exists := activeTransfers[chunk.FileID]
		transfersMu.RUnlock()
		if !exists {
			fmt.Printf("[APP] Received chunk for unknown file: %s\n", chunk.FileID)
			return
		}
		transfer.mu.Lock()
		transfer.Chunks[chunk.ChunkIndex] = chunk.Data
		receivedChunks := len(transfer.Chunks)
		transfer.mu.Unlock()
		if (chunk.ChunkIndex+1)%10 == 0 || receivedChunks == transfer.TotalChunks {
			fmt.Printf("[APP] Received chunk %d/%d for %s\n",
				receivedChunks, transfer.TotalChunks, transfer.FileName)
		}
	}

	// File chunk complete handler
	connMgr.OnFileChunkComplete = func(complete network.FileChunkComplete) {
		transfersMu.Lock()
		transfer, exists := activeTransfers[complete.FileID]
		if !exists {
			transfersMu.Unlock()
			fmt.Printf("[APP] Completed unknown file: %s\n", complete.FileID)
			return
		}
		delete(activeTransfers, complete.FileID)
		transfersMu.Unlock()
		transfer.mu.RLock()
		if len(transfer.Chunks) != transfer.TotalChunks {
			transfer.mu.RUnlock()
			fmt.Printf("[APP] Missing chunks: got %d, expected %d\n",
				len(transfer.Chunks), transfer.TotalChunks)
			fyne.Do(func() {
				ui.NotifyError(fmt.Sprintf("File transfer incomplete: %s", transfer.FileName))
			})
			return
		}
		transfer.mu.RUnlock()
		transfer.mu.RLock()
		fileData := make([]byte, 0, transfer.TotalSize)
		for i := 0; i < transfer.TotalChunks; i++ {
			chunkData, ok := transfer.Chunks[i]
			if !ok {
				transfer.mu.RUnlock()
				fmt.Printf("[APP] Missing chunk %d\n", i)
				fyne.Do(func() {
					ui.NotifyError(fmt.Sprintf("File transfer incomplete: %s", transfer.FileName))
				})
				return
			}
			fileData = append(fileData, chunkData...)
		}
		transfer.mu.RUnlock()
		actualChecksum := clipboard.ComputeFileChecksum(fileData)
		if actualChecksum != transfer.Checksum {
			fmt.Printf("[APP] Checksum mismatch for %s\n", transfer.FileName)
			fyne.Do(func() {
				ui.NotifyError(fmt.Sprintf("File corrupted: %s", transfer.FileName))
			})
			return
		}
		contentType := clipboard.ContentTypeFile
		if clipboard.IsImageFile(transfer.FileName) {
			contentType = clipboard.ContentTypeImage
		}
		clipContent := clipboard.ClipboardContent{
			Type:     contentType,
			FileName: transfer.FileName,
			FileData: fileData,
		}
		if clipboardMgr != nil {
			err := clipboardMgr.SetClipboard(clipContent)
			if err != nil {
				fmt.Printf("Failed to set clipboard: %v\n", err)
			} else {
				fyne.Do(func() {
					ui.NotifySuccess("File Received",
						fmt.Sprintf("%s from %s (%d KB)",
							transfer.FileName, transfer.FromIP, len(fileData)/1024))
				})
				fmt.Printf("[APP] File received successfully: %s (%d bytes)\n",
					transfer.FileName, len(fileData))
			}
		}
	}

	// Clipboard watcher: send text and files/images chunked
	if clipboardMgr != nil {
		go func() {
			for clipContent := range clipboardMgr.Watch() {
				switch clipContent.Type {
				case clipboard.ContentTypeText:
					connMgr.BroadcastClipboard(clipContent.Text)
				case clipboard.ContentTypeImage, clipboard.ContentTypeFile:
					if len(clipContent.FileData) > 0 {
						checksum := clipboard.ComputeFileChecksum(clipContent.FileData)
						connMgr.BroadcastFileClipboard(
							clipContent.FileName,
							clipContent.FileData,
							checksum,
						)
						fmt.Printf("[APP] Broadcasting file: %s (%d bytes)\n",
							clipContent.FileName, len(clipContent.FileData))
					}
				}
			}
		}()
	}

	prevBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if page > 0 {
			page--
			triggerUpdate()
		}
	})
	nextBtn := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		ds.DevicesMu.RLock()
		maxPage := (len(ds.Devices) + pageSize - 1) / pageSize
		ds.DevicesMu.RUnlock()
		if page < maxPage-1 {
			page++
			triggerUpdate()
		}
	})
	scanTrigger := make(chan struct{}, 1)
	updateBtn := widget.NewButtonWithIcon("Update", theme.ViewRefreshIcon(), func() {
		select {
		case scanTrigger <- struct{}{}:
		default:
		}
	})
	updateBtn.Importance = widget.HighImportance

	title := widget.NewLabelWithStyle("Devices on the Network", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	pagination := container.NewHBox(prevBtn, layout.NewSpacer(), pageLabel, layout.NewSpacer(), nextBtn)
	paginationCentered := container.NewCenter(pagination)

	deviceListContainer := container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(cardsBox),
		ui.NewMargin(5),
		container.NewCenter(updateBtn),
		ui.NewMargin(5),
		paginationCentered,
	)
	deviceListContainer.Resize(fyne.NewSize(300, 450))

	content := container.NewVBox(
		container.NewCenter(widget.NewLabelWithStyle("Share My Clipboard", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})),
		widget.NewSeparator(),
		container.NewCenter(deviceListContainer),
	)
	w.SetContent(content)

	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-scanTrigger:
				if ds.Scan(hostName) {
					fyne.Do(func() {
						a.SendNotification(&fyne.Notification{
							Title:   "Network Scan",
							Content: "Device list updated!",
						})
					})
					triggerUpdate()
				}
			case <-ticker.C:
				if ds.Scan(hostName) {
					triggerUpdate()
				}
				connMgr.CheckDisconnects(ds, triggerUpdate)
			case <-updateTrigger:
				fyne.Do(updatePage)
			}
		}
	}()

	w.SetOnClosed(func() {
		if clipboardMgr != nil {
			clipboardMgr.Stop()
		}
		if len(connMgr.GetConnectedIPs()) > 0 {
			connMgr.ShutdownAsHub()
		}
		if ipcServer != nil {
			ipcServer.Stop()
		}
	})

	updatePage()
	w.ShowAndRun()
}
