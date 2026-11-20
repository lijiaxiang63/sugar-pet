package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type comparePage struct {
	g1, g2    fileList
	method    string
	reference string
	region    regionChoice
	// The smoothing of the reference, as last read from its folder.
	fwhmKnown bool
	fwhmOf    string
	fwhmRef   *float64
	name      string
	output    string

	// swapped is whether the groups were swapped: the controls of the
	// project are then group 1 and its patients group 2.
	swapped bool
	// The method last taken from the individual analysis (follow): it is
	// taken again when chosen anew there, until the user chooses another
	// here.
	methodFrom string

	st     status
	result *core.GroupComparison
	how    string
	table  ui.ListState
	pic    picture
}

// swap makes group 1 group 2 and the other way round.  The result on the
// page goes: its T-map has the sign of the groups as they were.
func (p *comparePage) swap() {
	p.g1.files, p.g2.files = p.g2.files, p.g1.files
	p.g1.query, p.g2.query = p.g2.query, p.g1.query
	p.swapped = !p.swapped
	p.result, p.how, p.st = nil, "", status{}
}

// followCompare takes the method of the individual analysis, so that it is
// not chosen again here.
func (a *app) followCompare() {
	p := &a.cmp
	if m := leadingMethod(a.norm.chosen()); m != "" && m != p.methodFrom {
		if p.method == p.methodFrom {
			p.method = m
		}
		p.methodFrom = m
	}
}
func (p *comparePage) init() {
	p.method, p.methodFrom, p.name = "hn", "hn", "patients_vs_controls"
	p.region.mask = "brainstem"
}

// referenceFWHM is the smoothing (mm) of the chosen reference, if there is
// one that can be read.
func (p *comparePage) referenceFWHM() (fwhm float64, ok bool) {
	if !p.fwhmKnown || p.fwhmOf != p.reference {
		p.fwhmKnown, p.fwhmOf, p.fwhmRef = true, p.reference, nil
		if p.reference != "" {
			if info := readRefInfo(p.reference); info.err == "" {
				p.fwhmRef = &info.FWHM
			}
		}
	}
	if p.fwhmRef == nil {
		return 0, false
	}
	return *p.fwhmRef, true
}

// smoothing is the smoothing (mm) before the test.  Left to the app, it is
// that of the chosen reference, whether or not the method needs one, or 8 mm
// without a reference.
func (p *comparePage) smoothing(o *options) float64 {
	if !o.CmpFWHMAuto {
		return o.CmpFWHM
	}
	if fwhm, ok := p.referenceFWHM(); ok {
		return fwhm
	}
	return 8
}

// referenceProblem is what is missing while the method needs a reference
// and none is chosen, or "".
func (p *comparePage) referenceProblem() string {
	if !core.NeedsReference(p.method) || p.reference != "" {
		return ""
	}
	return Tf("%s needs a reference: build it on the Reference page, or import one there.", "%s 需要参考库：请先到“参考库”页面建立，或导入已有的参考库。", methodName(p.method))
}

