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

type normRow struct {
	image, method, factor, voxels, excluded, mask string
	group                                         string  // "patients" or "controls"
	value                                         float64 // the factor
}

type normPage struct {
	// files are the images of the project, the patients and then the
	// healthy controls, and controls those of them that are controls: both
	// groups are normalised the same way, by the same run.
	files     fileList
	controls  []string
	methods   map[string]bool
	reference string
	region    regionChoice
	output    string

	st     status
	rows   []normRow
	table  ui.ListState
	notes  []string
	outDir string
	// done is what the last run that went well did: what step 2 takes.
	done *normDone
}

// regionChoice is a region of the user's: a built-in mask, a mask file or
// a set of atlas labels.
type regionChoice struct {
	kind   int // 0 built-in mask, 1 mask file, 2 atlas labels
	mask   string
	file   string
	labels string
}

func (p *normPage) init() {
	p.methods = map[string]bool{"hn": true}
	p.region.mask = "brainstem"
}

// spec turns the choice into a mask and the text that names it in tables.
func (r *regionChoice) spec() (core.MaskSpec, string, error) {
	switch r.kind {
	case 0:
		return core.MaskSpec{Name: r.mask}, "", nil
	case 1:
		return core.MaskSpec{Name: r.file}, "", nil
	}
	var numbers []int
	var parts []string
	for _, f := range strings.FieldsFunc(r.labels, func(c rune) bool { return c == ',' || c == ' ' || c == '，' || c == ';' }) {
		n, err := strconv.Atoi(f)
		if err != nil {
			return core.MaskSpec{}, "", errorString(Tf("'%s' is not a label number", "“%s”不是标签编号", f))
		}
		numbers = append(numbers, n)
		parts = append(parts, f)
	}
	vol, err := core.LabelsMask(numbers)
	if err != nil {
		return core.MaskSpec{}, "", err
	}
	return core.MaskSpec{Vol: vol}, "labels:" + strings.Join(parts, ","), nil
}

func (r *regionChoice) problem() string {
	switch {
	case r.kind == 1 && r.file == "":
		return T("Choose the mask file of the custom region.", "请选择自选区域的掩膜文件。")
	case r.kind == 2 && strings.TrimSpace(r.labels) == "":
		return T("Enter the atlas label numbers of the custom region.", "请输入自选区域的图谱标签编号。")
	case r.kind == 2:
		if _, _, err := r.spec(); err != nil {
			return T("Custom region: ", "自选区域：") + err.Error()
		}
	}
	return ""
}

// regionRows are the settings of a custom region.
func (a *app) regionRows(c *ui.Context, key string, r *regionChoice) []func() {
	out := []func(){func() {
		setting(c, T("Region from", "区域来源"), "", func() {
			segmented(c.Key(key+"-kind"), &r.kind, T("Built-in mask", "内置掩膜"), T("Mask file", "掩膜文件"), T("Atlas labels", "图谱标签"))
		})
	}}
	switch r.kind {
	case 0:
		out = append(out, func() {
			setting(c, T("Mask", "掩膜"), maskAbout(r.mask), func() {
				var ids, names []string
				for _, m := range core.Masks {
					ids, names = append(ids, m.Name), append(names, maskName(m.Name))
				}
				selectOf(c.Key(key+"-mask"), &r.mask, ids, names).Width(190)
			})
		})
	case 1:
		out = append(out, func() {
			a.pathSetting(c, key+"-file", T("Mask file", "掩膜文件"),
				T("A NIfTI image in MNI space; voxels above zero make the region.", "MNI 空间的 NIfTI 图像，大于 0 的体素构成区域。"),
				&r.file, pathFile, niftiFilter, nil)
		})
	default:
		out = append(out, func() {
			setting(c, T("Label numbers", "标签编号"),
				T("CerebrA labels separated by commas, e.g. 34,85. The Masks & Atlas page lists them.", "CerebrA 图谱标签，逗号分隔，如 34,85。全部标签见“掩膜与图谱”页面。"),
				func() { ui.TextInput(c.Key(key+"-labels"), &r.labels).Placeholder("34,85").Width(190) })
		})
	}
	return out
}

