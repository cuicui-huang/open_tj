package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// ViewMenu draws the menu bar panel.
func (c *Controller) ViewMenu(ctx *ui.Context) {
	t := ctx.Theme()
	if c.IsConnected() {
		ctx.After(time.Second)
	}
	ui.Column(ctx).Fill().Background(t.Background).Padding(16).Gap(12).Children(func() {
		ui.Row(ctx).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(ctx, "Open TJ").Bold()
			ui.Spacer(ctx)
			ui.Text(ctx, c.Status()).FontSize(11).TextColor(t.TextMuted)
		})
		if c.IsConnected() {
			ui.Row(ctx).Children(func() {
				ui.Text(ctx, "连接时长").FontSize(11).TextColor(t.TextMuted)
				ui.Spacer(ctx)
				ui.Text(ctx, Duration(c.ConnectedAt(), time.Now())).FontSize(11).TextColor(t.TextMuted)
			})
		}
		snapshot := c.Snapshot()
		ui.Row(ctx).Children(func() {
			down, downUnit := RateParts(snapshot.DownloadBPS)
			up, upUnit := RateParts(snapshot.UploadBPS)
			ui.Text(ctx, "↓ "+down+" "+downUnit).FontSize(13).TextColor(downloadColor)
			ui.Spacer(ctx)
			ui.Text(ctx, "↑ "+up+" "+upUnit).FontSize(13).TextColor(uploadColor)
		})
		ui.Divider(ctx)
		if c.Active() {
			label := "断开 VPN"
			if c.Stopping() {
				label = "正在恢复网络…"
			}
			ui.Button(ctx.Key("menu-disconnect"), label).Disabled(c.Stopping()).OnClick(func() { c.Disconnect() })
		} else {
			ui.Button(ctx.Key("menu-connect"), "连接 VPN").OnClick(func() { c.Connect() })
		}
		ui.Button(ctx.Key("menu-open"), "打开主窗口").OnClick(func() { c.OpenMain() })
		ui.Divider(ctx)
		ui.Button(ctx.Key("menu-quit"), "退出 Open TJ").OnClick(func() { c.Quit() })
	})
}

// ViewMenuFallback draws a small panel for platforms whose tray icons cannot
// anchor flyouts, so the menu still works everywhere.
func (c *Controller) ViewMenuFallback(ctx *ui.Context) { c.ViewMenu(ctx) }
