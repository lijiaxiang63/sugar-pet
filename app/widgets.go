package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

const pagePad = 32

// status is what a page says about its last run.
type status struct {
	err string
	ok  string
}

// pageColumn is the column, in the middle of the page's card, that the
// page's title, content and action line up in.
func pageColumn(c *ui.Context) ui.Element {
	return ui.Column(c).FillWidth().MaxWidth(contentWidth+2*pagePad).Margin(0, ui.Auto)
}

// scroll is how far a page is scrolled, kept while the app runs.
func (a *app) scroll(page string) *ui.ScrollState {
	if a.scrolls == nil {
		a.scrolls = map[string]*ui.ScrollState{}
	}
	sc := a.scrolls[page]
	if sc == nil {
		sc = &ui.ScrollState{}
		a.scrolls[page] = sc
	}
	return sc
}

// frame lays out a page on its card: its title with an icon badge at the
// top, its content in a scrolling column under it, and a bar at the bottom
// for its main action.
func (a *app) frame(c *ui.Context, title, subtitle string, body func(), footer func()) {
	pal := paletteFor(c)
	bar := c.TitleBar()
	sc := a.scroll(a.page)
	info := pageInfoOf(a.page)
	ui.Column(c).Shrink(0).Padding(0, bar.Right, 0, 0).DragWindow().Children(func() {
		pageColumn(c).Padding(20, pagePad, 14, pagePad).Children(func() {
			ui.Row(c).Gap(13).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(38, 38).Radius(10).Center().Shrink(0).Background(pal.accentSoft).
					InsetShadow(0, 0, 0, 1, pal.accentBorder).Children(func() {
					ui.Icon(c, info.icon).Size(18, 18).TextColor(pal.accent)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, title).FontSize(20.5).FontWeight(700).LetterSpacing(-0.3).SingleLine()
					ui.Text(c, subtitle).FontSize(12.5).TextColor(pal.muted).SingleLine().Ellipsis("…")
				})
			})
		})
	})
	// A line under the title once the content is scrolled under it.
	edge := ui.Box(c.Key("edge")).Height(1).Shrink(0)
	shown := ui.Transparent
	if sc.Y > 0.5 {
		shown = pal.line
	}
	edge.Background(edge.AnimateColor("on", shown, ui.Animation{Duration: 140 * time.Millisecond}))
	ui.Scroll(c.Key("scroll-" + a.page)).Grow(1).TrackScroll(sc).Children(func() {
		pageColumn(c).Padding(10, pagePad, 34, pagePad).Gap(24).Children(body)
	})
	if footer != nil {
		ui.Box(c).Height(1).Shrink(0).Background(pal.line)
		ui.Column(c).Shrink(0).Background(pal.card).Children(func() {
			pageColumn(c).Children(func() {
				ui.Row(c).Height(62).Padding(0, pagePad).Gap(12).AlignItems(ui.Center).Children(footer)
			})
		})
	}
}

// section is a titled group of the page, with what it is for under its
// title.
func section(c *ui.Context, title, caption string, body func()) {
	pal := paletteFor(c)
	ui.Column(c).Gap(10).Children(func() {
		if title != "" || caption != "" {
			ui.Column(c).Gap(3).Padding(0, 2).Children(func() {
				if title != "" {
					ui.Text(c, title).FontSize(14).FontWeight(650)
				}
				if caption != "" {
					ui.Text(c, caption).FontSize(12.5).TextColor(pal.muted).LineHeight(1.4)
				}
			})
		}
		body()
	})
}

// results is the section of what a task made.  It comes into view when
// the task of the page has just finished.
func (a *app) results(c *ui.Context, title, caption string, body func()) {
	box := ui.Column(c.Key("results-" + a.page))
	if a.reveal == a.page {
		a.reveal = ""
		box.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollStart, Animation: &ui.Smooth})
	}
	box.Children(func() { section(c, title, caption, body) })
}

