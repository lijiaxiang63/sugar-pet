package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// inUse is the study folder the pages write into, kept between launches:
	// the folder itself, not a link to it, which may lead elsewhere later.
	inUse string
	// inUseIs tells that folder from another that has its name later.
	inUseIs os.FileInfo
	// last is the study folder of the last launch, if it cannot be reached
	// now.  The settings may still hold files of it.
	last string
	// Whether the folder chosen is the one in use under another spelling:
	// asked on every frame, answered by the file system once per choice,
	// and again after a while, as a link may come to lead elsewhere.
	sameKey string
	same    bool
	sameAt  time.Time
	// statusOf is the study folder that st speaks of.
	statusOf string
	// chosen is the study folder picked as it is (choose): while location
	// and name are still its own, it is the target to the letter, also if
	// its name begins or ends with a space, which a typed name does not.
	chosen string
}

func (p *studyPage) init() { p.name = "study" }

// target is the study folder as chosen, or "" while it is not chosen.
func (p *studyPage) target() string {
	name := strings.TrimSpace(p.name)
	if p.parent == "" || name == "" {
		return ""
	}
	if p.chosen != "" && filepath.Join(p.parent, p.name) == p.chosen {
		return p.chosen
	}
	return filepath.Join(p.parent, name)
}

// choose makes a study folder that is there already the one chosen: its
// location and its name, as if they had been filled in.  Nothing is written
// and the pages are left alone until the folders are created.
func (p *studyPage) choose(study string) {
	p.parent, p.name, p.chosen = filepath.Dir(study), filepath.Base(study), study
}

// holds reports whether a folder is the study in use, however it is spelt:
// the folder the pages went over to, not another that has its name by now.
func (p *studyPage) holds(study string) bool {
	if study == "" || p.inUse == "" {
		return false
	}
	if p.inUseIs == nil {
		return sameDir(study, p.inUse)
	}
	info, err := os.Stat(study)
	return err == nil && os.SameFile(p.inUseIs, info)
}

// kept is the study folder to use again at the next launch.
func (p *studyPage) kept() string {
	if p.inUse != "" {
		return p.inUse
	}
	return p.last
}

