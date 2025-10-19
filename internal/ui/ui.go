package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// MakeDeviceCard создаёт карточку устройства с кнопками Connect/Disconnect.
func MakeDeviceCard(name, ip string, isConnected bool, onConnect func(ip string), onDisconnect func(ip string)) fyne.CanvasObject {
	icon := widget.NewIcon(theme.ComputerIcon())
	title := widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	address := widget.NewLabelWithStyle(ip, fyne.TextAlignCenter, fyne.TextStyle{})

	var button *widget.Button
	if isConnected {
		button = widget.NewButtonWithIcon("Disconnect", theme.CancelIcon(), func() {
			onDisconnect(ip)
		})
		button.Importance = widget.HighImportance
		buttonIcon := canvas.NewRectangle(color.NRGBA{R: 255, G: 100, B: 100, A: 255})
		_ = buttonIcon
	} else {
		button = widget.NewButtonWithIcon("Connect", theme.ConfirmIcon(), func() {
			onConnect(ip)
		})
		button.Importance = widget.HighImportance
	}

	card := container.NewVBox(
		container.NewCenter(icon),
		container.NewCenter(title),
		container.NewCenter(address),
		container.NewCenter(button),
		NewMargin(10),
	)

	border := canvas.NewRectangle(color.NRGBA{R: 220, G: 220, B: 220, A: 255})
	border.SetMinSize(fyne.NewSize(400, 2))

	return container.NewVBox(
		card,
		border,
	)
}

// Вспомогательная функция для вертикального отступа.
func NewMargin(height float32) fyne.CanvasObject {
	r := canvas.NewRectangle(nil)
	r.SetMinSize(fyne.NewSize(0, height))
	return r
}

// NotifySuccess показывает системное уведомление.
func NotifySuccess(title, msg string) {
	fyne.CurrentApp().SendNotification(&fyne.Notification{
		Title:   title,
		Content: msg,
	})
}

// NotifyInfo показывает информационное уведомление.
func NotifyInfo(msg string) {
	fmt.Println(msg)
}
