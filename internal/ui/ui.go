package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func MakeDeviceCard(name, ip string) fyne.CanvasObject {
	return container.NewVBox(
		container.NewCenter(widget.NewIcon(theme.ComputerIcon())),
		container.NewCenter(widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})),
		container.NewCenter(widget.NewLabel("IP: "+ip)),
	)
}

func NewMargin(height float32) *canvas.Rectangle {
	r := canvas.NewRectangle(nil)
	r.SetMinSize(fyne.NewSize(0, height))
	return r
}
