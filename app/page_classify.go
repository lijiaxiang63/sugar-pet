package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type classifyRow struct {
	image, reading, detail, report string
	source, zmap                   string // the image, and the map made of it
	wscore                         bool
	scores                         []patternScore
	leading                        string // the pattern the reading names, if any
	category, confidence           string
	// badge is the pill beside the reading, in the language of the reading:
	// both are worded when the run ends.
	badge string
}

// classifyBadge is the confidence of a reading that names a pattern, as a
// pill beside it, and its colour.  The other readings say in their own words
// how far they go, and have none: a pill there would only repeat them, or,
// where no atypical pattern is found, pass for a normal scan.
func classifyBadge(category, confidence string, pal *palette) (string, ui.Color) {
	if category != "pattern" && category != "pd" {
		return "", pal.muted
	}
	switch confidence {
	case "high":
		return T("High confidence", "置信度高"), pal.success
	case "moderate":
		return T("Moderate confidence", "置信度中等"), pal.accent
	case "low":
		return T("Low confidence", "置信度低"), pal.warning
	}
	return "", pal.muted
}

type classifyPage struct {
	reference  string
	loadedFrom string
	info       *refInfo
	output     string
	// main is the method the classification rests on, chosen here when
	// step 2 made maps with several; "" for the one the app takes.
	main string

	st     status
	rows   []classifyRow
	notes  []string
	outDir string
	shown  int // the row the picture is of
	pic    picture
	// from is the run of step 2 the last run that went well took, method
	// the one it rested on, and checked whether it cross-checked its
	// findings with other normalisations.
	from    *zmapDone
	method  string
	checked bool
}

func (p *classifyPage) init() {}

// load reads what the reference in use says about itself.
func (p *classifyPage) load() {
	if p.reference != p.loadedFrom {
		p.loadedFrom, p.info = p.reference, nil
		if p.reference != "" {
			p.info = readRefInfo(p.reference)
		}
	}
}

// leading is the method the classification rests on: the one chosen here
// if step 2 made a map with it, or else HN, or the first of step 2.
func (p *classifyPage) leading(z *zmapDone) string {
	if slices.Contains(z.methods, p.main) {
		return p.main
	}
	return leadingMethod(z.methods)
}

// checks are the other normalisations the findings are checked with, where
// the settings ask for the cross-check: none otherwise.
func (a *app) classifyChecks(method string) []string {
	p := &a.classify
	out := []string{}
	if !a.opt.CrossCheck {
		return out
	}
	for _, m := range []string{"ps", "white-matter"} {
		stored := m == "ps"
		if p.info != nil {
			for _, s := range p.info.StatsFor {
				stored = stored || s == m
			}
		}
		if stored && m != method {
			out = append(out, m)
		}
	}
	return out
}

// reportLang is the language reports are written in.
func (o *options) reportLang() string {
	if o.ReportLanguage == 2 || (o.ReportLanguage == 0 && chinese) {
		return "zh"
	}
	return "en"
}

// classifyProblem is what keeps step 3 from running, or "".
func (a *app) classifyProblem() string {
	p := &a.classify
	if _, why := a.lock("classify"); why != "" {
		return why
	}
	z := a.zmapCurrent()
	switch {
	case p.reference == "":
		return needReference()
	case p.info != nil && p.info.err != "":
		return p.info.err
	}
	// Two reports under one name: the second patient's would replace the first's.
	if first, second, stem, found := core.SharedReportStem(z.files); found {
		short := func(path string) string { return filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path)) }
		return Tf("%s and %s would both be written as %s: rename one, or run them separately.",
			"%s 与 %s 的报告都会写为 %s：请重命名其中一个，或分开运行。",
			short(first), short(second), stem+"_report.html")
	}
	return ""
}

// reading says in a line what a classification concluded.
func reading(res *core.Classification) (text, detail string) {
	lang := uiLang()
	name := func(id string) string { return core.ReportText("name-"+id, lang) }
	match := func(id string) int {
		for _, p := range res.Patterns {
			if p.ID == id {
				return core.Percent(p.Score)
			}
		}
		return 0
	}
	found := res.Conclusion
	switch found.Category {
	case "pattern", "pd":
		// The confidence is the pill beside the reading (classifyBadge).
		return name(found.Pattern), Tf("Pattern match %d/100", "模式匹配度 %d/100", match(found.Pattern))
	case "indeterminate":
		return Tf("%s or %s", "%s 或 %s", name(found.Pattern), name(found.Alternative)),
			Tf("Cannot be told apart: %d/100 and %d/100", "无法可靠区分：%d/100 与 %d/100", match(found.Pattern), match(found.Alternative))
	case "possible":
		return Tf("Partial features of %s", "%s的部分特征", name(found.Pattern)),
			Tf("Pattern match %d/100: not enough to assign a pattern", "模式匹配度 %d/100：不足以判定", match(found.Pattern))
	}
	return T("No pattern of atypical parkinsonism", "未见非典型帕金森综合征的模式"),
		T("Favours Parkinson's disease in a patient with parkinsonism", "对帕金森综合征患者而言更倾向于帕金森病")
}