// card is a soft panel on the page holding rows, as the groups of System
// Settings.
func card(c *ui.Context) ui.Element {
	pal := paletteFor(c)
	return ui.Column(c).Radius(12).Background(pal.panel).InsetShadow(0, 0, 0, 1, pal.border).Clip()
}

// divider is the line between two rows of a card.
func divider(c *ui.Context) {
	ui.Box(c).Height(1).Shrink(0).Margin(0, 14).Background(paletteFor(c).line)
}

// rows builds a card of rows with a line between each two.
func rows(c *ui.Context, builders ...func()) {
	card(c).Padding(2, 0).Children(func() {
		first := true
		for _, build := range builders {
			if build == nil {
				continue
			}
			if !first {
				divider(c)
			}
			first = false
			build()
		}
	})
}

// setting is a row with a title and a description on the left and a
// control on the right.
func setting(c *ui.Context, title, desc string, control func()) {
	pal := paletteFor(c)
	ui.Row(c).Padding(10, 14).Gap(16).MinHeight(46).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, title).FontSize(13.5)
			if desc != "" {
				ui.Text(c, desc).FontSize(12).TextColor(pal.muted).LineHeight(1.35)
			}
		})
		if control != nil {
			ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(control)
		}
	})
}

// selectOf is a drop-down of values shown by their labels.
func selectOf(c *ui.Context, value *string, values, labels []string) ui.Element {
	shown := labels[0]
	for i, v := range values {
		if v == *value {
			shown = labels[i]
		}
	}
	e := ui.Select(c, &shown, labels)
	for i, l := range labels {
		if l == shown {
			*value = values[i]
		}
	}
	return e
}

func number(c *ui.Context, value *float64, lo, hi, step float64) ui.Element {
	return ui.NumberInput(c, value, lo, hi, step).Width(120)
}

// fileList is a list of image files the user gathers.
type fileList struct {
	files []string
	state ui.ListState
	query string // optional filter for large lists
}

type seenKey struct {
	path string
	dir  bool
}

type seenFile struct {
	there bool
	at    time.Time
}

// onDisk reports whether a file is there or, with dir, a folder.  The pages
// ask on every frame; the file system, which may be a slow one, answers once
// a second, as what is there may change.  A file is a regular one: a pipe
// of its name would hang whoever reads it.
func (a *app) onDisk(path string, dir bool) bool {
	key := seenKey{path, dir}
	if s, ok := a.seen[key]; ok && time.Since(s.at) < time.Second {
		return s.there
	}
	info, err := os.Stat(path)
	there := err == nil && (dir && info.IsDir() || !dir && info.Mode().IsRegular())
	if a.seen == nil {
		a.seen = map[seenKey]seenFile{}
	}
	a.seen[key] = seenFile{there, time.Now()}
	return there
}

// inStudy is a folder (dir) or a file of the study in use, or "" if it is
// not there, or if the folder in use is no longer that study: moved away,
// with another under its name, whose images are not those the pages hold.
func (a *app) inStudy(name string, dir bool) string {
	p := &a.study
	if p.inUse == "" {
		return ""
	}
	// The folder in use is known by its own key: "" is no path.
	held := seenKey{"", true}
	s, ok := a.seen[held]
	if !ok || time.Since(s.at) >= time.Second {
		if a.seen == nil {
			a.seen = map[seenKey]seenFile{}
		}
		s = seenFile{p.holds(p.inUse), time.Now()}
		a.seen[held] = s
	}
	if path := filepath.Join(p.inUse, name); s.there && a.onDisk(path, dir) {
		return path
	}
	return ""
}

// stillInStudy asks the file system now, when a shortcut to the study in
// use is taken, whether what it leads to is there and of that study.
func (a *app) stillInStudy(name string, dir bool) bool {
	a.seen = nil
	return a.inStudy(name, dir) != ""
}

// commonDir returns the shared parent folder if all files live in the same
// directory, or "" otherwise.
func commonDir(files []string) string {
	if len(files) == 0 {
		return ""
	}
	dir := filepath.Dir(files[0])
	for _, p := range files[1:] {
		if filepath.Dir(p) != dir {
			return ""
		}
	}
	return dir
}

