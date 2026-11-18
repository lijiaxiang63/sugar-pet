package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type zmapRow struct {
	image, method, file, lowest, share string
}

type zmapPage struct {
	files      fileList
	methods    map[string]bool
	reference  string
	loadedFrom string
	info       *refInfo
	covTable   string
	iter       iterOptions
	advanced   bool
	output     string

	st     status
	rows   []zmapRow
	table  ui.ListState
	notes  []string
	outDir string
}

func (p *zmapPage) init() {
	p.methods = map[string]bool{"hn": true}
	p.iter = iterOptions{pThreshold: 0.01, maxIter: 1}
}

// allowed tells whether the reference can serve a method.
func (p *zmapPage) allowed(id string) (bool, string) {
	switch id {
	case "ps", "ips", "hn", "ihn":
		return true, ""
	case "region":
		return false, T("Z-score maps cannot use a custom region.", "Z 值图不支持自选区域。")
	}
	if p.info != nil {
		for _, m := range p.info.StatsFor {
			if m == id {
				return true, ""
			}
		}
	}
	return false, T("The reference holds no control statistics for this method. Rebuild it with this method ticked.",
		"参考库中没有该方法的对照统计。请重建参考库并勾选该方法。")
}

func (p *zmapPage) chosen() []string {
	var out []string
	for _, m := range core.Methods {
		if ok, _ := p.allowed(m.Name); ok && p.methods[m.Name] {
			out = append(out, m.Name)
		}
	}
	return out
}

func (p *zmapPage) problem() string {
	switch {
	case len(p.files.files) == 0:
		return T("Add the images to compare with the controls.", "请添加要与对照比较的图像。")
	case p.reference == "":
		return T("Choose a reference, or build one on the Reference page.", "请选择参考库，或先到“参考库”页面建立。")
	case p.info != nil && p.info.err != "":
		return p.info.err
	case len(p.chosen()) == 0:
		return T("Choose at least one method.", "请至少选择一种方法。")
	case p.output == "":
		return T("Choose the output folder.", "请选择输出文件夹。")
	}
	return ""
}

func (a *app) zmapView(c *ui.Context) {
	p := &a.zmap
	if p.reference != p.loadedFrom {
		p.loadedFrom, p.info = p.reference, nil
		if p.reference != "" {
			p.info = readRefInfo(p.reference)
		}
	}
	hasModels := p.info != nil && len(p.info.Covariates) > 0
	a.frame(c, T("Z-score Maps", "Z 值图"),
		T("How far each voxel is from the healthy controls", "每个体素偏离健康对照的程度"),
		func() {
			section(c, T("Images", "图像"), T("Each image is intensity-normalised, smoothed like the controls, and compared with them voxel by voxel. Negative values mean lower uptake.",
				"每幅图像先做强度归一化，按对照的方式平滑，再逐体素与对照比较。负值表示摄取低于对照。"), func() {
				a.filesCard(c, "z-files", &p.files, T("Images", "图像"), T("FDG-PET images in MNI space", "MNI 空间的 FDG-PET 图像"))
			})
			section(c, T("Reference", "参考库"), "", func() {
				desc := T("The healthy controls the images are compared with.", "用于比较的健康对照。")
				if p.info != nil && p.info.err == "" {
					desc = Tf("%d controls, smoothed %g mm.", "%d 例对照，平滑 %g mm。", p.info.N, p.info.FWHM)
				}
				covDesc := T("This reference was built without covariates, so only plain z-scores are available.", "该参考库建立时未提供协变量，只能输出普通 Z 值。")
				if hasModels {
					covDesc = Tf("Optional. With the %s of these images, the maps are W-scores: compared with what the controls predict for such a person.",
						"可选。提供这些图像的 %s 后输出 W 值图：与对照对同样条件的人的预测值比较。", strings.Join(p.info.covariateNames(), T(" and ", "、")))
				}
				var covRow func()
				if hasModels {
					covRow = func() {
						a.pathSetting(c, "z-cov", T("Covariate table", "协变量表"), covDesc, &p.covTable, pathFile, tableFilter, nil)
					}
				} else if p.reference != "" {
					covRow = func() { setting(c, T("Covariate table", "协变量表"), covDesc, nil) }
				}
				rows(c,
					func() {
						a.pathSetting(c, "z-ref", T("Reference", "参考库"), desc, &p.reference, pathFolder, nil,
							func() { a.setReference(p.reference); a.saveSettings() })
					},
					covRow,
				)
			})
			section(c, T("Methods", "方法"), T("Grayed methods need a reference built with them ticked.", "灰色的方法需要在建立参考库时勾选。"), func() {
				a.methodPicker(c, p.methods, p.allowed)
			})
			section(c, T("Output", "输出"), "", func() {
				rows(c, func() {
					a.pathSetting(c, "z-out", T("Output folder", "输出文件夹"), T("One map per image and method, e.g. sub-01_desc-hn_zmap.nii.gz.", "每幅图像每种方法输出一幅，如 sub-01_desc-hn_zmap.nii.gz。"),
						&p.output, pathFolder, nil, nil)
				})
			})
			section(c, "", "", func() {
				options(c, T("Options for iPS and iHN", "iPS 与 iHN 的选项"), &p.advanced, func() {
					ui.Column(c).Children(func() { rows(c, iterRows(c, "z", &p.iter)...) })
				})
			})
			if len(p.rows) > 0 || len(p.notes) > 0 {
				a.results(c, T("Results", "结果"), T("“Below −2” is the share of brain voxels with z < −2 (about 2.3% is expected by chance).", "“低于 −2”为脑内 z < −2 的体素比例（随机情况下约为 2.3%）。"), func() {
					if len(p.rows) > 0 {
						cols := []ui.TableColumn{
							{Title: T("Image", "图像")},
							{Title: T("Method", "方法"), Width: 150},
							{Title: T("Lowest", "最低值"), Width: 80, Align: ui.End},
							{Title: T("Below −2", "低于 −2"), Width: 90, Align: ui.End},
						}
						card(c).Children(func() {
							ui.Table(c, &p.table, cols, len(p.rows), func(row, col int) {
								r := p.rows[row]
								ui.Text(c, [...]string{r.image, methodName(r.method), r.lowest, r.share}[col]).FontSize(12.5).SingleLine()
							}).Height(float32(min(len(p.rows), 8)*32 + 34))
						})
						ui.Row(c).Children(func() { revealButton(c, "z-reveal", revealLabel(), p.outDir) })
					}
					notesCard(c, p.notes)
				})
			}
		},
		func() { a.runBar(c, "zmap", p.problem(), T("Make Maps", "生成 Z 值图"), &p.st, a.runZMap) },
	)
}