func (a *app) classifyView(c *ui.Context) {
	p := &a.classify
	t := c.Theme()
	pal := paletteFor(c)
	p.load()
	z := a.zmapCurrent()
	a.frame(c, T("Classify Parkinsonism", "帕金森综合征分类"),
		T("Which pattern a patient's scan matches, with a report", "判断患者图像符合哪种代谢模式，并生成报告"),
		func() {
			a.steps(c)
			section(c, T("From steps 1 and 2", "来自前两步"), T("The map of decreased and increased uptake that step 2 made of each patient is read region by region and matched against the patterns of MSA, PSP, CBD, DLB and PD.",
				"直接读取第 2 步为每位患者写出的代谢减低和增高的分布图：按脑区读取，并与 MSA、PSP、CBD、DLB 和 PD 的代谢模式逐一匹配。"), func() {
				if z == nil {
					a.taken(c, "classify")
					return
				}
				method := p.leading(z)
				shown := methodName(method)
				if checks := a.classifyChecks(method); len(checks) > 0 {
					shown += Tf(" · checked with %s", " · 另以%s复核", methodNames(checks))
				}
				reference := referenceText(p.reference, p.info)
				if p.info != nil && p.info.err == "" && p.info.N < 30 {
					reference += T(" (30 or more are recommended)", "（建议至少 30 例）")
				}
				table := T("None: the scores are not adjusted for age and sex", "未提供：评分不校正年龄和性别")
				if z.table != "" {
					table = filepath.Base(z.table)
				}
				a.taken(c, "classify",
					fact(c, T("Images", "图像"), imagesText(z.files, len(a.pageImages(z.from))-len(z.files))),
					fact(c, T("Method", "方法"), shown),
					fact(c, T("Reference", "参考库"), reference),
					fact(c, T("Covariate table", "协变量表"), table),
				)
			})
			section(c, T("Output", "输出"), "", func() {
				var mainRow func()
				if z != nil && len(z.methods) > 1 {
					mainRow = func() {
						setting(c, T("Map to classify by", "分类所依据的图"), T("Step 2 made maps with several methods; the classification rests on one. HN needs no region that the disease spares, which is unknown for a new patient.",
							"第 2 步用多种方法各出了一幅图，分类以其中一种为准。HN 不需要预先指定未受累的区域，适合诊断未明的患者。"), func() {
							labels := make([]string, len(z.methods))
							for i, m := range z.methods {
								labels[i] = methodName(m)
							}
							method := p.leading(z)
							selectOf(c.Key("cls-main"), &method, z.methods, labels).Width(200)
							p.main = method
						})
					}
				}
				rows(c, func() {
					setting(c, T("Written to", "输出位置"), T("For each patient a report (HTML, prints to PDF), the map, and the numbers; for all, classification.tsv.",
						"每位患者输出一份报告（HTML，可打印为 PDF）、一幅图和一份数据文件；另有汇总表 classification.tsv。"), func() {
						ui.Text(c, a.writesTo(p.output)).FontSize(12.5).TextColor(pal.muted).SingleLine()
					})
				}, mainRow)
				a.optionsLine(c, "classify", "classify", groupClassify)
			})
			caution(c, T("Decision support, not a diagnosis. The patterns are matched by fixed rules from the literature that were not validated on your patients. Read the result with the clinical findings, MRI and dopamine-transporter imaging.",
				"本功能仅为辅助决策，不构成诊断。模式匹配依据文献中的固定规则，未经本单位患者数据验证。结果须结合临床表现、MRI 和多巴胺转运体显像综合判断。"))
			if len(p.rows) > 0 || len(p.notes) > 0 {
				a.results(c, T("Results", "结果"), "", func() {
					if len(p.rows) > 0 {
						p.shown = min(max(p.shown, 0), len(p.rows)-1)
						card(c).Padding(2, 0).Children(func() {
							for i, r := range p.rows {
								if i > 0 {
									divider(c)
								}
								selected := i == p.shown && len(p.rows) > 1
								row := ui.ButtonBase(c.Key("cls-row-"+strconv.Itoa(i))).Padding(11, 14, 11, 10).Gap(12).AlignItems(ui.Center).Label(r.image)
								if selected {
									row.Background(pal.accentSoft)
								} else if row.Hovered() && len(p.rows) > 1 {
									row.Background(pal.rowHover)
								}
								if row.Clicked() {
									p.shown = i
								}
								row.Children(func() {
									barColor := ui.Transparent
									if selected {
										barColor = pal.accent
									}
									ui.Box(c).Size(3, 30).Radius(1.5).Background(barColor).Shrink(0)
									ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
										ui.Row(c).Gap(7).AlignItems(ui.Center).Wrap().GapY(3).Children(func() {
											ui.Text(c, r.reading).FontSize(13.5).FontWeight(600)
											if r.badge != "" {
												_, col := classifyBadge(r.category, r.confidence, pal)
												pill(c, r.badge, col)
											}
										})
										ui.Text(c, r.image+" · "+r.detail).FontSize(12).TextColor(t.TextMuted)
									})
									if len(r.scores) > 0 {
										scoreBars(c, r.scores, r.leading)
									}
									if smallButton(c.Key("open"), T("Open Report", "打开报告")).Clicked() {
										openPath(r.report)
									}
								})
							}
						})
						if r := p.rows[p.shown]; r.zmap != "" {
							unit := "z"
							if r.wscore {
								unit = "W"
							}
							a.show(&p.pic, r.zmap, mapPicture(r.source, r.zmap))
							a.figure(c, &p.pic, r.image, legend{kind: legendScores, unit: unit, from: "2", full: "5"})
						}
						ui.Row(c).Children(func() { revealButton(c, "cls-reveal", revealLabel(), p.outDir) })
					}
					notesCard(c, p.notes)
				})
			}
		},
		func() {
			a.runBar(c, "classify", a.classifyProblem(), T("Classify", "开始分类"), &p.st, a.runClassify)
		},
	)
}

