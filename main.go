package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.NewWithID("com.krasnov.clipboard")
	// Можно настроить тему
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Share My Clipboard")
	w.Resize(fyne.NewSize(400, 250))

	title := widget.NewLabelWithStyle("Share My Clipboard", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Кроссплатформенный обмен буфером обмена")
	subtitle.Alignment = fyne.TextAlignCenter

	status := widget.NewLabel("Добро пожаловать!\nНачни работу с приложением — дальнейшие функции будут добавлены.")

	// Базовая вертикальная компоновка
	content := container.NewVBox(
		title,
		subtitle,
		widget.NewSeparator(),
		status,
	)

	r, _ := fyne.LoadResourceFromPath("Icons/main_icon.jpg")
	w.SetIcon(r)

	w.SetContent(content)
	w.ShowAndRun()
}
