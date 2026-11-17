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
}

type normPage struct {
	files     fileList
	methods   map[string]bool
	reference string
	region    regionChoice
	statistic int // 0 mean, 1 median
	target    float64
	fwhmAuto  bool
	fwhm      float64
	iter      iterOptions
	literal   bool
	images    bool // write the normalised images
	saveMasks bool
	output    string
	advanced  bool

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

// iterOptions are the settings of the iterative methods.
type iterOptions struct {
	pThreshold float64
	exclude    int // 0 hypo, 1 both
	maxIter    float64
}

func (p *normPage) init() {
	p.methods = map[string]bool{"hn": true}
	p.region.mask = "brainstem"
	p.target, p.fwhmAuto, p.fwhm = 1, true, 8
	p.iter = iterOptions{pThreshold: 0.01, maxIter: 1}
	p.images = true
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
			ui.Segmented(c.Key(key+"-kind"), &r.kind, T("Built-in mask", "内置掩膜"), T("Mask file", "掩膜文件"), T("Atlas labels", "图谱标签"))
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

// iterRows are the settings of the iterative methods.
func iterRows(c *ui.Context, key string, o *iterOptions) []func() {
	return []func(){
		func() {
			setting(c, T("Exclusion p-value", "剔除体素的 p 值"),
				T("Voxels that differ from the controls at this uncorrected p are left out.", "与对照相比达到该未校正 p 值的体素被剔除。"),
				func() { number(c.Key(key+"-p"), &o.pThreshold, 0.0001, 0.5, 0.001) })
		},
		func() {
			setting(c, T("Exclude", "剔除范围"), "", func() {
				ui.Segmented(c.Key(key+"-excl"), &o.exclude, T("Hypometabolic only", "仅低代谢"), T("Hypo- and hypermetabolic", "低代谢和高代谢"))
			})
		},
		func() {
			setting(c, T("Re-estimations", "重新估计次数"), T("The paper uses 1.", "文章中为 1 次。"),
				func() { number(c.Key(key+"-iter"), &o.maxIter, 0, 20, 1) })
		},
	}
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

// methodPicker is a card of check boxes, one per method.
func (a *app) methodPicker(c *ui.Context, chosen map[string]bool, allowed func(id string) (ok bool, why string)) {
	t := c.Theme()
	card(c).Children(func() {
		for g, group := range methodGroups {
			if g > 0 {
				ui.Box(c).Height(1).Margin(0, 0, 0, 14).Background(hairline(t))
			}
			ui.Column(c).Padding(10, 14, 10, 14).Gap(6).Children(func() {
				ui.Text(c, group.title.String()).FontSize(11.5).TextColor(t.TextMuted)
				ui.Row(c).Wrap().GapY(4).Children(func() {
					for _, id := range group.methods {
						ok, why := true, ""
						if allowed != nil {
							ok, why = allowed(id)
						}
						on := chosen[id] && ok
						cell := ui.Row(c.Key(id)).WidthPercent(33.33).MinWidth(180).Gap(6).AlignItems(ui.Center).Height(26)
						cell.Children(func() {
							box := ui.Checkbox(c, &on, methodName(id)).Disabled(!ok)
							if ok {
								box.Tooltip(methodAbout(id))
							} else {
								box.Tooltip(why)
							}
							if id == "hn" {
								ui.Text(c, T("Recommended", "推荐")).FontSize(10).TextColor(t.Accent).
									Padding(1, 6).Radius(999).Background(t.Accent.Alpha(0.13)).SingleLine()
							}
						})
						if ok {
							chosen[id] = on
						}
					}
				})
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
		return T("Add the images to normalise.", "请添加要归一化的图像。")
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
	a.frame(c, T("Normalise", "强度归一化"),
		T("Divide each image by its normalisation factor", "每幅图像除以各自的归一化因子"),
		func() {
			section(c, T("Images", "图像"), T("FDG-PET images that are already in MNI space.", "已配准到 MNI 空间的 FDG-PET 图像。"), func() {
				a.filesCard(c, "norm-files", &p.files, T("Images to normalise", "待归一化的图像"),
					T("or use the buttons above · .nii and .nii.gz", "也可使用上方按钮添加 · 支持 .nii 与 .nii.gz"))
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
							&p.reference, pathFolder, nil, func() { a.setReference(p.reference); a.saveSettings() })
					},
					func() {
						a.pathSetting(c, "norm-out", T("Output folder", "输出文件夹"),
							T("Normalised images and factors.tsv are written here.", "归一化图像和 factors.tsv 写入此处。"),
							&p.output, pathFolder, nil, nil)
					},
				)
			})
			section(c, "", "", func() {
				ui.Collapsible(c, T("Options", "选项"), &p.advanced, func() {
					ui.Column(c).Gap(14).Padding(8, 0, 0, 0).Children(func() {
						rows(c,
							func() {
								setting(c, T("Region statistic", "区域统计量"), T("For the reference regions and PS.", "用于参考区法和 PS。"), func() {
									ui.Segmented(c, &p.statistic, T("Mean", "均值"), T("Median", "中位数"))
								})
							},
							func() {
								setting(c, T("Target value", "目标值"), T("The reference takes this value in the output, e.g. 50 for “global mean = 50”.", "输出图像中参考值等于该数，例如 50 即“全局均值 = 50”。"),
									func() { number(c, &p.target, 0.001, 100000, 1) })
							},
							func() {
								setting(c, T("Smoothing before estimating", "估计因子前的平滑"),
									T("Automatic: that of the reference for HN, iPS and iHN, none otherwise. The output is never smoothed.", "自动：HN、iPS、iHN 采用参考库的平滑，其他方法不平滑。输出图像不会被平滑。"),
									func() {
										ui.Checkbox(c, &p.fwhmAuto, T("Automatic", "自动"))
										number(c, &p.fwhm, 0, 30, 1).Disabled(p.fwhmAuto).Tooltip("FWHM, mm")
									})
							},
						)
						rows(c, append(iterRows(c, "norm", &p.iter), func() {
							setting(c, T("Plain mean for iPS", "iPS 用普通均值"), T("As written in the paper, instead of the mean relative to the control template.", "按文章字面实现，而非相对对照模板的均值。"),
								func() { ui.Switch(c, &p.literal).Label(T("Plain mean for iPS", "iPS 用普通均值")) })
						})...)
						rows(c,
							func() {
								setting(c, T("Write normalised images", "输出归一化图像"), T("Turn off to write only the table of factors.", "关闭后只输出因子表。"),
									func() { ui.Switch(c, &p.images).Label(T("Write normalised images", "输出归一化图像")) })
							},
							func() {
								setting(c, T("Save masks", "保存掩膜"), T("Also save the voxels each factor came from.", "同时保存估计每个因子所用的体素。"),
									func() { ui.Switch(c, &p.saveMasks).Label(T("Save masks", "保存掩膜")) })
							},
						)
					})
				})
			})
			if len(p.rows) > 0 || len(p.notes) > 0 {
				a.results(c, T("Results", "结果"), "", func() {
					if len(p.rows) > 0 {
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
			a.runBar(c, "normalize", p.problem(), T("Normalise", "开始归一化"), &p.st, a.runNormalize)
		},
	)
}

func (a *app) runNormalize() {
	p := &a.norm
	files := append([]string(nil), p.files.files...)
	methods := p.chosen()
	opt := core.DefaultNormOptions()
	if !p.fwhmAuto {
		fwhm := p.fwhm
		opt.FWHM = &fwhm
	}
	if p.statistic == 1 {
		opt.Statistic = "median"
	}
	opt.Target, opt.PThreshold, opt.Exclude = p.target, p.iter.pThreshold, excludeName(p.iter.exclude)
	opt.MaxIter, opt.LiteralIPS = int(p.iter.maxIter), p.literal
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
	reference, out, images, saveMasks := p.reference, p.output, p.images, p.saveMasks
	p.rows, p.notes = nil, nil
	a.start("normalize", T("Normalising", "正在归一化"), func(ctx context.Context, step func(int, int)) (func(), error) {
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
				o := opt
				label := method
				if method == "region" {
					o.Mask, label = regionSpec, regionLabel
				}
				res, err := core.Normalize(img, method, o)
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
				row := normRow{image: name, method: method, factor: strconv.FormatFloat(res.Factor, 'g', 6, 64), mask: res.MaskName}
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
				p.st.ok = Tf("%d image(s) normalised with %d method(s).", "已完成：%d 幅图像 × %d 种方法。", len(files), len(methods))
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
