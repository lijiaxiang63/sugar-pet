package core

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The clinical report of a classification: one HTML page that prints well.

// ReportLanguages are the languages a report can be written in.
var ReportLanguages = []string{"en", "zh"}

// Axial slices of the figure, as indices into the 2 mm atlas grid:
// cerebellum and pons, midbrain, basal ganglia and thalamus, up to the
// central region.
var figureSlices = []int{20, 26, 30, 36, 40, 44, 50, 56, 62, 68}

const (
	figureColumns = 5
	figureZoom    = 2   // screen pixels per voxel
	figureGap     = 6   // pixels between slices
	figureWhite   = 1.5 // normalised uptake shown as white
	figureFrom    = 2.0 // score at which the colour starts
	figureFull    = 5.0 // score at which it saturates
	figureLetter  = 200 // gray level of the letters
)

// 5 x 7 letters marking the sides of each slice.
var figureGlyphs = map[byte][7]string{
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
}

var reportTexts = sync.OnceValue(func() map[string]map[string]string {
	raw, err := dataFS.ReadFile("data/classify_text.json")
	if err != nil {
		panic(err)
	}
	var out map[string]map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
})

var reportCSS = sync.OnceValue(func() string {
	raw, err := dataFS.ReadFile("data/report.css")
	if err != nil {
		panic(err)
	}
	return string(raw)
})

// ReportText returns a sentence of the report in the given language, with
// {0}, {1}, ... filled in.
func ReportText(key, lang string, args ...string) string {
	out := reportTexts()[key][lang]
	for i, arg := range args {
		out = strings.ReplaceAll(out, "{"+itoa(i)+"}", arg)
	}
	return out
}

func escapeHTML(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(value)
}

// ScoreText prints a score as the report does: fixed decimals and a real
// minus sign; "–" for a missing one.
func ScoreText(value *float64, digits int) string {
	if value == nil {
		return "–"
	}
	out := strconv.FormatFloat(*value, 'f', digits, 64)
	if parsed, _ := strconv.ParseFloat(out, 64); parsed == 0 {
		out = strings.TrimLeft(out, "-")
	}
	return strings.ReplaceAll(out, "-", "−")
}

// Percent rounds a share (0 to 1) to whole percent.
func Percent(value float64) int { return int(math.Floor(100.0*value + 0.5)) }

// EncodePNG encodes rows x columns pixels of three bytes each as a PNG file.
func EncodePNG(rgb []byte, rows, columns int) []byte {
	raw := make([]byte, 0, rows*(1+3*columns))
	for r := 0; r < rows; r++ {
		raw = append(raw, 0)
		raw = append(raw, rgb[r*3*columns:(r+1)*3*columns]...)
	}
	var packed bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&packed, zlib.BestCompression)
	zw.Write(raw)
	zw.Close()

	var out bytes.Buffer
	chunk := func(tag string, data []byte) {
		binary.Write(&out, binary.BigEndian, uint32(len(data)))
		body := append([]byte(tag), data...)
		out.Write(body)
		binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(body))
	}
	out.WriteString("\x89PNG\r\n\x1a\n")
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[0:], uint32(columns))
	binary.BigEndian.PutUint32(header[4:], uint32(rows))
	header[8], header[9] = 8, 2 // 8 bits per sample, RGB
	chunk("IHDR", header)
	chunk("IDAT", packed.Bytes())
	chunk("IEND", nil)
	return out.Bytes()
}

// finite is numpy's nan_to_num.
func finite(data []float32) []float32 {
	out := make([]float32, len(data))
	for i, v := range data {
		switch f := float64(v); {
		case math.IsNaN(f):
		case math.IsInf(f, 1):
			out[i] = math.MaxFloat32
		case math.IsInf(f, -1):
			out[i] = -math.MaxFloat32
		default:
			out[i] = v
		}
	}
	return out
}

