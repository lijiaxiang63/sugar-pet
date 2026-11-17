package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type classifyRow struct {
	image, reading, detail, report string
}

type classifyPage struct {
	files      fileList
	reference  string
	loadedFrom string
	info       *refInfo
	covTable   string
	method     string
	language   int // of the report: 0 as the app, 1 English, 2 Chinese
	output     string

	st     status
	rows   []classifyRow
	notes  []string
	outDir string
}

func (p *classifyPage) init() { p.method = "hn" }

// methods are the normalisations the reference can make maps with.
func (p *classifyPage) methods() []string {
	out := []string{"hn", "ihn", "ps", "ips"}
	if p.info != nil {
		for _, m := range core.Methods {
			for _, stored := range p.info.StatsFor {
				if m.Name == stored {
					out = append(out, m.Name)
				}
			}
		}
	}
	return out
}

// checks are the other normalisations the findings are checked with.
func (p *classifyPage) checks() []string {
	var out []string
	for _, m := range []string{"ps", "white-matter"} {
		stored := m == "ps"
		if p.info != nil {
			for _, s := range p.info.StatsFor {
				stored = stored || s == m
			}
		}
		if stored && m != p.method {
			out = append(out, m)
		}
	}
	return out
}

func (p *classifyPage) lang() string {
	if p.language == 2 || (p.language == 0 && chinese) {
		return "zh"
	}
	return "en"
}

