package app

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"open-tj.local/desktop/internal/settings"
	"open-tj.local/desktop/internal/vpn"
)

// Palette of the traffic columns.
var (
	downloadColor = ui.Hex("#3B82F6")
	uploadColor   = ui.Hex("#14B8A6")
)

// ViewMain draws the main window: the dashboard, with the log beside it when
// the log pane is open.
func (c *Controller) ViewMain(ctx *ui.Context) {
	ctx.OnWindowShortcut(ui.Cmd, ui.KeyL, c.ShowLogToggle)
	if c.ShowLog {
		ui.Split(ctx, &c.SplitWidth, func() { c.viewDashboard(ctx) }, func() { c.viewLog(ctx) }).
			SplitReset(460).Fill()
		return
	}
	c.viewDashboard(ctx)
}

func (c *Controller) viewDashboard(ctx *ui.Context) {
	t := ctx.Theme()
	ui.Column(ctx).Fill().Background(t.Background).Children(func() {
		c.viewHeader(ctx, true)
		ui.Divider(ctx)
		ui.Scroll(ctx).Grow(1).Children(func() {
			ui.Column(ctx).Padding(18).Gap(14).Children(func() {
				if !c.IsConnected() && c.Detail() != "" {
					ui.Text(ctx, c.Detail()).FontSize(13).TextColor(t.TextMuted).Selectable()
				}
				c.viewTraffic(ctx)
				ui.Divider(ctx)
				if c.Active() {
					c.viewActiveSettings(ctx)
				} else {
					c.viewForm(ctx)
				}
			})
		})
		if !c.Active() || c.Stopping() {
			ui.Divider(ctx)
			c.viewConnectFooter(ctx)
		}
	})
}

func (c *Controller) viewHeader(ctx *ui.Context, showDisconnect bool) {
	t := ctx.Theme()
	if c.IsConnected() {
		ctx.After(time.Second)
	}
	ui.Row(ctx).Padding(18).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(ctx).Grow(1).Gap(5).Children(func() {
			ui.Row(ctx).Gap(7).AlignItems(ui.Center).Children(func() {
				ui.Text(ctx, c.Status()).FontSize(20).Bold()
				if c.Active() && !c.IsConnected() {
					ui.Spinner(ctx)
				}
			})
			if c.IsConnected() {
				ui.Text(ctx, "连接时长 "+Duration(c.ConnectedAt(), time.Now())).
					FontSize(12).TextColor(t.TextMuted)
			} else {
				ui.Text(ctx, "同济大学校园网络").FontSize(12).TextColor(t.TextMuted)
			}
		})
		logLabel := "日志"
		if c.ShowLog {
			logLabel = "隐藏日志"
		}
		ui.Button(ctx.Key("toggle-log"), logLabel).Label(logLabel + "（⌘L）").
			OnClick(c.ShowLogToggle)
		if showDisconnect && c.Active() {
			label := "断开"
			if c.Stopping() {
				label = "恢复中…"
			}
			ui.Button(ctx.Key("disconnect"), label).Disabled(c.Stopping()).
				Label(label + " VPN").OnClick(c.Disconnect)
		}
	})
}

func (c *Controller) viewTraffic(ctx *ui.Context) {
	t := ctx.Theme()
	snapshot := c.Snapshot()
	ui.Column(ctx).Padding(14).Gap(10).Radius(12).Background(t.Surface).Children(func() {
		ui.Row(ctx).AlignItems(ui.Center).Children(func() {
			ui.Text(ctx, "网络流量").Bold()
			ui.Spacer(ctx)
			ui.Text(ctx, "最近 60 秒").FontSize(12).TextColor(t.TextMuted)
		})
		ui.Row(ctx).Gap(24).Children(func() {
			c.viewSpeed(ctx, "下载", snapshot.DownloadBPS, snapshot.DownloadBytes,
				snapshot.DownHistory, downloadColor)
			c.viewSpeed(ctx, "上传", snapshot.UploadBPS, snapshot.UploadBytes,
				snapshot.UploadHistory, uploadColor)
		})
	})
}

