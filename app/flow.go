package main

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

// The individual analysis is three steps, each resting on the one before:
// Intensity Normalisation takes the images and the methods, Z-score Maps
// compares the patients it normalised with the controls, and Classify
// Parkinsonism reads the maps.  Compare Groups rests on the first as well:
// it tests the patients against the controls as that step normalised them.  A step shows what it takes from the one before and asks
// for none of it again: to change it, the user goes back to where it was
// chosen, and a step whose ground has changed waits until that is run anew.
var flowSteps = []string{"normalize", "zmap", "classify"}

// normDone is what the last run of Intensity Normalisation that went well
// did: the ground of step 2.
type normDone struct {
	asked []string // the images the run was asked for: patients and controls
	files []string // those that every method normalised
	// controls are those of asked that are healthy controls of the project;
	// the others are its patients.
	controls  []string
	methods   []string
	reference string
	// refSig is app.refFactorSignature as the run found it: what of the
	// settings the reference of the project was built with the factors rest
	// on ("" with an imported one, or with none).
	refSig   string
	settings string // options.normSettings as the run had them
	// What the run wrote of each image, by image and method: the normalised
	// image, which step 2 reads, and the factor it was divided by.
	images  map[string]map[string]string
	factors map[string]map[string]float64
	target  float64
	// How the factors were estimated: by the median of a region and not its
	// mean, and after which smoothing (nil: the default of each method).  A
	// map is made only against controls whose factors were estimated so.
	median bool
	fwhm   *float64
	// rebuilt: the reference was built anew, in its folder, after the run.
	rebuilt bool
	// The images the run read and the reference it used, as they were on
	// disk then (stamp), and whether an image was found changed when the
	// project was opened again (look).
	stamps   map[string]fileStamp
	refStamp fileStamp
	changed  bool
}

// zmapDone is what the last run of Z-score Maps that went well did: the
// ground of step 3.
type zmapDone struct {
	from    *normDone // the run of step 1 the images came from
	files   []string  // the images that every method made a map of
	methods []string
	table   string // the covariate table; "" for plain z-scores
	// refSig is app.refSignature as the run found it: the model of the
	// covariates and the statistics of the reference of the project, which
	// the maps rest on and the factors of step 1 do not.
	refSig string
	// The maps the run wrote, by image and method, which step 3 reads.
	maps map[string]map[string]string
}

// of returns the images of the patients, or of the controls, that every
// method normalised, and how many more of them the run was asked for.
func (d *normDone) of(controls bool) (files []string, dropped int) {
	for _, path := range d.asked {
		if slices.Contains(d.controls, path) != controls {
			continue
		}
		if slices.Contains(d.files, path) {
			files = append(files, path)
		} else {
			dropped++
		}
	}
	return files, dropped
}

// pageImages are the patients a run of step 1 normalised: what the steps
// after it work on.
func (a *app) pageImages(d *normDone) []string {
	patients, _ := d.of(false)
	return patients
}

// normChanged names what was changed on step 1 since it ran: its images,
// methods, reference or settings.  "" while it holds what it ran with.
func (a *app) normChanged() string {
	p, d := &a.norm, a.norm.done
	if d == nil {
		return ""
	}
	var what []string
	if !slices.Equal(d.asked, p.files.files) || d.changed {
		what = append(what, T("images", "图像"))
	}
	if !slices.Equal(d.methods, p.chosen()) {
		what = append(what, T("methods", "方法"))
	}
	if d.reference != p.reference || d.rebuilt {
		what = append(what, T("reference", "参考库"))
	} else if d.refSig != a.refFactorSignature() {
		// The reference of the project would be built anew, and the factors
		// estimated otherwise.
		what = append(what, T("reference settings", "参考库设置"))
	}
	if d.settings != a.opt.normSettings(d.methods) {
		what = append(what, T("settings", "设置"))
	}
	return strings.Join(what, T(", ", "、"))
}

