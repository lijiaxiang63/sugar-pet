package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The look of the app: a wash of warm paper under the sidebar, the page on
// a card raised off it, soft panels for groups of settings on the card,
// and controls with faces of their own on the panels. Surfaces stand out
// by their color and a soft edge, not by lines around them.

// palette is the app's colors, for the light and the dark appearance.
type palette struct {
	dark bool

	wash  ui.Color // the window, under the sidebar and around the card
	card  ui.Color // the page
	panel ui.Color // a group of rows on the card
	well  ui.Color // the track of a segmented control, an empty drop zone

	text   ui.Color
	muted  ui.Color // descriptions, captions
	faint  ui.Color // hints, counts
	line   ui.Color // dividers
	border ui.Color // subtle hairline borders on panels and cards

	control      ui.Color // the face of buttons, fields, chips
	controlHover ui.Color
	rowHover     ui.Color
	rowSelected  ui.Color // the chosen page of the sidebar

	accent       ui.Color
	accentHover  ui.Color
	accentSoft   ui.Color // behind what is chosen
	accentBorder ui.Color // soft accent ring
	onAccent     ui.Color

	success ui.Color
	warning ui.Color
	danger  ui.Color
	cool    ui.Color // decreases: negative peaks
}

var lightPalette = palette{
	wash:  ui.Hex("#f0eee6"),
	card:  ui.Hex("#fefdfb"),
	panel: ui.RGBA(60, 50, 30, 0.045),
	well:  ui.RGBA(60, 50, 30, 0.07),

	text:   ui.Hex("#1f1e1d"),
	muted:  ui.Hex("#6a6760"),
	faint:  ui.Hex("#9d9990"),
	line:   ui.RGBA(40, 30, 10, 0.08),
	border: ui.RGBA(40, 30, 10, 0.07),

	control:      ui.Hex("#ffffff"),
	controlHover: ui.Hex("#faf8f4"),
	rowHover:     ui.RGBA(60, 50, 30, 0.06),
	rowSelected:  ui.Hex("#ffffff"),

	accent:       ui.Hex("#c96442"),
	accentHover:  ui.Hex("#b95838"),
	accentSoft:   ui.RGBA(201, 100, 66, 0.1),
	accentBorder: ui.RGBA(201, 100, 66, 0.24),
	onAccent:     ui.Hex("#ffffff"),

	success: ui.Hex("#2f7d52"),
	warning: ui.Hex("#b4690e"),
	danger:  ui.Hex("#c4372c"),
	cool:    ui.Hex("#2f6fd6"),
}

var darkPalette = palette{
	dark:  true,
	wash:  ui.Hex("#1b1a19"),
	card:  ui.Hex("#262624"),
	panel: ui.RGBA(255, 250, 240, 0.045),
	well:  ui.RGBA(255, 250, 240, 0.07),

	text:   ui.Hex("#f1efe8"),
	muted:  ui.Hex("#b3b0a6"),
	faint:  ui.Hex("#87847b"),
	line:   ui.RGBA(255, 250, 240, 0.08),
	border: ui.RGBA(255, 250, 240, 0.07),

	control:      ui.Hex("#3b3a37"),
	controlHover: ui.Hex("#454440"),
	rowHover:     ui.RGBA(255, 250, 240, 0.06),
	rowSelected:  ui.RGBA(255, 250, 240, 0.1),

	accent:       ui.Hex("#d97757"),
	accentHover:  ui.Hex("#e08565"),
	accentSoft:   ui.RGBA(217, 119, 87, 0.16),
	accentBorder: ui.RGBA(217, 119, 87, 0.3),
	onAccent:     ui.Hex("#ffffff"),

	success: ui.Hex("#62bd88"),
	warning: ui.Hex("#e2a650"),
	danger:  ui.Hex("#f0756c"),
	cool:    ui.Hex("#6fa6ff"),
}

// paletteFor is the colors of the window c draws, by its appearance.
func paletteFor(c *ui.Context) *palette {
	if c.Theme().Dark {
		return &darkPalette
	}
	return &lightPalette
}

