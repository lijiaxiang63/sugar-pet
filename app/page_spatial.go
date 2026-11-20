package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type spatialRow struct {
	image, output string
	path          string // the image in MNI space
	failed        bool
}

// spatialPage warps native-space PET to MNI space with SPM, the step before
// everything else in the app for images that are not there yet.
type spatialPage struct {
	files  fileList
	spm    string
	spmBad bool   // the folder chosen is not SPM12 or SPM25
	matlab string // chosen by the user; "" is the MATLAB found on this computer
	// warped is what the last run made of each image: the image in MNI
	// space, by the image in native space.  The pages after this one read
	// these in place of the images of the project.
	warped map[string]string

	st      status
	rows    []spatialRow
	table   ui.ListState
	notes   []string
	written []string
	outDir  string
	shown   int // the row the picture is of
	pic     picture
}

// foundMatlab is the MATLAB installed on this computer, "" if none.
var foundMatlab = sync.OnceValue(core.FindMatlab)

func (p *spatialPage) setSPM(path string) {
	p.spm, p.spmBad = path, path != "" && core.CheckSPM(path) != nil
}

func (p *spatialPage) matlabPath() string {
	if p.matlab != "" {
		return p.matlab
	}
	return foundMatlab()
}

// ready is whether SPM and MATLAB are set up.
func (p *spatialPage) ready() bool {
	return p.spm != "" && !p.spmBad && p.matlabPath() != ""
}

// setupProblem is what the set-up of SPM and MATLAB lacks, which the
// settings mend, or "".
func (p *spatialPage) setupProblem() string {
	switch {
	case p.spm == "":
		return T("Set up SPM first: choose the folder of SPM12 or SPM25.", "请先设置 SPM：选择 SPM12 或 SPM25 的文件夹。")
	case p.spmBad:
		return T("The folder chosen is not SPM12 or SPM25: it has no spm_jobman.m or no toolbox/OldNorm.",
			"所选文件夹不是 SPM12 或 SPM25：其中没有 spm_jobman.m 或 toolbox/OldNorm。")
	case p.matlabPath() == "":
		return T("MATLAB was not found on this computer: choose it under SPM and MATLAB.", "未在本机找到 MATLAB：请在“SPM 与 MATLAB”中选择。")
	}
	return ""
}

// spatialTarget is where the image in MNI space of an image of the project
// is written: the folder of its group.
func (a *app) spatialTarget(path string) string {
	group := "patients"
	if slices.Contains(a.proj.controls.files, path) {
		group = "controls"
	}
	return filepath.Join(a.proj.in(group), core.SpatialOutputName(path))
}

// notNative is what the page says of a project whose images are in MNI
// space already.
func notNative() string {
	return T("Not needed: the images of this project are in MNI space already.", "无需此步：本项目的图像已在 MNI 空间。")
}

