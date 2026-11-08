package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Tray is the menu bar icon and its panel.
type Tray struct {
	ctrl     *Controller
	tray     *mygo.Tray
	panel    *mygo.Window
	icon     []byte
	active   bool
	stopping bool
	menuSet  bool
}

// NewTray creates the menu bar icon. Call it once the app is ready.
func NewTray(ctrl *Controller) (*Tray, error) {
	t := &Tray{ctrl: ctrl}
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "opentj",
		Icon:           trayIcon(ctrl.IsConnected(), ctrl.Active()),
		IconIsTemplate: true,
		ToolTip:        "Open TJ",
	})
	if err != nil {
		return nil, err
	}
	t.tray = tray

	if runtime.GOOS == "linux" {
		// AppIndicator shows a menu and reports no clicks.
		t.tray.SetMenu(t.menu())
	} else {
		t.tray.OnClick(t.Toggle)
		t.tray.OnRightClick(func() {
			if t.panel != nil {
				t.panel.Close()
			}
			t.tray.SetMenu(t.menu())
			defer t.tray.SetMenu(nil)
			t.tray.PopUpMenu()
		})
	}
	t.Refresh()
	return t, nil
}

// Toggle opens or closes the panel next to the icon.
func (t *Tray) Toggle() {
	if t.panel != nil {
		t.panel.Close()
		return
	}
	t.panel = mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:      t.tray,
		Placement: mygo.PlacementBottom,
		Gap:       4,
		Width:     300,
		Height:    330,
		Focusable: true,
		Shadow:    true,
		Popover:   true,
		Content:   ui.View(t.ctrl.ViewMenu),
	})
	t.panel.OnClosed(func() { t.panel = nil })
}

// Refresh updates the icon, title and menu after a state change.
func (t *Tray) Refresh() {
	if t == nil || t.tray == nil {
		return
	}
	connected, active, stopping := t.ctrl.IsConnected(), t.ctrl.Active(), t.ctrl.Stopping()
	icon := trayIcon(connected, active)
	if !bytes.Equal(icon, t.icon) {
		t.icon = icon
		if err := t.tray.SetIcon(icon, true); err != nil {
			log.Println("tray icon:", err)
		}
	}
	if !t.menuSet || t.active != active || t.stopping != stopping {
		t.menuSet, t.active, t.stopping = true, active, stopping
		t.tray.SetMenu(t.menu())
	}
	title := ""
	if active {
		value, unit := RateParts(t.ctrl.Snapshot().DownloadBPS)
		title = padSpeed(value) + " " + unit
	}
	t.tray.SetTitle(title)
}

// Destroy removes the icon.
func (t *Tray) Destroy() {
	if t != nil && t.tray != nil {
		t.tray.Destroy()
	}
}

func (t *Tray) menu() *mygo.Menu {
	items := []*mygo.MenuItem{
		{Label: "打开 Open TJ", Click: func(*mygo.MenuItem, *mygo.Window) { t.ctrl.OpenMain() }},
	}
	if t.ctrl.Active() {
		items = append(items, &mygo.MenuItem{Label: "断开 VPN", Click: func(*mygo.MenuItem, *mygo.Window) { t.ctrl.Disconnect() }})
	} else {
		items = append(items, &mygo.MenuItem{Label: "连接 VPN", Click: func(*mygo.MenuItem, *mygo.Window) { t.ctrl.Connect() }})
	}
	items = append(items, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleQuit})
	return mygo.NewMenu(items)
}

// padSpeed keeps the tray title's width steady with figure spaces.
func padSpeed(value string) string {
	for len(value) < 5 {
		value = "\u2007" + value
	}
	return value
}

// trayIcon draws a 32×32 template icon: a filled dot over a shield outline
// while connected, a ring while connecting, a dot otherwise.
func trayIcon(connected, active bool) []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	ink := color.NRGBA{A: 255}
	cx, cy := 16.0, 16.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := dx*dx + dy*dy
			switch {
			case connected && d <= 11*11:
				img.SetNRGBA(x, y, ink)
			case !connected && active && d >= 8*8 && d <= 11*11:
				img.SetNRGBA(x, y, ink)
			case !connected && !active && d <= 11*11 && (d <= 6*6 || d >= 8*8):
				img.SetNRGBA(x, y, ink)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
