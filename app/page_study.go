package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

// studyPage creates the folders of a study, laid out as the user guide
// recommends.
type studyPage struct {
	parent string // the folder that holds the study folder
	name   string // the name of the study folder
	st     status
	// The study folder last laid out, and which of its entries were created
	// then (the others were there already).
	made    string
	created []bool
	// inUse is the study folder the pages write into, kept between launches.
	inUse string
	// statusOf is the study folder that st speaks of.
	statusOf string
}

func (p *studyPage) init() { p.name = "study" }

// target is the study folder as chosen, or "" while it is not chosen.
func (p *studyPage) target() string {
	name := strings.TrimSpace(p.name)
	if p.parent == "" || name == "" {
		return ""
	}
	return filepath.Join(p.parent, name)
}

func (p *studyPage) problem() string {
	name := strings.TrimSpace(p.name)
	switch {
	case p.parent == "":
		return T("Choose where to create the study folder.", "请选择研究文件夹建在哪里。")
	case name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:`):
		return T(`Give the study folder a name, without / \ or :.`, `请填写研究文件夹的名称，不能含 / \ 或 :。`)
	}
	return ""
}

// What each entry of a study folder is for, as shown.
var studyLabels = map[string]label{
	"raw": {"Images still in native scanner space. Only needed if yours are not in MNI space yet: Spatial Normalisation warps them into patients and controls.",
		"仍在原始扫描空间的图像。只有图像尚未配准到 MNI 空间时才需要：“空间标准化”把它们配准后写入 patients 和 controls。"},
	"patients": {"Patient images in MNI space, one scan per subject, e.g. sub-pt01_space-MNI_pet.nii.gz.",
		"MNI 空间的患者图像，每人一幅，如 sub-pt01_space-MNI_pet.nii.gz。"},
	"controls": {"Healthy-control images in MNI space, from the same scanner and preprocessing as the patients.",
		"MNI 空间的健康对照图像，须与患者来自同一台扫描仪、经过相同的预处理。"},
	"participants.tsv": {"Age and sex of every subject, patients and controls, one row each. Choose it as the covariate table to adjust for them.",
		"所有受试者（患者和对照）的年龄和性别，每人一行。在各页面选作协变量表，用于校正年龄和性别。"},
	"hc_reference": {"The reference that the Reference page builds from the controls. Most other pages read it.",
		"“参考库”页面用健康对照建立的参考库，其他页面大多要用到它。"},
	"normalised": {"Intensity-normalised images and factors.tsv, from Intensity Normalisation.",
		"“强度归一化”输出的归一化图像和 factors.tsv。"},
	"zmaps": {"Z-score or W-score maps of the patients, from Z-score Maps.",
		"“Z 值图”输出的患者 Z 值图或 W 值图。"},
	"reports": {"Classification reports of the patients, from Classify Parkinsonism.",
		"“帕金森综合征分类”输出的患者分类报告。"},
	"compare": {"T-maps and cluster tables of patients against controls, from Compare Groups.",
		"“组间比较”输出的 T 图和团块表。"},
}

// studyRow is an entry of the study folder: its name, what it is for and,
// once the folder is laid out, whether it was created.
func studyRow(c *ui.Context, e core.StudyEntry, state string, created bool) {
	pal := paletteFor(c)
	ic, name := iconFolder, e.Name+"/"
	if !e.Dir {
		ic, name = iconTable, e.Name
	}
	ui.Row(c).Padding(10, 14).Gap(12).MinHeight(46).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, ic).Size(15, 15).TextColor(pal.accent).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, name).FontSize(12.5).Font("monospace").FontWeight(600).SingleLine()
			ui.Text(c, studyLabels[e.Name].String()).FontSize(12).TextColor(pal.muted).LineHeight(1.35)
		})
		if state != "" {
			color := pal.muted
			if created {
				color = pal.success
			}
			pill(c, state, color)
		}
	})
}

func (a *app) studyView(c *ui.Context) {
	p := &a.study
	target := p.target()
	// How the last run went says nothing of another folder.
	if p.statusOf != target {
		p.st, p.statusOf = status{}, target
	}
	// What the last run made is shown while its folder is still the one chosen.
	shown := target != "" && target == p.made && len(p.created) == len(core.StudyLayout)
	a.frame(c, T("Study Folder", "研究文件夹"),
		T("The recommended folders of a study, created in one go", "一次建好一项研究推荐的文件夹结构"),
		func() {
			section(c, T("Where", "位置"), T("Everything below is created inside one study folder. Whatever is there already is kept as it is.",
				"下面的内容都建在同一个研究文件夹里。已经存在的内容保持原样，不会被改动。"), func() {
				where := T("The name of the study folder.", "研究文件夹的名称。")
				if target != "" {
					where = Tf("The study folder is %s", "研究文件夹为 %s", shortDir(target))
				}
				rows(c,
					func() {
						a.pathSetting(c, "study-parent", T("Location", "存放位置"),
							T("The folder that holds the study folder.", "研究文件夹建在这个文件夹里。"), &p.parent, pathFolder, nil, nil)
					},
					func() { setting(c, T("Name", "名称"), where, func() { ui.TextInput(c, &p.name).Width(210) }) },
				)
				if p.inUse != "" && target != "" && target != p.inUse {
					caution(c, Tf("The pages still write into %s. Creating the folders here makes them write into this folder, and takes the images, tables and reference of %[1]s off them.",
						"各页面目前仍写入 %s。在这里创建文件夹之后，各页面改为写入这个文件夹，并移除其中属于 %[1]s 的图像、表格和参考库。", shortDir(p.inUse)))
				}
			})
			group := func(title, caption string, yours bool) {
				section(c, title, caption, func() {
					var list []func()
					for i, e := range core.StudyLayout {
						if e.Yours != yours {
							continue
						}
						list = append(list, func() {
							state, created := "", false
							if shown {
								state, created = T("Already there", "原已存在"), p.created[i]
								if created {
									state = T("Created", "已创建")
								}
							}
							studyRow(c, e, state, created)
						})
					}
					rows(c, list...)
				})
			}
			group(T("For you to fill", "由你放入"), T("Put your data here before running the analyses.", "运行分析之前，把你的数据放到这里。"), true)
			group(T("Written by the analyses", "由各项分析写入"), T("Once the folders are created, each is the output folder of its page.", "创建之后，它们各自成为对应页面的输出文件夹。"), false)
			if shown {
				ui.Row(c).Children(func() { revealButton(c, "study-reveal", revealLabel(), p.made) })
			}
		},
		func() { a.runBar(c, "study", p.problem(), T("Create Folders", "创建文件夹"), &p.st, a.runStudy) })
}

func (a *app) runStudy() {
	p := &a.study
	target := p.target()
	created, err := core.InitStudy(target)
	if err != nil {
		// What an earlier run made of this folder no longer holds.
		p.st.err, p.made, p.created = err.Error(), "", nil
		return
	}
	p.made, p.created = target, created
	a.useStudy(target)
	n := 0
	for _, yes := range created {
		if yes {
			n++
		}
	}
	p.st.ok = Tf("Study folder ready: %d created, %d already there. The pages now write into it.",
		"研究文件夹已就绪：新建 %d 项，%d 项原已存在。各页面的输出已指向这里。", n, len(created)-n)
}

// sameDir reports whether two paths name the same folder, however they are
// spelt: through a link, or in other letter case where that is the same.
func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// insideDir reports whether a file or folder lies in a folder (or is it).
func insideDir(path, dir string) bool {
	if path == "" || dir == "" {
		return false
	}
	if rel, err := filepath.Rel(dir, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	for p := path; ; {
		if sameDir(p, dir) {
			return true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
}

// studyOutputs makes a study folder the one the pages write into: each
// page's output is its folder of the study.  Spatial Normalisation is left
// alone, as it writes patients and controls into different folders.
func (a *app) studyOutputs(study string) {
	in := func(name string) string { return filepath.Join(study, name) }
	a.ref.output, a.norm.output, a.zmap.output = in("hc_reference"), in("normalised"), in("zmaps")
	a.classify.output, a.cmp.output = in("reports"), in("compare")
	a.cluster.output = in("reference_cluster.nii.gz")
	a.study.inUse = study
}

// useStudy goes over to a study folder, as its page does when it has laid
// the folder out.  The pages write into it, and what they hold of the
// study left behind is taken off them, so that the images of one study are
// not written into another, or compared with its controls.  The reference
// the study holds is used if none is in use; one chosen from elsewhere is
// kept.
func (a *app) useStudy(study string) {
	if left := a.study.inUse; left != "" && !sameDir(left, study) {
		a.leaveStudy(left)
	}
	a.studyOutputs(study)
	if own := filepath.Join(study, "hc_reference"); a.norm.reference == "" && core.IsReferenceFolder(own) {
		a.setReference(own)
	}
}

// leaveStudy takes off the pages what lies in a study folder that is no
// longer the one in use: its images, its tables and its reference.
func (a *app) leaveStudy(left string) {
	known := map[string]bool{} // by folder: the images of a study lie in few
	in := func(path string) bool {
		if path == "" {
			return false
		}
		dir := filepath.Dir(path)
		yes, ok := known[dir]
		if !ok {
			yes = insideDir(dir, left)
			known[dir] = yes
		}
		return yes
	}
	sp := &a.spatial
	for _, list := range []*fileList{&sp.files, &a.norm.files, &a.ref.files, &a.zmap.files, &a.classify.files, &a.cmp.g1, &a.cmp.g2, &a.cluster.files} {
		var kept []string
		for _, f := range list.files {
			if !in(f) {
				kept = append(kept, f)
			}
		}
		list.files = kept
	}
	for _, path := range []*string{&a.zmap.covTable, &a.classify.covTable, &a.cmp.covExtra} {
		if in(*path) {
			*path = ""
		}
	}
	for _, cov := range []*covChoice{&a.ref.cov, &a.cmp.cov} {
		if in(cov.path) {
			cov.path = ""
			cov.load()
		}
	}
	if insideDir(sp.output, left) {
		sp.output = ""
	}
	if insideDir(a.norm.reference, left) {
		a.setReference("")
	}
}

// restoreStudy writes into the study folder of the last launch again, if it
// is still there.  The reference is not touched: the one the app kept is
// the one that was in use, be it none, or one that cannot be reached now.
func (a *app) restoreStudy(study string) {
	if info, err := os.Stat(study); study == "" || err != nil || !info.IsDir() {
		return
	}
	a.study.parent, a.study.name = filepath.Dir(study), filepath.Base(study)
	a.studyOutputs(study)
}