// expandPaths replaces folders by the NIfTI images in them.
func expandPaths(paths []string) []string {
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			lower := strings.ToLower(p)
			if core.IsNiftiName(p) || strings.HasSuffix(lower, ".img") || strings.HasSuffix(lower, ".hdr") {
				out = append(out, p)
			}
			continue
		}
		// A folder that is a link, as the patients of a study may be, is
		// walked where it leads: WalkDir does not follow it.  The images
		// keep the names they have under the link.
		root := p
		if real, err := filepath.EvalSymlinks(p); err == nil {
			root = real
		}
		var found []string
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				// A saved reference is not a set of subjects.
				if core.IsReferenceFolder(path) || strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if core.IsNiftiName(d.Name()) && !strings.HasPrefix(d.Name(), ".") {
				if rel, err := filepath.Rel(root, path); err == nil && root != p {
					path = filepath.Join(p, rel)
				}
				found = append(found, path)
			}
			return nil
		})
		sort.Strings(found)
		out = append(out, found...)
	}
	return out
}

func (f *fileList) add(paths []string) {
	have := map[string]bool{}
	for _, p := range f.files {
		have[p] = true
	}
	for _, p := range expandPaths(paths) {
		if !have[p] {
			have[p] = true
			f.files = append(f.files, p)
		}
	}
}

func (f *fileList) remove(path string) {
	for i, p := range f.files {
		if p == path {
			f.files = append(f.files[:i:i], f.files[i+1:]...)
			return
		}
	}
}

// filesCard shows a file list: drop files or folders on it, or add them
// with its buttons.
func (a *app) filesCard(c *ui.Context, key string, f *fileList, title, hint string) {
	a.filesCardFromStudy(c, key, f, title, hint, "")
}

