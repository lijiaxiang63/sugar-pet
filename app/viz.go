package main

import (
	"fmt"
	"image"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"sugarpet/core"
)

// What the results of a page show besides their tables: pictures of the
// brain, drawn in the background from the files a run wrote, and charts.

// picture is a picture of the brain in the results of a page.  It is of
// one thing at a time, named by key: when the page asks for another, it is
// drawn anew.  It is also drawn anew after a run, which may have written
// other things under the same names: forget says so.
type picture struct {
	key     string
	run     int // counts the runs: what was drawn of an earlier one is dropped
	bitmap  *ui.Bitmap
	err     string
	loading bool
}

// forget drops the picture, and the one being drawn: the files it is of
// may have been written anew.
func (pic *picture) forget() {
	*pic = picture{run: pic.run + 1}
}

// pictureOf is the picture of a page's results, nil for a page without one.
func (a *app) pictureOf(page string) *picture {
	switch page {
	case "spatial":
		return &a.spatial.pic
	case "zmap":
		return &a.zmap.pic
	case "classify":
		return &a.classify.pic
	case "compare":
		return &a.cmp.pic
	case "cluster":
		return &a.cluster.pic
	case "quick":
		return &a.quick.mapPic
	}
	return nil
}

// show asks for the picture of key, which draw makes in the background.
func (a *app) show(pic *picture, key string, draw func() (*image.RGBA, error)) {
	if pic.key == key {
		return
	}
	pic.key, pic.err, pic.loading = key, "", true
	run := pic.run
	if a.win == nil && a.post == nil {
		pic.loading = false // nothing to draw it for, as in the snapshots
		return
	}
	go func() {
		var img *image.RGBA
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = errorString(toString(r))
				}
			}()
			img, err = draw()
		}()
		a.update(func() {
			if pic.key != key || pic.run != run {
				return // another was asked for meanwhile, or the files were written anew
			}
			pic.loading = false
			if err != nil {
				pic.bitmap, pic.err = nil, err.Error()
				return
			}
			pic.bitmap = ui.NewBitmap(img)
		})
	}()
}

// set shows a picture made already.
func (pic *picture) set(key string, img *image.RGBA) {
	pic.key, pic.err, pic.loading, pic.bitmap = key, "", false, ui.NewBitmap(img)
}

// legend says what the colours of a picture mean.
type legend struct {
	kind       int    // legendScores, legendMask, legendOutline or 0
	unit       string // of scores: "z", "W", "T"
	from, full string // the |score| where the colours start and where they saturate
	text       string // of a mask or an outline
}

const (
	legendScores = iota + 1
	legendMask
	legendOutline
)

// figure is a picture of the brain on the page, with what it shows under
// it: a title, and what its colours mean.
func (a *app) figure(c *ui.Context, pic *picture, title string, lg legend) {
	dim, faint := ui.Hex("#e8e6e0"), ui.Hex("#a8a59c")
	box := ui.Column(c).Radius(12).Clip().Background(ui.Hex("#000000")).InsetShadow(0, 0, 0, 1, ui.RGBA(255, 255, 255, 0.08))
	box.Children(func() {
		switch {
		case pic.bitmap != nil:
			w, h := pic.bitmap.Size()
			img := ui.Image(c, pic.bitmap).FillWidth().AspectRatio(float32(w) / float32(h)).Fit(ui.Contain).Label(title)
			if pic.loading {
				img.Opacity(0.45)
			}
		default:
			ui.Column(c).FillWidth().AspectRatio(2.13).Center().Gap(8).Children(func() {
				switch {
				case pic.loading:
					ui.Spinner(c).Size(18, 18)
					ui.Text(c, T("Drawing the picture…", "正在绘制图像…")).FontSize(12).TextColor(faint)
				case pic.err != "":
					ui.Text(c, T("The picture cannot be drawn: ", "无法绘制图像：")+pic.err).FontSize(12).TextColor(faint).
						TextAlign(ui.Center).Padding(0, 24).Selectable()
				}
			})
		}
		ui.Row(c).Padding(9, 14, 10, 14).Gap(7).AlignItems(ui.Center).Background(ui.Hex("#141414")).Children(func() {
			ui.Text(c, title).FontSize(12).FontWeight(500).TextColor(dim).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			caption := func(text string) {
				ui.Text(c, text).FontSize(11.5).TextColor(faint).SingleLine().Shrink(0)
			}
			swatch := func(from, to string) {
				ui.Box(c).Height(8).Width(40).Radius(4).Shrink(0).LinearGradient(ui.LinearGradient{From: ui.Hex(from), To: ui.Hex(to), Angle: 90})
			}
			switch lg.kind {
			case legendScores:
				caption(lg.unit + " −" + lg.full)
				swatch("#00ffff", "#0000ff")
				if lg.from == "0" {
					caption("0")
				} else {
					caption("−" + lg.from)
					ui.Box(c).Width(4)
					caption("+" + lg.from)
				}
				swatch("#ff0000", "#ffff00")
				caption("+" + lg.full)
			case legendMask:
				ui.Box(c).Size(10, 10).Radius(3).Background(ui.Hex("#e8784e")).Shrink(0)
				caption(lg.text)
			case legendOutline:
				ui.Box(c).Size(10, 10).Radius(3).Border(1.5, ui.Hex("#5ed6a6")).Shrink(0)
				caption(lg.text)
			}
			ui.Box(c).Width(6)
			caption(T("R, L: the patient's sides", "R、L 为患者的右、左侧"))
		})
	})
}