// Montage draws axial slices of the image in gray with the abnormal scores
// in colour.  It returns the picture (three bytes per pixel), its size, and
// the MNI z of each slice.  Slices are in radiological convention: the
// patient's left is on the right.
func Montage(image, zmap *Volume) (rgb []byte, rows, columns int, levels []float64) {
	a := atlasOnce()
	uptake := finite(resampleTo(image.Data[:image.NVox()], image.Dim, image.Aff, a.dim, a.aff))
	score := finite(resampleTo(zmap.Data[:zmap.NVox()], zmap.Dim, zmap.Aff, a.dim, a.aff))
	nx, ny := a.dim[0], a.dim[1]
	width, height := nx*figureZoom, ny*figureZoom
	nRows := (len(figureSlices) + figureColumns - 1) / figureColumns
	rows = nRows*height + (nRows-1)*figureGap
	columns = figureColumns*width + (figureColumns-1)*figureGap
	rgb = make([]byte, rows*columns*3)
	level := func(v float64) byte { return byte(math.Floor(clamp01(v)*255.0 + 0.5)) }
	for n, k := range figureSlices {
		y0, x0 := (n/figureColumns)*(height+figureGap), (n%figureColumns)*(width+figureGap)
		set := func(r, c int, red, green, blue byte) {
			p := ((y0+r)*columns + x0 + c) * 3
			rgb[p], rgb[p+1], rgb[p+2] = red, green, blue
		}
		// Rows from anterior to posterior, columns from the patient's right to left.
		for r := 0; r < height; r++ {
			for c := 0; c < width; c++ {
				v := (nx - 1 - c/figureZoom) + nx*((ny-1-r/figureZoom)+ny*k)
				u, s := float64(uptake[v]), float64(score[v])
				gray := level(u / figureWhite)
				depth := level((math.Abs(s) - figureFrom) / (figureFull - figureFrom))
				switch {
				case s >= figureFrom:
					set(r, c, 255, depth, 0)
				case s <= -figureFrom:
					set(r, c, 0, depth, 255)
				default:
					set(r, c, gray, gray, gray)
				}
			}
		}
		top := height/2 - 7*figureZoom/2
		for _, mark := range []struct {
			letter byte
			left   int
		}{{'R', 4}, {'L', width - 4 - 5*figureZoom}} {
			for r, line := range figureGlyphs[mark.letter] {
				for c := range line {
					if line[c] != '1' {
						continue
					}
					for dy := 0; dy < figureZoom; dy++ {
						for dx := 0; dx < figureZoom; dx++ {
							set(top+r*figureZoom+dy, mark.left+c*figureZoom+dx, figureLetter, figureLetter, figureLetter)
						}
					}
				}
			}
		}
		levels = append(levels, a.aff[2][2]*float64(k)+a.aff[2][3])
	}
	return rgb, rows, columns, levels
}

func regionName(name, lang string) string { return ReportText("region-"+name, lang) }

// RegionPlace names a region with its side, as it stands inside a sentence.
func RegionPlace(name, side string, paired bool, lang string) string {
	region := strings.ToLower(regionName(name, lang))
	if paired {
		return ReportText("of-"+side, lang, region)
	}
	return region
}

func methodText(name, lang string) string {
	if entry, ok := reportTexts()["method-"+name]; ok {
		return entry[lang]
	}
	return name
}

// regionItems lists the regions whose status is wanted, most abnormal
// first, as one line of text.
func regionItems(regions []RegionEntry, wanted func(*RegionScore) bool, lang string) string {
	type item struct {
		size float64
		line string
	}
	var found []item
	star := func(robust bool) string {
		if robust {
			return ""
		}
		return "*"
	}
	for i := range regions {
		entry := &regions[i]
		name := strings.ToLower(regionName(entry.ID, lang))
		if !entry.Paired {
			if wanted(entry.Both) {
				line := ReportText("item-mid", lang, name, ScoreText(entry.Both.Z, 1)) + star(entry.Both.Robust)
				found = append(found, item{math.Abs(*entry.Both.Z), line})
			}
			continue
		}
		left, right := wanted(entry.Left), wanted(entry.Right)
		switch {
		case left && right:
			line := ReportText("item-both", lang, name, ScoreText(entry.Left.Z, 1), ScoreText(entry.Right.Z, 1))
			size := math.Max(math.Abs(*entry.Left.Z), math.Abs(*entry.Right.Z))
			found = append(found, item{size, line + star(entry.Left.Robust && entry.Right.Robust)})
		case left:
			line := ReportText("item-left", lang, name, ScoreText(entry.Left.Z, 1))
			found = append(found, item{math.Abs(*entry.Left.Z), line + star(entry.Left.Robust)})
		case right:
			line := ReportText("item-right", lang, name, ScoreText(entry.Right.Z, 1))
			found = append(found, item{math.Abs(*entry.Right.Z), line + star(entry.Right.Robust)})
		}
	}
	if len(found) == 0 {
		return ReportText("none", lang)
	}
	// Stable: equal scores stay in the order of the table.
	sort.SliceStable(found, func(i, j int) bool { return found[i].size > found[j].size })
	lines := make([]string, len(found))
	for i, f := range found {
		lines[i] = f.line
	}
	return strings.Join(lines, ReportText("sep", lang))
}