// normCurrent is what step 1 did, while the page and the settings still hold
// what it ran with.  What was chosen since is not what was normalised.
func (a *app) normCurrent() *normDone {
	if a.norm.done == nil || a.normChanged() != "" {
		return nil
	}
	return a.norm.done
}

// statisticText and factorSmoothingText say how factors are estimated.
func statisticText(median bool) string {
	if median {
		return T("the median of a region", "区域中位数")
	}
	return T("the mean of a region", "区域均值")
}

func factorSmoothingText(fwhm *float64) string {
	if fwhm == nil {
		return T("automatic smoothing", "自动平滑")
	}
	return Tf("a smoothing of %s mm", "%s mm 的平滑", num(*fwhm))
}

// unfit says why no z-score map can be made, by these methods, of what step
// 1 normalised, or "": an image is compared with the controls of a reference
// only when its factor was estimated the way theirs were, by the same
// statistic of a region and after the same smoothing.  Either side can be
// made to agree: the settings of step 1, or the reference, built anew.
func (d *normDone) unfit(methods []string, ref *refInfo) string {
	if ref == nil || ref.err != "" {
		return ""
	}
	var mine, theirs []string
	if (d.fwhm == nil) != (ref.FactorFWHM == nil) || d.fwhm != nil && *d.fwhm != *ref.FactorFWHM {
		mine, theirs = append(mine, factorSmoothingText(d.fwhm)), append(theirs, factorSmoothingText(ref.FactorFWHM))
	}
	// The statistic is that of the reference regions and of PS.
	regional := slices.ContainsFunc(methods, func(m string) bool { return !core.NeedsReference(m) && m != "spm-global" })
	if regional && d.median != (ref.Statistic == "median") {
		mine, theirs = append(mine, statisticText(d.median)), append(theirs, statisticText(ref.Statistic == "median"))
	}
	if len(mine) == 0 {
		return ""
	}
	and := T(" and ", "和")
	return Tf("Step 1 estimated its factors with %s, the imported reference those of its controls with %s: the images cannot be compared with the controls. Make Settings › Intensity Normalisation agree with the reference and run step 1 again, or use a reference built with the settings as they are.",
		"第 1 步估计因子时用的是%s，而导入的参考库估计对照的因子时用的是%s，两者无法比较。请把“设置 › 强度归一化”改成与参考库一致并重新运行第 1 步，或者改用按现在的设置建立的参考库。",
		strings.Join(mine, and), strings.Join(theirs, and))
}

// covariates is the covariate table a run of step 2 uses: none while the
// reference holds no model of covariates.
func (p *zmapPage) covariates() string {
	p.load()
	if p.info == nil || len(p.info.Covariates) == 0 {
		return ""
	}
	return p.covTable
}

// usable are the methods of step 1 that maps can be made with, and those
// that cannot: a custom region, or a method the reference holds no control
// statistics for.
func (p *zmapPage) usable(methods []string) (yes, no []string) {
	p.load()
	for _, m := range methods {
		if ok, _ := p.allowed(m); ok {
			yes = append(yes, m)
		} else {
			no = append(no, m)
		}
	}
	return yes, no
}

// zmapCurrent is what step 2 did, while it still rests on what step 1 did,
// on the covariate table it ran with and on the reference as it was built.
func (a *app) zmapCurrent() *zmapDone {
	d, n := a.zmap.done, a.normCurrent()
	if d == nil || n == nil || d.from != n || d.table != a.zmap.covariates() || d.refSig != a.refSignature() {
		return nil
	}
	return d
}

// classified reports whether step 3 ran on what step 2 did.
func (a *app) classified() bool {
	z := a.zmapCurrent()
	return z != nil && a.classify.from == z && a.classify.method == a.norm.primary() && a.classify.checked == a.opt.CrossCheck
}

// stepDone reports whether a step of the individual analysis is done, and
// still stands on what is before it.
func (a *app) stepDone(page string) bool {
	switch page {
	case "normalize":
		return a.normCurrent() != nil
	case "zmap":
		return a.zmapCurrent() != nil
	case "classify":
		return a.classified()
	}
	return false
}