func (a *app) runClassify() {
	p := &a.classify
	from := a.zmapCurrent()
	if from == nil { // the button is disabled while step 2 is not done
		return
	}
	files := append([]string(nil), from.files...)
	norm := from.from
	reference, out, table, method, lang := p.reference, p.output, from.table, p.leading(from), a.opt.reportLang()
	// Without the cross-check nothing but the maps of step 2 is read: no
	// map of another normalisation is computed for it.
	checked := a.opt.CrossCheck
	p.rows, p.notes, p.shown, p.from = nil, nil, 0, nil
	a.start("classify", T("Classifying", "正在分类"), func(ctx context.Context, step func(int, int)) (func(), error) {
		ref, err := core.LoadReference(reference)
		if err != nil {
			return nil, err
		}
		var covariates *core.Table
		if table != "" {
			if covariates, err = core.LoadTable(table); err != nil {
				return nil, err
			}
		}
		if err := os.MkdirAll(out, 0o755); err != nil {
			return nil, err
		}
		var rows []classifyRow
		var notes []string
		failed := 0
		summary := strings.Join(append(append([]string{"image", "category", "pattern", "alternative", "confidence", "side"}, core.Patterns...), "report"), "\t") + "\n"
		step(0, len(files))
		for i, path := range files {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			name := filepath.Base(path)
			opt := core.DefaultClassifyOptions()
			if !checked {
				opt.Checks = []string{}
			}
			var err error
			if covariates != nil {
				var found []map[string]string
				if found, err = covariates.Lookup([]string{path}, ref.Covariates()); err == nil {
					opt.Covariates = found[0]
				}
			}
			// What the steps before made of the image: the image normalised
			// with the method, and the maps.  Only the maps the cross-checks
			// need and step 2 did not make are computed here.
			in := core.Scored{Target: norm.target, Factors: norm.factors[path], Maps: map[string]*core.Volume{}}
			if err == nil {
				in.Normalised, err = core.LoadImage(norm.images[path][method])
			}
			for _, m := range from.methods {
				if err == nil {
					in.Maps[m], err = core.LoadImage(from.maps[path][m])
				}
			}
			var res *core.Classification
			if err == nil {
				res, err = core.ClassifyScored(in, name, method, ref, opt)
			}
			var saved core.ClassFiles
			if err == nil {
				res.MapFile = from.maps[path][method] // the map is step 2's: no copy of it is written
				saved, err = res.Save(out, core.ReportStem(path), lang)
			}
			if err != nil {
				failed++
				notes = append(notes, name+": "+err.Error())
				step(i+1, len(files))
				continue
			}
			for _, w := range res.Warnings {
				notes = append(notes, name+": "+w)
			}
			text, detail := reading(res)
			found := res.Conclusion
			row := classifyRow{image: name, reading: text, detail: detail, report: saved.Report,
				source: path, zmap: saved.Map, wscore: res.Score == "w", category: found.Category, confidence: found.Confidence}
			row.badge, _ = classifyBadge(row.category, row.confidence, &lightPalette)
			for _, pattern := range res.Patterns {
				row.scores = append(row.scores, patternScore{pattern.ID, core.Percent(pattern.Score)})
			}
			if found.Category != "none" {
				row.leading = found.Pattern
			}
			rows = append(rows, row)
			cells := []string{path, found.Category, found.Pattern, found.Alternative, found.Confidence, found.Side}
			for _, pattern := range res.Patterns {
				cells = append(cells, strconv.Itoa(core.Percent(pattern.Score)))
			}
			summary += strings.Join(append(cells, filepath.Base(saved.Report)), "\t") + "\n"
			step(i+1, len(files))
		}
		if err := os.WriteFile(filepath.Join(out, "classification.tsv"), []byte(summary), 0o644); err != nil {
			return nil, err
		}
		return func() {
			p.rows, p.notes, p.outDir = rows, notes, out
			if len(rows) > 0 {
				p.from, p.method, p.checked = from, method, checked
			}
			if failed > 0 {
				p.st.err = Tf("%d of %d failed; see the notes under the results.", "%d / %d 项失败，详见结果下方的说明。", failed, len(files))
			} else {
				p.st.ok = Tf("%d report(s) written.", "已生成 %d 份报告。", len(rows))
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
