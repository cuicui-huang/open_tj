package app

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTrayIconsArePNG(t *testing.T) {
	for _, state := range []struct{ connected, active bool }{
		{false, false}, {false, true}, {true, true},
	} {
		data := trayIcon(state.connected, state.active)
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("state %+v: %v", state, err)
		}
		if img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
			t.Fatalf("state %+v: %v", state, img.Bounds())
		}
	}
}

func TestMenuView(t *testing.T) {
	c, q := newTestController(t, &memStore{}, nil)
	tt := ui.NewTester(c.ViewMenu, 300, 330)
	if !tt.HasText("打开主窗口") || !tt.HasText("连接 VPN") {
		t.Fatalf("menu texts %q", tt.Texts())
	}
	c.active = true
	c.Connecting()
	c.Connected("utun3")
	q.Drain()
	tt.Frame()
	if !tt.HasText("断开 VPN") {
		t.Fatalf("menu texts after connect %q", tt.Texts())
	}
	c.Disconnect()
	q.Drain()
	tt.Frame()
}