// chosenInUse reports whether the folder chosen is the study in use.
func (p *studyPage) chosenInUse(target string) bool {
	if target == "" || p.inUse == "" {
		return false
	}
	if key := target + "\x00" + p.inUse; key != p.sameKey || time.Since(p.sameAt) > time.Second {
		p.sameKey, p.same, p.sameAt = key, p.holds(target), time.Now()
	}
	return p.same
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
					func() {
						// Choosing the folder writes nothing and leaves the pages as
						// they are: it is Create Folders that goes over to it, after
						// the caution below has said what that takes off the pages.
						setting(c, T("Already have a study folder?", "已有研究文件夹？"),
							T("Choose the folder itself to fill in its location and name. Create Folders then adds only what is missing.",
								"直接选择该文件夹，自动填好存放位置和名称。之后点“创建文件夹”只补上缺少的部分。"),
							func() {
								if button(c.Key("choose-existing-study"), T("Choose Study Folder…", "选择研究文件夹…"), iconFolder, buttonSoft, 26).Clicked() {
									a.pick(pickOptions{Title: T("Choose an existing study folder", "选择已有的研究文件夹"), Directory: true}, func(paths []string) {
										p.choose(paths[0])
									})
								}
							})
					},
				)
				switch {
				case p.inUse == "" || p.chosenInUse(target):
				case target == "":
					caution(c, Tf("The pages write into %s.", "各页面目前写入 %s。", shortDir(p.inUse)))
				default:
					caution(c, Tf("The pages still write into %s. Creating the folders here makes them write into this folder, and takes the images, tables, masks, reference and results of %[1]s off them.",
						"各页面目前仍写入 %s。在这里创建文件夹之后，各页面改为写入这个文件夹，并移除其中属于 %[1]s 的图像、表格、掩膜、参考库和运行结果。", shortDir(p.inUse)))
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
				ui.Row(c).Gap(8).Wrap().GapY(6).Children(func() {
					revealButton(c, "study-reveal", revealLabel(), p.made)
					if button(c.Key("study-to-ref"), T("Go to Reference", "前往参考库"), iconArrowRight, buttonSoft, 26).Clicked() {
						a.page = "reference"
					}
					if button(c.Key("study-to-norm"), T("Go to Intensity Normalisation", "前往强度归一化"), iconArrowRight, buttonSoft, 26).Clicked() {
						a.page = "normalize"
					}
				})
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
	below := func(path, dir string) bool {
		rel, err := filepath.Rel(dir, path)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if below(path, dir) {
		return true
	}
	// A link elsewhere may lead into the folder.
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		real = path
	} else if realDir, err := filepath.EvalSymlinks(dir); err == nil && below(real, realDir) {
		return true
	}
	// The folder may be spelt otherwise: in other letter case, where that
	// is the same folder.  It is then known by what it is, not by its name.
	above := func(p string) bool {
		for {
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
	return above(path) || real != path && above(real)
}

// studyOutputs makes a study folder the one the pages write into: each
// page's output is its folder of the study.  Spatial Normalisation is left
// alone, as it writes patients and controls into different folders.
//
// The pages are given the folder itself.  Given a link, they would write
// the results of this study into another once the link leads there.
func (a *app) studyOutputs(study string) {
	if real, err := filepath.EvalSymlinks(study); err == nil {
		study = real
	}
	in := func(name string) string { return filepath.Join(study, name) }
	a.ref.output, a.norm.output, a.zmap.output = in("hc_reference"), in("normalised"), in("zmaps")
	a.classify.output, a.cmp.output = in("reports"), in("compare")
	a.cluster.output = in("reference_cluster.nii.gz")
	a.study.inUse, a.study.inUseIs, a.study.last, a.study.sameKey = study, nil, "", ""
	a.seen = nil // what the folders hold is asked anew: they may have been made just now
	if info, err := os.Stat(study); err == nil {
		os.SameFile(info, info) // Windows reads what the folder is only now, by its name
		a.study.inUseIs = info
	}
}

// useStudy goes over to a study folder, as its page does when it has laid
// the folder out.  The pages write into it, and what they hold of the
// study left behind is taken off them, so that the images of one study are
// not written into another, or compared with its controls.  The reference
// the study holds is used if none is in use; one chosen from elsewhere is
// kept.
func (a *app) useStudy(study string) {
	// The study left is the folder the pages went over to, also if this one
	// has its name by now, or is led to by the same link.  A study that was
	// given another name is left under its old one: what the pages hold of
	// it is named so, and is no longer there.
	switch p := &a.study; {
	case p.inUse != "":
		if !p.holds(study) || !sameDir(p.inUse, study) {
			a.leaveStudy(p.inUse)
		}
	case p.last != "" && !sameDir(p.last, study):
		a.leaveStudy(p.last)
	}
	a.studyOutputs(study)
	own := filepath.Join(a.study.inUse, "hc_reference")
	switch held := core.IsReferenceFolder(own); {
	case a.norm.reference == "":
		if held {
			a.setReference(own)
		}
	case !held && sameDir(a.norm.reference, own):
		// The study's own reference is gone: its folder was made anew, empty.
		a.setReference("")
	}
}

// leaveStudy takes off the pages what lies in a study folder that is no
// longer the one in use: its images, its tables, its reference, and the
// results of the runs that wrote into it.
func (a *app) leaveStudy(left string) {
	a.forgetResults(left)
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
		if !yes {
			// A file that is itself a link may lead into the study.
			if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
				yes = insideDir(path, left)
			}
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
	// The mask of a custom region too: the reference cluster of a study is
	// made from its patients.  And the files chosen in the settings: a
	// template or a mask of one study would be used on the next unseen.
	for _, path := range []*string{&a.zmap.covTable, &a.classify.covTable, &a.cmp.covExtra, &a.norm.region.file, &a.cmp.region.file,
		&a.opt.Template, &a.opt.RefMaskFile, &a.opt.AnalysisFile} {
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

// forgetResults takes off the pages the results of the runs that wrote into
// a study folder, and how those runs went.  What a page offers to do with
// its results, such as normalising the images it warped, goes with them.
func (a *app) forgetResults(left string) {
	// Where a page's last run wrote or, if it wrote nothing, would have.
	wrote := func(outDir, output string) bool {
		if outDir == "" {
			outDir = output
		}
		return insideDir(outDir, left)
	}
	if p := &a.spatial; wrote(p.outDir, p.output) {
		p.rows, p.notes, p.written, p.outDir, p.shown, p.st = nil, nil, nil, "", 0, status{}
		p.pic.forget()
	}
	if p := &a.norm; wrote(p.outDir, p.output) {
		p.rows, p.notes, p.outDir, p.st = nil, nil, "", status{}
	}
	if p := &a.ref; wrote(p.built, p.output) {
		p.built, p.st = "", status{}
	}
	if p := &a.zmap; wrote(p.outDir, p.output) {
		p.rows, p.notes, p.outDir, p.shown, p.st = nil, nil, "", 0, status{}
		p.pic.forget()
	}
	if p := &a.classify; wrote(p.outDir, p.output) {
		p.rows, p.notes, p.outDir, p.shown, p.st = nil, nil, "", 0, status{}
		p.pic.forget()
	}
	if p := &a.cmp; p.result != nil && insideDir(p.result.TMap, left) || p.result == nil && insideDir(p.output, left) {
		p.result, p.how, p.st = nil, "", status{}
		p.pic.forget()
	}
	if p := &a.cluster; wrote(p.saved, p.output) {
		p.saved, p.volume, p.st = "", 0, status{}
		p.pic.forget()
	}
	if p := &a.masks; insideDir(p.outDir, left) {
		p.outDir, p.st = "", status{}
	}
	// How a run failed is of no use once its study is left, and a page does
	// not know where a run that failed was to write.
	for _, st := range []*status{&a.spatial.st, &a.norm.st, &a.ref.st, &a.zmap.st, &a.classify.st, &a.cmp.st, &a.cluster.st, &a.masks.st} {
		st.err = ""
	}
}

// restoreStudy writes into the study folder of the last launch again, if it
// is still there.  The reference is not touched: the one the app kept is
// the one that was in use, be it none, or one that cannot be reached now.
// A study that cannot be reached, as on a drive not plugged in, is kept in
// mind: it is used again once it can be, and left when another is used.
func (a *app) restoreStudy(study string) {
	if info, err := os.Stat(study); study == "" || err != nil || !info.IsDir() {
		a.study.last = study
		return
	}
	a.study.parent, a.study.name = filepath.Dir(study), filepath.Base(study)
	a.studyOutputs(study)
}
