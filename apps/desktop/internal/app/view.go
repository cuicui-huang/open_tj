package app

import (
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"open-tj.local/desktop/internal/client"
	"open-tj.local/desktop/internal/settings"
)

// Pages of the sidebar. Four is the most it holds; anything else is grouped.
const (
	pageOverview = "overview"
	pageAccount  = "account"
	pageLog      = "log"
	pageSettings = "settings"
)

const sidebarWidth = 176

// ViewMain draws the window: a sidebar of pages beside the page chosen.
func (c *Controller) ViewMain(ctx *ui.Context) {
	p := applyTheme(ctx)
	ctx.OnWindowShortcut(ui.Cmd, ui.Key1, func() { c.choosePage(pageOverview) })
	ctx.OnWindowShortcut(ui.Cmd, ui.Key2, func() { c.choosePage(pageAccount) })
	ctx.OnWindowShortcut(ui.Cmd, ui.Key3, func() { c.choosePage(pageLog) })
	ctx.OnWindowShortcut(ui.Cmd, ui.Key4, func() { c.choosePage(pageSettings) })
	ctx.OnWindowShortcut(ui.Cmd, ui.KeyL, c.ShowLogToggle)

	if ctx.Vibrancy() {
		ctx.Root().Background(ui.Transparent)
	}
	ui.Row(ctx).Fill().AlignItems(ui.Stretch).Children(func() {
		side := ui.Column(ctx).Width(sidebarWidth)
		if !ctx.Vibrancy() {
			side.Background(p.sidebar)
		}
		side.Children(func() { c.sidebar(ctx, p) })

		// The page fades up as it arrives, and the key is the page: a
		// navigation is a new element entering, not the old one changing.
		ui.Box(ctx).Grow(1).Background(p.bg).Children(func() {
			ui.Column(ctx).Key(c.Page).Transition(pageTransition).Fill().Children(func() {
				switch c.Page {
				case pageAccount:
					c.viewAccount(ctx, p)
				case pageLog:
					c.viewLog(ctx)
				case pageSettings:
					c.viewSettings(ctx, p)
				default:
					c.viewOverview(ctx, p)
				}
			})
		})
	})
}

func (c *Controller) choosePage(page string) {
	if c.Page == page {
		return
	}
	c.Page = page
	c.ShowLog = page == pageLog
	c.Persist()
}

// shortcutHint is the hint beside a sidebar item: ⌘1 on macOS, Ctrl+1 elsewhere.
func shortcutHint(n string) string {
	if runtime.GOOS == "darwin" {
		return "⌘" + n
	}
	return "Ctrl+" + n
}

// sidebar draws the navigation: the mark, the connection and the pages.
func (c *Controller) sidebar(ctx *ui.Context, p palette) {
	bar := ctx.TitleBar()
	top := bar.Height
	if top < 28 {
		top = 28
	}
	ui.Column(ctx).Fill().Children(func() {
		// The window buttons own the title bar. On a window without one, the
		// brand takes that strip as the drag handle instead.
		if bar.Height > 0 {
			ui.Box(ctx).Height(top).DragWindow()
			ui.Row(ctx).Padding(0, 12, 2, 12).Gap(7).AlignItems(ui.Center).Children(func() {
				c.brand(ctx, p)
			})
		} else {
			ui.Row(ctx).Height(top).Padding(0, 12, 4, 12).Gap(7).
				AlignItems(ui.Center).DragWindow().Children(func() {
				c.brand(ctx, p)
			})
		}

		// Three bands, not one column: the connection at the top, the pages
		// under a hairline, and the gateway at the foot. The gaps between the
		// bands are what make a sidebar read as laid out rather than stacked.
		// Nothing to report before a session exists: an empty chip with a grey
		// dot and the campus name only repeated the card beside it.
		if c.IsConnected() || c.Active() {
			ui.Column(ctx).Padding(2, 12, 14, 12).Children(func() { c.statusChip(ctx, p) })
		}

		ui.Column(ctx).Padding(0, 12).Children(func() {
			ui.Box(ctx).Height(1).Fill().Background(p.divider)
		})

		ui.Column(ctx).Padding(6, 8).Gap(2).Children(func() {
			c.navItem(ctx, p, pageOverview, "gauge.with.dots.needle.67percent", iconGauge, "概览", shortcutHint("1"))
			c.navItem(ctx, p, pageAccount, "person.crop.circle", iconPerson, "账号", shortcutHint("2"))
			c.navItem(ctx, p, pageLog, "doc.text", iconDoc, "日志", shortcutHint("3"))
			c.navItem(ctx, p, pageSettings, "gearshape", iconGear, "设置", shortcutHint("4"))
		})

		ui.Spacer(ctx)
		ui.Column(ctx).Padding(0, 12).Children(func() {
			ui.Box(ctx).Height(1).Fill().Background(p.divider)
		})
		c.sidebarFoot(ctx, p)
	})
}