// themeFor is the theme of the widgets that keep MyGo's look (fields,
// drop-downs, tables), in the app's colors.
func themeFor(c *ui.Context, pal *palette) *ui.Theme {
	t := *c.Theme().WithAccent(pal.accent)
	t.Background = pal.card
	t.Surface, t.SurfaceHover = pal.control, pal.controlHover
	t.SurfacePressed = pal.controlHover.Mix(pal.text, 0.06)
	t.Border = pal.text.Alpha(0.14)
	t.Text, t.TextMuted = pal.text, pal.muted
	t.AccentHover, t.AccentText = pal.accentHover, pal.onAccent
	t.Success, t.Warning, t.Danger = pal.success, pal.warning, pal.danger
	t.Scrollbar = pal.text.Alpha(0.22)
	t.Radius = 7
	return &t
}

// Levels. In the light a surface is lifted by a soft shadow and no line; in
// the dark, where shadows hardly show, by a ring a little lighter than the
// surface, brighter along its top, as if lit from above. The rings are
// inner shadows, which take no room, so that nothing moves between the two.

// raised lifts the page's card off the wash.
func raised(e ui.Element, pal *palette) ui.Element {
	if pal.dark {
		return e.InsetShadow(0, 0, 0, 1, pal.text.Alpha(0.07)).InsetShadow(0, 1, 0, 0, pal.text.Alpha(0.06)).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.3)).Shadow(0, 6, 16, -4, ui.RGBA(0, 0, 0, 0.35))
	}
	return e.Shadow(0, 0, 0, 0.5, ui.RGBA(40, 30, 10, 0.1)).Shadow(0, 1, 2, 0, ui.RGBA(40, 30, 10, 0.05)).
		Shadow(0, 4, 14, -2, ui.RGBA(40, 30, 10, 0.06))
}

// face is the face of a control: a button, a chip, the choice of a
// segmented control, the chosen page of the sidebar.
func face(e ui.Element, pal *palette) ui.Element {
	if pal.dark {
		return e.InsetShadow(0, 0, 0, 1, pal.text.Alpha(0.06)).InsetShadow(0, 1, 0, 0, pal.text.Alpha(0.08)).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.28))
	}
	return e.Shadow(0, 0, 0, 0.5, ui.RGBA(40, 30, 10, 0.1)).Shadow(0, 1, 2, 0, ui.RGBA(40, 30, 10, 0.09))
}

var quick = ui.Animation{Duration: 110 * time.Millisecond}

type buttonKind int

const (
	buttonPlain   buttonKind = iota // a face on the panel
	buttonPrimary                   // the accent's: what the page is for
	buttonGhost                     // text alone, tinted under the pointer
	buttonSoft                      // accent-tinted soft action for shortcuts & workflow links
)

// button is the app's button, height DIPs high, with an icon before its
// label when ic is not nil.
func button(c *ui.Context, label string, ic *ui.SVG, kind buttonKind, height float32) ui.Element {
	pal := paletteFor(c)
	size, pad, radius := float32(12.5), float32(10), float32(7)
	if height >= 30 {
		size, pad, radius = 13.5, 16, 9
	}
	if kind == buttonGhost {
		pad = 8
	}
	b := ui.ButtonBase(c).Height(height).Padding(0, pad).Gap(6).Radius(radius).AlignItems(ui.Center).Shrink(0).Label(label)
	bg, fg := pal.control, pal.text
	switch kind {
	case buttonPrimary:
		bg, fg = pal.accent, pal.onAccent
		switch {
		case b.Pressed():
			bg = pal.accentHover.Mix(ui.Hex("#000000"), 0.08)
		case b.Hovered():
			bg = pal.accentHover
		}
		b.Shadow(0, 1, 2, 0, pal.accent.Alpha(0.35)).InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.2))
	case buttonSoft:
		bg, fg = pal.accentSoft, pal.accent
		if b.Hovered() {
			bg = pal.accent.Alpha(0.18)
		}
		if b.Pressed() {
			bg = pal.accent.Alpha(0.24)
		}
		b.InsetShadow(0, 0, 0, 1, pal.accentBorder)
	case buttonGhost:
		bg, fg = ui.Transparent, pal.muted
		if b.Hovered() {
			bg, fg = pal.rowHover, pal.text
		}
		if b.Pressed() {
			bg = pal.rowHover.Mix(pal.text, 0.06)
		}
	default:
		switch {
		case b.Pressed():
			bg = pal.controlHover.Mix(pal.text, 0.05)
		case b.Hovered():
			bg = pal.controlHover
		}
		face(b, pal)
	}
	weight := 500
	if kind == buttonPrimary {
		weight = 600
	}
	b.Background(b.AnimateColor("bg", bg, quick)).TextColor(fg).Children(func() {
		if ic != nil {
			ui.Icon(c, ic).Size(size, size)
		}
		ui.Text(c, label).FontSize(size).FontWeight(weight).SingleLine()
	})
	return b
}