func excludeName(i int) string {
	if i == 1 {
		return "both"
	}
	return "hypo"
}

var methodGroups = []struct {
	title   label
	methods []string
}{
	{label{"Data-driven, with a reference", "数据驱动 · 需要参考库"}, []string{"hn", "ihn", "ips"}},
	{label{"Global", "全局"}, []string{"ps", "spm-global", "whole-brain"}},
	{label{"Reference regions", "参考区"}, []string{"cerebellum", "cerebellar-gm", "pons", "white-matter", "occipital", "sensorimotor", "region"}},
}

// tileColumns is how many check tiles go side by side in a card of the
// page, by the room the window gives it.
func tileColumns(c *ui.Context) float32 {
	w, _ := c.Size()
	if min(w-sidebarWidth-cardGap-2*pagePad, contentWidth) < 600 {
		return 2
	}
	return 3
}

// tile is a cell of a grid of check tiles, cols to a row; build makes the
// tile.
func tile(c *ui.Context, key string, cols float32, build func()) {
	ui.Column(c.Key(key)).WidthPercent(100 / cols).Padding(3).Children(build)
}

// methodPicker is a card of check tiles, one per method.  The reference
// regions, which are many, stay out of sight until they are asked for or
// one of them is chosen.
func (a *app) methodPicker(c *ui.Context, chosen map[string]bool, allowed func(id string) (ok bool, why string)) {
	pal := paletteFor(c)
	cols := tileColumns(c)
	card(c).Padding(4, 0, 8, 0).Children(func() {
		hidden := 0
		for n, group := range methodGroups {
			if n >= 2 && !a.allMethods {
				used := false
				for _, id := range group.methods {
					used = used || chosen[id]
				}
				if !used {
					hidden += len(group.methods)
					continue
				}
			}
			ui.Column(c).Padding(9, 11, 0, 11).Gap(5).Children(func() {
				ui.Text(c, group.title.String()).FontSize(11.5).FontWeight(650).TextColor(pal.muted).Padding(0, 3)
				ui.Row(c).Wrap().Children(func() {
					for _, id := range group.methods {
						ok, why := true, ""
						if allowed != nil {
							ok, why = allowed(id)
						}
						on := chosen[id] && ok
						tile(c, id, cols, func() {
							var extra func()
							if id == "hn" {
								extra = func() { tag(c, T("Recommended", "推荐")) }
							}
							box := checkTileWithDesc(c, &on, methodName(id), methodHint(id), extra).Disabled(!ok)
							if ok {
								box.Tooltip(methodAbout(id))
							} else {
								box.Tooltip(why)
							}
						})
						if ok {
							chosen[id] = on
						}
					}
				})
			})
		}
		if hidden > 0 || a.allMethods {
			label := Tf("Reference regions: %d more methods", "参考区法：另有 %d 种方法", hidden)
			if a.allMethods {
				label = T("Fewer methods", "收起参考区法")
			}
			ui.Row(c).Padding(6, 8, 0, 8).Children(func() {
				more := button(c.Key("more-methods"), label, iconChevron, buttonGhost, 26)
				if more.Clicked() {
					a.allMethods = !a.allMethods
				}
			})
		}
	})
}
func (p *normPage) chosen() []string {
	var out []string
	for _, m := range core.Methods {
		if p.methods[m.Name] {
			out = append(out, m.Name)
		}
	}
	return out
}

// normReferenceProblem is what is missing while a method chosen needs a
// reference, and the project has none and cannot build one, or "".
func (a *app) normReferenceProblem() string {
	p := &a.norm
	if p.reference != "" || a.buildsReference() {
		return ""
	}
	for _, m := range p.chosen() {
		if core.NeedsReference(m) {
			return Tf("%s needs a reference: add at least 3 healthy controls on the Data page, or import a reference on the Reference page.",
				"%s 需要参考库：请在“数据”页面添加至少 3 例健康对照，或到“参考库”页面导入。", methodName(m))
		}
	}
	return ""
}