// brand is the mark and the name. It sits below the traffic lights, so it is
// never read as one of them.
func (c *Controller) brand(ctx *ui.Context, p palette) {
	mark := ui.Box(ctx).Size(20, 20).Radius(5).Center().Background(p.accent)
	mark.Children(func() { symbol(ctx, "shield.fill", iconShield, 11, p.onAccent) })
	ui.Text(ctx, "Open TJ").FontSize(14).FontWeight(640).TextColor(p.text).SingleLine()
}

// sidebarFoot is the gateway and the authentication method, with the tunnel
// interface once one is up. A row with nothing real to say is left out.
func (c *Controller) sidebarFoot(ctx *ui.Context, p palette) {
	gateway := c.Gateway()
	method := strings.TrimSpace(c.account.Method)
	second := method
	if c.IsConnected() && c.iface != "" {
		if method != "" {
			second = method + " · " + c.iface
		} else {
			second = c.iface
		}
	}
	if gateway == "" && second == "" {
		return
	}
	// One line, with the auth method as a label on the right: two stacked
	// lines of grey text at this size read as an afterthought.
	ui.Row(ctx).Padding(8, 12, 14, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		if gateway != "" {
			ui.Text(ctx, gateway).FontSize(11).TextColor(p.muted).SingleLine().Grow(1)
		}
		if second != "" {
			ui.Text(ctx, second).FontSize(10.5).TextColor(p.faint).SingleLine()
		}
	})
}

// statusChip is the small connection summary under the mark. Its duration
// ticks by the second, so the frame repeats while a tunnel is up: only the
// overview shows it, and only there is the redraw worth its cost.
func (c *Controller) statusChip(ctx *ui.Context, p palette) {
	if c.IsConnected() && c.Page == pageOverview {
		ctx.After(time.Second)
	}
	color := p.faint
	switch {
	case c.IsConnected():
		color = ctx.Theme().Success
	case c.Active():
		color = ctx.Theme().Warning
	}
	ui.Row(ctx).Gap(8).Padding(7, 9).Radius(8).AlignItems(ui.Center).Fill().
		Background(p.card).Children(func() {
		// The dot is the one thing in the sidebar that says "live", so it is
		// the one thing that pulses: a slow breath while the tunnel is up.
		// Keyframes loops it; AnimatePose would only move once, since its
		// target never changes.
		dot := ui.Box(ctx).Size(7, 7).Radius(999).Background(color).Shrink(0)
		if c.IsConnected() {
			level := dot.Keyframes("live", []ui.Keyframe{
				{At: 0, Value: 0},
				{At: 900 * time.Millisecond, Value: 1, Ease: ui.EaseInOut},
				{At: 1800 * time.Millisecond, Value: 0, Ease: ui.EaseInOut},
			}, ui.KeyframeOptions{Repeat: ui.RepeatForever})
			dot.AnimatePose(ui.Pose{Scale: 1 + 0.16*level}, ui.Animation{Duration: 16 * time.Millisecond})
		}
		// The card beside this one already says the status in large type and
		// names the campus network, so this row carries only what that card
		// cannot: how long the tunnel has been up. Wording it as a phrase
		// keeps a bare clock from looking like a stray counter.
		// The dot and the neighbouring card already carry the state, so the
		// row says the one thing neither can: how long. Before a tunnel exists
		// there is nothing to count, and the row stays a label.
		detail := "同济大学校园网"
		if c.IsConnected() {
			detail = Duration(c.ConnectedAt(), c.cfg.Now())
		} else if c.Active() {
			detail = "正在连接"
		}
		ui.Text(ctx, detail).FontSize(12).FontWeight(550).Grow(1).
			SingleLine().TextColor(p.muted).ContentTransition(ui.ContentTransition{Morph: true})
		if c.Active() && !c.IsConnected() {
			ui.Spinner(ctx)
		}
	})
}