func smallButton(c *ui.Context, label string) ui.Element {
	return button(c, label, nil, buttonPlain, 26)
}

// iconButton is a small round button of an icon alone.
func iconButton(c *ui.Context, ic *ui.SVG, label string) ui.Element {
	pal := paletteFor(c)
	b := ui.ButtonBase(c).Size(20, 20).Radius(10).Center().Shrink(0).TextColor(pal.faint).Label(label)
	if b.Hovered() {
		b.Background(pal.text.Alpha(0.1)).TextColor(pal.text)
	}
	b.Children(func() { ui.Icon(c, ic).Size(10, 10) })
	return b
}

// toggle is the app's switch: a track in the accent when on, with a knob
// that springs across.
func toggle(c *ui.Context, on *bool, label string) ui.Element {
	pal := paletteFor(c)
	sw := ui.SwitchBase(c, on).Size(36, 21).Radius(10.5).Padding(2).Shrink(0).Label(label)
	track := pal.text.Alpha(0.17)
	x := float32(0)
	if *on {
		track, x = pal.accent, 15
	}
	sw.Background(sw.AnimateColor("track", track, ui.Animation{Duration: 150 * time.Millisecond}))
	x = sw.AnimateTo("knob", x, ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: 0.15}})
	sw.Children(func() {
		ui.Box(c).Size(17, 17).Radius(8.5).Background(ui.Hex("#ffffff")).Translate(x, 0).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.22)).Shadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.06))
	})
	return sw
}

// segmented chooses one of labels into *value: a soft track, the choice a
// face raised a hair. It reports a new choice.
func segmented(c *ui.Context, value *int, labels ...string) bool {
	return segmentedIn(c, value, false, labels...)
}

// segmentedIn is segmented, across the width it has when fill is set, its
// segments sharing it.
func segmentedIn(c *ui.Context, value *int, fill bool, labels ...string) bool {
	pal := paletteFor(c)
	was := *value
	seg := ui.SegmentedBase(c, value, len(labels))
	track := seg.Track.Height(28).Padding(2).Radius(8).Background(pal.well).AlignItems(ui.Center)
	if fill {
		track.FillWidth()
	}
	track.Children(func() {
		for n, label := range labels {
			s := seg.Segment(n).FillHeight().Padding(0, 12).Radius(6).Center()
			if fill {
				s.Grow(1).Basis(0).MinWidth(0).Padding(0, 6)
			}
			fg := pal.muted
			if n == *value {
				fg = pal.text
				face(s.Background(pal.control), pal)
				if pal.dark {
					s.Background(ui.RGBA(255, 250, 240, 0.16))
				}
			} else if s.Hovered() {
				fg = pal.text
			}
			s.Children(func() {
				ui.Text(c, label).FontSize(12.5).FontWeight(500).TextColor(fg).SingleLine().Ellipsis("…").MinWidth(0)
			})
		}
	})
	return *value != was
}

