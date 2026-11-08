package app

import (
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// levelNames maps the filter's labels to the levels entries keep.
var levelNames = []struct{ label, value string }{
	{"全部", "all"},
	{"Info", "info"},
	{"Warning", "warning"},
	{"Error", "error"},
}

func (c *Controller) viewLog(ctx *ui.Context) {
	t := ctx.Theme()
	// The list stops following its end when the user scrolls up; mirror that
	// in the checkbox.
	if c.FollowLogs && !c.LogList.FollowEnd {
		c.FollowLogs = false
	}
	entries := c.FilteredEntries()
	c.LogList.Key = func(row int) any { return entries[row].ID }
	warnings, errors := c.Counts()

	ui.Column(ctx).Fill().Background(t.Background).Children(func() {
		ui.Row(ctx).Padding(14).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(ctx, "日志").Bold()
			if warnings > 0 {
				ui.Text(ctx, fmt.Sprintf("⚠ %d", warnings)).FontSize(11).TextColor(t.Warning)
			}
			if errors > 0 {
				ui.Text(ctx, fmt.Sprintf("✕ %d", errors)).FontSize(11).TextColor(t.Danger)
			}
			ui.Spacer(ctx)
			if ui.Checkbox(ctx, &c.FollowLogs, "跟随").Changed() {
				c.LogList.FollowEnd = c.FollowLogs
			}
			ui.Button(ctx.Key("copy-log"), "复制").Label("复制日志").
				OnClick(func() { mygo.Clipboard.WriteText(c.LogText()) })
			ui.Button(ctx.Key("clear-log"), "清空").Label("清空界面日志").OnClick(c.ClearLog)
		})
		ui.Row(ctx).Padding(0, 14, 12, 14).Gap(8).Children(func() {
			level := c.LogLevel
			label := "全部"
			for _, option := range levelNames {
				if option.value == level {
					label = option.label
				}
			}
			chosen := label
			labels := make([]string, len(levelNames))
			for i, option := range levelNames {
				labels[i] = option.label
			}
			if ui.Select(ctx, &chosen, labels).Changed() {
				for _, option := range levelNames {
					if option.label == chosen {
						c.SetLogLevel(option.value)
					}
				}
			}
			ui.TextInput(ctx, &c.LogSearch).Placeholder("搜索消息").Grow(1)
		})
		ui.Divider(ctx)
		ui.List(ctx, &c.LogList, len(entries), func(i int) {
			c.viewLogEntry(ctx, t, entries[i])
		}).Grow(1).Children(func() {
			if len(entries) == 0 {
				ui.Text(ctx, "暂无匹配日志").TextColor(t.TextMuted).Padding(18)
			}
		})
	})
}

func (c *Controller) viewLogEntry(ctx *ui.Context, t *ui.Theme, entry LogEntry) {
	expanded := c.Expanded[entry.ID]
	ui.Column(ctx).Children(func() {
		ui.Row(ctx).Padding(9, 12).Gap(8).AlignItems(ui.Start).Children(func() {
			ui.Text(ctx, entry.Time.Format("15:04:05")).FontSize(11).
				TextColor(t.TextMuted).Width(59)
			ui.Text(ctx, LevelLabel(entry.Level)).FontSize(11).
				TextColor(levelColor(t, entry.Level)).Width(38)
			ui.Text(ctx, entry.Component).FontSize(11).
				TextColor(t.TextMuted).Width(54).NoWrap()
			message := entry.Message
			lineLimit(ui.Text(ctx, message).FontSize(11).Selectable().Grow(1), entry)
			if entry.HasDetails() {
				chevron := "▸"
				if expanded {
					chevron = "▾"
				}
				ui.Button(ctx.Key(entry.ID), chevron).
					Label("展开或收起完整详情").
					Tooltip("展开或收起完整详情").
					OnClick(func() { c.ToggleExpanded(entry.ID) })
			}
		})
		if expanded {
			ui.Text(ctx, entry.Message).FontSize(11).Selectable().
				Padding(10).Background(t.Surface).Radius(6).
				Margin(0, 12, 9, 12)
		}
		ui.Divider(ctx)
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

// lineLimit keeps a collapsed entry to its first line.
func lineLimit(element ui.Element, entry LogEntry) ui.Element {
	if entry.HasDetails() {
		return element.NoWrap()
	}
	return element
}