func (p *classifyPage) problem() string {
	switch {
	case len(p.files.files) == 0:
		return T("Add the images of the patients.", "请添加患者的图像。")
	case p.reference == "":
		return T("Choose a reference, or build one on the Reference page.", "请选择参考库，或先到“参考库”页面建立。")
	case p.info != nil && p.info.err != "":
		return p.info.err
	case p.output == "":
		return T("Choose the output folder.", "请选择输出文件夹。")
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
		return name(found.Pattern), Tf("Pattern match %d/100, confidence %s", "模式匹配度 %d/100，置信度%s",
			match(found.Pattern), core.ReportText("confidence-"+found.Confidence, lang))
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
	if p.reference != p.loadedFrom {
		p.loadedFrom, p.info = p.reference, nil
		if p.reference != "" {
			p.info = readRefInfo(p.reference)
		}
	}
	methods := p.methods()
	known := false
	for _, m := range methods {
		known = known || m == p.method
	}
	if !known {
		p.method = "hn"
	}
	hasModels := p.info != nil && len(p.info.Covariates) > 0
	a.frame(c, T("Classify Parkinsonism", "帕金森综合征分类"),
		T("Which pattern a patient's scan matches, with a report", "判断患者图像符合哪种代谢模式，并生成报告"),
		func() {
			section(c, T("Patients", "患者"), T("Each image is compared with the healthy controls. The map of decreased and increased uptake is read region by region and matched against the patterns of MSA, PSP, CBD, DLB and PD.",
				"每幅图像与健康对照比较，得到代谢减低和增高的分布图；再按脑区读取，并与 MSA、PSP、CBD、DLB 和 PD 的代谢模式逐一匹配。"), func() {
				a.filesCard(c, "cls-files", &p.files, T("Patient images", "患者图像"), T("FDG-PET images in MNI space, one per patient", "MNI 空间的 FDG-PET 图像，每位患者一幅"))
			})
			section(c, T("Reference", "参考库"), "", func() {
				desc := T("The healthy controls the patients are compared with: same scanner and reconstruction, 30 or more if possible.", "与患者比较的健康对照：应来自同一扫描仪和重建方式，尽量不少于 30 例。")
				if p.info != nil && p.info.err == "" {
					desc = Tf("%d controls, smoothed %g mm.", "%d 例对照，平滑 %g mm。", p.info.N, p.info.FWHM)
					if p.info.N < 30 {
						desc += T(" At least 30 are recommended.", "建议至少 30 例。")
					}
				}
				covDesc := T("This reference was built without covariates, so the scores are not adjusted for age and sex.", "该参考库建立时未提供协变量，评分不校正年龄和性别。")
				if hasModels {
					covDesc = Tf("Recommended. With the %s of the patients, each is compared with what the controls predict for such a person.",
						"建议提供。给出患者的 %s 后，每位患者将与对照对同样条件的人的预测值比较。", strings.Join(p.info.covariateNames(), T(" and ", "、")))
				}
				var covRow func()
				if hasModels {
					covRow = func() {
						a.pathSetting(c, "cls-cov", T("Covariate table", "协变量表"), covDesc, &p.covTable, pathFile, tableFilter, nil)
					}
				} else if p.reference != "" {
					covRow = func() { setting(c, T("Covariate table", "协变量表"), covDesc, nil) }
				}
				rows(c,
					func() {
						a.pathSetting(c, "cls-ref", T("Reference", "参考库"), desc, &p.reference, pathFolder, nil,
							func() { a.setReference(p.reference); a.saveSettings() })
					},
					covRow,
				)
			})
			section(c, T("Settings", "设置"), "", func() {
				labels := make([]string, len(methods))
				for i, m := range methods {
					labels[i] = methodName(m)
				}
				checks := p.checks()
				names := make([]string, len(checks))
				for i, m := range checks {
					names[i] = methodName(m)
				}
				checked := T("A finding that the other normalisations do not show is marked in the report.", "其他归一化方式下未出现的发现会在报告中标出。")
				shown := strings.Join(names, T(", ", "、"))
				if len(checks) == 0 {
					shown = T("None", "无")
				}
				hasWhite := p.method == "white-matter"
				for _, m := range checks {
					hasWhite = hasWhite || m == "white-matter"
				}
				if !hasWhite {
					checked += T(" Tick White matter when building the reference to add a check that does not rest on gray matter.", "建立参考库时勾选“深部白质”，可增加一种不依赖灰质的核对方式。")
				}
				rows(c,
					func() {
						setting(c, T("Intensity normalisation", "强度归一化"), T("The classification rests on this one. HN needs no region that the disease spares, which is unknown for a new patient.",
							"分类以此为准。HN 不需要预先指定未受累的区域，适合诊断未明的患者。"),
							func() { selectOf(c.Key("cls-method"), &p.method, methods, labels).Width(200) })
					},
					func() {
						setting(c, T("Checked with", "交叉核对"), checked, func() { ui.Text(c, shown).FontSize(12.5).TextColor(t.TextMuted) })
					},
					func() {
						setting(c, T("Language of the report", "报告语言"), "", func() {
							ui.Segmented(c.Key("cls-lang"), &p.language, T("As the app", "与界面一致"), "English", "中文")
						})
					},
					func() {
						a.pathSetting(c, "cls-out", T("Output folder", "输出文件夹"), T("For each patient a report (HTML, prints to PDF), the map, and the numbers; for all, classification.tsv.",
							"每位患者输出一份报告（HTML，可打印为 PDF）、一幅图和一份数据文件；另有汇总表 classification.tsv。"),
							&p.output, pathFolder, nil, nil)
					},
				)
			})
			ui.Row(c).Radius(10).Padding(10, 14).Gap(10).AlignItems(ui.Start).Background(t.Warning.Alpha(0.1)).Border(1, t.Warning.Alpha(0.3)).Children(func() {
				ui.Icon(c, iconWarn).Size(15, 15).TextColor(t.Warning).Shrink(0).Margin(1, 0, 0, 0)
				ui.Text(c, T("Decision support, not a diagnosis. The patterns are matched by fixed rules from the literature that were not validated on your patients. Read the result with the clinical findings, MRI and dopamine-transporter imaging.",
					"本功能仅为辅助决策，不构成诊断。模式匹配依据文献中的固定规则，未经本单位患者数据验证。结果须结合临床表现、MRI 和多巴胺转运体显像综合判断。")).FontSize(12).Grow(1).MinWidth(0)
			})
			if len(p.rows) > 0 || len(p.notes) > 0 {
				a.results(c, T("Results", "结果"), "", func() {
					if len(p.rows) > 0 {
						card(c).Children(func() {
							for i, r := range p.rows {
								if i > 0 {
									ui.Box(c).Height(1).Margin(0, 0, 0, 14).Background(hairline(t))
								}
								ui.Row(c.Key("cls-row-"+strconv.Itoa(i))).Padding(8, 14).Gap(14).AlignItems(ui.Center).Children(func() {
									ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
										ui.Text(c, r.reading).FontSize(13).Bold()
										ui.Text(c, r.image+" · "+r.detail).FontSize(11.5).TextColor(t.TextMuted)
									})
									if smallButton(c.Key("open"), T("Open Report", "打开报告")).Clicked() {
										openPath(r.report)
									}
								})
							}
						})
						ui.Row(c).Children(func() { revealButton(c, "cls-reveal", revealLabel(), p.outDir) })
					}
					notesCard(c, p.notes)
				})
			}
		},
		func() { a.runBar(c, "classify", p.problem(), T("Classify", "开始分类"), &p.st, a.runClassify) },
	)
}

func (a *app) runClassify() {
	p := &a.classify
	files := append([]string(nil), p.files.files...)
	reference, out, table, method, lang := p.reference, p.output, p.covTable, p.method, p.lang()
	if p.info == nil || len(p.info.Covariates) == 0 {
		table = ""
	}
	p.rows, p.notes = nil, nil
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
			img, err := core.LoadImage(path)
			if err == nil && covariates != nil {
				var found []map[string]string
				if found, err = covariates.Lookup([]string{path}, ref.Covariates()); err == nil {
					opt.Covariates = found[0]
				}
			}
			var res *core.Classification
			if err == nil {
				res, err = core.Classify(img, name, method, ref, opt)
			}
			var saved core.ClassFiles
			if err == nil {
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
			rows = append(rows, classifyRow{image: name, reading: text, detail: detail, report: saved.Report})
			found := res.Conclusion
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
			if failed > 0 {
				p.st.err = Tf("%d of %d failed; see the notes under the results.", "%d / %d 项失败，详见结果下方的说明。", failed, len(files))
			} else {
				p.st.ok = Tf("%d report(s) written.", "已生成 %d 份报告。", len(rows))
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