// checkTile is a check box as a tile, tinted in the accent while checked.
// extra, when not nil, builds what follows its label.
func checkTile(c *ui.Context, on *bool, label string, extra func()) ui.Element {
	return checkTileWithDesc(c, on, label, "", extra)
}

// checkTileWithDesc is checkTile with an optional short caption under the label.
func checkTileWithDesc(c *ui.Context, on *bool, label, desc string, extra func()) ui.Element {
	pal := paletteFor(c)
	b := ui.CheckboxBase(c, on).Padding(7, 10, 7, 9).Gap(8).Radius(9).AlignItems(ui.Center).Label(label)
	if desc == "" {
		b.Height(34).Padding(0, 10, 0, 9)
	} else {
		b.MinHeight(48)
	}
	bg := pal.control
	switch {
	case *on:
		// The accent's tint on a face, with a ring of the accent.
		bg = pal.control.Mix(pal.accent, 0.09)
		b.InsetShadow(0, 0, 0, 1.25, pal.accent.Alpha(0.55))
	case b.Hovered():
		bg = pal.controlHover
		face(b, pal)
	default:
		face(b, pal)
	}
	b.Background(b.AnimateColor("bg", bg, quick)).Children(func() {
		checkMark(c, *on)
		if desc == "" {
			ui.Text(c, label).FontSize(13).FontWeight(500).SingleLine().Ellipsis("…").MinWidth(0).Grow(1)
			if extra != nil {
				extra()
			}
		} else {
			ui.Column(c).Grow(1).MinWidth(0).Gap(1.5).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, label).FontSize(12.5).FontWeight(600).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
					if extra != nil {
						extra()
					}
				})
				ui.Text(c, desc).FontSize(11).TextColor(pal.muted).SingleLine().Ellipsis("…").MinWidth(0)
			})
		}
	})
	return b
}

// checkMark is the box of a check box: filled in the accent, with a tick,
// while on.
func checkMark(c *ui.Context, on bool) {
	pal := paletteFor(c)
	mark := ui.Box(c).Size(16, 16).Radius(5).Center().Shrink(0)
	if on {
		mark.Background(pal.accent).Children(func() {
			ui.Icon(c, iconTick).Size(10, 10).TextColor(pal.onAccent)
		})
		return
	}
	mark.Background(pal.control).InsetShadow(0, 0, 0, 1.25, pal.text.Alpha(0.26))
}

// check is a check box with its label beside it.
func check(c *ui.Context, on *bool, label string) ui.Element {
	pal := paletteFor(c)
	b := ui.CheckboxBase(c, on).Gap(7).AlignItems(ui.Center).Shrink(0).Label(label)
	b.Children(func() {
		checkMark(c, *on)
		ui.Text(c, label).FontSize(13).TextColor(pal.text).SingleLine()
	})
	return b
}

// pill is a small label on a tint of its color: a count, a mark.
func pill(c *ui.Context, text string, color ui.Color) ui.Element {
	return ui.Text(c, text).FontSize(10.5).FontWeight(600).TextColor(color).
		Padding(1.5, 7).Radius(999).Background(color.Alpha(0.13)).SingleLine().Shrink(0)
}

// callout is a note that stands out from the page: a caution, the
// warnings of a run. ic may be nil.
func callout(c *ui.Context, ic *ui.SVG, color ui.Color, body func()) ui.Element {
	box := ui.Row(c).Radius(12).Clip().Background(color.Alpha(0.085)).InsetShadow(0, 0, 0, 1, color.Alpha(0.18)).AlignItems(ui.Stretch)
	box.Children(func() {
		ui.Box(c).Width(3).Shrink(0).Background(color.Alpha(0.75))
		ui.Row(c).Grow(1).MinWidth(0).Padding(11, 14, 11, 12).Gap(10).AlignItems(ui.Start).Children(func() {
			if ic != nil {
				ui.Icon(c, ic).Size(15, 15).TextColor(color).Shrink(0).Margin(1, 0, 0, 0)
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(body)
		})
	})
	return box
}
