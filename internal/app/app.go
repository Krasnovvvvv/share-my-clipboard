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

	"github.com/Krasnovvvvv/share-my-clipboard/internal/network"
	"github.com/Krasnovvvvv/share-my-clipboard/internal/ui"
)

// Run запускает приложение "Share My Clipboard".
func Run() {
	a := app.NewWithID("com.krasnov.clipboard")
	a.Settings().SetTheme(theme.DarkTheme())

	w := a.NewWindow("Share My Clipboard")
	w.Resize(fyne.NewSize(440, 530))

	page := 0
	const pageSize = 4

	ds := network.DeviceStore{}
	connMgr := network.NewConnectionManager()
	cardsBox := container.NewVBox()
	pageLabel := widget.NewLabel("")

	var updatePage func()

	updatePage = func() {
		devs := ds.GetPage(page, pageSize)
		cardsBox.Objects = nil

		for _, d := range devs {
			isConn := connMgr.IsConnected(d.IP)
			card := container.NewCenter(ui.MakeDeviceCard(
				d.Name,
				d.IP,
				isConn,
				func(ip string) {
					connMgr.Connect(ip)
					ui.NotifySuccess("Connected", fmt.Sprintf("Connection with %s established", ip))
					fyne.Do(updatePage) // безопасный вызов
				},
				func(ip string) {
					connMgr.Disconnect(ip)
					ui.NotifyInfo(fmt.Sprintf("Disconnected from %s", ip))
					fyne.Do(updatePage)
				},
			))
			cardsBox.Add(card)
		}

		totalPages := len(ds.Devices)/pageSize - 1
		if len(ds.Devices)%pageSize != 0 {
			totalPages++
		}
		if totalPages < 0 {
			totalPages = 0
		}

		pageLabel.SetText(fmt.Sprintf("Page %d of %d", page+1, totalPages+1))
		cardsBox.Refresh()
	}

	prevBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if page > 0 {
			page--
			fyne.Do(updatePage)
		}
	})

	nextBtn := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		ds.DevicesMu.RLock()
		maxPage := len(ds.Devices)/pageSize - 1
		ds.DevicesMu.RUnlock()
		if page < maxPage {
			page++
			fyne.Do(updatePage)
		}
	})

	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}

	scanTrigger := make(chan struct{}, 1)
	updateBtn := widget.NewButtonWithIcon("Update", theme.ViewRefreshIcon(), func() {
		select {
		case scanTrigger <- struct{}{}:
		default:
		}
	})
	updateBtn.Importance = widget.HighImportance

	title := widget.NewLabelWithStyle(
		"Devices on the Network",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

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
		container.NewCenter(widget.NewLabelWithStyle(
			"Share My Clipboard",
			fyne.TextAlignCenter,
			fyne.TextStyle{Bold: true},
		)),
		widget.NewSeparator(),
		container.NewCenter(deviceListContainer),
	)

	w.SetIcon(ui.ResourceMainiconPng)
	w.SetContent(content)

	go func() {
		for {
			select {
			case <-scanTrigger:
				if changed := ds.Scan(hostName); changed {
					fyne.Do(func() {
						fyne.CurrentApp().SendNotification(&fyne.Notification{
							Title:   "Network scan",
							Content: "Device list updated!",
						})
						updatePage()
					})
				}
			case <-time.After(4 * time.Second):
				if changed := ds.Scan(hostName); changed {
					fyne.Do(updatePage)
				}
			}
		}
	}()

	fyne.Do(updatePage)
	w.ShowAndRun()
}
