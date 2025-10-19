package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// MakeDeviceCard — карточка устройства
func MakeDeviceCard(name, ip string, isConnected bool, onConnect func(ip string), onDisconnect func(ip string)) fyne.CanvasObject {
	icon := widget.NewIcon(theme.ComputerIcon())
	title := widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	address := widget.NewLabelWithStyle(ip, fyne.TextAlignCenter, fyne.TextStyle{})

	var btn *widget.Button
	if isConnected {
		btn = widget.NewButtonWithIcon("Disconnect", theme.CancelIcon(), func() {
			onDisconnect(ip)
		})
	} else {
		btn = widget.NewButtonWithIcon("Connect", theme.ConfirmIcon(), func() {
			onConnect(ip)
		})
	}
	btn.Importance = widget.HighImportance

	card := container.NewVBox(
		container.NewCenter(icon),
		container.NewCenter(title),
		container.NewCenter(address),
		container.NewCenter(btn),
		NewMargin(10),
	)
	border := canvas.NewRectangle(color.NRGBA{R: 210, G: 210, B: 210, A: 255})
	border.SetMinSize(fyne.NewSize(400, 2))
	return container.NewVBox(card, border)
}

// ConfirmConnection выводит окно согласия
func ConfirmConnection(w fyne.Window, requester string, cb func(bool)) {
	dialog.ShowConfirm(
		"Incoming connection",
		"Device '"+requester+"' wants to connect. Allow?",
		cb,
		w,
	)
}

// Нативные уведомления
func NotifySuccess(title, msg string) {
	fyne.CurrentApp().SendNotification(&fyne.Notification{Title: title, Content: msg})
}
func NotifyInfo(msg string) {
	fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "Info", Content: msg})
}
func NotifyError(msg string) {
	fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "Error", Content: msg})
}

// Отступ
func NewMargin(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(nil)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}