func (a *app) cmpProblem() string {
	p, o := &a.cmp, &a.opt
	for i, g := range [][]string{p.g1.files, p.g2.files} {
		group := [...]string{"patients", "controls"}[i]
		if p.swapped {
			group = [...]string{"controls", "patients"}[i]
		}
		switch {
		case len(g) == 0:
			return a.needImages(group)
		case len(g) < 2:
			return T("Each group needs at least 2 images: add more on the Data page.", "每组至少需要 2 幅图像：请到“数据”页面添加。")
		}
	}
	switch {
	case p.referenceProblem() != "":
		return p.referenceProblem()
	case p.method == "region" && p.region.problem() != "":
		return p.region.problem()
	case o.Analysis == "file" && o.AnalysisFile == "":
		return T("Choose the mask file of the voxels to test in the settings.", "请在设置中选择检验范围的掩膜文件。")
	}
	if msg := a.cov.problem(); msg != "" {
		return msg
	}
	if strings.TrimSpace(p.name) == "" || strings.ContainsAny(p.name, `/\:`) {
		return T("Give the result files a name (without slashes).", "请填写结果文件的名称（不含斜杠）。")
	}
	return ""
}
func (a *app) compareView(c *ui.Context) {
	p := &a.cmp
	a.followCompare()
	t := c.Theme()
	a.frame(c, T("Compare Groups", "组间比较"),
		T("Voxel-wise two-sample t-test after intensity normalisation", "强度归一化后逐体素双样本 t 检验"),
		func() {
			section(c, T("Groups", "分组"), T("The T-map is group 1 minus group 2: with patients as group 1 and controls as group 2, negative values mean hypometabolism.",
				"T 图为第 1 组减第 2 组：以患者为第 1 组、对照为第 2 组时，负值表示低代谢。"), func() {
				names := [2]string{T("Patients", "患者"), T("Healthy controls", "健康对照")}
				if p.swapped {
					names[0], names[1] = names[1], names[0]
				}
				a.source(c, "cmp-data", "data", changeInData(),
					fact(c, T("Group 1", "第 1 组")+" · "+names[0], countText(p.g1.files)),
					fact(c, T("Group 2", "第 2 组")+" · "+names[1], countText(p.g2.files)))
				ui.Row(c).Justify(ui.End).Children(func() {
					// Not while the comparison runs: it would end under
					// the groups the other way round.
					swap := button(c.Key("cmp-swap"), T("Swap Groups", "互换两组"), iconSwap, buttonGhost, 26).
						Tooltip(T("Swap Group 1 and Group 2 (reverses the sign of the T-map)", "交换第 1 组与第 2 组（将反转 T 图正负号）")).
						Disabled(a.job != nil && a.job.page == "compare")
					if swap.Clicked() {
						p.swap()
					}
				})
			})
			section(c, T("Intensity normalisation", "强度归一化"), "", func() {
				var ids, names []string
				for _, m := range core.Methods {
					ids, names = append(ids, m.Name), append(names, methodName(m.Name))
				}
				list := []func(){func() {
					setting(c, T("Method", "方法"), methodAbout(p.method), func() { selectOf(c, &p.method, ids, names).Width(210) })
				}}
				if p.method == "region" {
					list = append(list, a.regionRows(c, "cmp-region", &p.region)...)
				}
				list = append(list, a.referenceFact(c))
				a.source(c, "cmp-ref", "reference", changeInReference(), list...)
			})
			section(c, T("Covariates", "协变量"), T("The comparison adjusts for the columns chosen on the Data page; the table must list every image of both groups.", "按“数据”页面选定的列做校正；表格须包含两组的全部图像。"), func() {
				a.source(c, "cmp-cov", "data", changeInData(), fact(c, T("Adjusted for", "校正的列"), a.covText()))
			})
			section(c, T("Output", "输出"), "", func() {
				rows(c,
					fact(c, T("Written to", "输出位置"), a.writesTo(p.output)),
					func() {
						setting(c, T("Name of the result files", "结果文件名称"), "", func() { ui.TextInput(c, &p.name).Width(210) })
					},
				)
				a.optionsLine(c, "compare", "compare", groupCompare)
			})
			if r := p.result; r != nil {
				a.results(c, T("Results", "结果"), p.how, func() {
					items := [][2]string{
						{strconv.FormatFloat(r.DOF, 'g', 6, 64), T("Degrees of freedom", "自由度")},
						{fmt.Sprintf("%.1f × %.1f × %.1f", r.FWHM[0], r.FWHM[1], r.FWHM[2]), T("Smoothness (mm)", "平滑度 (mm)")},
					}
					if r.Corrected {
						items = append(items, [2]string{strconv.Itoa(r.NClusters), T("Significant clusters", "显著团块")})
					}
					stats(c, items...)
					if thresholded := r.File("thresholded"); r.Corrected && thresholded != "" {
						a.show(&p.pic, thresholded, overlayPicture(thresholded, core.PictureOptions{From: 1e-6, Full: 8}))
						a.figure(c, &p.pic, T("T-map of the clusters that survive the correction", "通过校正的团块的 T 图"), legend{kind: legendScores, unit: "T", from: "0", full: "8"})
					} else {
						a.show(&p.pic, r.TMap, overlayPicture(r.TMap, core.PictureOptions{From: 3, Full: 8}))
						a.figure(c, &p.pic, T("T-map, uncorrected, where |T| > 3", "未校正的 T 图，仅显示 |T| > 3"), legend{kind: legendScores, unit: "T", from: "3", full: "8"})
					}
					if len(r.Clusters) > 0 {
						cols := []ui.TableColumn{
							{Title: "#", Width: 36, Align: ui.End},
							{Title: T("Voxels", "体素数"), Width: 70, Align: ui.End},
							{Title: T("Peak", "峰值"), Width: 70, Align: ui.End},
							{Title: "MNI (x, y, z)", Width: 130},
							{Title: T("Peak region (AAL)", "峰值所在脑区 (AAL)"), Width: 170},
							{Title: T("Regions covered (AAL)", "覆盖的脑区 (AAL)")},
						}
						card(c).Children(func() {
							ui.Table(c, &p.table, cols, len(r.Clusters), func(row, col int) {
								cl := r.Clusters[row]
								text := ""
								switch col {
								case 0:
									text = strconv.Itoa(row + 1)
								case 1:
									text = strconv.Itoa(cl.Size)
								case 2:
									text = core.PeakText(cl.Peak)
								case 3:
									text = fmt.Sprintf("%g, %g, %g", cl.MNI[0], cl.MNI[1], cl.MNI[2])
								case 4:
									text = cl.PeakAtlas[0]
								case 5:
									text = core.RegionsText(cl.Regions[0])
								}
								e := ui.Text(c, text).FontSize(12.5).SingleLine()
								if col == 2 && cl.Peak < 0 {
									e.TextColor(paletteFor(c).cool)
								} else if col == 2 {
									e.TextColor(t.Danger)
								}
								if col == 5 {
									e.Tooltip(text)
								}
							}).Height(float32(min(len(r.Clusters), 8)*32 + 34))
						})
					}
					ui.Row(c).Gap(8).Children(func() {
						revealButton(c, "cmp-reveal", revealLabel(), r.TMap)
						if report := r.File("cluster_report"); report != "" {
							if smallButton(c, T("Open Cluster Report", "打开团块报告")).Clicked() {
								openPath(report)
							}
						}
					})
				})
			}
		},
		func() { a.runBar(c, "compare", a.cmpProblem(), T("Compare", "开始比较"), &p.st, a.runCompare) },
	)
}

