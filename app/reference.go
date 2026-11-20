package main

import (
	"context"
	"encoding/json"
	"fmt"
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

// needReference is what a page says while the controls of the project are
// not summarised yet, which the run bar offers to mend (remedy).
func needReference() string {
	return T("The healthy controls are not summarised yet: Intensity Normalisation does that first, from at least 3 of them.",
		"健康对照还没有汇总成对照统计：运行强度归一化时会先完成这一步（至少需要 3 例对照）。")
}

// referenceFact is the line of a page that says which controls it compares
// with.
func (a *app) referenceFact(c *ui.Context) func() {
	return fact(c, T("Healthy controls", "健康对照"), a.referenceLine())
}

// referenceLine says in a line how many controls the project compares with,
// as they were summarised, or when they are.
func (a *app) referenceLine() string {
	ref, n := a.reference, len(a.controls())
	switch {
	case ref == "" && !a.buildsReference():
		return Tf("%d: it takes at least 3", "%d 例：至少需要 3 例", n)
	case ref == "":
		return Tf("%d · summarised when Intensity Normalisation runs", "%d 例 · 运行强度归一化时汇总", n)
	case a.refStale():
		return Tf("%d · summarised anew when %s runs", "%[1]d 例 · 下次运行%[2]s时重新汇总", n, a.rebuiltWhen())
	}
	return controlsText(a.refInfo())
}

// controlsText says in a line what the summary of the controls holds.
func controlsText(info *refInfo) string {
	if info == nil {
		return T("Not summarised yet", "尚未汇总")
	}
	if info.err != "" {
		return info.err
	}
	return Tf("%d · smoothed %g mm", "%d 例 · 平滑 %g mm", info.N, info.FWHM)
}

// controlStats is the section of Intensity Normalisation on the summary of
// the controls that it makes before it normalises the images: what it
// holds, and how it is made.
func (a *app) controlStats(c *ui.Context) {
	p := &a.ref
	info := a.refInfo()
	section(c, T("Control statistics", "对照统计"),
		T("Before it normalises the images, this step summarises the healthy controls: the mean and spread of every voxel for each method, and, with a covariate table, what age and sex do (W-scores). HN, iPS, iHN, the z-score maps and the reference cluster all compare with them. They are summarised anew once the controls or these settings change; when only the covariates change, Z-score Maps does it. Below about 10 controls the voxel statistics get unreliable.",
			"本步在归一化图像之前，先把健康对照汇总成对照统计：每种方法下每个体素的均值和离散程度；提供协变量表时，还会对年龄和性别的影响建模（W 值）。HN、iPS、iHN、Z 值图和参考簇都与它比较。对照或这些设置变动后会重新汇总；只改了协变量时，由 Z 值图重新汇总。少于约 10 例对照时，体素统计不够可靠。"), func() {
			if a.refStale() {
				caution(c, Tf("The controls, the covariates or the settings have changed since the controls were summarised: %s does it anew the next time it runs.",
					"汇总对照统计之后，对照、协变量或设置有了变化：下次运行%s时会重新汇总。", a.rebuiltWhen()))
			}
			if info != nil && info.err == "" {
				extra := T("PS, iPS, HN, iHN", "PS、iPS、HN、iHN")
				for _, m := range info.StatsFor {
					extra += T(", ", "、") + methodName(m)
				}
				cov := T("None", "无")
				if names := info.covariateNames(); len(names) > 0 {
					cov = strings.Join(names, T(", ", "、"))
				}
				stats(c,
					[2]string{strconv.Itoa(info.N), T("Controls", "对照人数")},
					[2]string{strconv.FormatFloat(info.FWHM, 'g', 4, 64) + " mm", T("Smoothing (FWHM)", "平滑 (FWHM)")},
					[2]string{cov, T("Covariates (W-scores)", "协变量（W 值）")},
				)
				rows(c,
					func() { setting(c, T("Z-score maps available for", "可制作 Z 值图的方法"), extra, nil) },
					func() {
						setting(c, T("Saved in", "保存位置"), a.writesTo(a.reference), func() {
							revealButton(c, "ref-reveal", revealLabel(), a.reference)
						})
					},
				)
				a.show(&p.pic, a.reference, referencePicture(a.reference))
				a.figure(c, &p.pic, T("Mean of the controls", "对照的均值图"), legend{})
			} else {
				rows(c,
					fact(c, T("Healthy controls", "健康对照"), a.referenceLine()),
					fact(c, T("Covariates", "协变量"), a.covText()),
					fact(c, T("Saved in", "保存位置"), a.writesTo(p.output)))
			}
			a.optionsLine(c, "reference", "reference", groupReference)
		})
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
	Statistic  string    `json:"statistic"`
	FactorFWHM *float64  `json:"factor_fwhm_mm"`
	Covariates []refTerm `json:"covariates"`
	err        string
}

// refTerm is a covariate the control statistics model: a number, or one of
// two levels (F or M, say).
type refTerm struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Levels []string `json:"levels"`
}