func (a *app) spatialProblem() string {
	p := &a.spatial
	switch {
	case !a.proj.native:
		return notNative()
	case len(p.files.files) == 0:
		return a.dataProblem()
	case p.setupProblem() != "":
		return p.setupProblem()
	}
	targets := map[string]string{}
	for _, path := range p.files.files {
		target := a.spatialTarget(path)
		if other, ok := targets[target]; ok {
			return Tf("%s and %s would both be written as %s: rename one of them.",
				"%s 与 %s 都会输出为 %s：请重命名其中一个。",
				filepath.Join(filepath.Base(filepath.Dir(other)), filepath.Base(other)),
				filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path)), filepath.Base(target))
		}
		targets[target] = path
	}
	return ""
}
func (a *app) spatialView(c *ui.Context) {
	p := &a.spatial
	t := c.Theme()
	pal := paletteFor(c)
	a.frame(c, T("Spatial Normalisation", "空间标准化"),
		T("Warp native-space PET into MNI space with SPM", "用 SPM 把原始空间的 PET 配准到 MNI 空间"),
		func() {
			section(c, T("Images", "图像"), T("The images of the project as they come from the scanner; no MRI is needed. 4D images are averaged over time. What is warped is written into the patients and controls folders of the project, and the pages after this one read it from there.",
				"项目中扫描得到的原始空间 FDG-PET 图像，不需要 MRI；4D 图像按时间取平均。配准结果写入项目的 patients 和 controls 文件夹，之后的页面都从那里读取。"), func() {
				if !a.proj.native {
					a.source(c, "sp-data", "data", changeInData(), fact(c, T("Space of the images", "图像所在的空间"), notNative()))
					return
				}
				done := func(files []string) string {
					n := 0
					for _, f := range files {
						if p.warped[f] != "" {
							n++
						}
					}
					return countText(files) + Tf(" · %d warped", " · 已配准 %d 幅", n)
				}
				a.source(c, "sp-data", "data", changeInData(),
					fact(c, T("Patients", "患者"), done(a.proj.patients.files)),
					fact(c, T("Healthy controls", "健康对照"), done(a.proj.controls.files)))
			})
			section(c, T("Set-up and output", "设置与输出"), "", func() {
				rows(c,
					func() {
						ready := p.ready()
						desc := T("The warp is SPM's “Old Normalise”, run by MATLAB in the background. Tell the app once where they are.",
							"配准使用 SPM 的 “Old Normalise”，由 MATLAB 在后台运行。只需告诉应用一次它们的位置。")
						label, kind := T("Set Up…", "设置…"), buttonPrimary
						if ready {
							desc = "SPM: " + shortDir(p.spm) + " · MATLAB: " + shortDir(p.matlabPath())
							label, kind = T("Change…", "更改…"), buttonGhost
						}
						setting(c, T("SPM and MATLAB", "SPM 与 MATLAB"), desc, func() {
							if ready {
								ui.Icon(c, iconCheck).Size(15, 15).TextColor(pal.success)
							}
							if button(c.Key("sp-setup"), label, nil, kind, 26).Clicked() {
								a.openSettings("spatial")
							}
						})
					},
					fact(c, T("Written to", "输出位置"), T("patients/ and controls/ · 2 mm voxels, named like sub-01_space-MNI_pet.nii.gz", "patients/ 与 controls/ · 体素 2 mm，文件名形如 sub-01_space-MNI_pet.nii.gz")),
				)
				a.optionsLine(c, "spatial", "spatial", groupSpatial)
			})
			if len(p.rows) > 0 {
				a.results(c, T("Results", "结果"), "", func() {
					if len(p.written) > 0 {
						caution(c, T("Look at every image before going on: a warp from PET alone can go wrong without an error. The brain should fill the outline of the template.",
							"继续之前请逐幅目视检查：只凭 PET 的配准可能出错而不报错。脑组织应当填满模板的轮廓。"))
					}
					p.shown = min(max(p.shown, 0), len(p.rows)-1)
					picker(c, "sp-pick", &p.shown, len(p.rows), p.rows[p.shown].image)
					if r := p.rows[p.shown]; !r.failed {
						a.show(&p.pic, r.path, imagePicture(r.path, true))
						a.figure(c, &p.pic, r.output, legend{kind: legendOutline, text: T("Outline of the template's brain", "模板的脑轮廓")})
					}
					cols := []ui.TableColumn{
						{Title: T("Image", "图像")},
						{Title: T("In MNI space", "MNI 空间图像")},
					}
					p.table.Selected = &p.shown
					card(c).Children(func() {
						ui.Table(c, &p.table, cols, len(p.rows), func(row, col int) {
							r := p.rows[row]
							text := ui.Text(c, [...]string{r.image, r.output}[col]).FontSize(12.5).SingleLine()
							if col == 1 && r.failed && row != p.shown {
								text.TextColor(t.Danger)
							}
						}).Height(float32(min(len(p.rows), 6)*32 + 34))
					})
					ui.Row(c).Children(func() { revealButton(c, "sp-reveal", revealLabel(), p.outDir) })
					notesCard(c, p.notes)
				})
			}
		},
		func() {
			a.runBar(c, "spatial", a.spatialProblem(), T("Warp to MNI Space", "开始空间标准化"), &p.st, a.runSpatial)
			a.next(c, "reference", a.spatialDone())
		},
	)
}

func (a *app) runSpatial() {
	p := &a.spatial
	files, out := append([]string(nil), p.files.files...), a.proj.dir
	targets := make([]string, len(files))
	for i, path := range files {
		targets[i] = a.spatialTarget(path)
	}
	opt := core.SpatialOptions{SPMPath: p.spm, Template: a.opt.Template, Matlab: p.matlabPath(), Jobs: int(a.opt.Jobs)}
	p.rows, p.notes, p.written, p.shown = nil, nil, nil, 0
	a.start("spatial", T("Warping to MNI space", "正在空间标准化"), func(ctx context.Context, step func(int, int)) (func(), error) {
		for _, target := range targets {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return nil, err
			}
		}
		opt.Progress = step
		results, err := core.SpatialNormalize(ctx, files, targets, opt)
		if err != nil {
			return nil, err
		}
		var rows []spatialRow
		var notes, written []string
		warped := map[string]string{}
		for _, r := range results {
			row := spatialRow{image: filepath.Base(r.Input), output: filepath.Base(r.Output), path: r.Output}
			if r.Err != nil {
				row.output, row.failed = T("Failed", "失败"), true
				notes = append(notes, row.image+": "+r.Err.Error())
			} else {
				written = append(written, r.Output)
				warped[r.Input] = r.Output
			}
			rows = append(rows, row)
		}
		return func() {
			p.rows, p.notes, p.written, p.outDir, p.warped = rows, notes, written, out, warped
			for i, r := range rows { // the picture is of the first that worked
				if !r.failed {
					p.shown = i
					break
				}
			}
			if failed := len(results) - len(written); failed > 0 {
				p.st.err = Tf("%d of %d failed; see the notes under the results.", "%d / %d 项失败，详见结果下方的说明。", failed, len(results))
			} else {
				p.st.ok = Tf("%d image(s) warped to MNI space.", "已完成：%d 幅图像已配准到 MNI 空间。", len(written))
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
