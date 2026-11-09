package app

import (
	"time"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
)

// palette is the app's own colors, for the light and the dark appearance:
// a near-black content well, a slightly lifted sidebar and Tongji blue as
// the one accent. Red is kept for errors, through the theme's Danger.
type palette struct {
	dark     bool
	bg       ui.Color
	sidebar  ui.Color
	text     ui.Color
	muted    ui.Color
	faint    ui.Color
	hairline ui.Color
	// divider is the line between the sidebar's bands: firmer than a hairline,
	// because a sidebar with no visible breaks reads as one crowded stack.
	divider  ui.Color
	hover    ui.Color
	press    ui.Color
	accent   ui.Color
	onAccent ui.Color
	down     ui.Color
	up       ui.Color
	card     ui.Color
}

func paletteFor(t *ui.Theme) palette {
	if t.Dark {
		return palette{
			dark:     true,
			bg:       ui.Hex("#141416"),
			sidebar:  ui.Hex("#1c1c1f"),
			text:     ui.Hex("#f5f5f7"),
			muted:    ui.RGBA(235, 235, 245, 0.58),
			faint:    ui.RGBA(235, 235, 245, 0.32),
			hairline: ui.RGBA(255, 255, 255, 0.10),
			divider:  ui.RGBA(255, 255, 255, 0.14),
			hover:    ui.RGBA(255, 255, 255, 0.06),
			press:    ui.RGBA(255, 255, 255, 0.11),
			accent:   ui.Oklch(0.62, 0.13, 258),
			onAccent: ui.Hex("#ffffff"),
			down:     ui.Oklch(0.74, 0.13, 255),
			up:       ui.Oklch(0.78, 0.12, 168),
			card:     ui.RGBA(255, 255, 255, 0.045),
		}
	}
	return palette{
		bg:       ui.Hex("#ffffff"),
		sidebar:  ui.Hex("#f4f4f6"),
		text:     ui.Hex("#1d1d1f"),
		muted:    ui.RGBA(60, 60, 67, 0.62),
		faint:    ui.RGBA(60, 60, 67, 0.34),
		hairline: ui.RGBA(0, 0, 0, 0.10),
		divider:  ui.RGBA(0, 0, 0, 0.14),
		hover:    ui.RGBA(0, 0, 0, 0.045),
		press:    ui.RGBA(0, 0, 0, 0.09),
		accent:   ui.Oklch(0.52, 0.13, 258),
		onAccent: ui.Hex("#ffffff"),
		down:     ui.Oklch(0.56, 0.14, 255),
		up:       ui.Oklch(0.60, 0.12, 168),
		card:     ui.RGBA(0, 0, 0, 0.035),
	}
}

// applyTheme makes the window's theme the app's: the accent, and the hover
// and pressed shades primary buttons would otherwise keep as MyGo's blue.
func applyTheme(c *ui.Context) palette {
	t := *c.Theme()
	p := paletteFor(&t)
	black, white := ui.Hex("#000000"), ui.Hex("#ffffff")
	t.Accent, t.AccentText = p.accent, p.onAccent
	t.Success, t.Warning, t.Danger = ui.Hex("#16a34a"), ui.Hex("#d97706"), ui.Hex("#dc2626")
	t.AccentHover, t.AccentPressed = p.accent.MixOklab(black, 0.1), p.accent.MixOklab(black, 0.2)
	if p.dark {
		t.AccentHover, t.AccentPressed = p.accent.MixOklab(white, 0.15), p.accent.MixOklab(black, 0.15)
	}
	t.Selection = p.accent.Alpha(0.28)
	t.InactiveAccent = p.accent
	// Dividers take the theme's border colour, so setting it here is what lets
	// the hairlines in the sidebar and the log stand a chance of being seen.
	t.Border = p.hairline
	c.SetTheme(&t)
	return p
}

// symbol draws an SF Symbol, falling back to svg on Windows and wherever the
// system has no symbol of that name.
func symbol(c *ui.Context, name string, svg *ui.SVG, size float32, color ui.Color) ui.Element {
	return sfsymbols.Symbol(c, name).Fallback(svg).Size(size).Color(color).Weight(sfsymbols.Medium).Element
}

// iconButton is a borderless round button of one icon, with a soft disc
// under the pointer and a small spring as it is pressed.
func iconButton(c *ui.Context, name string, svg *ui.SVG, size float32, color ui.Color, label string) ui.Element {
	b := ui.ButtonBase(c).Size(size+16, size+16).Center().Radius(999).Label(label).TextColor(color)
	bg := ui.Transparent
	if b.Hovered() {
		bg = ui.RGBA(128, 128, 128, 0.16)
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.9
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() { symbol(c, name, svg, size, color) })
	return b
}

// pill is a rounded button with an icon and a label, filled with bg.
func pill(c *ui.Context, name string, svg *ui.SVG, label string, bg, fg ui.Color) ui.Element {
	b := ui.ButtonBase(c).Height(38).PaddingX(18).Gap(8).Radius(999).AlignItems(ui.Center).TextColor(fg).Label(label)
	fill := bg
	if b.Hovered() {
		fill = bg.Mix(ui.RGB(255, 255, 255), 0.12)
	}
	if b.Pressed() {
		fill = bg.Mix(ui.Hex("#000000"), 0.12)
	}
	b.Background(b.AnimateColor("bg", fill, ui.Animation{Duration: 120 * time.Millisecond}))
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.97
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() {
		symbol(c, name, svg, 15, fg)
		ui.Text(c, label).FontSize(14).FontWeight(600).SingleLine()
	})
	return b
}
