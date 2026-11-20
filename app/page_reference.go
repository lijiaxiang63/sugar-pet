package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

// covChoice is a table of covariates and the columns to adjust for.  The
// table is a copy of the file, which may change under it (a participants.tsv
// is filled in with another program): sync brings the copy up to date while
// the page is shown, and fresh reads the file anew for a run.
type covChoice struct {
	path   string
	table  *core.Table
	err    string
	adjust map[string]bool

	columns []string  // of the table last read: those whose ticks are the user's
	stamp   fileStamp // the file as it was when it was read
	checked time.Time // when the file was last looked at
}

// fileStamp tells one state of a file from another without reading it.
type fileStamp struct {
	found    bool
	size     int64
	modified int64 // ns
}

func stampOf(path string) fileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{found: true, size: info.Size(), modified: info.ModTime().UnixNano()}
}

// covCheckEvery is how often, at most, a page looks at the file of its table.
const covCheckEvery = time.Second

// load reads the table after its path changed.
func (cv *covChoice) load() {
	cv.adjust, cv.columns = map[string]bool{}, nil
	cv.read()
}

// read reads the table from the file as it is now.  A column the last table
// had keeps its tick, a new one is ticked if it is age or sex, and the tick of
// a column that is gone is dropped.  A file that cannot be read leaves the
// ticks alone, for when it can be read again.
func (cv *covChoice) read() {
	cv.table, cv.err, cv.stamp = nil, "", fileStamp{}
	if cv.path == "" {
		return
	}
	// The stamp is taken first: a change during the reading shows later.
	cv.stamp = stampOf(cv.path)
	table, err := core.LoadTable(cv.path)
	if err != nil {
		cv.err = err.Error()
		return
	}
	adjust := map[string]bool{}
	for _, col := range table.Columns() {
		if slices.Contains(cv.columns, col) {
			adjust[col] = cv.adjust[col]
		} else {
			adjust[col] = col == "age" || col == "sex"
		}
	}
	cv.table, cv.adjust, cv.columns = table, adjust, table.Columns()
}

// sync reads the table again if its file has changed since it was read.  It
// is for the view, which runs on every frame: now is the time of the frame,
// and the file is looked at, not read, once in covCheckEvery.  A file that
// could not be read is read at each look: what was in the way (the permission
// to read it, say) may be gone without a change of its size or time.
func (cv *covChoice) sync(now time.Time) {
	if cv.path == "" {
		return
	}
	if since := now.Sub(cv.checked); since >= 0 && since < covCheckEvery {
		return
	}
	cv.checked = now
	if cv.err != "" || stampOf(cv.path) != cv.stamp {
		cv.read()
	}
}

// fresh reads the table anew for a run, whatever the stamp of its file says,
// and returns what keeps the run from starting: the file cannot be read, or
// the columns to adjust for are not those the page showed.
func (cv *covChoice) fresh() string {
	if cv.path == "" {
		return ""
	}
	shown := cv.names()
	cv.read()
	now := cv.names()
	slices.Sort(shown) // the columns may be in another order
	slices.Sort(now)
	if cv.err == "" && !slices.Equal(shown, now) {
		return T("The columns of the covariate table have changed. Check the columns to adjust for, then start again.",
			"协变量表的列已发生变化。请检查要校正的列，然后重新开始。")
	}
	return cv.problem()
}

// names are the chosen columns, in the order of the table.
func (cv *covChoice) names() []string {
	var out []string
	if cv.table != nil {
		for _, col := range cv.table.Columns() {
			if cv.adjust[col] {
				out = append(out, col)
			}
		}
	}
	return out
}

func (cv *covChoice) problem() string {
	switch {
	case cv.path == "":
		return ""
	case cv.err != "":
		return T("The covariate table cannot be read: ", "无法读取协变量表：") + cv.err
	case len(cv.names()) == 0:
		return T("Choose the columns of the covariate table to adjust for.", "请选择协变量表中要校正的列。")
	}
	return ""
}

// needReference is what a page says while it has no reference, which the
// run bar offers to mend (remedy).
func needReference() string {
	return T("The project has no reference yet: build one on the Reference page, or import one there.", "项目还没有参考库：请先到“参考库”页面建立，或导入已有的参考库。")
}