func (a *app) runZMap() {
	p := &a.zmap
	files, methods := append([]string(nil), p.files.files...), p.chosen()
	reference, out, table := p.reference, p.output, p.covTable
	if p.info == nil || len(p.info.Covariates) == 0 {
		table = ""
	}
	base := core.DefaultZMapOptions()
	base.PThreshold, base.Exclude, base.MaxIter = p.iter.pThreshold, excludeName(p.iter.exclude), int(p.iter.maxIter)
	p.rows, p.notes = nil, nil
	a.start("zmap", T("Making maps", "正在生成 Z 值图"), func(ctx context.Context, step func(int, int)) (func(), error) {
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
			opt := base
			img, loadErr := core.LoadImage(path)
			if loadErr == nil && covariates != nil {
				found, err := covariates.Lookup([]string{path}, ref.Covariates())
				if err != nil {
					loadErr = err
				} else {
					opt.Covariates = found[0]
				}
			}
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
				z, warnings, err := core.ZMap(img, method, ref, opt)
				for _, w := range warnings {
					notes = append(notes, name+" ["+method+"]: "+w)
				}
				if err != nil {
					failed++
					notes = append(notes, name+" ["+method+"]: "+err.Error())
					step(done, total)
					continue
				}
				suffix := "zmap"
				if opt.Covariates != nil {
					suffix = "wmap"
				}
				file := core.OutputName(path, method, suffix)
				if err := core.SaveNifti(filepath.Join(out, file), z, false); err != nil {
					return nil, err
				}
				lowest, below, inside := math.Inf(1), 0, 0
				for _, v := range z.Data {
					if v != 0 {
						inside++
						lowest = math.Min(lowest, float64(v))
						if v < -2 {
							below++
						}
					}
				}
				row := zmapRow{image: name, method: method, file: file, lowest: "–", share: "–"}
				if inside > 0 {
					row.lowest = strconv.FormatFloat(lowest, 'f', 2, 64)
					row.share = strconv.FormatFloat(100*float64(below)/float64(inside), 'f', 1, 64) + "%"
				}
				rows = append(rows, row)
				step(done, total)
			}
		}
		return func() {
			p.rows, p.notes, p.outDir = rows, notes, out
			if failed > 0 {
				p.st.err = Tf("%d of %d failed; see the notes under the results.", "%d / %d 项失败，详见结果下方的说明。", failed, total)
			} else {
				p.st.ok = Tf("%d map(s) written.", "已输出 %d 幅图。", len(rows))
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
