package app

import (
	"os"
	"strconv"
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

func Run() {
	a := app.NewWithID("com.krasnov.clipboard")
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Share My Clipboard")
	w.Resize(fyne.NewSize(440, 530))

	page := 0
	const pageSize = 4
	ds := &network.DeviceStore{}
	cardsBox := container.NewVBox()

	pageLabel := widget.NewLabel("")
	updatePage := func() {
		devs := ds.GetPage(page, pageSize)
		cardsBox.Objects = nil
		for _, d := range devs {
			cardsBox.Add(container.NewCenter(ui.MakeDeviceCard(d.Name, d.IP)))
		}
		totalPages := (len(ds.Devices) + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		pageLabel.SetText("Page " + strconv.Itoa(page+1) + " / " + strconv.Itoa(totalPages))
		cardsBox.Refresh()
	}

	prevBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if page > 0 {
			page--
			updatePage()
		}
	})
	nextBtn := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		ds.DevicesMu.RLock()
		maxPage := (len(ds.Devices) - 1) / pageSize
		ds.DevicesMu.RUnlock()
		if (page + 1) <= maxPage {
			page++
			updatePage()
		}
	})

	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}
	scanTrigger := make(chan struct{}, 1)

	marginBefore := ui.NewMargin(5)
	marginAfter := ui.NewMargin(5)
	updateBtn := widget.NewButtonWithIcon("Update", theme.ViewRefreshIcon(), func() {
		select {
		case scanTrigger <- struct{}{}:
		default:
		}
	})
	updateBtn.Importance = widget.HighImportance

	title := widget.NewLabelWithStyle("Devices on the Network", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	pagination := container.NewHBox(
		prevBtn,
		layout.NewSpacer(),
		pageLabel,
		layout.NewSpacer(),
		nextBtn,
	)
	paginationCentered := container.NewCenter(pagination)

	deviceListContainer := container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(cardsBox),
		marginBefore,
		container.NewCenter(updateBtn),
		marginAfter,
		paginationCentered,
	)
	deviceListContainer.Resize(fyne.NewSize(300, 450))

	content := container.NewVBox(
		container.NewCenter(widget.NewLabelWithStyle("Share My Clipboard", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})),
		widget.NewSeparator(),
		container.NewCenter(deviceListContainer),
	)

	w.SetIcon(ui.ResourceMainiconPng)
	w.SetContent(content)

	go func() {
		for {
			changed := ds.Scan(hostName)
			if changed {
				fyne.CurrentApp().SendNotification(&fyne.Notification{
					Title:   "Network scan",
					Content: "Device list updated!",
				})
			}
			fyne.Do(func() { updatePage() })
			select {
			case <-scanTrigger:
			case <-time.After(4 * time.Second):
			}
		}
	}()

	w.ShowAndRun()
}