// picker chooses which of the results the picture is of, when there are
// several: the one before, the one after.
func picker(c *ui.Context, key string, at *int, n int, name string) {
	pal := paletteFor(c)
	if n < 2 {
		return
	}
	*at = min(max(*at, 0), n-1)
	ui.Row(c.Key(key)).Padding(6, 12, 6, 8).Radius(10).Gap(6).AlignItems(ui.Center).
		Background(pal.panel).InsetShadow(0, 0, 0, 1, pal.border).Children(func() {
		prev := iconButton(c.Key("prev"), iconChevron, T("Previous", "上一个")).Disabled(*at == 0)
		prev.Rotate(180)
		if prev.Clicked() && *at > 0 {
			*at--
		}
		ui.Text(c, strconv.Itoa(*at+1)+" / "+strconv.Itoa(n)).FontSize(12).FontWeight(600).TextColor(pal.muted).SingleLine().Width(52).TextAlign(ui.Center)
		next := iconButton(c.Key("next"), iconChevron, T("Next", "下一个")).Disabled(*at == n-1)
		if next.Clicked() && *at < n-1 {
			*at++
		}
		ui.Box(c).Size(1, 14).Background(pal.line).Margin(0, 4).Shrink(0)
		ui.Text(c, name).FontSize(12.5).FontWeight(500).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
	})
}

// seriesColors tell the series of a chart apart, in the light and in the
// dark.
func seriesColors(pal *palette) []ui.Color {
	if pal.dark {
		return []ui.Color{pal.accent, pal.cool, ui.Hex("#62bd88"), ui.Hex("#e2a650"), ui.Hex("#b594e8"), ui.Hex("#5cc4c9"), ui.Hex("#e58bb3")}
	}
	return []ui.Color{pal.accent, pal.cool, ui.Hex("#2f7d52"), ui.Hex("#b4690e"), ui.Hex("#7a52c7"), ui.Hex("#1f8a90"), ui.Hex("#c2447e")}
}

