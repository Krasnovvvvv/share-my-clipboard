package main

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Device struct {
	Name string
	IP   string
}

func main() {
	a := app.NewWithID("com.krasnov.clipboard")
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Share My Clipboard")
	w.Resize(fyne.NewSize(440, 480))

	devices := []Device{
		{"ПК", "192.168.0.2"},
		{"Lenovo", "192.168.0.29"},
		{"Redmi", "192.168.0.17"},
		{"Стационарник", "192.168.0.4"},
		{"Рабочий ПК", "192.168.0.100"},
		{"MacBook", "192.168.0.31"},
		{"Tablet", "192.168.0.9"},
	}
	page := 0
	const pageSize = 4

	getPage := func() []Device {
		start := page * pageSize
		end := start + pageSize
		if end > len(devices) {
			end = len(devices)
		}
		return devices[start:end]
	}

	// Одиночная карточка
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
			frame.Resize(fyne.NewSize(260, 80)) // фикс. ширина и высота
			cardsBox.Add(container.NewCenter(frame))
		}
		pageLabel.SetText("Стр. " + strconv.Itoa(page+1) + " / " + strconv.Itoa((len(devices)+pageSize-1)/pageSize))
		cardsBox.Refresh()
	}

	prevBtn := widget.NewButtonWithIcon("Назад", theme.NavigateBackIcon(), func() {
		if page > 0 {
			page--
			update()
		}
	})
	nextBtn := widget.NewButtonWithIcon("Вперед", theme.NavigateNextIcon(), func() {
		if (page+1)*pageSize < len(devices) {
			page++
			update()
		}
	})

	update()

	deviceListCard := widget.NewCard("Устройства в сети", "",
		container.NewVBox(
			layout.NewSpacer(), // небольшой вертикальный отступ сверху
			container.NewCenter(cardsBox),
			layout.NewSpacer(), // отступ снизу,
		),
	)
	deviceListCard.Resize(fyne.NewSize(300, 450)) // фиксированная ширина списка

	content := container.NewVBox(
		container.NewCenter(widget.NewLabelWithStyle("Share My Clipboard", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})),
		container.NewCenter(widget.NewLabel("Кроссплатформенный обмен буфером обмена")),
		widget.NewSeparator(),
		container.NewCenter(deviceListCard),
		container.NewCenter(
			container.NewHBox(
				prevBtn, layout.NewSpacer(), pageLabel, layout.NewSpacer(), nextBtn,
			),
		),
	)

	r, _ := fyne.LoadResourceFromPath("Icons/main_icon.jpg")
	w.SetIcon(r)
	w.SetContent(content)
	w.ShowAndRun()
}
