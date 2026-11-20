package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type zmapRow struct {
	image, method, file, lowest, share string
	source, path                       string // the image, and the map made of it
	wscore                             bool
}

type zmapPage struct {
	output string

	st     status
	rows   []zmapRow
	table  ui.ListState
	notes  []string
	outDir string
	shown  int // the row the picture is of
	pic    picture
	// done is what the last run that went well did: what step 3 takes.
	done *zmapDone
}

func (p *zmapPage) init() {}

// zmapAllowed tells whether the reference in use can serve a method.
func (a *app) zmapAllowed(id string) (bool, string) {
	switch id {
	case "ps", "ips", "hn", "ihn":
		return true, ""
	case "region":
		return false, T("Z-score maps cannot use a custom region.", "Z 值图不支持自选区域。")
	}
	if info := a.refInfo(); info != nil {
		for _, m := range info.StatsFor {
			if m == id {
				return true, ""
			}
		}
	}
	return false, T("The control statistics hold none for this method.",
		"对照统计中没有该方法的统计。")
}

// noMethod is what the page says when no method of step 1 can make maps.
func noMethod(methods []string) string {
	return Tf("Z-score maps cannot be made with %s: the control statistics hold none for it. Go back to step 1 and choose HN.",
		"无法用%s制作 Z 值图：对照统计中没有它的统计。请返回第 1 步改用 HN。", methodNames(methods))
}

// zmapProblem is what keeps step 2 from running, or "".
func (a *app) zmapProblem() string {
	if _, why := a.lock("zmap"); why != "" {
		return why
	}
	n := a.normCurrent()
	switch {
	case a.reference == "":
		return needReference()
	case a.refInfo() != nil && a.refInfo().err != "":
		return a.refInfo().err
	}
	if patients, _ := n.of(false); len(patients) == 0 {
		if len(n.asked) == len(n.controls) {
			return a.needImages("patients")
		}
		return T("Step 1 normalised none of the patients: see its notes.", "第 1 步没有成功归一化任何患者图像：详见该步的说明。")
	}
	yes, no := a.usable(n.methods)
	if len(yes) == 0 {
		return noMethod(no)
	}
	// The run begins by building the reference anew.
	if a.refStale() {
		if msg := a.buildProblem(); msg != "" {
			return msg
		}
	}
	return ""
}