func (c *Controller) navItem(ctx *ui.Context, p palette, id, symbolName string, svg *ui.SVG, label, shortcut string) {
	chosen := c.Page == id
	ui.Row(ctx).Height(32).AlignItems(ui.Center).Children(func() {
		// The selection bar grows out of nothing as the item is chosen, so
		// the choice reads as a thing moving rather than a color appearing.
		bar := ui.Box(ctx).Width(3).Radius(2)
		if chosen {
			bar.Height(15).Background(p.accent)
		} else {
			bar.Height(0).Background(ui.Transparent)
		}
		bar.Transition(ui.ElementTransition{Height: true, Ease: ui.EaseOut,
			Duration: 160 * time.Millisecond})
		b := ui.ButtonBase(ctx).Grow(1).Height(32).Padding(8, 10).Gap(10).Radius(7).
			AlignItems(ui.Center).Label(label)
		fg := p.muted
		weight := 500
		bg := ui.Transparent
		switch {
		case chosen:
			fg, weight, bg = p.text, 620, p.hover
		case b.Hovered():
			bg = p.hover
		}
		b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 140 * time.Millisecond}))
		if b.Clicked() {
			c.choosePage(id)
		}
		b.Children(func() {
			symbol(ctx, symbolName, svg, 15, fg)
			ui.Text(ctx, label).FontSize(12.5).FontWeight(weight).Grow(1).SingleLine().TextColor(fg)
			// The shortcut is a hint, not a column: showing it always stole
			// width from the label and made four rows look busy. It fades in
			// on the row the pointer is on, and on the chosen one.
			if b.Hovered() || chosen {
				ui.Text(ctx, shortcut).FontSize(11).TextColor(p.faint).
					Transition(ui.ElementTransition{
						Duration: 140 * time.Millisecond,
						Enter:    &ui.Motion{Opacity: 1},
					})
			}
		})
	})
}

func (c *Controller) viewOverview(ctx *ui.Context, p palette) {
	ui.Scroll(ctx).Fill().Children(func() {
		ui.Column(ctx).Padding(16, 16, 18, 16).Gap(10).Children(func() {
			c.viewHero(ctx, p)
			c.viewProbes(ctx, p)
			c.viewTraffic(ctx, p)
			c.viewSession(ctx, p)
		})
	})
}