// filesCardFromStudy is filesCard with an optional one-click shortcut to load
// images from a subfolder of the active study (e.g. "patients", "controls").
func (a *app) filesCardFromStudy(c *ui.Context, key string, f *fileList, title, hint, studySubdir string) {
	pal := paletteFor(c)
	n := len(f.files)
	if n <= 8 && f.query != "" {
		f.query = ""
	}
	box := ui.Column(c.Key(key)).Radius(12).Clip().Background(pal.panel)
	if files := box.DroppedFiles(); files != nil {
		f.add(files)
	}
	over := box.FileDragOver()
	if over {
		box.Background(pal.accentSoft)
	}
	switch {
	case n == 0 && over:
		box.Border(1, pal.accent).BorderStyle(ui.BorderDashed)
	case n == 0:
		box.Border(1, pal.text.Alpha(0.18)).BorderStyle(ui.BorderDashed)
	case over:
		box.InsetShadow(0, 0, 0, 1.5, pal.accent)
	default:
		box.InsetShadow(0, 0, 0, 1, pal.border)
	}
	studyDir := ""
	if studySubdir != "" {
		studyDir = a.inStudy(studySubdir, true)
	}
	addButtons := func(kind buttonKind, height float32) {
		if studyDir != "" {
			label := Tf("Load %s/", "加载研究 %s/", studySubdir)
			if button(c.Key("from-study"), label, iconFolder, buttonSoft, height).Tooltip(studyDir).Clicked() && a.stillInStudy(studySubdir, true) {
				f.add([]string{studyDir})
			}
		}
		if button(c.Key("files"), T("Add Files…", "添加文件…"), iconPlus, kind, height).Clicked() {
			a.pick(pickOptions{Title: title, Multiple: true, Filters: niftiFilter}, f.add)
		}
		if button(c.Key("folder"), T("Add Folder…", "添加文件夹…"), iconFolder, kind, height).Clicked() {
			a.pick(pickOptions{Title: title, Directory: true, Multiple: true}, f.add)
		}
	}
	shared := commonDir(f.files)
	box.Children(func() {
		ui.Row(c).Padding(11, 14, 0, 14).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(13).FontWeight(600).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
			if n > 0 {
				pill(c, strconv.Itoa(n), pal.accent)
			}
			if shared != "" && n > 0 {
				ui.Text(c, shortDir(shared)).FontSize(11).TextColor(pal.faint).SingleLine().Ellipsis("…").Grow(1).MinWidth(0).TextAlign(ui.End).Tooltip(shared)
			}
		})
		if n == 0 {
			ui.Column(c).Padding(4, 16, 22, 16).Gap(3).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(38, 38).Radius(19).Center().Background(pal.accentSoft).InsetShadow(0, 0, 0, 1, pal.accentBorder).Children(func() {
					ui.Icon(c, iconTray).Size(18, 18).TextColor(pal.accent)
				})
				ui.Text(c, T("Drop NIfTI images or folders here", "将 NIfTI 图像或文件夹拖到这里")).FontSize(13.5).FontWeight(600).
					Margin(8, 0, 0, 0).TextAlign(ui.Center)
				ui.Text(c, hint).FontSize(12).TextColor(pal.muted).TextAlign(ui.Center).LineHeight(1.35)
				ui.Row(c).Gap(8).Margin(11, 0, 0, 0).Wrap().GapY(6).Justify(ui.Center).Children(func() {
					addButtons(buttonPlain, 28)
				})
			})
			return
		}
		shown := f.files
		if n > 8 {
			ui.Row(c).Padding(8, 12, 2, 12).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.SearchField(c.Key("filter"), &f.query).Placeholder(T("Filter images…", "筛选图像…")).Grow(1).MinWidth(0)
			})
			if q := strings.ToLower(strings.TrimSpace(f.query)); q != "" {
				shown = nil
				for _, p := range f.files {
					if strings.Contains(strings.ToLower(filepath.Base(p)), q) {
						shown = append(shown, p)
					}
				}
			}
		}
		ui.List(c, &f.state, len(shown), func(i int) {
			path := shown[i]
			row := ui.Row(c.Key(path)).Margin(0, 6).Padding(0, 4, 0, 8).Height(30).Radius(7).Gap(8).AlignItems(ui.Center)
			if row.Hovered() {
				row.Background(pal.rowHover)
			}
			row.Children(func() {
				ui.Icon(c, iconDoc).Size(14, 14).TextColor(pal.faint).Shrink(0)
				nameEl := ui.Text(c, filepath.Base(path)).FontSize(12.5).SingleLine().Ellipsis("…").MinWidth(0).Tooltip(path)
				if shared != "" {
					nameEl.Grow(1)
				} else {
					nameEl.MinWidth(80)
					ui.Text(c, shortDir(filepath.Dir(path))).FontSize(11.5).TextColor(pal.faint).SingleLine().Ellipsis("…").Grow(1).MinWidth(0).TextAlign(ui.End)
				}
				del := iconButton(c, iconClose, T("Remove", "移除")).Tooltip(T("Remove", "移除"))
				del.OnClick(func() { f.remove(path) })
			})
		}).MaxHeight(205).Padding(6, 0, 4, 0)
		divider(c)
		ui.Row(c).Padding(6, 8).Gap(4).Wrap().GapY(4).AlignItems(ui.Center).Children(func() {
			addButtons(buttonGhost, 26)
			ui.Spacer(c)
			if button(c.Key("clear"), T("Clear", "清空"), nil, buttonGhost, 26).Clicked() {
				f.files, f.query = nil, ""
			}
		})
	})
}

// shortDir abbreviates the home folder.
func shortDir(dir string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(dir, home) {
		return "~" + dir[len(home):]
	}
	return dir
}

type pathKind int

const (
	pathFolder pathKind = iota
	pathFile
	pathSave
)

