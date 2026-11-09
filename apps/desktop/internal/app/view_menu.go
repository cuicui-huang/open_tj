package app

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// ViewMenu draws the menu bar panel: the connection, the two rates and the
// actions, on the same glass as the main window.
func (c *Controller) ViewMenu(ctx *ui.Context) {
	p := applyTheme(ctx)
	if c.IsConnected() {
		ctx.After(time.Second)
	}
	if ctx.Vibrancy() {
		ctx.Root().Background(ui.Transparent)
	}
	ui.Column(ctx).Fill().Padding(16).Gap(14).Children(func() {
		head := ui.Column(ctx).Padding(14).Gap(10).Radius(16).Material(glass.Glass{Tint: p.accent.Alpha(0.08)})
		head.Children(func() {
			ui.Row(ctx).AlignItems(ui.Center).Gap(8).Children(func() {
				mark := ui.Box(ctx).Size(22, 22).Radius(7).Center().Background(p.accent).Shrink(0)
				mark.Children(func() { symbol(ctx, "shield.fill", iconShield, 13, p.onAccent) })
				ui.Text(ctx, "Open TJ").FontWeight(650)
				ui.Spacer(ctx)
				ui.Text(ctx, c.Status()).FontSize(12).TextColor(p.muted)
			})
			if c.IsConnected() {
				ui.Text(ctx, "已连接 "+Duration(c.ConnectedAt(), time.Now())).
					FontSize(12).TextColor(p.muted)
			}
		})

		snapshot := c.Snapshot()
		ui.Row(ctx).Gap(8).Children(func() {
			menuRate(ctx, p, "下载", snapshot.DownloadBPS, p.down)
			menuRate(ctx, p, "上传", snapshot.UploadBPS, p.up)
		})

		ui.Column(ctx).Gap(8).Children(func() {
			if c.Active() {
				label := "断开 VPN"
				if c.Stopping() {
					label = "正在恢复网络…"
				}
				pill(ctx, "xmark", nil, label, ui.RGBA(128, 128, 128, 0.16), p.text).
					Fill().Disabled(c.Stopping()).OnClick(c.Disconnect)
			} else {
				pill(ctx, "bolt.fill", nil, "连接 VPN", p.accent, p.onAccent).Fill().
					OnClick(func() { c.Connect() })
			}
			pill(ctx, "macwindow", nil, "打开主窗口", p.card, p.text).Fill().
				OnClick(func() { c.OpenMain() })
			pill(ctx, "power", nil, "退出 Open TJ", ui.Transparent, p.muted).Fill().
				OnClick(func() { c.Quit() })
		})
	})
}

func menuRate(ctx *ui.Context, p palette, title string, rate float64, color ui.Color) {
	value, unit := RateParts(rate)
	ui.Column(ctx).Grow(1).Padding(10, 12).Gap(2).Radius(12).Background(p.card).Children(func() {
		ui.Text(ctx, title).FontSize(11).TextColor(color)
		ui.Row(ctx).Gap(4).AlignItems(ui.End).Children(func() {
			ui.Text(ctx, value).FontSize(16).FontWeight(660)
			ui.Text(ctx, unit).FontSize(10).TextColor(p.muted).Margin(0, 0, 2, 0)
		})
	})
}

// ViewMenuFallback draws the same panel where a tray icon cannot anchor a
// flyout.
func (c *Controller) ViewMenuFallback(ctx *ui.Context) { c.ViewMenu(ctx) }