// referenceFact is the line of a page that says which reference it uses.
func (a *app) referenceFact(c *ui.Context) func() {
	a.zmap.load()
	value := referenceText(a.zmap.reference, a.zmap.info)
	if a.zmap.reference == "" {
		value = T("None yet", "尚未建立")
	}

	return fact(c, T("Reference", "参考库"), value)
}

// covText says in a line which covariates the project adjusts for.
func (a *app) covText() string {
	cv := &a.cov
	switch {
	case cv.path == "":
		return T("None: no table chosen", "无：未选择协变量表")
	case cv.err != "":
		return T("The table cannot be read", "表格无法读取")
	}
	names := cv.names()
	if len(names) == 0 {
		return filepath.Base(cv.path) + T(" · no column chosen", " · 未选择列")
	}
	return strings.Join(names, T(", ", "、")) + " · " + filepath.Base(cv.path)
}

// studyParticipants returns the path to the participants.tsv of the project
// if it exists, or "" otherwise.
func (a *app) studyParticipants() string {
	return a.inProject("participants.tsv", false)
}

// covRows are the settings of a covariate table.
func (a *app) covRows(c *ui.Context, key string, cv *covChoice, desc string) []func() {
	t := c.Theme()
	cv.sync(c.Now())
	out := []func(){func() {
		a.pathSetting(c, key, T("Covariate table", "协变量表"), desc, &cv.path, pathFile, tableFilter, cv.load)
	}}
	if cv.path == "" {
		if sp := a.studyParticipants(); sp != "" {
			out = append(out, func() {
				setting(c, T("The project's own table", "项目自带的表格"), T("participants.tsv in the project folder: fill it in, one row per subject, then use it.", "项目文件夹中的 participants.tsv：每位受试者填一行，然后选用它。"), func() {
					if button(c.Key(key+"-use-study"), T("Use participants.tsv", "使用 participants.tsv"), iconTable, buttonSoft, 26).Clicked() && a.stillInProject("participants.tsv", false) {
						cv.path = sp
						cv.load()
					}
				})
			})
		}
	}
	switch {
	case cv.err != "":
		out = append(out, func() {
			ui.Text(c, cv.err).FontSize(12).TextColor(t.Danger).Padding(8, 14).Selectable()
		})
	case cv.table != nil:
		out = append(out, func() {
			ui.Column(c).Padding(9, 14, 10, 14).Gap(6).Children(func() {
				ui.Text(c, T("Adjust for", "校正的列")).FontSize(13)
				ui.Row(c).Wrap().GapY(8).GapX(18).Children(func() {
					for _, col := range cv.table.Columns() {
						on := cv.adjust[col]
						check(c.Key(col), &on, col)
						cv.adjust[col] = on
					}
				})
				ui.Text(c, Tf("%d rows. Numeric columns are used as they are; others need exactly two values (e.g. F and M).",
					"共 %d 行。数值列直接使用；其他列必须恰好有两种取值（如 F 和 M）。", len(cv.table.Rows))).FontSize(11.5).TextColor(t.TextMuted)
			})
		})
	}
	return out
}

// refInfo is what reference.json says about a reference.
type refInfo struct {
	N        int      `json:"n_controls"`
	FWHM     float64  `json:"fwhm_mm"`
	Mask     string   `json:"mask"`
	StatsFor []string `json:"stats_for"`
	Models   []string `json:"models"`
	// How the factors of the controls were estimated, where not by the mean
	// of a region and the default smoothing of each method.
	Statistic  string   `json:"statistic"`
	FactorFWHM *float64 `json:"factor_fwhm_mm"`
	Covariates []struct {
		Name string `json:"name"`
	} `json:"covariates"`
	err string
}

func readRefInfo(folder string) *refInfo {
	info := &refInfo{}
	raw, err := os.ReadFile(filepath.Join(folder, "reference.json"))
	if err != nil {
		info.err = T("This folder is not a reference (it has no reference.json).", "该文件夹不是参考库（没有 reference.json）。")
		return info
	}
	if err := json.Unmarshal(raw, info); err != nil {
		info.err = err.Error()
	}
	return info
}