// pathControl shows a chosen file or folder with a button to choose it,
// for the right side of a setting.  It reports a new choice.
func (a *app) pathControl(c *ui.Context, key string, value *string, kind pathKind, title string, filters []mygo.FileFilter, changed func()) {
	pal := paletteFor(c)
	if *value != "" {
		chip := ui.Row(c.Key(key+"-chip")).Height(26).Padding(0, 3, 0, 9).Gap(6).Radius(7).AlignItems(ui.Center).MaxWidth(280).
			Background(pal.control).Tooltip(*value)
		face(chip, pal)
		chip.Children(func() {
			ic := iconFolder
			if kind != pathFolder {
				ic = iconDoc
			}
			ui.Icon(c, ic).Size(13, 13).TextColor(pal.accent).Shrink(0)
			ui.Text(c, filepath.Base(*value)).FontSize(12.5).FontWeight(500).SingleLine().Ellipsis("…").MinWidth(0)
			clear := iconButton(c, iconClose, T("Clear", "清除"))
			clear.OnClick(func() {
				*value = ""
				if changed != nil {
					changed()
				}
			})
		})
	}
	label := T("Choose…", "选择…")
	if *value != "" {
		label = T("Change…", "更改…")
	}
	kindOf := buttonPlain
	if *value != "" {
		kindOf = buttonGhost
	}
	if button(c.Key(key+"-pick"), label, nil, kindOf, 26).Clicked() {
		set := func(path string) {
			*value = path
			if changed != nil {
				changed()
			}
		}
		if kind == pathSave {
			go func() {
				path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.dialogs, Title: title, DefaultPath: "reference-cluster.nii.gz"})
				if err == nil && path != "" {
					a.update(func() { set(path) })
				}
			}()
		} else {
			a.pick(pickOptions{Title: title, Directory: kind == pathFolder, Filters: filters}, func(p []string) { set(p[0]) })
		}
	}
}

// pathSetting is a setting whose control chooses a file or folder; files
// dropped on the row are taken too.
func (a *app) pathSetting(c *ui.Context, key, title, desc string, value *string, kind pathKind, filters []mygo.FileFilter, changed func()) {
	pal := paletteFor(c)
	row := ui.Column(c.Key(key + "-row"))
	if kind != pathSave {
		if files := row.DroppedFiles(); len(files) > 0 {
			info, err := os.Stat(files[0])
			if err == nil && info.IsDir() == (kind == pathFolder) {
				*value = files[0]
				if changed != nil {
					changed()
				}
			}
		}
		if row.FileDragOver() {
			row.Background(pal.accentSoft)
		}
	}
	row.Children(func() {
		setting(c, title, desc, func() { a.pathControl(c, key, value, kind, title, filters, changed) })
	})
}

// remedy is where the problem a page reports is mended, when a button of
// its run bar can lead there: "reference" for the Reference page, "spm" for
// the settings of SPM and MATLAB, or "".  The problem is known by being the
// very one the page gives for that lack, not by its wording.
func (a *app) remedy(page, problem string) string {
	if problem == "" {
		return ""
	}
	lack := ""
	switch page {
	case "spatial":
		if problem == a.spatial.setupProblem() {
			return "spm"
		}
	case "normalize":
		lack = a.norm.referenceProblem()
	case "compare":
		lack = a.cmp.referenceProblem()
	case "zmap":
		if a.zmap.reference == "" {
			lack = needReference()
		}
	case "classify":
		if a.classify.reference == "" {
			lack = needReference()
		}
	case "cluster":
		if a.cluster.reference == "" {
			lack = needReference()
		}
	}
	if problem == lack {
		return "reference"
	}
	return ""
}