func (c *Controller) viewSpeed(ctx *ui.Context, title string, rate float64, total int64, history []float64, color ui.Color) {
	t := ctx.Theme()
	value, unit := RateParts(rate)
	ceiling := GraphCeiling(history)
	peak := 0.0
	for _, sample := range history {
		if sample > peak {
			peak = sample
		}
	}
	xs := make([]float32, len(history))
	ys := make([]float32, len(history))
	for i, sample := range history {
		xs[i] = float32(i) / float32(historyLength-1)
		ys[i] = float32(sample / ceiling)
	}

	ui.Column(ctx).Grow(1).Gap(6).Children(func() {
		ui.Text(ctx, title).FontSize(13).TextColor(color)
		ui.Row(ctx).Gap(4).AlignItems(ui.End).Children(func() {
			ui.Text(ctx, value).FontSize(22).Bold()
			ui.Text(ctx, unit).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Row(ctx).Fill().Height(32).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X, r.Y+r.H-1, r.X+r.W, r.Y+r.H-1, 1, t.Border)
			if len(xs) < 2 {
				return
			}
			p.StrokePath(chart.Line(r, xs, ys, chart.CurveLinear), 1.5, color)
		})
		ui.Text(ctx, "峰值 "+Bytes(peak)+"/s").FontSize(10).TextColor(t.TextMuted)
		ui.Text(ctx, "累计 "+Bytes(float64(total))).FontSize(11).TextColor(t.TextMuted)
	})
}

func connectionRow(ctx *ui.Context, t *ui.Theme, title, value string) {
	ui.Row(ctx).Children(func() {
		ui.Text(ctx, title).FontSize(13).TextColor(t.TextMuted)
		ui.Spacer(ctx)
		ui.Text(ctx, value).FontSize(13).Selectable()
	})
}

func (c *Controller) viewForm(ctx *ui.Context) {
	t := ctx.Theme()
	ui.Column(ctx).Gap(12).Children(func() {
		ui.Form(ctx, func() {
			ui.Field(ctx, "VPN 用户名", func() {
				ui.TextInput(ctx, &c.account.Username).Placeholder("学号或工号")
			})
			ui.Field(ctx, "VPN 密码", func() {
				ui.Row(ctx).Gap(10).AlignItems(ui.Center).Children(func() {
					input := ui.TextInput(ctx, &c.account.Password).Placeholder("校园网密码").Grow(1)
					if !c.ShowPassword {
						input.Password()
					}
					ui.Checkbox(ctx, &c.ShowPassword, "显示").Label("显示密码")
				})
			})
			ui.Field(ctx, "认证方式", func() {
				ui.Select(ctx, &c.account.Method, settings.Methods)
			})
			ui.Field(ctx, "在钥匙串中记住密码", func() {
				if ui.Switch(ctx, &c.account.Remember).Changed() {
					c.Persist()
				}
			})
		})
		if c.Active() {
			ui.Text(ctx, "断开连接后可修改设置").FontSize(11).TextColor(t.TextMuted)
		} else {
			ui.Text(ctx, "连接后可从菜单栏查看状态、断开或重新打开窗口。").
				FontSize(11).TextColor(t.TextMuted)
		}
	})
}

func (c *Controller) viewActiveSettings(ctx *ui.Context) {
	t := ctx.Theme()
	ui.Column(ctx).Gap(10).Children(func() {
		label := "连接设置"
		if c.ShowSettings {
			label = "收起连接设置"
		}
		if ui.Button(ctx.Key("settings"), label).Clicked() {
			c.ShowSettings = !c.ShowSettings
		}
		if c.ShowSettings {
			ui.Column(ctx).Gap(8).Children(func() {
				connectionRow(ctx, t, "账号", c.account.Username)
				connectionRow(ctx, t, "认证方式", c.account.Method)
			})
		}
		ui.Text(ctx, "断开后可编辑账号设置").FontSize(11).TextColor(t.TextMuted)
	})
}

func (c *Controller) viewConnectFooter(ctx *ui.Context) {
	t := ctx.Theme()
	ui.Column(ctx).Padding(16).Gap(8).Children(func() {
		if !c.Active() {
			label := "连接 VPN"
			if c.Stopping() {
				label = "正在恢复网络…"
			}
			unsupported := !vpn.Supported()
			if unsupported {
				label = "仅桌面版支持连接"
			}
			button := ui.PrimaryButton(ctx.Key("connect"), label).
				Disabled(unsupported || c.Stopping() || strings.TrimSpace(c.account.Username) == "" || c.account.Password == "")
			if button.Clicked() {
				c.Connect()
			}
		}
		message := "断开后自动恢复网络设置"
		if c.Stopping() {
			message = "请稍候，正在恢复网络设置"
		}
		if !vpn.Supported() {
			message = "此构建用于界面预览，VPN 连接目前只支持 macOS 和 Windows。"
		}
		ui.Text(ctx, message).FontSize(11).TextColor(t.TextMuted)
	})
}