// lock is the step a page that rests on the individual analysis waits for,
// and what the page says while it waits; "", "" when the step before it is
// done.  Compare Groups waits for step 1, as Z-score Maps does.
func (a *app) lock(page string) (step, why string) {
	if page != "zmap" && page != "classify" && page != "compare" {
		return "", ""
	}
	if a.normCurrent() == nil {
		if what := a.normChanged(); what != "" {
			return "normalize", Tf("Step 1 ran with other %s than it has now: run Intensity Normalisation again.", "第 1 步运行之后改了%s：请重新运行强度归一化。", what)
		}
		return "normalize", T("Finish step 1 first: Intensity Normalisation.", "请先完成第 1 步：强度归一化。")
	}
	if page == "classify" && a.zmapCurrent() == nil {
		if d := a.zmap.done; d != nil {
			if d.from == a.normCurrent() && d.refSig != a.refSignature() {
				return "zmap", T("The covariates or the reference settings changed after step 2 ran: run Z-score Maps again, which builds the reference anew.",
					"第 2 步运行之后改了协变量或参考库设置：请重新运行 Z 值图，它会先重新建立参考库。")
			}
			return "zmap", T("Step 2 no longer rests on what is before it: run Z-score Maps again.", "第 2 步与它之前的内容已不一致：请重新运行 Z 值图。")
		}
		return "zmap", T("Finish step 2 first: Z-score Maps.", "请先完成第 2 步：Z 值图。")
	}
	return "", ""
}

// stepNumber is the place of a page in the individual analysis, from 1, or 0.
func stepNumber(page string) int {
	return slices.Index(flowSteps, page) + 1
}

// leadingMethod is the method an analysis that rests on one takes of
// several, where none was made primary: HN, which needs no region that the
// disease spares, or the first that is no custom region.
func leadingMethod(methods []string) string {
	if slices.Contains(methods, "hn") {
		return "hn"
	}
	for _, m := range methods {
		if m != "region" {
			return m
		}
	}
	return ""
}

func methodNames(methods []string) string {
	names := make([]string, len(methods))
	for i, m := range methods {
		names[i] = methodName(m)
	}
	return strings.Join(names, T(", ", "、"))
}

// imagesText says how many images a step takes and where they lie.
func imagesText(files []string, dropped int) string {
	text := Tf("%d image(s)", "%d 幅", len(files))
	if dir := commonDir(files); dir != "" {
		text += " · " + shortDir(dir)
	}
	if dropped > 0 {
		text += Tf(" · %d more failed in the step before", " · 另有 %d 幅在上一步失败", dropped)
	}
	return text
}

// steps is the line at the top of a page of the individual analysis: its
// three steps in order, the one shown marked, those done ticked.
func (a *app) steps(c *ui.Context) {
	pal := paletteFor(c)
	ui.Row(c.Key("steps")).Gap(5).AlignItems(ui.Center).Wrap().GapY(5).Children(func() {
		for i, id := range flowSteps {
			if i > 0 {
				ui.Icon(c, iconChevron).Size(11, 11).TextColor(pal.faint).Shrink(0)
			}
			info := pageInfoOf(id)
			title := T(info.en, info.zh)
			here, done := a.page == id, a.stepDone(id)
			step := ui.ButtonBase(c.Key("step-"+id)).Height(30).Padding(0, 12, 0, 5).Gap(8).Radius(15).AlignItems(ui.Center).Shrink(0).
				Label(Tf("Step %d: %s", "第 %d 步：%s", i+1, title))
			bg, fg, ring := ui.Transparent, pal.muted, pal.border
			switch {
			case here:
				bg, fg, ring = pal.accentSoft, pal.text, pal.accentBorder
			case step.Hovered():
				bg = pal.rowHover
			}
			step.Background(step.AnimateColor("bg", bg, quick)).InsetShadow(0, 0, 0, 1, ring).Children(func() {
				dot, mark := pal.text.Alpha(0.1), pal.muted
				switch {
				case done:
					dot, mark = pal.success, pal.onAccent
				case here:
					dot, mark = pal.accent, pal.onAccent
				}
				ui.Box(c).Size(20, 20).Radius(10).Center().Shrink(0).Background(dot).Children(func() {
					if done {
						ui.Icon(c, iconTick).Size(11, 11).TextColor(mark)
					} else {
						ui.Text(c, strconv.Itoa(i+1)).FontSize(11).FontWeight(700).TextColor(mark).SingleLine()
					}
				})
				weight := 500
				if here {
					weight = 650
				}
				ui.Text(c, title).FontSize(12.5).FontWeight(weight).TextColor(fg).SingleLine()
			})
			if step.Clicked() {
				a.page = id
			}
		}
	})
}