// viewHero is the connection card: status, duration and the one action.
func (c *Controller) viewHero(ctx *ui.Context, p palette) {
	t := ctx.Theme()
	if c.IsConnected() {
		ctx.After(time.Second)
	}
	failed := !c.Active() && (c.status == "连接失败" || c.status == "暂不可用")
	ui.Column(ctx).Padding(14, 16).Gap(10).Radius(14).Background(p.card).Children(func() {
		ui.Row(ctx).Gap(12).AlignItems(ui.Center).Children(func() {
			ring := ui.Box(ctx).Size(40, 40).Radius(999).Center().Shrink(0)
			ringColor := p.hover
			iconColor := p.muted
			iconName, iconSVG := "shield.fill", iconShield
			switch {
			case failed:
				ringColor, iconColor = t.Danger, ui.Hex("#ffffff")
				iconName, iconSVG = "exclamationmark.triangle.fill", iconWarn
			case c.IsConnected():
				ringColor, iconColor = p.accent, p.onAccent
			case c.Active():
				ringColor, iconColor = t.Warning.Alpha(0.18), t.Warning
			}
			ring.Background(ring.AnimateColor("bg", ringColor, ui.Snappy))
			// The ring swells when the state it stands for changes, and it is
			// keyed by that state so the new one arrives as a new element.
			ring.Key(stateKey(c)).AnimatePose(ringPose(c), heroSpring)
			ring.Children(func() {
				if c.Active() && !c.IsConnected() {
					ui.Spinner(ctx)
					return
				}
				symbol(ctx, iconName, iconSVG, 23, iconColor)
			})

			ui.Column(ctx).Grow(1).Gap(2).Children(func() {
				ui.Text(ctx, c.Status()).FontSize(19).FontWeight(660).SingleLine().
					ContentTransition(ui.ContentTransition{Morph: true})
				subtitle := "同济大学校园网"
				if c.IsConnected() {
					subtitle = "同济大学校园网 · " + Duration(c.ConnectedAt(), c.cfg.Now())
				}
				ui.Text(ctx, subtitle).FontSize(12.5).TextColor(p.muted)
			})

			switch {
			case c.Active():
				label := "断开"
				if c.Stopping() {
					label = "恢复中…"
				}
				pill(ctx, "xmark", iconX, label, ui.RGBA(0, 0, 0, 0.06), p.text).
					Disabled(c.Stopping()).OnClick(c.Disconnect)
			default:
				// After a failure the button names the retry, so the state it
				// would fix is on the control the user presses.
				label := "连接 VPN"
				if failed {
					label = "重试"
				}
				if !c.supported() {
					label = "当前系统不支持连接"
				}
				ready := c.supported() && c.MissingCredential() == ""
				fill := p.accent
				if !ready {
					fill = p.accent.Alpha(0.38)
				}
				pill(ctx, "bolt.fill", iconBolt, label, fill, p.onAccent).Disabled(!ready).
					OnClick(func() { c.Connect() })
			}
		})

		if missing := c.MissingCredential(); missing != "" && !c.Active() && c.supported() {
			ui.Row(ctx).Gap(6).AlignItems(ui.Center).Children(func() {
				symbol(ctx, "exclamationmark.circle", iconWarn, 12, t.Warning)
				ui.Text(ctx, missing).FontSize(12).TextColor(t.Warning)
				ui.Spacer(ctx)
				link := ui.ButtonBase(ctx).Label("去账号页填写")
				if link.Clicked() {
					c.choosePage(pageAccount)
				}
				link.Children(func() {
					ui.Text(ctx, "去账号页填写").FontSize(12).TextColor(p.accent).FontWeight(560)
				})
			})
		} else if detail := c.Detail(); detail != "" && !c.IsConnected() && c.Active() {
			ui.Text(ctx, detail).FontSize(13).TextColor(p.muted).Selectable()
		} else if failed && c.Detail() != "" {
			ui.Text(ctx, c.Detail()).FontSize(13).TextColor(t.Danger).Selectable()
		}
	})
}

// stateKey names the connection state as a string the view can key an
// element by: a change of state is a change of element, which is what makes
// the transition run.
func stateKey(c *Controller) string {
	switch {
	case c.IsConnected():
		return "connected"
	case c.Stopping():
		return "stopping"
	case c.Active():
		return "connecting"
	case !c.Active() && (c.status == "连接失败" || c.status == "暂不可用"):
		return "failed"
	default:
		return "idle"
	}
}

// ringPose is where the status ring sits in each state: a touch larger once
// connected, a touch smaller when nothing is happening. The spring between
// the two is what the eye reads as the app answering the click.
func ringPose(c *Controller) ui.Pose {
	switch {
	case c.IsConnected():
		return ui.Pose{Scale: 1.04}
	case c.Active():
		return ui.Pose{Scale: 1.0}
	default:
		return ui.Pose{Scale: 0.96}
	}
}