func (a *app) normProblem() string {
	p := &a.norm
	methods := p.chosen()
	if a.proj != nil && len(p.files.files) > 0 && a.dataProblem() != "" {
		return a.dataProblem()
	}
	switch {
	case len(p.files.files) == 0:
		return a.needImages("patients")
	case len(methods) == 0:
		return T("Choose at least one method.", "请至少选择一种方法。")
	}
	if msg := a.normReferenceProblem(); msg != "" {
		return msg
	}
	if a.buildsReference() {
		if msg := a.buildProblem(); msg != "" {
			return msg
		}
	}
	if p.methods["region"] {
		if msg := p.region.problem(); msg != "" {
			return msg
		}
	}
	return ""
}
func (a *app) normalizeView(c *ui.Context) {
	p := &a.norm
	a.frame(c, T("Intensity Normalisation", "强度归一化"),
		T("Divide each image by its normalisation factor", "每幅图像除以各自的归一化因子"),
		func() {
			a.steps(c)
			section(c, T("Images", "图像"), T("Every image of the project, in MNI space. The patients and the healthy controls are normalised the same way, so that they can be compared.",
				"项目的全部图像，须在 MNI 空间。患者和健康对照按同样的方式归一化，这样两者才能比较。"), func() {
				patients := p.files.files[:len(p.files.files)-min(len(p.controls), len(p.files.files))]
				a.source(c, "norm-data", "data", changeInData(),
					fact(c, T("Patients", "患者"), countText(patients)),
					fact(c, T("Healthy controls", "健康对照"), countText(p.controls)))
			})
			section(c, T("Methods", "方法"), T("One method is enough. Several are for comparison only: each writes its own image from the original.", "选一种即可。多选仅用于对比：每种方法都从原图各自输出一幅图像，互不叠加。"), func() {
				a.methodPicker(c, p.methods, nil)
			})
			if p.methods["region"] {
				section(c, T("Custom region", "自选区域"), "", func() { rows(c, a.regionRows(c, "norm-region", &p.region)...) })
			}
			section(c, T("Reference and output", "参考库与输出"), "", func() {
				a.source(c, "norm-ref", "reference", T("Open Reference", "查看参考库"),
					a.referenceFact(c),
					fact(c, T("Written to", "输出位置"), a.writesTo(p.output)+T(" · the normalised images, by group, and factors.tsv", " · 按组存放的归一化图像和 factors.tsv")))
				a.optionsLine(c, "normalize", "normalize", groupNormalize, groupIterative)
			})
			if len(p.rows) > 0 || len(p.notes) > 0 {
				a.results(c, T("Results", "结果"), "", func() {
					if len(p.rows) > 0 {
						factorChart(c, p.rows)
						// Which of the images along the chart are of which group.
						if pt, hc := groupSizes(p.rows); pt > 0 && hc > 0 {
							ui.Text(c, Tf("Images 1–%d are the patients, %d–%d the healthy controls: the factors of the two groups should lie in the same range.",
								"图像 1–%d 为患者，%d–%d 为健康对照：两组的因子应处于相近的范围。", pt, pt+1, pt+hc)).
								FontSize(12).TextColor(paletteFor(c).muted).LineHeight(1.4).Padding(0, 2)
						}
						cols := []ui.TableColumn{
							{Title: T("Image", "图像")},
							{Title: T("Group", "组别"), Width: 80},
							{Title: T("Method", "方法"), Width: 150},
							{Title: T("Factor", "因子"), Width: 100, Align: ui.End},
							{Title: T("Voxels", "体素数"), Width: 80, Align: ui.End},
							{Title: T("Excluded", "剔除比例"), Width: 80, Align: ui.End},
						}
						card(c).Children(func() {
							ui.Table(c, &p.table, cols, len(p.rows), func(row, col int) {
								r := p.rows[row]
								text := [...]string{r.image, groupName(r.group), methodName(r.method), r.factor, r.voxels, r.excluded}[col]
								ui.Text(c, text).FontSize(12.5).SingleLine()
							}).Height(float32(min(len(p.rows), 8)*32 + 34))
						})
						ui.Row(c).Gap(8).Wrap().GapY(6).Children(func() {
							revealButton(c, "norm-reveal", revealLabel(), p.outDir)
							if b := smallButton(c, T("Open factors.tsv", "打开 factors.tsv")); b.Clicked() {
								openPath(filepath.Join(p.outDir, "factors.tsv"))
							}
						})
					}
					notesCard(c, p.notes)
				})
			}
		},
		func() {
			a.runBar(c, "normalize", a.normProblem(), T("Normalise Intensity", "开始强度归一化"), &p.st, a.runNormalize)
			a.nextStep(c, "normalize")
		},
	)
}

