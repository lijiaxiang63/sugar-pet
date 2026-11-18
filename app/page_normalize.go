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

type normRow struct {
	image, method, factor, voxels, excluded, mask string
	value                                         float64 // the factor
}

type normPage struct {
	files     fileList
	methods   map[string]bool
	reference string
	region    regionChoice
	output    string

	st     status
	rows   []normRow
	table  ui.ListState
	notes  []string
	outDir string
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
	card(c).Padding(3, 0, 8, 0).Children(func() {
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
			ui.Column(c).Padding(9, 11, 0, 11).Gap(4).Children(func() {
				ui.Text(c, group.title.String()).FontSize(11.5).FontWeight(600).TextColor(pal.muted).Padding(0, 3)
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
							box := checkTile(c, &on, methodName(id), extra).Disabled(!ok)
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

func (p *normPage) problem() string {
	methods := p.chosen()
	switch {
	case len(p.files.files) == 0:
		return T("Add the images to intensity-normalise.", "请添加要做强度归一化的图像。")
	case len(methods) == 0:
		return T("Choose at least one method.", "请至少选择一种方法。")
	}
	if p.reference == "" {
		for _, m := range methods {
			if core.NeedsReference(m) {
				return Tf("%s needs a reference: choose one, or build it from healthy controls on the Reference page.",
					"%s 需要参考库：请选择参考库，或先到“参考库”页面用健康对照建立。", methodName(m))
			}
		}
	}
	if p.methods["region"] {
		if msg := p.region.problem(); msg != "" {
			return msg
		}
	}
	if p.output == "" {
		return T("Choose the output folder.", "请选择输出文件夹。")
	}
	return ""
}

func (a *app) normalizeView(c *ui.Context) {
	p := &a.norm
	a.frame(c, T("Intensity Normalisation", "强度归一化"),
		T("Divide each image by its normalisation factor", "每幅图像除以各自的归一化因子"),
		func() {
			section(c, T("Images", "图像"), T("FDG-PET images that are already in MNI space. If yours are not, warp them on the Spatial Normalisation page first.", "已在 MNI 空间的 FDG-PET 图像。尚未配准的，请先到“空间标准化”页面处理。"), func() {
				a.filesCard(c, "norm-files", &p.files, T("Images in MNI space", "MNI 空间的图像"),
					T(".nii and .nii.gz files, or whole folders", "支持 .nii 与 .nii.gz 文件，也可添加整个文件夹"))
			})
			section(c, T("Methods", "方法"), T("Choose several to compare them: each image is written once per method.", "可多选以便比较：每种方法各输出一幅图像。"), func() {
				a.methodPicker(c, p.methods, nil)
			})
			if p.methods["region"] {
				section(c, T("Custom region", "自选区域"), "", func() { rows(c, a.regionRows(c, "norm-region", &p.region)...) })
			}
			section(c, T("Reference and output", "参考库与输出"), "", func() {
				rows(c,
					func() {
						a.pathSetting(c, "norm-ref", T("Reference", "参考库"),
							T("Needed by HN, iPS and iHN. With one, PS uses the same voxels as they do.", "HN、iPS、iHN 需要。选了参考库后，PS 与它们使用相同的体素。"),
							&p.reference, pathFolder, nil, func() { a.setReference(p.reference) })
					},
					func() {
						a.pathSetting(c, "norm-out", T("Output folder", "输出文件夹"),
							T("Intensity-normalised images and factors.tsv are written here.", "强度归一化后的图像和 factors.tsv 写入此处。"),
							&p.output, pathFolder, nil, nil)
					},
				)
				a.optionsLine(c, "normalize", "normalize", groupNormalize, groupIterative)
			})
			if len(p.rows) > 0 || len(p.notes) > 0 {
				a.results(c, T("Results", "结果"), "", func() {
					if len(p.rows) > 0 {
						factorChart(c, p.rows)
						cols := []ui.TableColumn{
							{Title: T("Image", "图像")},
							{Title: T("Method", "方法"), Width: 150},
							{Title: T("Factor", "因子"), Width: 100, Align: ui.End},
							{Title: T("Voxels", "体素数"), Width: 80, Align: ui.End},
							{Title: T("Excluded", "剔除比例"), Width: 80, Align: ui.End},
						}
						card(c).Children(func() {
							ui.Table(c, &p.table, cols, len(p.rows), func(row, col int) {
								r := p.rows[row]
								text := [...]string{r.image, methodName(r.method), r.factor, r.voxels, r.excluded}[col]
								ui.Text(c, text).FontSize(12.5).SingleLine()
							}).Height(float32(min(len(p.rows), 8)*32 + 34))
						})
						ui.Row(c).Gap(8).Children(func() {
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
			a.runBar(c, "normalize", p.problem(), T("Normalise Intensity", "开始强度归一化"), &p.st, a.runNormalize)
		},
	)
}

func (a *app) runNormalize() {
	p := &a.norm
	files := append([]string(nil), p.files.files...)
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
	reference, out, images, saveMasks := p.reference, p.output, o.Images, o.SaveMasks
	p.rows, p.notes = nil, nil
	a.start("normalize", T("Normalising intensity", "正在做强度归一化"), func(ctx context.Context, step func(int, int)) (func(), error) {
		if reference != "" {
			ref, err := core.LoadReference(reference)
			if err != nil {
				return nil, err
			}
			opt.Reference = ref
		}
		if err := os.MkdirAll(out, 0o755); err != nil {
			return nil, err
		}
		var rows []normRow
		var notes []string
		table := "image\tmethod\tfactor\tn_voxels\texcluded_fraction\tmask\n"
		total, done, failed := len(files)*len(methods), 0, 0
		step(0, total)
		for _, path := range files {
			name := filepath.Base(path)
			img, loadErr := core.LoadImage(path)
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
					notes = append(notes, name+" ["+method+"]: "+err.Error())
					step(done, total)
					continue
				}
				for _, w := range res.Warnings {
					notes = append(notes, name+" ["+method+"]: "+w)
				}
				file := core.OutputName(path, label, "pet")
				if images {
					if err := core.SaveNifti(filepath.Join(out, file), res.Image, false); err != nil {
						return nil, err
					}
				}
				if saveMasks && res.Mask != nil {
					maskFile := strings.Replace(file, ".nii.gz", "_mask.nii.gz", 1)
					if err := core.SaveMask(filepath.Join(out, maskFile), res.Mask, img.Dim, img.Aff, ""); err != nil {
						return nil, err
					}
				}
				row := normRow{image: name, method: method, factor: strconv.FormatFloat(res.Factor, 'g', 6, 64), mask: res.MaskName, value: res.Factor}
				if res.Mask != nil {
					row.voxels = strconv.Itoa(res.NVoxels)
				}
				if res.Iterative {
					row.excluded = strconv.FormatFloat(res.ExcludedFraction, 'f', 4, 64)
				}
				if method == "region" && regionText != "" {
					row.mask = regionText
				}
				table += strings.Join([]string{path, method, row.factor, row.voxels, row.excluded, row.mask}, "\t") + "\n"
				if res.Iterative {
					row.excluded = strconv.FormatFloat(res.ExcludedFraction*100, 'f', 1, 64) + "%"
				}
				rows = append(rows, row)
				step(done, total)
			}
		}
		if err := os.WriteFile(filepath.Join(out, "factors.tsv"), []byte(table), 0o644); err != nil {
			return nil, err
		}
		return func() {
			p.rows, p.notes, p.outDir = rows, notes, out
			if failed > 0 {
				p.st.err = Tf("%d of %d failed; see the notes under the results.", "%d / %d 项失败，详见结果下方的说明。", failed, total)
			} else {
				p.st.ok = Tf("%d image(s) intensity-normalised with %d method(s).", "强度归一化已完成：%d 幅图像 × %d 种方法。", len(files), len(methods))
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