// viewProbes is the connectivity card. With no tunnel up it says so, and
// never invents a result.
func (c *Controller) viewProbes(ctx *ui.Context, p palette) {
	probes, at := c.Probes()
	empty := len(probes) == 0
	if !empty || c.Checking() {
		ctx.After(time.Second)
	}
	ui.Column(ctx).Gap(7).Children(func() {
		ui.Row(ctx).AlignItems(ui.Center).Children(func() {
			ui.Text(ctx, "连通性").FontSize(14).FontWeight(640)
			ui.Spacer(ctx)
			switch {
			case c.Checking():
				ui.Text(ctx, "检测中").FontSize(12).TextColor(p.faint)
			case !empty:
				ui.Text(ctx, "更新于 "+RelativeTime(at, c.cfg.Now())).FontSize(12).TextColor(p.faint)
				ui.Box(ctx).Width(6)
				iconButton(ctx, "arrow.clockwise", iconRefresh, 11, p.muted, "重新检测").
					OnClick(c.RefreshProbes)
			}
		})
		if empty {
			ui.Row(ctx).Padding(16, 0).Height(64).Gap(9).AlignItems(ui.Center).Center().
				Radius(12).Background(p.card).Children(func() {
				symbol(ctx, "dot.radiowaves.left.and.right", iconWaves, 15, p.faint)
				ui.Text(ctx, "连接后自动检测校园网连通性").FontSize(12.5).TextColor(p.muted)
			})
			return
		}
		ui.Column(ctx).Padding(1, 12).Radius(12).Background(p.card).Children(func() {
			first := true
			for i, probe := range probes {
				if !first {
					ui.Divider(ctx)
				}
				first = false
				// The rows land one after another instead of all at once, so
				// a fresh check reads as results arriving.
				ui.Box(ctx).Key(probe.URL).Transition(ui.ElementTransition{
					Enter: &rowEnter, Animation: rowDelay(i),
				}).Children(func() {
					c.probeRow(ctx, p, probe, false)
				})
				for _, hop := range probe.Redirects {
					ui.Row(ctx).Padding(0, 0, 8, 14).Children(func() {
						ui.Text(ctx, "→").FontSize(12).TextColor(p.faint).Shrink(0)
						ui.Box(ctx).Width(8)
						ui.Column(ctx).Grow(1).Children(func() { c.probeRow(ctx, p, hop, true) })
					})
				}
			}
		})
	})
}

func (c *Controller) probeRow(ctx *ui.Context, p palette, probe client.ProbeResult, hop bool) {
	level := ProbeLevel(probe)
	dot := ctx.Theme().Success
	noteColor := ctx.Theme().Warning
	switch level {
	case 2:
		dot, noteColor = ctx.Theme().Danger, ctx.Theme().Danger
	case 1:
		dot = ctx.Theme().Warning
	}
	note := ""
	if level != 0 {
		note = ExplainProbe(probe)
	}
	ui.Row(ctx).Gap(9).Padding(6, 0).AlignItems(ui.Center).Children(func() {
		ui.Column(ctx).Grow(1).Gap(1).Children(func() {
			// The label says which campus service this is; the host name
			// below it says exactly what was reached, for anyone who wants to
			// reproduce it. A bare host like "xlcp" answers nothing.
			name := probe.Label
			if name == "" {
				name = ProbeHost(probe.URL)
			}
			ui.Text(ctx, name).FontSize(12.5).FontWeight(550).SingleLine()
			if host := ProbeHost(probe.URL); name != host {
				ui.Text(ctx, host).FontSize(10.5).TextColor(p.faint).SingleLine()
			}
			if note != "" {
				// An explanation appears under the host rather than replacing
				// it: it collapses open so the row grows instead of jumping.
				ui.Text(ctx, note).FontSize(11.5).TextColor(noteColor).
					Transition(ui.ElementTransition{
						Duration: 200 * time.Millisecond, Ease: ui.EaseOut,
						Enter: &ui.Motion{Collapse: true, Opacity: 1},
					})
			}
		})
		if note == "" && !hop {
			ui.Text(ctx, Millis(probe.SystemDNSMS)).FontSize(11.5).TextColor(p.muted)
			ui.Text(ctx, "·").FontSize(11.5).TextColor(p.faint)
			ui.Text(ctx, Millis(probe.TCPMS)).FontSize(11.5).TextColor(p.muted)
			ui.Text(ctx, "·").FontSize(11.5).TextColor(p.faint)
			ui.Text(ctx, Millis(probe.TLSMS)).FontSize(11.5).TextColor(p.muted)
		} else if note == "" {
			ui.Text(ctx, Millis(probe.TotalMS)).FontSize(11.5).TextColor(p.muted)
		}
		ui.Box(ctx).Size(7, 7).Radius(999).Background(dot).Shrink(0)
	})
}