func featureChip(f Feature, paired map[string]bool, lang string) string {
	z := ScoreText(f.Z, 1)
	var label string
	switch f.Direction {
	case "asymmetry":
		label = ReportText("chip-asymmetry", lang, z)
	case "island":
		label = ReportText("chip-island", lang, z)
	case "spared":
		label = ReportText("chip-spared", lang)
	default:
		place := regionName(f.Region, lang)
		if f.Side != "both" {
			place = RegionPlace(f.Region, f.Side, paired[f.Region], lang)
		}
		arrow := "↓"
		if f.Direction == "hyper" {
			arrow = "↑"
		}
		label = place + " " + arrow + " " + z
	}
	level := "off"
	if f.Evidence >= 0.5 {
		level = "on"
	} else if f.Evidence > 0 {
		level = "part"
	}
	return `<span class="chip ` + level + `">` + escapeHTML(label) + "</span>"
}

// regionFinding is the last column of the table of regions.
func regionFinding(entry *RegionEntry, lang string) string {
	star := func(robust bool) string {
		if robust {
			return ""
		}
		return " *"
	}
	if !entry.Paired {
		if entry.Both.Status == "normal" {
			return ""
		}
		return ReportText("status-"+entry.Both.Status, lang) + star(entry.Both.Robust)
	}
	var parts []string
	if entry.Left.Status == entry.Right.Status {
		if entry.Left.Status != "normal" {
			parts = append(parts, ReportText("finding-side", lang, ReportText("status-"+entry.Left.Status, lang),
				ReportText("side-both", lang))+star(entry.Left.Robust && entry.Right.Robust))
		}
	} else {
		for _, side := range []string{"left", "right"} {
			if found := entry.Side(side); found.Status != "normal" {
				parts = append(parts, ReportText("finding-side", lang, ReportText("status-"+found.Status, lang),
					ReportText("side-"+side, lang))+star(found.Robust))
			}
		}
	}
	if d := entry.Difference; d != nil && math.Abs(*d) >= 1.5 && len(parts) > 0 {
		parts = append(parts, ReportText("finding-asymmetric", lang))
	}
	return strings.Join(parts, "; ")
}