// factorChart draws the factors of a run: one line per method, one place
// along the bottom per image.
func factorChart(c *ui.Context, rows []normRow) {
	pal := paletteFor(c)
	var images, methods []string
	at := map[string]int{}
	values := map[string][]float64{} // by method, one per image; NaN where it failed
	for _, r := range rows {
		if _, ok := at[r.image]; !ok {
			at[r.image] = len(images)
			images = append(images, r.image)
		}
		if _, ok := values[r.method]; !ok {
			methods = append(methods, r.method)
			values[r.method] = nil
		}
	}
	lo, hi := 0.0, 0.0
	first := true
	for _, r := range rows {
		v := values[r.method]
		for len(v) < len(images) {
			v = append(v, nan)
		}
		v[at[r.image]] = r.value
		values[r.method] = v
		if first || r.value < lo {
			lo = r.value
		}
		if first || r.value > hi {
			hi = r.value
		}
		first = false
	}
	if len(images) < 2 || first {
		return
	}
	if hi == lo {
		lo, hi = lo*0.95, hi*1.05
	}
	pad := (hi - lo) * 0.12
	x := chart.NewBand(images...).Padding(0.2, 0.1)
	y := chart.NewLinear(lo-pad, hi+pad).Nice(4)
	ticks := y.Ticks(4)
	colors := seriesColors(pal)
	label := func(side chart.Side, tick chart.Tick) {
		ui.Text(c, tick.Label).NoWrap().FontSize(11).TextColor(pal.muted)
	}
	// Along the bottom the images are numbered: their names are in the table.
	var bottom []chart.Tick
	step := max(1, (len(images)+9)/10)
	for i := 0; i < len(images); i += step {
		bottom = append(bottom, chart.Tick{At: x.At(i), Label: strconv.Itoa(i + 1)})
	}
	card(c).Padding(14, 16, 12, 12).Gap(10).Children(func() {
		ui.Row(c).Gap(14).GapY(4).Wrap().AlignItems(ui.Center).Padding(0, 4).Children(func() {
			ui.Text(c, T("Normalisation factor of each image", "各图像的归一化因子")).FontSize(12.5).FontWeight(600).Grow(1)
			for i, m := range methods {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(colors[i%len(colors)])
					ui.Text(c, methodName(m)).FontSize(11.5).TextColor(pal.muted).SingleLine()
				})
			}
		})
		chart.Plot(c, chart.Axes{Left: ticks, Bottom: bottom, Label: label}, func(area ui.Element) {
			area.Draw(func(p *ui.Painter, r ui.Rect) {
				for _, tick := range ticks {
					p.Line(r.X, chart.Y(r, tick.At), r.X+r.W, chart.Y(r, tick.At), 1, pal.line)
				}
				for i, m := range methods {
					xs, ys := make([]float32, len(images)), make([]float32, len(images))
					for n, v := range values[m] {
						xs[n], ys[n] = x.At(n), y.At(v)
					}
					p.StrokePath(chart.Line(r, xs, ys, chart.CurveLinear), 1.5, colors[i%len(colors)].Alpha(0.55))
				}
			})
			for i, m := range methods {
				for n, v := range values[m] {
					if v != v {
						continue
					}
					chart.Point(c, x.At(n), y.At(v), 7, 7).Radius(3.5).Background(colors[i%len(colors)]).
						Tooltip(fmt.Sprintf("%s · %s: %s", images[n], methodName(m), num(v)))
				}
			}
		}).Label(T("Normalisation factor of each image", "各图像的归一化因子")).Height(170).Gap(6)
	})
}

var nan = func() float64 { var zero float64; return zero / zero }()

// scoreBars shows how well a patient matches each pattern, 0 to 100.
func scoreBars(c *ui.Context, scores []patternScore, leading string) {
	pal := paletteFor(c)
	ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
		for _, s := range scores {
			color := pal.text.Alpha(0.28)
			if s.id == leading {
				color = pal.accent
			}
			ui.Column(c).Gap(3).AlignItems(ui.Center).Width(34).Shrink(0).Tooltip(fmt.Sprintf("%s: %d/100", core.ReportText("name-"+s.id, uiLang()), s.percent)).Children(func() {
				ui.Column(c).Height(26).Width(14).Radius(3).Justify(ui.End).Background(pal.well).Clip().Children(func() {
					ui.Box(c).FillWidth().Height(26 * float32(s.percent) / 100).Background(color)
				})
				ui.Text(c, patternShort(s.id)).FontSize(9.5).FontWeight(600).TextColor(pal.muted).SingleLine()
			})
		}
	})
}

type patternScore struct {
	id      string
	percent int
}

// patternShort is the abbreviation of a pattern, as under its bar.
func patternShort(id string) string {
	return map[string]string{"msa-p": "MSA-P", "msa-c": "MSA-C", "psp": "PSP", "cbd": "CBD", "dlb": "DLB", "pd": "PD"}[id]
}

// Pictures of the results, each from the files a run wrote.

// imagePicture is an image in gray, with the outline of the template's
// brain over it when outline is set: how well a warp went.
func imagePicture(path string, outline bool) func() (*image.RGBA, error) {
	return func() (*image.RGBA, error) {
		img, err := core.LoadImage(path)
		if err != nil {
			return nil, err
		}
		return core.Picture(img, nil, core.PictureOptions{Outline: outline}), nil
	}
}

// mapPicture is a map of scores over the image it was made of.
func mapPicture(imagePath, mapPath string) func() (*image.RGBA, error) {
	return func() (*image.RGBA, error) {
		img, err := core.LoadImage(imagePath)
		if err != nil {
			return nil, err
		}
		scores, err := core.LoadImage(mapPath)
		if err != nil {
			return nil, err
		}
		return core.Picture(img, scores, core.PictureOptions{}), nil
	}
}

// overlayPicture is a map or a mask over the template.
func overlayPicture(path string, opt core.PictureOptions) func() (*image.RGBA, error) {
	return func() (*image.RGBA, error) {
		overlay, err := core.LoadImage(path)
		if err != nil {
			return nil, err
		}
		return core.Picture(nil, overlay, opt), nil
	}
}

// referencePicture is the mean of the controls, as they were summarised.
func referencePicture(folder string) func() (*image.RGBA, error) {
	return imagePicture(filepath.Join(folder, "template.nii.gz"), false)
}