func (r *refInfo) covariateNames() []string {
	names := make([]string, len(r.Covariates))
	for i, c := range r.Covariates {
		names[i] = c.Name
	}
	return names
}

type refPage struct {
	files  fileList
	output string

	st      status
	built   string // where the last build wrote, or was to write
	current string // the reference the summary below is of
	info    *refInfo
	pic     picture
}

func (p *refPage) init() {}

func (a *app) refProblem() string {
	p, o := &a.ref, &a.opt
	switch {
	case len(p.files.files) == 0:
		return a.needImages("controls")
	case len(p.files.files) < 3:
		return T("A reference needs at least 3 healthy controls (10 or more is better): add them on the Data page, or import a reference.",
			"建立参考库至少需要 3 例健康对照（建议 10 例以上）：请到“数据”页面添加，或导入已有的参考库。")
	}
	if o.RefMask == "file" && o.RefMaskFile == "" {
		return T("Choose the mask file of the reference in the settings.", "请在设置中选择参考库的掩膜文件。")
	}
	return a.cov.problem()
}

// ownReference reports whether the reference in use is the one built in
// the project, and not one imported from elsewhere.
func (a *app) ownReference() bool {
	return a.norm.reference != "" && sameDir(a.norm.reference, a.ref.output)
}
func (a *app) referenceView(c *ui.Context) {
	p := &a.ref
	if ref := a.norm.reference; ref != p.current {
		p.current, p.info = ref, nil
		if ref != "" {
			p.info = readRefInfo(ref)
		}
	}
	a.frame(c, T("Reference", "参考库"),
		T("Summarise healthy controls once, then use them for every patient", "用健康对照建立一次，之后每个患者都可使用"),
		func() {
			section(c, T("Reference in use", "当前参考库"), "", func() {
				if p.info == nil || p.info.err != "" {
					rows(c, func() {
						desc := T("HN, iPS, iHN and z-score maps need a reference. Build one below from the controls of the project, or import one built elsewhere.",
							"HN、iPS、iHN 和 Z 值图都需要参考库。可以在下方用项目的健康对照建立，也可以导入在别处建好的参考库。")
						if p.info != nil {
							desc = p.info.err
						}
						setting(c, T("No reference yet", "还没有参考库"), desc, func() {
							if smallButton(c.Key("ref-import"), T("Import…", "导入…")).Clicked() {
								a.chooseReference()
							}
							if own := a.ref.output; core.IsReferenceFolder(own) {
								if smallButton(c.Key("ref-own"), T("Use the Project's Own", "使用项目自己的")).Clicked() {
									a.setReference(own)
								}
							}
						})
					})
					return
				}
				info := p.info
				extra := T("PS, iPS, HN, iHN", "PS、iPS、HN、iHN")
				for _, m := range info.StatsFor {
					extra += T(", ", "、") + methodName(m)
				}
				cov := T("None", "无")
				if names := info.covariateNames(); len(names) > 0 {
					cov = strings.Join(names, T(", ", "、"))
				}
				ui.Column(c).Gap(8).Children(func() {
					stats(c,
						[2]string{strconv.Itoa(info.N), T("Controls", "对照人数")},
						[2]string{strconv.FormatFloat(info.FWHM, 'g', 4, 64) + " mm", T("Smoothing (FWHM)", "平滑 (FWHM)")},
						[2]string{cov, T("Covariates (W-scores)", "协变量（W 值）")},
					)
					rows(c,
						func() {
							origin := T("Built from the controls of this project", "由本项目的健康对照建立")
							if !a.ownReference() {
								origin = T("Imported from ", "导入自 ") + shortDir(filepath.Dir(p.current))
							}
							setting(c, filepath.Base(p.current), origin, func() {
								revealButton(c, "ref-reveal", revealLabel(), p.current)
								if smallButton(c.Key("ref-import"), T("Import Another…", "导入其他…")).Clicked() {
									a.chooseReference()
								}
								if own := a.ref.output; !a.ownReference() && core.IsReferenceFolder(own) {
									if smallButton(c.Key("ref-own"), T("Use the Project's Own", "使用项目自己的")).Clicked() {
										a.setReference(own)
									}
								}
							})
						},
						func() { setting(c, T("Z-score maps available for", "可制作 Z 值图的方法"), extra, nil) },
					)
					a.show(&p.pic, p.current, referencePicture(p.current))
					a.figure(c, &p.pic, T("Mean of the controls", "对照的均值图"), legend{})
					ui.Row(c).Gap(8).Wrap().GapY(6).Children(func() {
						// The individual analysis begins at its first step; the
						// others are reached from there.
						if button(c.Key("ref-to-norm"), T("Go to Intensity Normalisation", "前往强度归一化"), iconArrowRight, buttonSoft, 26).Clicked() {
							a.page = "normalize"
						}
						if button(c.Key("ref-to-cmp"), T("Go to Compare Groups", "前往组间比较"), iconArrowRight, buttonSoft, 26).Clicked() {
							a.page = "compare"
						}
					})
				})
			})
			ui.Column(c).Gap(18).Children(func() {
				ui.Box(c).Height(1).Background(paletteFor(c).line)
				ui.Text(c, T("Build the project's reference", "建立项目的参考库")).FontSize(17).FontWeight(700).LetterSpacing(-0.2).Padding(0, 2)
			})
			section(c, T("Healthy controls", "健康对照"), T("The controls of the project, in MNI space and all on the same grid. Below about 10 controls the voxel statistics get unreliable. With a covariate table, the reference also models age and sex (W-scores).",
				"项目中的健康对照图像，须在 MNI 空间且在同一网格上。少于约 10 例时，体素统计不够可靠。提供协变量表后，参考库还会对年龄和性别建模（W 值）。"), func() {
				a.source(c, "ref-data", "data", changeInData(),
					fact(c, T("Controls", "对照"), countText(p.files.files)),
					fact(c, T("Covariates", "协变量"), a.covText()))
			})
			section(c, T("Save", "保存"), "", func() {
				rows(c, fact(c, T("Written to", "输出位置"), a.writesTo(p.output)+T(" · it becomes the reference in use", " · 建好后自动成为当前参考库")))
				a.optionsLine(c, "reference", "reference", groupReference)
			})
		},
		func() {
			a.runBar(c, "reference", a.refProblem(), T("Build Reference", "建立参考库"), &p.st, a.runReference)
			a.next(c, "normalize", a.norm.reference != "" && len(a.norm.files.files) > 0)
		},
	)
}