// groupSizes counts the images of the patients and of the controls among
// the rows of a run, which lists the patients first.
func groupSizes(rows []normRow) (patients, controls int) {
	seen := map[string]bool{}
	for _, r := range rows {
		if key := r.group + "/" + r.image; !seen[key] {
			seen[key] = true
			if r.group == "controls" {
				controls++
			} else if r.group == "patients" {
				patients++
			}
		}
	}
	return patients, controls
}

// groupName is the name of a group of the project in a table.
func groupName(group string) string {
	switch group {
	case "patients":
		return T("Patient", "患者")
	case "controls":
		return T("Control", "对照")
	}
	return ""
}

func (a *app) runNormalize() {
	p := &a.norm
	files, controls := append([]string(nil), p.files.files...), append([]string(nil), p.controls...)
	methods := p.chosen()
	o := a.opt
	opt := core.DefaultNormOptions()
	if !o.FWHMAuto {
		fwhm := o.FWHM
		opt.FWHM = &fwhm
	}
	if o.Statistic == 1 {
		opt.Statistic = "median"
	}
	opt.Target, opt.PThreshold, opt.Exclude = o.Target, o.PThreshold, excludeName(o.Exclude)
	opt.MaxIter, opt.LiteralIPS = int(o.MaxIter), o.LiteralIPS
	var regionSpec core.MaskSpec
	regionText, regionLabel := "", "region"
	if p.methods["region"] {
		var err error
		if regionSpec, regionText, err = p.region.spec(); err != nil {
			p.st.err = err.Error()
			return
		}
		if p.region.kind == 0 {
			regionLabel = p.region.mask // name the files after the mask
		}
	}
	// The reference of the project is its controls: where it is not built
	// yet, or they or what it is built with have changed, the run begins
	// by building it.
	build, problem := a.referenceBuild()
	if problem != "" {
		p.st.err = problem
		return
	}
	reference, out, saveMasks := p.reference, p.output, o.SaveMasks
	if build != nil {
		reference = build.out
	}
	// What the steps after this one take of it.
	record := &normDone{asked: files, controls: controls, methods: methods, reference: reference, refSig: a.refFactorSignature(),
		settings: o.normSettings(methods), images: map[string]map[string]string{}, factors: map[string]map[string]float64{},
		target: opt.Target, median: opt.Statistic == "median", fwhm: opt.FWHM}
	record.stamp()
	p.rows, p.notes, p.done = nil, nil, nil
	label := T("Normalising intensity", "正在做强度归一化")
	first := label
	if build != nil {
		first = T("Building the reference from the controls", "正在用对照建立参考库")
	}
	a.start("normalize", first, func(ctx context.Context, step func(int, int)) (func(), error) {
		if build != nil {
			built, err := build.run(ctx, step)
			if err != nil {
				return nil, err
			}
			// The reference stands, however the rest of the run ends.
			a.update(func() {
				a.ref.built = built
				a.setReference(build.out)
				if a.job != nil {
					a.job.label, a.job.done, a.job.total = label, 0, 0
				}
			})
		}
		record.stampReference()
		if reference != "" {
			ref, err := core.LoadReference(reference)
			if err != nil {
				return nil, err
			}
			opt.Reference = ref
		}
		var rows []normRow
		var notes []string
		table := "image\tgroup\tmethod\tfactor\tn_voxels\texcluded_fraction\tmask\n"
		total, done, failed := len(files)*len(methods), 0, 0
		step(0, total)
		for _, path := range files {
			name := filepath.Base(path)
			group := "patients"
			if slices.Contains(controls, path) {
				group = "controls"
			}
			// Each group in a folder of its own: a patient and a control
			// may bear the same name.
			dir := filepath.Join(out, group)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
			img, loadErr := core.LoadImage(path)
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
				run := opt
				label := method
				if method == "region" {
					run.Mask, label = regionSpec, regionLabel
				}
				res, err := core.Normalize(img, method, run)
				if err != nil { // keep going with the other images
					failed++
					missed = true
					notes = append(notes, name+" ["+method+"]: "+err.Error())
					step(done, total)
					continue
				}
				for _, w := range res.Warnings {
					notes = append(notes, name+" ["+method+"]: "+w)
				}
				file := core.OutputName(path, label, "pet")
				if err := core.SaveNifti(filepath.Join(dir, file), res.Image, false); err != nil {
					return nil, err
				}
				if record.images[path] == nil {
					record.images[path], record.factors[path] = map[string]string{}, map[string]float64{}
				}
				record.images[path][method], record.factors[path][method] = filepath.Join(dir, file), res.Factor
				if saveMasks && res.Mask != nil {
					maskFile := strings.Replace(file, ".nii.gz", "_mask.nii.gz", 1)
					if err := core.SaveMask(filepath.Join(dir, maskFile), res.Mask, img.Dim, img.Aff, ""); err != nil {
						return nil, err
					}
				}
				row := normRow{image: name, group: group, method: method, factor: strconv.FormatFloat(res.Factor, 'g', 6, 64), mask: res.MaskName, value: res.Factor}
				if res.Mask != nil {
					row.voxels = strconv.Itoa(res.NVoxels)
				}
				if res.Iterative {
					row.excluded = strconv.FormatFloat(res.ExcludedFraction, 'f', 4, 64)
				}
				if method == "region" && regionText != "" {
					row.mask = regionText
				}
				table += strings.Join([]string{path, group, method, row.factor, row.voxels, row.excluded, row.mask}, "\t") + "\n"
				if res.Iterative {
					row.excluded = strconv.FormatFloat(res.ExcludedFraction*100, 'f', 1, 64) + "%"
				}
				rows = append(rows, row)
				step(done, total)
			}
			if !missed { // every method normalised it
				record.files = append(record.files, path)
			}
		}
		if err := os.MkdirAll(out, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(out, "factors.tsv"), []byte(table), 0o644); err != nil {
			return nil, err
		}
		return func() {
			p.rows, p.notes, p.outDir = rows, notes, out
			if len(record.files) > 0 {
				p.done = record
			}
			if failed > 0 {
				p.st.err = Tf("%d of %d failed; see the notes under the results.", "%d / %d 项失败，详见结果下方的说明。", failed, total)
				return
			}
			patients := len(files) - len(controls)
			p.st.ok = Tf("%d patient and %d control image(s) intensity-normalised with %d method(s).", "强度归一化已完成：%d 幅患者图像、%d 幅对照图像 × %d 种方法。", patients, len(controls), len(methods))
			if build != nil {
				p.st.ok = Tf("Reference built from %d controls. ", "已用 %d 例对照建立参考库。", len(build.files)) + p.st.ok
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