func (c *Controller) viewTraffic(ctx *ui.Context, p palette) {
	snapshot := c.Snapshot()
	// Before the first session there is nothing to plot, so one quiet line
	// says more than two empty cards.
	if len(snapshot.DownHistory) == 0 && len(snapshot.UploadHistory) == 0 {
		ui.Row(ctx).Padding(13, 14).Gap(10).Height(48).AlignItems(ui.Center).
			Radius(14).Background(p.card).Children(func() {
			symbol(ctx, "arrow.up.arrow.down", iconSwap, 13, p.faint)
			ui.Text(ctx, "连接后显示实时速率").FontSize(12.5).TextColor(p.muted)
			ui.Spacer(ctx)
			ui.Text(ctx, "最近 60 秒").FontSize(11).TextColor(p.faint)
		})
		return
	}
	ui.Row(ctx).Gap(10).AlignItems(ui.Stretch).Children(func() {
		c.trafficCard(ctx, p, "下载", "arrow.down", iconDown, snapshot.DownloadBPS,
			snapshot.DownloadBytes, snapshot.DownHistory, p.down)
		c.trafficCard(ctx, p, "上传", "arrow.up", iconUp, snapshot.UploadBPS,
			snapshot.UploadBytes, snapshot.UploadHistory, p.up)
	})
}

func (c *Controller) trafficCard(ctx *ui.Context, p palette, title, symbolName string, svg *ui.SVG, rate float64, total int64, history []float64, color ui.Color) {
	// The rate the card showed last frame, so a falling number rolls down.
	previous := c.displayedRate(title)
	defer c.noteDisplayedRate(title, rate)
	value, unit := RateParts(rate)
	peak := 0.0
	for _, sample := range history {
		if sample > peak {
			peak = sample
		}
	}
	idle := rate == 0 && len(history) == 0
	if idle {
		color = p.faint
	}
	ui.Column(ctx).Grow(1).Padding(13, 14).Gap(5).Radius(14).Background(p.card).Children(func() {
		ui.Row(ctx).Gap(6).AlignItems(ui.Center).Children(func() {
			symbol(ctx, symbolName, svg, 12, color)
			ui.Text(ctx, title).FontSize(12).TextColor(color).FontWeight(600)
		})
		// The rate changes every second. Rolling the digits rather than
		// swapping them keeps a fast link from looking like it is flickering;
		// Down tells the transition which way the number went.
		ui.Row(ctx).Gap(5).AlignItems(ui.Baseline).Children(func() {
			ui.Text(ctx, value).FontSize(24).FontWeight(660).
				ContentTransition(ui.ContentTransition{Numeric: true, Down: rate < previous})
			ui.Text(ctx, unit).FontSize(12).TextColor(p.muted).FontWeight(500).
				ContentTransition(ui.ContentTransition{Morph: true})
		})
		if idle {
			ui.Box(ctx).Fill().Height(26)
		} else {
			c.sparkline(ctx, history, color)
		}
		peakText, totalText := "峰值 —", "累计 —"
		if !idle {
			peakText = "峰值 " + Bytes(peak) + "/s"
			totalText = "累计 " + Bytes(float64(total))
		}
		ui.Row(ctx).Children(func() {
			ui.Text(ctx, peakText).FontSize(11).TextColor(p.faint)
			ui.Spacer(ctx)
			ui.Text(ctx, totalText).FontSize(11).TextColor(p.muted)
		})
	})
}

// sparkline fills its row with the last minute as a soft area under a line.
func (c *Controller) sparkline(ctx *ui.Context, history []float64, color ui.Color) {
	ceiling := GraphCeiling(history)
	xs := make([]float32, len(history))
	ys := make([]float32, len(history))
	for i, sample := range history {
		xs[i] = float32(i) / float32(historyLength-1)
		ys[i] = float32(sample / ceiling)
	}
	ui.Box(ctx).Fill().Height(34).Draw(func(painter *ui.Painter, r ui.Rect) {
		if len(xs) < 2 {
			return
		}
		painter.FillPath(chart.Area(r, xs, nil, ys, chart.CurveMonotone), color.Alpha(0.14))
		painter.StrokePath(chart.Line(r, xs, ys, chart.CurveMonotone), 1.6, color)
	})
}