func (a *app) zmapView(c *ui.Context) {
	p := &a.zmap
	hasModels := a.refInfo() != nil && len(a.refInfo().Covariates) > 0
	n := a.normCurrent()
	var methods, skipped []string
	if n != nil {
		methods, skipped = a.usable(n.methods)
	}
	a.frame(c, T("Z-score Maps", "Z 值图"),
		T("How far each voxel is from the healthy controls", "每个体素偏离健康对照的程度"),
		func() {
			a.steps(c)
			section(c, T("From step 1", "来自第 1 步"), T("The normalised images of the patients are read as step 1 wrote them: smoothed like the controls and compared with them voxel by voxel. Negative values mean lower uptake. No map is made of the controls: they are what the patients are compared with.",
				"直接读取第 1 步写出的患者归一化图像：按对照的方式平滑，再逐体素与对照比较。负值表示摄取低于对照。对照本身不做 Z 值图：患者正是与它们比较。"), func() {
				if n == nil {
					a.taken(c, "zmap")
					return
				}
				shown := methodNames(methods)
				if len(methods) == 0 {
					shown = T("None that can make maps", "没有可用于 Z 值图的方法")
				}
				a.taken(c, "zmap",
					fact(c, T("Patients", "患者"), imagesText(n.of(false))),
					fact(c, T("Methods", "方法"), shown),
					a.referenceFact(c),
				)
				if len(skipped) > 0 && len(methods) > 0 {
					caution(c, Tf("No map is made with %s: the control statistics hold none for it.",
						"本步不用%s制作 Z 值图：对照统计中没有它的统计。", methodNames(skipped)))
				}
			})
			if a.reference != "" && (a.refInfo() == nil || a.refInfo().err == "") {
				section(c, T("Age and sex (W-scores)", "年龄和性别（W 值）"), "", func() {
					about := T("The controls were summarised without covariates, so the maps are plain z-scores.", "汇总对照统计时未提供协变量，因此输出普通 Z 值图。")
					switch {
					case hasModels && a.cov.path != "":
						about = Tf("The control statistics model %s, and the table gives them for these images: the maps are W-scores, compared with what the controls predict for such a person.",
							"对照统计对%s建了模型，表格中有这些图像的取值：输出 W 值图，与对照对同样条件的人的预测值比较。", strings.Join(a.refInfo().covariateNames(), T(" and ", "、")))
					case hasModels:
						about = Tf("The control statistics model %s. Choose a covariate table on the Data page to get W-scores; without one the maps are plain z-scores.",
							"对照统计对%s建了模型。在“数据”页面选择协变量表即可输出 W 值图；不选则输出普通 Z 值图。", strings.Join(a.refInfo().covariateNames(), T(" and ", "、")))
					}
					table := T("None", "无")
					if a.cov.path != "" {
						table = filepath.Base(a.cov.path)
					}
					a.source(c, "z-cov", "data", changeInData(), func() {
						setting(c, T("Covariate table", "协变量表"), about, func() {
							ui.Text(c, table).FontSize(12.5).TextColor(paletteFor(c).muted).SingleLine()
						})
					})
				})
			}
			section(c, T("Output", "输出"), "", func() {
				rows(c, fact(c, T("Written to", "输出位置"), a.writesTo(p.output)+T(" · one map per image and method", " · 每幅图像每种方法一幅图")))
			})
			if len(p.rows) > 0 || len(p.notes) > 0 {
				a.results(c, T("Results", "结果"), T("“Below −2” is the share of brain voxels with z < −2 (about 2.3% is expected by chance).", "“低于 −2”为脑内 z < −2 的体素比例（随机情况下约为 2.3%）。"), func() {
					if len(p.rows) > 0 {
						p.shown = min(max(p.shown, 0), len(p.rows)-1)
						r := p.rows[p.shown]
						picker(c, "z-pick", &p.shown, len(p.rows), r.image+" · "+methodName(r.method))
						unit := "z"
						if r.wscore {
							unit = "W"
						}
						a.show(&p.pic, r.path, mapPicture(r.source, r.path))
						a.figure(c, &p.pic, r.file, legend{kind: legendScores, unit: unit, from: "2", full: "5"})
						cols := []ui.TableColumn{
							{Title: T("Image", "图像")},
							{Title: T("Method", "方法"), Width: 150},
							{Title: T("Lowest", "最低值"), Width: 80, Align: ui.End},
							{Title: T("Below −2", "低于 −2"), Width: 90, Align: ui.End},
						}
						p.table.Selected = &p.shown
						card(c).Children(func() {
							ui.Table(c, &p.table, cols, len(p.rows), func(row, col int) {
								r := p.rows[row]
								ui.Text(c, [...]string{r.image, methodName(r.method), r.lowest, r.share}[col]).FontSize(12.5).SingleLine()
							}).Height(float32(min(len(p.rows), 6)*32 + 34))
						})
						ui.Row(c).Children(func() { revealButton(c, "z-reveal", revealLabel(), p.outDir) })
					}
					notesCard(c, p.notes)
				})
			}
		},
		func() {
			a.runBar(c, "zmap", a.zmapProblem(), T("Make Maps", "生成 Z 值图"), &p.st, a.runZMap)
			a.nextStep(c, "zmap")
		},
	)
}

