package app

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Krasnovvvvv/share-my-clipboard/internal/clipboard"
	"github.com/Krasnovvvvv/share-my-clipboard/internal/network"
	"github.com/Krasnovvvvv/share-my-clipboard/internal/ui"
)

func Run() {
	a := app.NewWithID("com.krasnov.clipboard")
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Share My Clipboard")
	w.Resize(fyne.NewSize(440, 530))
	w.SetIcon(ui.ResourceMainiconPng)

	page := 0
	const pageSize = 3 // ИСПРАВЛЕНО: Показываем 3 устройства на странице

	// Initialize components
	ds := &network.DeviceStore{}

	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}

	connMgr := network.NewConnectionManager(hostName)
	clipboardMgr := clipboard.NewManager()

	// UI elements
	cardsBox := container.NewVBox()
	pageLabel := widget.NewLabel("")
	updateTrigger := make(chan struct{}, 1)

	// Helper function to trigger UI update
	triggerUpdate := func() {
		select {
		case updateTrigger <- struct{}{}:
		default:
		}
	}

	// Update page function
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
					// Connect button handler
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
					// Disconnect button handler
					if err := connMgr.Disconnect(ip); err != nil {
						fyne.Do(func() {
							ui.NotifyError(fmt.Sprintf("Failed to disconnect: %v", err))
						})
						return
					}

					fyne.Do(func() {
						ui.NotifyInfo(fmt.Sprintf("Disconnected from %s", ip))
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
					// ИСПРАВЛЕНО: НЕ вызываем Connect() здесь
					// Получатель просто ждет входящее персистентное соединение
				} else {
					ui.NotifyInfo(fmt.Sprintf("Connection request from %s declined", req.FromName))
				}

				triggerUpdate()
			})
		})
	}

	// Connection response handler
	connMgr.OnResult = func(resp network.ConnectionResponse) {
		fyne.Do(func() {
			if resp.Accept {
				// ИСПРАВЛЕНО: Только инициатор устанавливает персистентное соединение
				if err := connMgr.Connect(resp.FromIP, ""); err != nil {
					ui.NotifyError(fmt.Sprintf("Failed to connect: %v", err))
					triggerUpdate()
					return
				}
				ui.NotifySuccess("Connected", fmt.Sprintf("Connected with %s", resp.FromIP))
			} else {
				ui.NotifyInfo(fmt.Sprintf("%s declined connection", resp.FromIP))
			}

			triggerUpdate()
		})
	}

	// Connection established handler
	connMgr.SetOnConnEstablished(func(ip string) {
		fyne.Do(func() {
			fmt.Printf("[APP] Connection established with %s\n", ip)
			triggerUpdate()
		})
	})

	// Disconnect handler
	connMgr.OnDisconnect = func(ip string, reason string) {
		fyne.Do(func() {
			if reason == "Hub shutdown" {
				ui.NotifyInfo("Hub disconnected - all connections closed")
			} else {
				ui.NotifyInfo(fmt.Sprintf("Disconnected from %s: %s", ip, reason))
			}
			triggerUpdate()
		})
	}

	// Clipboard data handler
	connMgr.OnClipboard = func(data network.ClipboardData) {
		if clipboardMgr == nil {
			return
		}
		if err := clipboardMgr.SetClipboard(data.Content); err != nil {
			fmt.Printf("Failed to set clipboard: %v\n", err)
		} else {
			fyne.Do(func() {
				ui.NotifyInfo(fmt.Sprintf("Clipboard updated from %s", data.FromIP))
			})
		}
	}

	// Clipboard watcher
	if clipboardMgr != nil {
		go func() {
			for content := range clipboardMgr.Watch() {
				// Broadcast to all connected devices
				connMgr.BroadcastClipboard(content)
			}
		}()
	}

	// Navigation buttons
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

	// Scan trigger
	scanTrigger := make(chan struct{}, 1)
	updateBtn := widget.NewButtonWithIcon("Update", theme.ViewRefreshIcon(), func() {
		select {
		case scanTrigger <- struct{}{}:
		default:
		}
	})
	updateBtn.Importance = widget.HighImportance

	// Build UI
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

	// Background worker
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

	// Cleanup on window close
	w.SetOnClosed(func() {
		if clipboardMgr != nil {
			clipboardMgr.Stop()
		}
		if len(connMgr.GetConnectedIPs()) > 0 {
			connMgr.ShutdownAsHub()
		}
	})

	updatePage()
	w.ShowAndRun()
}