// viewSession lists what the tunnel is doing, once it is up.
func (c *Controller) viewSession(ctx *ui.Context, p palette) {
	if !c.IsConnected() {
		return
	}
	ui.Column(ctx).Padding(3, 12).Radius(12).Background(p.card).Children(func() {
		sessionRow(ctx, p, "账号", c.account.Username)
		ui.Divider(ctx)
		sessionRow(ctx, p, "认证方式", c.account.Method)
		ui.Divider(ctx)
		sessionRow(ctx, p, "网关", c.Gateway())
	})
}

func sessionRow(ctx *ui.Context, p palette, title, value string) {
	ui.Row(ctx).Height(30).AlignItems(ui.Center).Children(func() {
		ui.Text(ctx, title).FontSize(12.5).TextColor(p.muted)
		ui.Spacer(ctx)
		ui.Text(ctx, value).FontSize(12.5).FontWeight(550).Selectable().SingleLine()
	})
}

func (c *Controller) viewAccount(ctx *ui.Context, p palette) {
	ui.Scroll(ctx).Fill().Children(func() {
		ui.Column(ctx).Padding(16, 16, 18, 16).Gap(11).MaxWidth(380).Children(func() {
			ui.Column(ctx).Gap(4).Children(func() {
				ui.Text(ctx, "账号").FontSize(19).FontWeight(660)
				if strings.TrimSpace(c.cfg.Prefs.Username) == "" {
					ui.Text(ctx, "首次使用？用你的学号和校园网密码登录").FontSize(13).TextColor(p.muted)
				} else {
					ui.Text(ctx, "用于登录 vpn.tongji.cn。密码只保存在系统钥匙串里。").
						FontSize(13).TextColor(p.muted)
				}
			})

			locked := c.Active()
			ui.Column(ctx).Padding(14, 16).Gap(12).Radius(14).Background(p.card).Children(func() {
				ui.Form(ctx, func() {
					ui.Field(ctx, "VPN 用户名", func() {
						input := ui.TextInput(ctx, &c.account.Username).Placeholder("学号或工号").Disabled(locked)
						if strings.TrimSpace(c.cfg.Prefs.Username) == "" {
							input.Focus()
						}
					})
					ui.Field(ctx, "VPN 密码", func() {
						ui.Row(ctx).Gap(8).AlignItems(ui.Center).Children(func() {
							input := ui.TextInput(ctx, &c.account.Password).Placeholder("校园网密码").Grow(1).Disabled(locked)
							if !c.ShowPassword {
								input.Password()
							}
							iconButton(ctx, "eye", iconEye, 16, p.muted, "显示密码").
								OnClick(func() { c.ShowPassword = !c.ShowPassword })
						})
					})
					ui.Field(ctx, "认证方式", func() {
						ui.Select(ctx, &c.account.Method, settings.Methods).Disabled(locked)
					})
					ui.Field(ctx, "记住密码", func() {
						if ui.Switch(ctx, &c.account.Remember).Disabled(locked).Changed() {
							c.SetRemember(c.account.Remember)
						}
					})
				})
				if locked {
					ui.Text(ctx, "连接期间不能修改账号，断开后即可编辑。").FontSize(12).TextColor(p.faint)
				}
			})

			if !locked {
				ready := c.supported() && c.MissingCredential() == ""
				ui.Row(ctx).Gap(10).AlignItems(ui.Center).Children(func() {
					pill(ctx, "bolt.fill", iconBolt, "连接 VPN", p.accent, p.onAccent).Disabled(!ready).
						OnClick(func() { c.Connect() })
					// A saved password that could not be read is recoverable
					// here: without this the user has no way to retry it and
					// would have to type the password again.
					if c.NeedsCredential() {
						link := ui.ButtonBase(ctx).Label("从钥匙串载入密码")
						if link.Clicked() {
							c.LoadCredentials()
							c.redraw()
						}
						link.Children(func() {
							ui.Text(ctx, "从钥匙串载入密码").FontSize(12).
								TextColor(p.accent).FontWeight(560)
						})
					} else if missing := c.MissingCredential(); missing != "" && c.supported() {
						ui.Text(ctx, missing).FontSize(12).TextColor(ctx.Theme().Warning)
					}
				})
				if msg := c.CredentialError(); msg != "" {
					ui.Text(ctx, msg).FontSize(11.5).TextColor(ctx.Theme().Danger)
				}
			}
		})
	})
}