func (a *app) runZMap() {
	p := &a.zmap
	from := a.normCurrent()
	if from == nil { // the button is disabled while step 1 is not done
		return
	}
	files, _ := from.of(false) // the patients: the controls are the reference
	methods, _ := a.usable(from.methods)
	reference, out, table := a.reference, p.output, a.zmapCovariates()
	// Where the covariates, or the statistics the reference keeps, have
	// changed since it was built, the run begins by building it anew.  The
	// controls and what the factors rest on are as step 1 found them: the
	// images it normalised still stand.
	build, problem := a.referenceBuild()
	if problem != "" {
		p.st.err = problem
		return
	}
	if build != nil {
		reference, table = build.out, ""
		if len(build.opt.Covariates) > 0 {
			table = a.cov.path
		}
	}
	record := &zmapDone{from: from, methods: methods, table: table, refSig: a.refSignature(), maps: map[string]map[string]string{}}
	p.rows, p.notes, p.shown, p.done = nil, nil, 0, nil
	label := T("Making maps", "正在生成 Z 值图")
	first := label
	if build != nil {
		first = T("Summarising the healthy controls", "正在汇总健康对照")
	}
	a.start("zmap", first, func(ctx context.Context, step func(int, int)) (func(), error) {
		if build != nil {
			built, err := build.run(ctx, step)
			if err != nil {
				return nil, err
			}
			// The reference stands, however the rest of the run ends; step 1
			// still rests on it, as on the one it replaces.
			a.update(func() {
				a.ref.built = built
				a.setReference(build.out)
				from.refStamp = built.stamp
				if a.job != nil {
					a.job.label, a.job.done, a.job.total = label, 0, 0
				}
			})
		}
		ref, err := core.LoadReference(reference)
		if err != nil {
			return nil, err
		}
		for _, method := range methods { // stop early if the reference cannot serve a method
			if _, err := ref.ControlStats(method); err != nil {
				return nil, err
			}
			if table != "" {
				if _, err := ref.ControlModel(method); err != nil {
					return nil, err
				}
			}
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
		var rows []zmapRow
		var notes []string
		total, done, failed := len(files)*len(methods), 0, 0
		step(0, total)
		for _, path := range files {
			name := filepath.Base(path)
			// The age and sex of the patient, where the map is of W-scores.
			var values map[string]string
			var loadErr error
			if covariates != nil {
				found, err := covariates.Lookup([]string{path}, ref.Covariates())
				if err != nil {
					loadErr = err
				} else {
					values = found[0]
				}
			}
			missed := loadErr != nil
			for _, method := range methods {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				done++
				if loadErr != nil {
					failed++
					notes = append(notes, name+": "+loadErr.Error())
					step(done, total)
					continue
				}
				// The image as step 1 wrote it with this method: it is not
				// normalised a second time.
				var z *core.Volume
				var warnings []string
				img, err := core.LoadImage(from.images[path][method])
				if err == nil {
					z, warnings, err = core.ZMapOf(img, method, from.factors[path][method], from.target, ref, values)
				}
				for _, w := range warnings {
					notes = append(notes, name+" ["+method+"]: "+w)
				}
				if err != nil {
					failed++
					missed = true
					notes = append(notes, name+" ["+method+"]: "+err.Error())
					step(done, total)
					continue
				}
				suffix := "zmap"
				if values != nil {
					suffix = "wmap"
				}
				file := core.OutputName(path, method, suffix)
				if err := core.SaveNifti(filepath.Join(out, file), z, false); err != nil {
					return nil, err
				}
				if record.maps[path] == nil {
					record.maps[path] = map[string]string{}
				}
				record.maps[path][method] = filepath.Join(out, file)
				row := zmapRow{image: name, method: method, file: file,
					source: path, path: filepath.Join(out, file), wscore: values != nil}
				row.lowest, row.share = scoreSummary(z)
				rows = append(rows, row)
				step(done, total)
			}
			if !missed { // every method made a map of it
				record.files = append(record.files, path)
			}
		}
		return func() {
			p.rows, p.notes, p.outDir = rows, notes, out
			if len(record.files) > 0 { // what the next step takes
				p.done = record
			}
			if failed > 0 {
				p.st.err = Tf("%d of %d failed; see the notes under the results.", "%d / %d 项失败，详见结果下方的说明。", failed, total)
			} else {
				p.st.ok = Tf("%d map(s) written.", "已输出 %d 幅图。", len(rows))
				if build != nil {
					p.st.ok = Tf("Control statistics summarised anew from %d controls. ", "已由 %d 例对照重新汇总对照统计。", len(build.files)) + p.st.ok
				}
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