// Report returns the report of the classification as an HTML page, in
// English ("en") or Chinese ("zh").
func (c *Classification) Report(lang string) (string, error) {
	if !contains(ReportLanguages, lang) {
		return "", errorf("the report is available in %s, not '%s'", strings.Join(ReportLanguages, " and "), lang)
	}
	t := func(key string, args ...string) string { return ReportText(key, lang, args...) }
	e := escapeHTML
	byID := map[string]Pattern{}
	for _, p := range c.Patterns {
		byID[p.ID] = p
	}
	paired := map[string]bool{}
	for _, entry := range c.Regions {
		paired[entry.ID] = entry.Paired
	}
	sep := t("sep")
	var out []string
	add := func(line string) { out = append(out, line) }

	add("<!doctype html>")
	add(`<html lang="` + lang + `">`)
	add("<head>")
	add(`<meta charset="utf-8">`)
	add(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	add("<title>" + e(t("title")) + ": " + e(c.Image) + "</title>")
	add("<style>\n" + reportCSS() + "</style>")
	add("</head>")
	add("<body>")
	add("<main>")
	add("<h1>" + e(t("title")) + "</h1>")
	add(`<p class="subtitle">` + e(t("subtitle")) + "</p>")

	// What was analysed, and how.
	names := make([]string, len(c.Checks))
	for i, m := range c.Checks {
		names[i] = methodText(m, lang)
	}
	how := t("meta-method-alone", methodText(c.Method, lang))
	if len(names) > 0 {
		how = t("meta-method-checked", methodText(c.Method, lang), strings.Join(names, sep))
	}
	scores := t("meta-score-z")
	if c.Score == "w" {
		scores = t("meta-score-w", strings.Join(c.Reference.Covariates, sep))
	}
	rows := [][2]string{{t("meta-image"), c.Image}, {t("meta-date"), time.Now().Format("2006-01-02")}}
	if len(c.Covariates) > 0 {
		var values []string
		for _, name := range c.Reference.Covariates {
			values = append(values, name+" "+c.Covariates[name])
		}
		rows = append(rows, [2]string{t("meta-patient"), strings.Join(values, sep)})
	}
	rows = append(rows, [2]string{t("meta-reference"), t("meta-reference-value", itoa(c.Reference.NControls), fmtG(c.Reference.FWHM))})
	rows = append(rows, [2]string{t("meta-method"), how}, [2]string{t("meta-score"), scores})
	rows = append(rows, [2]string{t("meta-software"), "sugar-pet " + c.Version})
	add(`<table class="meta">`)
	for _, row := range rows {
		add("<tr><th>" + e(row[0]) + "</th><td>" + e(row[1]) + "</td></tr>")
	}
	add("</table>")

	// Impression.
	add("<h2>" + e(t("section-impression")) + "</h2>")
	category, chosen := c.Conclusion.Category, c.Conclusion.Pattern
	sentence := t("concl-none")
	if category != "none" {
		top := byID[chosen]
		name, match := e(t("name-"+chosen)), itoa(Percent(top.Score))
		switch category {
		case "indeterminate":
			other := byID[c.Conclusion.Alternative]
			sentence = t("concl-indeterminate", name, match, e(t("name-"+other.ID)), itoa(Percent(other.Score)))
		case "possible":
			sentence = t("concl-possible", name, match)
		default:
			sentence = t("concl-"+category, name, match, t("confidence-"+c.Conclusion.Confidence))
		}
	}
	add(`<p class="conclusion">` + sentence + "</p>")
	add(`<ul class="findings">`)
	is := func(status string) func(*RegionScore) bool {
		return func(v *RegionScore) bool { return v.Status == status }
	}
	add("<li>" + e(t("find-hypo")) + ": " + e(regionItems(c.Regions, is("hypo"), lang)) + "</li>")
	add("<li>" + e(t("find-mild")) + ": " + e(regionItems(c.Regions, is("mild-hypo"), lang)) + "</li>")
	hyper := regionItems(c.Regions, func(v *RegionScore) bool { return v.Status == "hyper" && v.Robust }, lang)
	add("<li>" + e(t("find-hyper")) + ": " + e(hyper) + "</li>")
	add("<li>" + e(t("find-brain", itoa(Percent(c.Brain.Below)), itoa(Percent(c.Brain.Above)))) + "</li>")
	add("</ul>")
	if (category == "pattern" || category == "possible") && c.Conclusion.Alternative != "" {
		other := byID[c.Conclusion.Alternative]
		add("<p>" + e(t("concl-alternative", t("name-"+other.ID), itoa(Percent(other.Score)))) + "</p>")
	}
	if c.Conclusion.Side != "" {
		add("<p>" + e(t("concl-side", t("side-"+c.Conclusion.Side))) + "</p>")
	}
	caveats := []string{chosen}
	if category == "indeterminate" {
		caveats = append(caveats, c.Conclusion.Alternative)
	}
	for _, name := range caveats {
		if name != "" {
			add(`<p class="caveat">` + e(t("caveat-"+name)) + "</p>")
		}
	}

	// Pattern match.
	add("<h2>" + e(t("section-patterns")) + "</h2>")
	add(`<p class="lead">` + e(t("patterns-lead")) + "</p>")
	sorted := append([]Pattern(nil), c.Patterns...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })
	for _, p := range sorted {
		match := itoa(Percent(p.Score))
		class := "pattern"
		if p.ID == chosen {
			class += " chosen"
		}
		add(`<div class="` + class + `">`)
		add(`<div class="name">` + e(t("name-"+p.ID)) + "</div>")
		add(`<div class="bar"><span style="width:` + match + `%"></span></div>`)
		add(`<div class="value">` + match + "</div>")
		add(`<div class="expect">` + e(t("expect-"+p.ID)) + "</div>")
		chips := ""
		for _, f := range p.Features {
			chips += featureChip(f, paired, lang)
		}
		add(`<div class="chips">` + chips + "</div>")
		add("</div>")
	}

	// The map.
	rgb, nRows, nColumns, levels := Montage(c.Normalised, c.ZMap)
	number := func(v float64) string { return ScoreText(&v, 0) }
	add("<h2>" + e(t("section-map")) + "</h2>")
	add("<figure>")
	add(`<img alt="" src="data:image/png;base64,` + base64.StdEncoding.EncodeToString(EncodePNG(rgb, nRows, nColumns)) + `">`)
	add(`<div class="legend"><span>` + number(-figureFull) + `</span><i class="low"></i><span>` + number(-figureFrom) + "</span>" +
		"<span>" + e(t("map-lower")) + "</span>" +
		`<i class="high"></i><span>` + number(figureFrom) + " … " + number(figureFull) + "</span>" +
		"<span>" + e(t("map-higher")) + "</span></div>")
	where := make([]string, len(levels))
	for i, level := range levels {
		where[i] = number(level)
	}
	add(`<figcaption class="caption">` + e(t("map-caption", strings.Join(where, sep))) + "</figcaption>")
	add("</figure>")

	// The regions.
	add("<h2>" + e(t("section-regions")) + "</h2>")
	add(`<table class="regions">`)
	head := append([]string{t("col-region"), t("col-left"), t("col-right"), t("col-difference")}, c.Checks...)
	head = append(head, t("col-finding"))
	line := "<tr>"
	for _, h := range head {
		line += "<th>" + e(h) + "</th>"
	}
	add(line + "</tr>")
	for i := range c.Regions {
		entry := &c.Regions[i]
		line := "<tr><td>" + e(regionName(entry.ID, lang)) + "</td>"
		if entry.Paired {
			for _, found := range []*RegionScore{entry.Left, entry.Right} {
				line += `<td class="` + found.Status + `">` + ScoreText(found.Z, 1) + "</td>"
			}
			line += "<td>" + ScoreText(entry.Difference, 1) + "</td>"
		} else {
			line += `<td class="span ` + entry.Both.Status + `" colspan="2">` + ScoreText(entry.Both.Z, 1) + "</td>"
			line += "<td></td>"
		}
		for _, m := range c.Checks {
			line += "<td>" + ScoreText(entry.Both.Checks[m], 1) + "</td>"
		}
		add(line + "<td>" + e(regionFinding(entry, lang)) + "</td></tr>")
	}
	add("</table>")
	add(`<p class="tablenote">` + e(t("regions-note")) + "</p>")

	// Notes and limitations.
	add("<h2>" + e(t("section-notes")) + "</h2>")
	add(`<ul class="notes">`)
	for _, note := range c.Notes {
		var text string
		if note.Regions != nil {
			places := make([]string, len(note.Regions))
			for i, r := range note.Regions {
				places[i] = RegionPlace(r[0], r[1], paired[r[0]], lang)
			}
			text = t("note-"+note.Key, strings.Join(places, sep))
		} else {
			values := make([]string, len(note.Values))
			for i, v := range note.Values {
				values[i] = itoa(v)
			}
			text = t("note-"+note.Key, values...)
		}
		add(`<li class="dynamic">` + e(text) + "</li>")
	}
	if len(c.Warnings) > 0 {
		add(`<li class="dynamic">` + e(t("note-warnings")+" "+strings.Join(c.Warnings, "; ")) + "</li>")
	}
	for n := 1; n <= 5; n++ {
		add("<li>" + e(t("limit-"+itoa(n))) + "</li>")
	}
	add("</ul>")
	add(`<div class="sign"><div>` + e(t("sign-physician")) + "</div><div>" + e(t("sign-date")) + "</div></div>")
	add("<footer>" + e(t("footer", c.Version)) + "</footer>")
	add("</main>")
	add("</body>")
	add("</html>")
	return strings.Join(out, "\n") + "\n", nil
}