// fact is a line of what a step takes from the one before: its name and,
// at the right, what it is.  It cannot be changed here.
func fact(c *ui.Context, title, value string) func() {
	return func() {
		pal := paletteFor(c)
		ui.Row(c).Padding(0, 14).Gap(16).MinHeight(40).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(13.5).SingleLine().Shrink(0)
			ui.Text(c, value).FontSize(12.5).TextColor(pal.muted).SingleLine().Ellipsis("…").Grow(1).MinWidth(0).TextAlign(ui.End).Tooltip(value)
		})
	}
}

// taken is the card of what a step takes from the steps before it: the
// facts, and the way back to where they are chosen.  While the page waits
// for a step the card is empty: the bar at the bottom says what is missing
// and leads there.
func (a *app) taken(c *ui.Context, page string, facts ...func()) {
	pal := paletteFor(c)
	back := flowSteps[stepNumber(page)-2]
	if step, _ := a.lock(page); step != "" {
		ui.Row(c.Key("locked")).Padding(13, 14).Gap(10).Radius(12).AlignItems(ui.Center).Border(1, pal.text.Alpha(0.18)).
			BorderStyle(ui.BorderDashed).Children(func() {
			ui.Icon(c, iconInfo).Size(15, 15).TextColor(pal.faint).Shrink(0)
			ui.Text(c, Tf("What this step takes is listed here once step %d is done.", "完成第 %d 步后，这里会列出本步沿用的内容。", stepNumber(step))).
				FontSize(12.5).TextColor(pal.muted).LineHeight(1.35).Grow(1).MinWidth(0)
		})
		return
	}
	facts = append(facts, func() {
		ui.Row(c).Padding(7, 10).Justify(ui.End).Children(func() {
			if button(c.Key("back"), Tf("Change in Step %d", "返回第 %d 步修改", stepNumber(back)), nil, buttonGhost, 26).Clicked() {
				a.page = back
			}
		})
	})
	rows(c, facts...)
}

// nextStep is the button, beside the one that runs a step, that leads to
// the step after it once this one is done.
func (a *app) nextStep(c *ui.Context, page string) {
	n := stepNumber(page)
	if n == 0 || n == len(flowSteps) || a.job != nil || !a.stepDone(page) {
		return
	}
	next := pageInfoOf(flowSteps[n])
	if button(c.Key("next-step"), Tf("Next: %s", "下一步：%s", T(next.en, next.zh)), iconArrowRight, buttonSoft, 34).Clicked() {
		a.page = next.id
	}
}

// referenceText says in a line which reference the steps use.
func referenceText(path string, info *refInfo) string {
	if path == "" {
		return T("None chosen", "尚未选择")
	}
	text := filepath.Base(path)
	if info != nil && info.err == "" {
		text += Tf(" · %d controls · smoothed %g mm", " · %d 例对照 · 平滑 %g mm", info.N, info.FWHM)
		// How its factors were estimated, where not the usual way.
		if info.Statistic == "median" {
			text += T(" · factors by the median", " · 因子取中位数")
		}
		if info.FactorFWHM != nil {
			text += Tf(" · factors after %s mm", " · 估计因子前平滑 %s mm", num(*info.FactorFWHM))
		}
	}
	return text
}
