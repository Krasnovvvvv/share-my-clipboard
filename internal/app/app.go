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

// getLocalIP автоматически определяет локальный IP-адрес устройства.
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

// Run запускает приложение Share My Clipboard.
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

	// предварительное объявление для видимости в замыканиях
	var updatePage func()

	// функция обновления интерфейса
	updatePage = func() {
		devs := ds.GetPage(page, pageSize)
		cardsBox.Objects = nil

		for _, d := range devs {
			isConn := connMgr.IsConnected(d.IP)
			devCopy := d
			card := container.NewCenter(ui.MakeDeviceCard(
				d.Name,
				d.IP,
				isConn,
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
		totalPages := total / pageSize
		if total%pageSize != 0 {
			totalPages++
		}
		if totalPages == 0 {
			totalPages = 1
		}
		pageLabel.SetText(fmt.Sprintf("Page %d of %d", page+1, totalPages))
		cardsBox.Refresh()
	}

	// обработчик входящих запросов
	connMgr.OnRequest = func(req network.ConnectionRequest) {
		fyne.Do(func() {
			ui.ConfirmConnection(w, req.FromName, func(approved bool) {
				resp := network.ConnectionResponse{FromIP: localIP, ToIP: req.FromIP, Accept: approved}
				connMgr.SendResponse(resp)
				if approved {
					ui.NotifySuccess("Connection accepted", fmt.Sprintf("Connected to %s", req.FromName))
				} else {
					ui.NotifyInfo(fmt.Sprintf("Connection request from %s declined", req.FromName))
				}
			})
		})
	}

	// обработчик ответа на запрос
	connMgr.OnResult = func(resp network.ConnectionResponse) {
		if resp.Accept {
			connMgr.Connect(resp.FromIP)
			ui.NotifySuccess("Connected", fmt.Sprintf("You are connected with %s", resp.FromIP))
		} else {
			ui.NotifyInfo(fmt.Sprintf("Device %s declined the connection request", resp.FromIP))
		}
		fyne.Do(updatePage)
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
		if len(ds.Devices)%pageSize != 0 {
			maxPage++
		}
		ds.DevicesMu.RUnlock()
		if page < maxPage {
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
		container.NewCenter(widget.NewLabelWithStyle(
			"Share My Clipboard",
			fyne.TextAlignCenter,
			fyne.TextStyle{Bold: true},
		)),
		widget.NewSeparator(),
		container.NewCenter(deviceListContainer),
	)
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
