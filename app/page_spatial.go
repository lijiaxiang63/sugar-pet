package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type spatialRow struct {
	image, output string
	failed        bool
}

// spatialPage warps native-space PET to MNI space with SPM, the step before
// everything else in the app for images that are not there yet.
type spatialPage struct {
	files    fileList
	spm      string
	spmBad   bool   // the folder chosen is not SPM12 or SPM25
	matlab   string // chosen by the user; "" is the MATLAB found on this computer
	template string
	jobs     float64
	output   string
	advanced bool

	st      status
	rows    []spatialRow
	table   ui.ListState
	notes   []string
	written []string
	outDir  string
}

// foundMatlab is the MATLAB installed on this computer, "" if none.
var foundMatlab = sync.OnceValue(core.FindMatlab)

func (p *spatialPage) init() {
	p.jobs = float64(min(max(runtime.NumCPU()/2, 1), 4))
}

func (p *spatialPage) setSPM(path string) {
	p.spm, p.spmBad = path, path != "" && core.CheckSPM(path) != nil
}

func (p *spatialPage) matlabPath() string {
	if p.matlab != "" {
		return p.matlab
	}
	return foundMatlab()
}

func (p *spatialPage) problem() string {
	switch {
	case len(p.files.files) == 0:
		return T("Add the images to bring into MNI space.", "请添加要配准到 MNI 空间的图像。")
	case p.spm == "":
		return T("Choose the folder of SPM12 or SPM25.", "请选择 SPM12 或 SPM25 的文件夹。")
	case p.spmBad:
		return T("The folder chosen is not SPM12 or SPM25: it has no spm_jobman.m or no toolbox/OldNorm.",
			"所选文件夹不是 SPM12 或 SPM25：其中没有 spm_jobman.m 或 toolbox/OldNorm。")
	case p.matlabPath() == "":
		return T("MATLAB was not found on this computer: choose it above.", "未在本机找到 MATLAB：请在上方选择。")
	case p.output == "":
		return T("Choose the output folder.", "请选择输出文件夹。")
	}
	names := map[string]string{}
	for _, path := range p.files.files {
		name := core.SpatialOutputName(path)
		if other, ok := names[name]; ok {
			return Tf("%s and %s would both be written as %s: rename one, or run them separately.",
				"%s 与 %s 都会输出为 %s：请重命名其中一个，或分开运行。",
				filepath.Join(filepath.Base(filepath.Dir(other)), filepath.Base(other)),
				filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path)), name)
		}
		names[name] = path
	}
	return ""
}