// runBar is the bar at the bottom of a page: what is missing or how the
// last run went on the left, and the button that starts the work, or the
// progress of the work and a button that stops it.
func (a *app) runBar(c *ui.Context, page, problem, runLabel string, st *status, run func()) {
	pal := paletteFor(c)
	if j := a.job; j != nil && j.page == page {
		ui.Column(c).Grow(1).Gap(7).MinWidth(0).Children(func() {
			value := -1.0
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, j.label).FontSize(12.5).FontWeight(500).SingleLine().Grow(1).MinWidth(0)
				if j.total > 0 {
					value = float64(j.done) / float64(j.total)
					pct := int(value*100 + 0.5)
					ui.Text(c, strconv.Itoa(pct)+"% · "+strconv.Itoa(j.done)+" / "+strconv.Itoa(j.total)).FontSize(12).TextColor(pal.muted).SingleLine()
				}
			})
			ui.Progress(c, value).Label(j.label).TextColor(pal.accent).Background(pal.well)
		})
		if button(c, T("Cancel", "取消"), nil, buttonPlain, 32).Clicked() {
			j.cancel()
			j.label = T("Stopping…", "正在停止…")
		}
		return
	}
	message := func(ic *ui.SVG, color ui.Color, text string) {
		ui.Row(c).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, ic).Size(15, 15).TextColor(color).Shrink(0)
			ui.Text(c, text).FontSize(12.5).TextColor(color).MaxLines(2).LineHeight(1.3).Grow(1).MinWidth(0).Selectable()
		})
	}
	// The way out of what is missing now, also beside the error of the last
	// run, which stays on the bar while the button is disabled for it.
	mend := func() {
		switch a.remedy(page, problem) {
		case "reference":
			if button(c.Key("fix-ref"), T("Go to Reference", "前往参考库"), iconArrowRight, buttonSoft, 28).Clicked() {
				a.page = "reference"
			}
		case "spm":
			if button(c.Key("fix-spm"), T("Set Up SPM…", "设置 SPM…"), iconGear, buttonSoft, 28).Clicked() {
				a.openSettings("spatial")
			}
		}
	}
	switch {
	case st.err != "":
		message(iconWarn, pal.danger, st.err)
		mend()
	case problem != "":
		message(iconInfo, pal.muted, problem)
		mend()
	case st.ok != "":
		message(iconCheck, pal.success, st.ok)
	case a.job != nil:
		message(iconInfo, pal.muted, T("Another task is running.", "另一项任务正在运行。"))
	default:
		message(iconCheck, pal.muted, T("Ready.", "准备就绪。"))
	}
	btn := button(c, runLabel, nil, buttonPrimary, 34).Disabled(problem != "" || a.job != nil)
	if btn.Clicked() {
		st.err, st.ok = "", ""
		run()
	}
}

// revealButton shows a file or folder in Finder, Explorer or the file manager.
func revealButton(c *ui.Context, key, label, path string) {
	if button(c.Key(key), label, iconReveal, buttonPlain, 26).Clicked() {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			mygo.Shell.OpenPath(path)
		} else {
			mygo.Shell.ShowItemInFolder(path)
		}
	}
}

func revealLabel() string {
	return T("Show in Finder", "在访达中显示")
}

// notesCard shows warnings below results.
func notesCard(c *ui.Context, lines []string) {
	if len(lines) == 0 {
		return
	}
	pal := paletteFor(c)
	callout(c, iconWarn, pal.warning, func() {
		for i, line := range lines {
			if i == 8 {
				ui.Text(c, Tf("… and %d more", "……另有 %d 条", len(lines)-8)).FontSize(12).TextColor(pal.muted)
				break
			}
			ui.Text(c, line).FontSize(12).LineHeight(1.35)
		}
	}).Selectable()
}

// caution is a note of what to mind before going on.
func caution(c *ui.Context, text string) {
	pal := paletteFor(c)
	callout(c, iconWarn, pal.warning, func() {
		ui.Text(c, text).FontSize(12.5).LineHeight(1.4)
	})
}

// stats shows numbers side by side, each with a caption, as the summary
// of a result.
func stats(c *ui.Context, items ...[2]string) {
	pal := paletteFor(c)
	ui.Row(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
		for _, it := range items {
			ui.Column(c).Gap(3).Grow(1).Basis(0).MinWidth(0).Padding(13, 15).Radius(12).Background(pal.panel).
				InsetShadow(0, 0, 0, 1, pal.border).Children(func() {
				ui.Text(c, it[1]).FontSize(11.5).FontWeight(500).TextColor(pal.muted).SingleLine().Ellipsis("…")
				ui.Text(c, it[0]).FontSize(20).FontWeight(700).LetterSpacing(-0.3).SingleLine().Ellipsis("…")
			})
		}
	})
}