func (a *app) runReference() {
	p := &a.ref
	p.built = p.output
	files := append([]string(nil), p.files.files...)
	o := a.opt
	opt := core.BuildOptions{FWHM: o.RefFWHM, Mask: core.MaskSpec{Name: o.RefMask}, StatsFor: o.statsFor()}
	// The factors of the controls are estimated as Intensity Normalisation
	// estimates those of an image, so that the two can be compared.
	if o.Statistic == 1 {
		opt.Statistic = "median"
	}
	if !o.FWHMAuto {
		fwhm := o.FWHM
		opt.FactorFWHM = &fwhm
	}
	if o.RefMask == "file" {
		opt.Mask = core.MaskSpec{Name: o.RefMaskFile}
	}
	if msg := a.cov.fresh(); msg != "" {
		p.st.err = msg
		return
	}
	if a.cov.table != nil {
		names := a.cov.names()
		found, err := a.cov.table.Lookup(files, names)
		if err != nil {
			p.st.err = err.Error()
			return
		}
		opt.Covariates = core.CovariatesFromRows(found, names)
	}
	out := p.output
	a.start("reference", T("Reading the controls", "正在读取对照图像"), func(ctx context.Context, step func(int, int)) (func(), error) {
		opt.Progress = step
		step(0, len(files))
		ref, err := core.BuildReference(ctx, files, opt)
		if err != nil {
			return nil, err
		}
		if err := ref.Save(out); err != nil {
			return nil, err
		}
		return func() {
			a.setReference(out)
			p.current = ""
			// Images normalised with the reference that was in this folder
			// are not normalised with this one.
			if d := a.norm.done; d != nil && d.reference == out {
				d.rebuilt = true
			}
			p.st.ok = Tf("Reference built from %d controls; it is now the reference in use.", "已用 %d 例对照建立参考库，并设为当前参考库。", len(files))
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
