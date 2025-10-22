package app

import (
	"fmt"
	"net"
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

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}

func Run() {
	a := app.NewWithID("com.krasnov.clipboard")
	a.Settings().SetTheme(theme.DarkTheme())

	w := a.NewWindow("Share My Clipboard")
	w.Resize(fyne.NewSize(440, 530))
	w.SetIcon(ui.ResourceMainiconPng)

	page := 0
	const pageSize = 4

	ds := network.DeviceStore{}
	connMgr := network.NewConnectionManager()
	cardsBox := container.NewVBox()
	pageLabel := widget.NewLabel("")

	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}
	localIP := getLocalIP()

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
						FromIP:   localIP,
						ToIP:     ip,
					}
					connMgr.SendRequest(req)
					ui.NotifyInfo(fmt.Sprintf("Connection request sent to %s", devCopy.Name))
				},
				func(ip string) {
					connMgr.Disconnect(ip)
					ui.NotifyInfo(fmt.Sprintf("Disconnected from %s", ip))
					fyne.Do(updatePage)
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

	connMgr.OnRequest = func(req network.ConnectionRequest) {
		fyne.Do(func() {
			updatePage()
			ui.ConfirmConnection(w, req.FromName, func(approved bool) {
				resp := network.ConnectionResponse{FromIP: localIP, ToIP: req.FromIP, Accept: approved}
				connMgr.SendResponse(resp)
				if approved {
					ui.NotifySuccess("Connection accepted", fmt.Sprintf("Connected to %s", req.FromName))
					connMgr.LegacyConnect(req.FromIP)
				} else {
					ui.NotifyInfo(fmt.Sprintf("Connection request from %s declined", req.FromName))
				}
				fyne.Do(updatePage)
			})
		})
	}

	connMgr.OnResult = func(resp network.ConnectionResponse) {
		fyne.Do(func() {
			updatePage()
			if resp.Accept {
				connMgr.LegacyConnect(resp.FromIP)
				ui.NotifySuccess("Connected", fmt.Sprintf("Connected with %s", resp.FromIP))
			} else {
				ui.NotifyInfo(fmt.Sprintf("%s declined connection", resp.FromIP))
			}
			updatePage()
		})
	}

	prevBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if page > 0 {
			page--
			fyne.Do(updatePage)
		}
	})
	nextBtn := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		ds.DevicesMu.RLock()
		maxPage := (len(ds.Devices) + pageSize - 1) / pageSize
		ds.DevicesMu.RUnlock()
		if page < maxPage-1 {
			page++
			fyne.Do(updatePage)
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
		for {
			select {
			case <-scanTrigger:
				if ds.Scan(hostName) {
					fyne.Do(func() {
						fyne.CurrentApp().SendNotification(&fyne.Notification{
							Title:   "Network Scan",
							Content: "Device list updated!",
						})
						updatePage()
					})
				}
			case <-time.After(4 * time.Second):
				if ds.Scan(hostName) {
					fyne.Do(updatePage)
				}
				connMgr.CheckDisconnects(&ds, func() { fyne.Do(updatePage) })
			}
		}
	}()

	fyne.Do(updatePage)
	w.ShowAndRun()
}
