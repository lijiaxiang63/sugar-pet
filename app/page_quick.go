package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

// quickPage classifies one more patient against the controls of the
// project, the way the patients of the project are: the image is warped
// into MNI space (unless it is there already), intensity-normalised with
// the primary method of step 1 and the settings of the project, mapped
// against the control statistics and classified.  The patient is not added
// to the project: no step of it changes, the project file records nothing
// of the run, and what it writes lies in a folder of its own, quick/<name>.
type quickPage struct {
	output string // the folder the runs write into, a folder for each image

	input string
	space int // 0: native space, warped first; 1: MNI space
	// covs are the covariates of the patient, by name: all of them give
	// W-scores, none plain z-scores.
	covs map[string]string

	st      status
	res     *quickResult
	mapPic  picture
	warpPic picture
}

// quickResult is what the last run that went well made of the image.
type quickResult struct {
	row           classifyRow // its source is the image in MNI space
	method        string
	folder        string
	native        bool // the image was warped
	lowest, share string
	notes         []string
}

func (p *quickPage) init() { p.covs = map[string]string{} }

// quickValues are the covariates of the patient, nil while none is given:
// the map is of z-scores then.
func (a *app) quickValues() map[string]string {
	info := a.refInfo()
	if info == nil || info.err != "" || len(info.Covariates) == 0 {
		return nil
	}
	values := map[string]string{}
	for _, term := range info.Covariates {
		if v := strings.TrimSpace(a.quick.covs[term.Name]); v != "" {
			values[term.Name] = v
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

// quickStale is what the page says while the control statistics are not
// what the controls and the settings make.
func (a *app) quickStale() string {
	return Tf("The control statistics are to be summarised anew: run %s first.", "对照统计需要重新汇总：请先运行%s。", a.rebuiltWhen())
}

// quickMethod is what the page says while the primary method of step 1
// makes no map.
func quickMethod(method string) string {
	return Tf("Quick Classification takes %s, the primary method of step 1, which makes no z-score map: choose another primary method in step 1.",
		"快速分类沿用第 1 步的主方法%s，但它不能制作 Z 值图：请在第 1 步换一种主方法。", methodName(method))
}

// quickProblem is what keeps the page from running, or "".
func (a *app) quickProblem() string {
	p := &a.quick
	// The image is treated as step 1 treated those of the project.
	if _, why := a.lock("zmap"); why != "" {
		return why
	}
	info := a.refInfo()
	switch {
	case a.reference == "":
		return needReference()
	case info.err != "":
		return info.err
	case a.refStale():
		return a.quickStale()
	}
	if ok, _ := a.zmapAllowed(a.norm.primary()); !ok {
		return quickMethod(a.norm.primary())
	}
	if p.input == "" {
		return T("Choose the PET image of the patient.", "请选择该患者的 PET 图像。")
	}
	if p.space == 0 {
		if msg := a.spatial.setupProblem(); msg != "" {
			return msg
		}
	}
	given := 0
	for _, term := range info.Covariates {
		v := strings.TrimSpace(p.covs[term.Name])
		if v == "" {
			continue
		}
		given++
		if _, err := strconv.ParseFloat(v, 64); term.Type == "numeric" && err != nil {
			return Tf("%s must be a number.", "%s 必须是数字。", term.Name)
		}
	}
	if given > 0 && given < len(info.Covariates) {
		return T("Give every covariate of the patient, or none for a map of plain z-scores.", "请填写该患者的全部协变量，或全部留空以输出普通 Z 值图。")
	}
	return ""
}

func (a *app) quickView(c *ui.Context) {
	p := &a.quick
	info := a.refInfo()
	a.frame(c, T("Quick Classification", "快速分类"),
		T("One more patient, classified against the controls of the project", "用本项目的对照为另一例患者分类"),
		func() {
			section(c, T("Patient", "患者"), T("One FDG-PET scan from outside the project. It is not added to the patients and no step of the project changes: it only goes through the steps they went through.",
				"项目之外的一例 FDG-PET 图像。它不会加入患者列表，项目的各个步骤也不受影响：它只是走一遍项目患者走过的流程。"), func() {
				changed := func() { p.res, p.st = nil, status{} }
				list := []func(){
					func() {
						a.pathSetting(c, "quick-image", T("PET image", "PET 图像"), T("A NIfTI image of the patient.", "该患者的 NIfTI 图像。"),
							&p.input, pathFile, niftiFilter, changed)
					},
					func() {
						desc := T("Warped into MNI space with SPM first, as Spatial Normalisation does; 4D images are averaged over time.",
							"先用 SPM 配准到 MNI 空间，与“空间标准化”相同；4D 图像按时间取平均。")
						if p.space == 1 {
							desc = T("Taken as it is: it must be on the grid of the images of the project.", "直接使用：须与项目图像处于同一网格。")
						}
						setting(c, T("The image is in", "图像所在的空间"), desc, func() {
							before := p.space
							segmented(c.Key("quick-space"), &p.space, T("Native Space", "原始空间"), T("MNI Space", "MNI 空间"))
							if p.space != before {
								changed()
							}
						})
					},
				}
				// The covariates the control statistics model, for W-scores.
				if info != nil && info.err == "" {
					for _, term := range info.Covariates {
						list = append(list, func() {
							value := p.covs[term.Name]
							desc := T("For W-scores, as the controls were modelled with it. Leave every covariate empty for plain z-scores.",
								"对照统计对它建了模型，填写后输出 W 值；全部留空则输出普通 Z 值图。")
							setting(c, term.Name, desc, func() {
								if term.Type == "categorical" && len(term.Levels) > 0 {
									ids := append([]string{""}, term.Levels...)
									names := append([]string{T("Not given", "未填")}, term.Levels...)
									selectOf(c.Key("quick-cov-"+term.Name), &value, ids, names).Width(120)
								} else {
									ui.TextInput(c.Key("quick-cov-"+term.Name), &value).Placeholder(T("Not given", "未填")).Width(120)
								}
							})
							if value != p.covs[term.Name] {
								p.covs[term.Name] = value
								changed()
							}
						})
					}
				}
				rows(c, list...)
			})
			section(c, T("From the project", "沿用本项目"), T("The image is normalised and mapped as step 1 and step 2 did those of the patients, and classified with the settings of Classify Parkinsonism.",
				"按第 1、2 步处理项目患者的方式做强度归一化并制作 Z 值图，再按“帕金森综合征分类”的设置分类。"), func() {
				method := methodName(a.norm.primary())
				if checks := a.classifyChecks(a.norm.primary()); len(checks) > 0 {
					method += Tf(" · checked with %s", " · 另以%s复核", methodNames(checks))
				}
				written := filepath.Join("quick", T("<name of the image>", "<图像名>"))
				if p.input != "" {
					written = filepath.Join("quick", core.ReportStem(p.input))
				}
				a.source(c, "quick-norm", "normalize", T("Open Step 1", "查看第 1 步"),
					fact(c, T("Method", "方法"), method),
					a.referenceFact(c),
					fact(c, T("Written to", "输出位置"), written+T(" · not part of the project", " · 不属于项目")))
			})
			caution(c, T("Decision support, not a diagnosis. The patterns are matched by fixed rules from the literature that were not validated on your patients. Read the result with the clinical findings, MRI and dopamine-transporter imaging.",
				"本功能仅为辅助决策，不构成诊断。模式匹配依据文献中的固定规则，未经本单位患者数据验证。结果须结合临床表现、MRI 和多巴胺转运体显像综合判断。"))
			if r := p.res; r != nil {
				a.results(c, T("Result", "结果"), "", func() {
					card(c).Children(func() {
						ui.Row(c).Padding(11, 14).Gap(12).AlignItems(ui.Center).Children(func() { readingLine(c, r.row) })
					})
					unit := "z"
					if r.row.wscore {
						unit = "W"
					}
					stats(c,
						[2]string{r.lowest, Tf("Lowest %s", "最低 %s 值", unit)},
						[2]string{r.share, Tf("Brain voxels with %s < −2", "%s < −2 的脑内体素", unit)},
						[2]string{methodName(r.method), T("Method", "方法")},
					)
					a.show(&p.mapPic, r.row.zmap, mapPicture(r.row.source, r.row.zmap))
					a.figure(c, &p.mapPic, Tf("%s map · %s", "%s 值图 · %s", unit, r.row.image), legend{kind: legendScores, unit: unit, from: "2", full: "5"})
					if r.native {
						caution(c, T("Look at the warp before trusting the map: a warp from PET alone can go wrong without an error. The brain should fill the outline of the template.",
							"采信 Z 值图之前请先检查配准：只凭 PET 的配准可能出错而不报错。脑组织应当填满模板的轮廓。"))
						a.show(&p.warpPic, r.row.source, imagePicture(r.row.source, true))
						a.figure(c, &p.warpPic, filepath.Base(r.row.source), legend{kind: legendOutline, text: T("Outline of the template's brain", "模板的脑轮廓")})
					}
					ui.Row(c).Children(func() { revealButton(c, "quick-reveal", revealLabel(), r.folder) })
					notesCard(c, r.notes)
				})
			}
		},
		func() {
			a.runBar(c, "quick", a.quickProblem(), T("Classify", "开始分类"), &p.st, a.runQuick)
		},
	)
}

func (a *app) runQuick() {
	p := &a.quick
	if a.normCurrent() == nil { // the button is disabled while step 1 is not done
		return
	}
	input, native, method := p.input, p.space == 0, a.norm.primary()
	stem := core.ReportStem(input)
	folder := filepath.Join(p.output, stem)
	reference, values := a.reference, a.quickValues()
	opt, sopt := a.normOptions(), a.spatialOptions()
	sopt.Jobs = 1
	checked, lang := a.opt.CrossCheck, a.opt.reportLang()
	p.res = nil
	p.mapPic.forget()
	p.warpPic.forget()
	label := T("Classifying", "正在分类")
	first := label
	if native {
		first = T("Warping to MNI space", "正在空间标准化")
	}
	a.start("quick", first, func(ctx context.Context, step func(int, int)) (func(), error) {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return nil, err
		}
		// Into MNI space, as Spatial Normalisation warps the images of the
		// project.
		warped := input
		if native {
			warped = filepath.Join(folder, core.SpatialOutputName(input))
			sopt.Progress = step
			results, err := core.SpatialNormalize(ctx, []string{input}, []string{warped}, sopt)
			if err != nil {
				return nil, err
			}
			if err := results[0].Err; err != nil {
				return nil, err
			}
			a.update(func() {
				if a.job != nil {
					a.job.label, a.job.done, a.job.total = label, 0, 0
				}
			})
		}
		step(0, 3)
		ref, err := core.LoadReference(reference)
		if err != nil {
			return nil, err
		}
		img, err := core.LoadImage(warped)
		if err != nil {
			return nil, err
		}
		// Intensity Normalisation, as step 1 normalised the patients.
		opt.Reference = ref
		res, err := core.Normalize(img, method, opt)
		if err != nil {
			return nil, err
		}
		normalisedFile := filepath.Join(folder, core.OutputName(warped, method, "pet"))
		if err := core.SaveNifti(normalisedFile, res.Image, false); err != nil {
			return nil, err
		}
		// The map is made of the image as it was written, as step 2 reads
		// what step 1 wrote.
		normalised, err := core.LoadImage(normalisedFile)
		if err != nil {
			return nil, err
		}
		step(1, 3)
		z, warnings, err := core.ZMapOf(normalised, method, res.Factor, opt.Target, ref, values)
		if err != nil {
			return nil, err
		}
		suffix := "zmap"
		if values != nil {
			suffix = "wmap"
		}
		mapFile := filepath.Join(folder, core.OutputName(warped, method, suffix))
		if err := core.SaveNifti(mapFile, z, false); err != nil {
			return nil, err
		}
		scores, err := core.LoadImage(mapFile)
		if err != nil {
			return nil, err
		}
		step(2, 3)
		// The classification, as step 3 reads the maps of step 2.
		copt := classifyOptions(checked)
		copt.Covariates = values
		in := core.Scored{Normalised: normalised, Target: opt.Target, Factors: map[string]float64{method: res.Factor},
			Maps: map[string]*core.Volume{method: scores}}
		cls, err := core.ClassifyScored(in, filepath.Base(input), method, ref, copt)
		if err != nil {
			return nil, err
		}
		cls.MapFile = mapFile
		saved, err := cls.Save(folder, stem, lang)
		if err != nil {
			return nil, err
		}
		step(3, 3)
		result := &quickResult{row: classifyRowOf(cls, saved, input), method: method, folder: folder, native: native}
		result.row.source = warped // the map lies over the image in MNI space
		for _, w := range slices.Concat(res.Warnings, warnings, cls.Warnings) {
			if !slices.Contains(result.notes, w) {
				result.notes = append(result.notes, w)
			}
		}
		result.lowest, result.share = scoreSummary(z)
		return func() {
			p.res = result
			p.st.ok = Tf("%s classified; the report is written.", "已完成 %s 的分类，并生成报告。", filepath.Base(input))
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}

// scoreSummary is the lowest score of a map and the share of the brain
// voxels below −2, as the results of Z-score Maps give them.
func scoreSummary(z *core.Volume) (lowest, share string) {
	least, below, inside := math.Inf(1), 0, 0
	for _, v := range z.Data {
		if v != 0 {
			inside++
			least = math.Min(least, float64(v))
			if v < -2 {
				below++
			}
		}
	}
	if inside == 0 {
		return "–", "–"
	}
	return strconv.FormatFloat(least, 'f', 2, 64), strconv.FormatFloat(100*float64(below)/float64(inside), 'f', 1, 64) + "%"
}