func readRefInfo(folder string) *refInfo {
	info := &refInfo{}
	raw, err := os.ReadFile(filepath.Join(folder, "reference.json"))
	if err != nil {
		info.err = T("The control statistics cannot be read (reference.json is missing): run Intensity Normalisation again.", "无法读取对照统计（缺少 reference.json）：请重新运行强度归一化。")
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

// The reference of a project is its healthy controls, summarised (the
// control statistics, as the pages call it): it is no step of its own.
// Intensity Normalisation builds it from the controls before it normalises
// the images, and builds it anew once the controls or the settings it was
// built with have changed.  Where no more than its model of the covariates,
// or the statistics it keeps, have changed, the images normalised still
// stand, and Z-score Maps builds it anew instead.  Every project has
// controls of its own, at least 3: a reference is never brought in from
// elsewhere, so that the maps and the group comparison rest on the same
// controls.
type refPage struct {
	output string // where the reference of the project is built

	built *refBuilt // what the reference in output was built from
	pic   picture
}

// refBuilt is what the reference of the project was last built from.
type refBuilt struct {
	sig      string               // app.refSignature as it was then
	statsFor []string             // the methods it keeps control statistics for
	controls map[string]fileStamp // the controls, as they were on disk
	stamp    fileStamp            // its reference.json, as the build wrote it
}

func (p *refPage) init() {}

// buildsReference reports whether the project can summarise its controls:
// it holds the 3 that it takes at least.
func (a *app) buildsReference() bool {
	return a.ref.output != "" && len(a.controls()) >= 3
}

// refStatsFor are the methods the reference of the project keeps control
// statistics for: those ticked in the settings, and those chosen for
// Intensity Normalisation, so that a map can be made with each of them.
func (a *app) refStatsFor() []string {
	var out []string
	for _, m := range core.StatsMethods() {
		if a.opt.RefStatsFor[m] || a.norm.methods[m] {
			out = append(out, m)
		}
	}
	return out
}

// refFactorSignature is what of the settings the reference of the project
// is built with that the factors of Intensity Normalisation rest on: its
// mask and smoothing, and how the factors of its controls are estimated.
// "" while the project builds none.  It names no folder, so that a project
// moved keeps it.
func (a *app) refFactorSignature() string {
	if !a.buildsReference() {
		return ""
	}
	o := &a.opt
	mask := o.RefMask
	if mask == "file" {
		mask = filepath.Base(o.RefMaskFile)
	}
	factors := "auto"
	if !o.FWHMAuto {
		factors = num(o.FWHM)
	}
	return fmt.Sprint("mask ", mask, "; smoothing ", num(o.RefFWHM), "; statistic ", o.Statistic, "; factors after ", factors)
}

// refSignature is all that, besides its controls, the reference of the
// project is built with: what refFactorSignature names, and its model of
// the covariates and the methods it keeps control statistics for, which
// only the maps rest on.  "" while the project builds none.
func (a *app) refSignature() string {
	if !a.buildsReference() {
		return ""
	}
	cv := &a.cov
	covariates := "none"
	if cv.path != "" {
		covariates = fmt.Sprint(cv.stamp.size, cv.stamp.modified, cv.names())
	}
	return fmt.Sprint(a.refFactorSignature(), "; covariates ", covariates, "; also ", a.opt.statsFor())
}

// refBuilder is the step whose next run builds the reference of the project
// anew: Z-score Maps while step 1 still stands, as when no more than the
// covariates or the statistics it keeps have changed, on which no factor
// rests; otherwise Intensity Normalisation.
func (a *app) refBuilder() string {
	if a.normCurrent() != nil {
		return "zmap"
	}
	return "normalize"
}

// rebuiltWhen says when the reference of the project is built anew.
func (a *app) rebuiltWhen() string {
	if a.refBuilder() == "zmap" {
		return T("Z-score Maps", "Z 值图")
	}
	return T("Intensity Normalisation", "强度归一化")
}

// refStale reports whether the reference of the project is not, or no
// longer, what its controls and the settings make: the next run of
// Intensity Normalisation builds it.
func (a *app) refStale() bool {
	if !a.buildsReference() {
		return false
	}
	b, files := a.ref.built, a.controls()
	if b == nil || b.sig != a.refSignature() || len(b.controls) != len(files) {
		return true
	}
	for _, m := range a.refStatsFor() {
		if !slices.Contains(b.statsFor, m) {
			return true
		}
	}
	for _, path := range files {
		if _, ok := b.controls[path]; !ok {
			return true
		}
	}
	return false
}

// buildProblem is what keeps the reference of the project from being built,
// or "".
func (a *app) buildProblem() string {
	if o := &a.opt; o.RefMask == "file" && o.RefMaskFile == "" {
		return T("Choose the mask file of the control statistics in the settings.", "请在设置中选择对照统计所用的掩膜文件。")
	}
	return a.cov.problem()
}

// refBuild is the building of the reference of the project, with which a
// run of Intensity Normalisation begins.
type refBuild struct {
	files []string
	out   string
	opt   core.BuildOptions
	built *refBuilt
}

// referenceBuild is the building a run of Intensity Normalisation, or of
// Z-score Maps, begins with: nil where the project builds no reference, or the one it has is
// still what its controls and the settings make.  Or what keeps it from
// being built.
func (a *app) referenceBuild() (*refBuild, string) {
	if !a.buildsReference() {
		return nil, ""
	}
	if msg := a.cov.fresh(); msg != "" {
		return nil, msg
	}
	files, out := slices.Clone(a.controls()), a.ref.output
	stamps := map[string]fileStamp{}
	for _, path := range files {
		stamps[path] = stampOf(path)
	}
	if b := a.ref.built; b != nil && !a.refStale() && b.stamp.found && stampOf(filepath.Join(out, "reference.json")) == b.stamp {
		same := true
		for path, now := range stamps {
			same = same && now == b.controls[path]
		}
		if same {
			return nil, ""
		}
	}
	o := a.opt
	opt := core.BuildOptions{FWHM: o.RefFWHM, Mask: core.MaskSpec{Name: o.RefMask}, StatsFor: a.refStatsFor()}
	// The factors of the controls are estimated as those of every image
	// are, so that the two can be compared.
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
	if a.cov.table != nil {
		names := a.cov.names()
		found, err := a.cov.table.Lookup(files, names)
		if err != nil {
			return nil, err.Error()
		}
		opt.Covariates = core.CovariatesFromRows(found, names)
	}
	return &refBuild{files: files, out: out, opt: opt,
		built: &refBuilt{sig: a.refSignature(), statsFor: opt.StatsFor, controls: stamps}}, ""
}

// run builds the reference and saves it in the project.
func (b *refBuild) run(ctx context.Context, step func(done, total int)) (*refBuilt, error) {
	b.opt.Progress = step
	step(0, len(b.files))
	ref, err := core.BuildReference(ctx, b.files, b.opt)
	if err != nil {
		return nil, err
	}
	if err := ref.Save(b.out); err != nil {
		return nil, err
	}
	b.built.stamp = stampOf(filepath.Join(b.out, "reference.json"))
	return b.built, nil
}
