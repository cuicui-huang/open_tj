package app

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// levelNames maps the filter's labels to the levels entries keep.
var levelNames = []struct{ label, value string }{
	{"全部", "all"},
	{"信息", "info"},
	{"警告", "warning"},
	{"错误", "error"},
}

func (c *Controller) viewLog(ctx *ui.Context) {
	p := paletteFor(ctx.Theme())
	// The list stops following its end when the user scrolls up; mirror that
	// in the toggle.
	if c.FollowLogs && !c.LogList.FollowEnd {
		c.FollowLogs = false
	}
	entries := c.FilteredEntries()
	c.LogList.Key = func(row int) any { return entries[row].ID }
	warnings, errors := c.Counts()

	ui.Column(ctx).Fill().Children(func() {
		ui.Row(ctx).Padding(20, 24, 12, 24).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(ctx, "日志").FontSize(20).FontWeight(680)
			if warnings > 0 {
				ui.Badge(ctx, fmt.Sprintf("%d 警告", warnings)).TextColor(ctx.Theme().Warning)
			}
			if errors > 0 {
				ui.Badge(ctx, fmt.Sprintf("%d 错误", errors)).Background(ctx.Theme().Danger).TextColor(ui.Hex("#ffffff"))
			}
			ui.Spacer(ctx)
			ui.Text(ctx, "跟随").FontSize(12).TextColor(p.muted)
			if ui.Switch(ctx, &c.FollowLogs).Label("跟随最新日志").Changed() {
				c.LogList.FollowEnd = c.FollowLogs
			}
			iconButton(ctx, "doc.on.doc", iconCopy, 15, p.muted, "复制日志").
				OnClick(func() {
					mygo.Clipboard.WriteText(c.LogText())
					ctx.Toast("日志已复制")
				})
			iconButton(ctx, "trash", iconTrash, 15, p.muted, "清空界面日志").OnClick(c.ClearLog)
		})

		ui.Row(ctx).Padding(0, 24, 14, 24).Gap(8).AlignItems(ui.Center).Children(func() {
			labels := make([]string, len(levelNames))
			index := 0
			for i, option := range levelNames {
				labels[i] = option.label
				if option.value == c.LogLevel {
					index = i
				}
			}
			if ui.Segmented(ctx, &index, labels...).Label("日志级别").Changed() && index >= 0 && index < len(levelNames) {
				c.SetLogLevel(levelNames[index].value)
			}
			ui.SearchField(ctx, &c.LogSearch).Placeholder("搜索消息").Grow(1)
		})

		ui.List(ctx, &c.LogList, len(entries), func(i int) {
			c.viewLogEntry(ctx, entries[i])
		}).Grow(1).Children(func() {
			if len(entries) == 0 {
				ui.Column(ctx).Padding(48, 24).Gap(6).AlignItems(ui.Center).Children(func() {
					symbol(ctx, "doc.text", iconDoc, 28, p.faint)
					ui.Text(ctx, "暂无匹配日志").TextColor(p.muted)
				})
			}
		})
	})
}

func (c *Controller) viewLogEntry(ctx *ui.Context, entry LogEntry) {
	t := ctx.Theme()
	p := paletteFor(t)
	expanded := c.Expanded[entry.ID]
	ui.Column(ctx).Children(func() {
		row := ui.Row(ctx).Padding(9, 24).Gap(10).AlignItems(ui.Center)
		if row.Hovered() {
			row.Background(p.hover)
		}
		row.Children(func() {
			ui.Text(ctx, entry.Time.Format("15:04:05")).FontSize(12).
				TextColor(p.faint).Width(62).Shrink(0)
			ui.Badge(ctx, LevelLabel(entry.Level)).TextColor(levelColor(t, entry.Level)).Shrink(0)
			ui.Text(ctx, entry.Component).FontSize(12).TextColor(p.muted).Width(64).NoWrap().Shrink(0)
			message := ui.Text(ctx, entry.Summary()).FontSize(12).Selectable().Grow(1)
			if entry.HasDetails() {
				message.NoWrap()
			}
			if entry.HasDetails() {
				chevron := "chevron.right"
				if expanded {
					chevron = "chevron.down"
				}
				iconButton(ctx, chevron, nil, 12, p.muted, "展开或收起完整详情").
					OnClick(func() { c.ToggleExpanded(entry.ID) })
			}
		})
		if expanded {
			ui.Text(ctx, entry.Message).FontSize(12).Selectable().
				Padding(12).Background(p.card).Radius(10).
				Margin(0, 24, 10, 24)
		}
		ui.Box(ctx).Height(1).Background(p.hairline).Margin(0, 24)
	})
}

func levelColor(t *ui.Theme, level string) ui.Color {
	switch level {
	case "error":
		return t.Danger
	case "warning":
		return t.Warning
	default:
		return t.TextMuted
	}
}