// viewSettings is the settings page: launch, notifications and the connection.
func (c *Controller) viewSettings(ctx *ui.Context, p palette) {
	ui.Scroll(ctx).Fill().Children(func() {
		ui.Column(ctx).Padding(16, 16).Gap(14).MaxWidth(430).Children(func() {
			ui.Column(ctx).Gap(3).Children(func() {
				ui.Text(ctx, "设置").FontSize(19).FontWeight(660)
				ui.Text(ctx, "这些选项只影响本机。").FontSize(12.5).TextColor(p.muted)
			})

			settingsGroup(ctx, p, "常规", func() {
				c.toggleRow(ctx, p, "开机时启动", "登录后自动驻留菜单栏", c.OpenAtLogin(), c.SetOpenAtLogin)
				if errText := c.LoginError(); errText != "" {
					ui.Text(ctx, errText).FontSize(11.5).TextColor(ctx.Theme().Danger).Padding(0, 0, 8, 0)
				}
				ui.Divider(ctx)
				c.toggleRow(ctx, p, "连接成功后发送通知", "在通知中心显示连接状态", c.NotifyOnConnect(), c.SetNotifyOnConnect)
			})

			settingsGroup(ctx, p, "连接", func() {
				note := "需要已保存账号与密码"
				if !c.account.Remember {
					note = "先在账号页打开「记住密码」"
				}
				c.toggleRow(ctx, p, "启动时自动连接", note, c.AutoConnect(), c.SetAutoConnect)
				ui.Divider(ctx)
				ui.Row(ctx).Padding(12, 0).Gap(16).AlignItems(ui.Center).Children(func() {
					ui.Text(ctx, "认证方式").FontSize(12.5).FontWeight(540).Shrink(0)
					ui.Spacer(ctx)
					if ui.Select(ctx, &c.account.Method, settings.Methods).Disabled(c.Active()).Changed() {
						c.Persist()
					}
				})
			})

			settingsGroup(ctx, p, "关于", func() {
				version := c.Version()
				if version == "" {
					version = "开发版"
				}
				sessionRow(ctx, p, "版本", version)
				ui.Divider(ctx)
				sessionRow(ctx, p, "网关", c.Gateway())
				ui.Divider(ctx)
				ui.Row(ctx).Height(30).AlignItems(ui.Center).Children(func() {
					ui.Text(ctx, "许可").FontSize(12.5).TextColor(p.muted)
					ui.Spacer(ctx)
					ui.Text(ctx, "MIT License").FontSize(12).TextColor(p.faint)
				})
			})
		})
	})
}

func settingsGroup(ctx *ui.Context, p palette, title string, rows func()) {
	ui.Column(ctx).Gap(5).Children(func() {
		ui.Text(ctx, title).FontSize(11).FontWeight(620).TextColor(p.faint)
		ui.Column(ctx).Padding(1, 14).Radius(12).Background(p.card).Children(rows)
	})
}

// toggleRow is one setting: a label, a note under it and a switch. Changing
// the switch calls set, which keeps the choice.
func (c *Controller) toggleRow(ctx *ui.Context, p palette, label, note string, on bool, set func(bool)) {
	ui.Row(ctx).Padding(10, 0).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(ctx).Grow(1).Gap(2).Children(func() {
			ui.Text(ctx, label).FontSize(12.5).FontWeight(540)
			ui.Text(ctx, note).FontSize(11).TextColor(p.faint)
		})
		if ui.Switch(ctx, &on).Label(label).Changed() {
			set(on)
		}
	})
}