func (a *app) spatialView(c *ui.Context) {
	p := &a.spatial
	t := c.Theme()
	a.frame(c, T("Spatial Normalisation", "空间标准化"),
		T("Warp native-space PET into MNI space with SPM", "用 SPM 把原始空间的 PET 配准到 MNI 空间"),
		func() {
			section(c, T("Images", "图像"), T("FDG-PET images as they come from the scanner; no MRI is needed. Skip this page if your images are in MNI space already.",
				"扫描得到的原始空间 FDG-PET 图像，不需要 MRI。图像已在 MNI 空间的可跳过本页。"), func() {
				a.filesCard(c, "sp-files", &p.files, T("Native-space images", "原始空间的图像"),
					T("or use the buttons above · .nii and .nii.gz · 4D images are averaged over time", "也可使用上方按钮添加 · 支持 .nii 与 .nii.gz · 4D 图像按时间取平均"))
			})
			section(c, T("SPM and MATLAB", "SPM 与 MATLAB"), T("The warp is SPM's “Old Normalise” onto a PET template, run by MATLAB in the background. No other page needs them.",
				"配准使用 SPM 的 “Old Normalise” 和 PET 模板，由 MATLAB 在后台运行。其他页面都不需要它们。"), func() {
				rows(c,
					func() {
						a.pathSetting(c, "sp-spm", T("SPM folder", "SPM 文件夹"), T("The folder of SPM12 or SPM25, the one holding spm.m.", "SPM12 或 SPM25 的文件夹，即 spm.m 所在的文件夹。"),
							&p.spm, pathFolder, nil, func() { p.setSPM(p.spm); a.saveSettings() })
					},
					func() {
						desc := T("Not found on this computer: choose the MATLAB program.", "未在本机找到：请选择 MATLAB 程序。")
						if p.matlab != "" {
							desc = ""
						} else if found := foundMatlab(); found != "" {
							desc = T("Found: ", "已找到：") + shortDir(found)
						}
						a.pathSetting(c, "sp-matlab", "MATLAB", desc, &p.matlab, pathFile, nil, func() {
							if _, err := core.ResolveMatlab(p.matlab); p.matlab != "" && err != nil {
								p.matlab = ""
								a.alert(T("Not MATLAB", "不是 MATLAB"), T("Choose the MATLAB program: the MATLAB app on macOS, bin/matlab elsewhere.",
									"请选择 MATLAB 程序：macOS 上为 MATLAB 应用，其他系统为 bin/matlab。"))
							}
							a.saveSettings()
						})
					},
				)
			})
			section(c, T("Output", "输出"), "", func() {
				rows(c, func() {
					a.pathSetting(c, "sp-out", T("Output folder", "输出文件夹"),
						T("MNI-space images of 2 mm voxels, named like sub-01_space-MNI_pet.nii.gz.", "体素 2 mm 的 MNI 空间图像，文件名形如 sub-01_space-MNI_pet.nii.gz。"),
						&p.output, pathFolder, nil, nil)
				})
			})
			section(c, "", "", func() {
				ui.Collapsible(c, T("Options", "选项"), &p.advanced, func() {
					ui.Column(c).Gap(14).Padding(8, 0, 0, 0).Children(func() {
						rows(c,
							func() {
								a.pathSetting(c, "sp-template", T("PET template", "PET 模板"),
									T("In MNI space. SPM's own PET template unless you choose one; the mean of your own warped controls usually fits better.",
										"须在 MNI 空间。不选则用 SPM 自带的 PET 模板；用自己已配准对照的均值图通常配得更好。"),
									&p.template, pathFile, niftiFilter, nil)
							},
							func() {
								setting(c, T("MATLAB processes", "MATLAB 进程数"), T("Images are shared out among them; each is a MATLAB of its own.", "图像分给各进程同时处理；每个进程都是一个独立的 MATLAB。"),
									func() { number(c, &p.jobs, 1, float64(max(runtime.NumCPU(), 1)), 1) })
							},
						)
					})
				})
			})
			if len(p.rows) > 0 {
				a.results(c, T("Results", "结果"), "", func() {
					cols := []ui.TableColumn{
						{Title: T("Image", "图像")},
						{Title: T("In MNI space", "MNI 空间图像")},
					}
					card(c).Children(func() {
						ui.Table(c, &p.table, cols, len(p.rows), func(row, col int) {
							r := p.rows[row]
							text := ui.Text(c, [...]string{r.image, r.output}[col]).FontSize(12.5).SingleLine()
							if col == 1 && r.failed {
								text.TextColor(t.Danger)
							}
						}).Height(float32(min(len(p.rows), 8)*32 + 34))
					})
					if len(p.written) > 0 {
						ui.Row(c).Radius(10).Padding(10, 14).Gap(10).AlignItems(ui.Start).Background(t.Warning.Alpha(0.1)).Border(1, t.Warning.Alpha(0.3)).Children(func() {
							ui.Icon(c, iconWarn).Size(15, 15).TextColor(t.Warning).Shrink(0).Margin(1, 0, 0, 0)
							ui.Text(c, T("Look at every image before going on: a warp from PET alone can go wrong without an error.",
								"继续之前请逐幅目视检查：只凭 PET 的配准可能出错而不报错。")).FontSize(12).Grow(1).MinWidth(0)
						})
						ui.Row(c).Gap(8).Children(func() {
							b := ui.PrimaryButton(c, T("Use for Intensity Normalisation", "用于强度归一化")).FontSize(12).Padding(3, 12)
							if b.Clicked() {
								a.norm.files.files = nil
								a.norm.files.add(p.written)
								a.page = "normalize"
							}
							revealButton(c, "sp-reveal", revealLabel(), p.outDir)
						})
					}
					notesCard(c, p.notes)
				})
			}
		},
		func() {
			a.runBar(c, "spatial", p.problem(), T("Warp to MNI Space", "开始空间标准化"), &p.st, a.runSpatial)
		},
	)
}

func (a *app) runSpatial() {
	p := &a.spatial
	files, out := append([]string(nil), p.files.files...), p.output
	opt := core.SpatialOptions{SPMPath: p.spm, Template: p.template, Matlab: p.matlabPath(), Jobs: int(p.jobs)}
	p.rows, p.notes, p.written = nil, nil, nil
	a.start("spatial", T("Warping to MNI space", "正在空间标准化"), func(ctx context.Context, step func(int, int)) (func(), error) {
		if err := os.MkdirAll(out, 0o755); err != nil {
			return nil, err
		}
		targets := make([]string, len(files))
		for i, path := range files {
			targets[i] = filepath.Join(out, core.SpatialOutputName(path))
		}
		opt.Progress = step
		results, err := core.SpatialNormalize(ctx, files, targets, opt)
		if err != nil {
			return nil, err
		}
		var rows []spatialRow
		var notes, written []string
		for _, r := range results {
			row := spatialRow{image: filepath.Base(r.Input), output: filepath.Base(r.Output)}
			if r.Err != nil {
				row.output, row.failed = T("Failed", "失败"), true
				notes = append(notes, row.image+": "+r.Err.Error())
			} else {
				written = append(written, r.Output)
			}
			rows = append(rows, row)
		}
		return func() {
			p.rows, p.notes, p.written, p.outDir = rows, notes, written, out
			if failed := len(results) - len(written); failed > 0 {
				p.st.err = Tf("%d of %d failed; see the notes under the results.", "%d / %d 项失败，详见结果下方的说明。", failed, len(results))
			} else {
				p.st.ok = Tf("%d image(s) warped to MNI space.", "已完成：%d 幅图像已配准到 MNI 空间。", len(written))
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