func (a *app) runCompare() {
	p := &a.cmp
	g1, g2 := append([]string(nil), p.g1.files...), append([]string(nil), p.g2.files...)
	opt := core.DefaultCompareOptions()
	o := a.opt
	opt.Method, opt.Name, opt.KeepImages = p.method, strings.TrimSpace(p.name), o.KeepImages
	opt.Correction = [...]string{"grf", "fdr", "none"}[o.Correction]
	opt.VoxelP, opt.ClusterP, opt.Q = o.VoxelP, o.ClusterP, o.Q
	p.fwhmKnown = false // the reference may have been built anew
	fwhm := p.smoothing(&o)
	opt.FWHM = &fwhm
	opt.AnalysisMask = core.MaskSpec{Name: o.Analysis}
	if o.Analysis == "file" {
		opt.AnalysisMask = core.MaskSpec{Name: o.AnalysisFile}
	}
	if p.method == "region" {
		spec, _, err := p.region.spec()
		if err != nil {
			p.st.err = err.Error()
			return
		}
		opt.Mask = spec
	}
	if msg := a.cov.fresh(); msg != "" {
		p.st.err = msg
		return
	}
	if a.cov.table != nil {
		names := a.cov.names()
		found, err := a.cov.table.Lookup(append(append([]string(nil), g1...), g2...), names)
		if err != nil {
			p.st.err = err.Error()
			return
		}
		opt.Covariates = core.CovariatesFromRows(found, names)
	}
	reference, out := p.reference, p.output
	if !core.NeedsReference(p.method) && p.method != "ps" {
		reference = ""
	}
	p.result = nil
	a.start("compare", T("Normalising the intensity of the images", "正在对图像做强度归一化"), func(ctx context.Context, step func(int, int)) (func(), error) {
		if reference != "" {
			ref, err := core.LoadReference(reference)
			if err != nil {
				return nil, err
			}
			opt.Reference = ref
		}
		total := len(g1) + len(g2)
		opt.Progress = func(done, _ int) {
			step(done, total)
			if done == total {
				a.update(func() {
					if a.job != nil {
						a.job.label, a.job.total = T("Fitting the model", "正在拟合模型"), 0
					}
				})
			}
		}
		step(0, total)
		res, err := core.CompareGroups(ctx, g1, g2, out, opt)
		if err != nil {
			return nil, err
		}
		return func() {
			p.result = res
			switch {
			case !res.Corrected:
				p.how = T("No correction was applied; only the T-map was written.", "未做校正，仅输出了 T 图。")
			case opt.Correction == "grf":
				p.how = Tf("GRF correction: voxel p < %g, cluster p < %g, clusters of at least %d voxels (|z| > %.2f).",
					"GRF 校正：体素 p < %g，团块 p < %g，团块至少 %d 个体素（|z| > %.2f）。", opt.VoxelP, opt.ClusterP, res.MinClusterVoxels, res.ZThreshold)
			default:
				p.how = Tf("FDR correction: q < %g (p < %.3g).", "FDR 校正：q < %g（p < %.3g）。", opt.Q, res.PThreshold)
			}
			if res.Corrected {
				p.st.ok = Tf("%d cluster(s) survive the correction.", "有 %d 个团块通过校正。", res.NClusters)
			} else {
				p.st.ok = T("T-map written.", "已输出 T 图。")
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
