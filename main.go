package main

import (
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/schollz/peerdiscovery"
)

type Device struct {
	Name string
	IP   string
}

// thread safe devices
var (
	devices   = []Device{}
	devicesMu sync.RWMutex
)

func main() {
	a := app.NewWithID("com.krasnov.clipboard")
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Share My Clipboard")
	w.Resize(fyne.NewSize(440, 530))

	page := 0
	const pageSize = 4

	getPage := func() []Device {
		devicesMu.RLock()
		defer devicesMu.RUnlock()
		start := page * pageSize
		end := start + pageSize
		if end > len(devices) {
			end = len(devices)
		}
		return devices[start:end]
	}

	makeDeviceCard := func(d Device) fyne.CanvasObject {
		return container.NewVBox(
			container.NewCenter(widget.NewIcon(theme.ComputerIcon())),
			container.NewCenter(widget.NewLabelWithStyle(d.Name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})),
			container.NewCenter(widget.NewLabel("IP: "+d.IP)),
		)
	}

	cardsBox := container.NewVBox()
	pageLabel := widget.NewLabel("")

	update := func() {
		cardsBox.Objects = nil
		for _, d := range getPage() {
			frame := widget.NewCard(
				"", "", makeDeviceCard(d))
			frame.Resize(fyne.NewSize(260, 80))
			cardsBox.Add(container.NewCenter(frame))
		}
		totalPages := (len(devices) + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		pageLabel.SetText("Стр. " + strconv.Itoa(page+1) + " / " + strconv.Itoa(totalPages))
		cardsBox.Refresh()
	}

	prevBtn := widget.NewButtonWithIcon("Назад", theme.NavigateBackIcon(), func() {
		if page > 0 {
			page--
			update()
		}
	})
	nextBtn := widget.NewButtonWithIcon("Вперед", theme.NavigateNextIcon(), func() {
		devicesMu.RLock()
		maxPage := (len(devices) - 1) / pageSize
		devicesMu.RUnlock()
		if (page + 1) <= maxPage {
			page++
			update()
		}
	})

	// discovery background worker
	go func() {
		for {
			discoveries, _ := peerdiscovery.Discover(peerdiscovery.Settings{
				Limit:     -1,
				Payload:   []byte("clipboard"),
				Port:      "8877",
				TimeLimit: 2 * time.Second,
			})

			found := []Device{}
			for _, d := range discoveries {
				found = append(found, Device{
					Name: "Clipboard Device", // Можно потом добавить передачу имени
					IP:   d.Address,
				})
			}
			devicesMu.Lock()
			devices = found
			devicesMu.Unlock()
			// обновляем UI
			fyne.CurrentApp().SendNotification(&fyne.Notification{
				Title:   "Network scan",
				Content: "Обновлен список устройств!",
			})
			time.Sleep(4 * time.Second)
		}
	}()

	deviceListCard := widget.NewCard("Устройства в сети", "",
		container.NewVBox(
			layout.NewSpacer(),
			container.NewCenter(cardsBox),
			layout.NewSpacer(),
		),
	)
	deviceListCard.Resize(fyne.NewSize(300, 450))

	content := container.NewVBox(
		container.NewCenter(widget.NewLabelWithStyle("Share My Clipboard", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})),
		widget.NewSeparator(),
		container.NewCenter(deviceListCard),
		container.NewCenter(
			container.NewHBox(
				prevBtn, layout.NewSpacer(), pageLabel, layout.NewSpacer(), nextBtn,
			),
		),
	)

	// переодически обновлять UI
	go func() {
		for {
			fyne.Do(func() {
				update()
			})
			time.Sleep(2 * time.Second)
		}
	}()

	r, _ := fyne.LoadResourceFromPath("Icons/main_icon.jpg")
	w.SetIcon(r)
	w.SetContent(content)
	w.ShowAndRun()
}
