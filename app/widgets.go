package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

func hairline(t *ui.Theme) ui.Color { return t.Text.Alpha(0.1) }
func cardFill(t *ui.Theme) ui.Color { return t.Text.Alpha(0.035) }

// status is what a page says about its last run.
type status struct {
	err string
	ok  string
}

// frame lays out a page: its title under the window controls, its content
// in a scrolling column, and a bar at the bottom for its main action.
func (a *app) frame(c *ui.Context, title, subtitle string, body func(), footer func()) {
	t := c.Theme()
	bar := c.TitleBar()
	ui.Row(c).Height(titleBarHeight).Shrink(0).AlignItems(ui.Center).Padding(0, bar.Right+24, 0, 28).Gap(12).DragWindow().Children(func() {
		ui.Text(c, title).FontSize(15).Bold().SingleLine().Shrink(0)
		ui.Text(c, subtitle).FontSize(12).TextColor(t.TextMuted).SingleLine().Grow(1).MinWidth(0)
	})
	ui.Box(c).Height(1).Shrink(0).Background(hairline(t))
	ui.Scroll(c.Key("scroll-" + a.page)).Grow(1).Children(func() {
		ui.Column(c).Padding(22, 28, 30, 28).Gap(24).MaxWidth(contentWidth+56).Margin(0, ui.Auto).FillWidth().Children(body)
	})
	if footer != nil {
		ui.Box(c).Height(1).Shrink(0).Background(hairline(t))
		ui.Row(c).Height(62).Shrink(0).Padding(0, 28).Gap(16).AlignItems(ui.Center).Children(footer)
	}
}

// section is a titled group of the page.
func section(c *ui.Context, title, caption string, body func()) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(13).Bold().Padding(0, 4)
		}
		body()
		if caption != "" {
			ui.Text(c, caption).FontSize(11.5).TextColor(t.TextMuted).Padding(0, 4)
		}
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

// card is a rounded box holding rows, as the groups of System Settings.
func card(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Column(c).Radius(10).Background(cardFill(t)).Border(1, hairline(t)).Clip()
}

// rows builds a card of rows with a line between each two.
func rows(c *ui.Context, builders ...func()) {
	t := c.Theme()
	card(c).Children(func() {
		first := true
		for _, build := range builders {
			if build == nil {
				continue
			}
			if !first {
				ui.Box(c).Height(1).Margin(0, 0, 0, 14).Background(hairline(t))
			}
			first = false
			build()
		}
	})
}

// setting is a row with a title and a description on the left and a
// control on the right.
func setting(c *ui.Context, title, desc string, control func()) {
	t := c.Theme()
	ui.Row(c).Padding(8, 14).Gap(16).MinHeight(42).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(13)
			if desc != "" {
				ui.Text(c, desc).FontSize(11.5).TextColor(t.TextMuted)
			}
		})
		if control != nil {
			ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(control)
		}
	})
}

func smallButton(c *ui.Context, label string) ui.Element {
	return ui.Button(c, label).FontSize(12).Padding(3, 10)
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
		var found []string
		filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				// A saved reference is not a set of subjects.
				if core.IsReferenceFolder(path) || strings.HasPrefix(d.Name(), ".") && path != p {
					return filepath.SkipDir
				}
				return nil
			}
			if core.IsNiftiName(d.Name()) && !strings.HasPrefix(d.Name(), ".") {
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
	t := c.Theme()
	box := card(c.Key(key))
	if files := box.DroppedFiles(); files != nil {
		f.add(files)
	}
	if box.FileDragOver() {
		box.Border(1.5, t.Accent).Background(t.Accent.Alpha(0.1))
	}
	n := len(f.files)
	box.Children(func() {
		ui.Row(c).Padding(8, 10, 8, 14).Gap(8).AlignItems(ui.Center).MinHeight(42).Children(func() {
			ui.Text(c, title).FontSize(13)
			if n > 0 {
				ui.Badge(c, strconv.Itoa(n))
			}
			ui.Spacer(c)
			if n > 0 {
				if smallButton(c.Key("clear"), T("Clear", "清空")).Clicked() {
					f.files = nil
				}
			}
			if smallButton(c.Key("folder"), T("Add Folder…", "添加文件夹…")).Clicked() {
				a.pick(pickOptions{Title: title, Directory: true, Multiple: true}, f.add)
			}
			if smallButton(c.Key("files"), T("Add Files…", "添加文件…")).Clicked() {
				a.pick(pickOptions{Title: title, Multiple: true, Filters: niftiFilter}, f.add)
			}
		})
		ui.Box(c).Height(1).Background(hairline(t))
		if n == 0 {
			ui.Column(c).Padding(20, 16, 22, 16).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconTray).Size(22, 22).TextColor(t.TextMuted)
				ui.Text(c, T("Drop NIfTI images or folders here", "将 NIfTI 图像或文件夹拖到这里")).FontSize(13).Margin(4, 0, 0, 0)
				ui.Text(c, hint).FontSize(11.5).TextColor(t.TextMuted).TextAlign(ui.Center)
			})
			return
		}
		ui.List(c, &f.state, n, func(i int) {
			path := f.files[i]
			row := ui.Row(c.Key(path)).Padding(0, 10, 0, 14).Height(28).Gap(8).AlignItems(ui.Center)
			if row.Hovered() {
				row.Background(t.Text.Alpha(0.045))
			}
			row.Children(func() {
				ui.Icon(c, iconDoc).Size(14, 14).TextColor(t.TextMuted)
				ui.Text(c, filepath.Base(path)).FontSize(12.5).SingleLine().MinWidth(80)
				ui.Text(c, shortDir(filepath.Dir(path))).FontSize(11.5).TextColor(t.TextMuted).SingleLine().Grow(1).MinWidth(0).TextAlign(ui.End)
				del := ui.ButtonBase(c).Size(18, 18).Radius(9).Center().Shrink(0).TextColor(t.TextMuted).
					Label(T("Remove", "移除")).Tooltip(T("Remove", "移除"))
				if del.Hovered() {
					del.Background(t.Text.Alpha(0.12))
				}
				del.Children(func() { ui.Icon(c, iconClose).Size(10, 10) })
				del.OnClick(func() { f.remove(path) })
			})
		}).MaxHeight(196).Padding(4, 0)
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
	t := c.Theme()
	if *value != "" {
		chip := ui.Row(c.Key(key+"-chip")).Padding(3, 4, 3, 8).Gap(6).Radius(6).AlignItems(ui.Center).MaxWidth(300).
			Background(t.Text.Alpha(0.06)).Tooltip(*value)
		chip.Children(func() {
			ic := iconFolder
			if kind != pathFolder {
				ic = iconDoc
			}
			ui.Icon(c, ic).Size(13, 13).TextColor(t.TextMuted)
			ui.Text(c, filepath.Base(*value)).FontSize(12.5).SingleLine().MinWidth(0)
			clear := ui.ButtonBase(c).Size(16, 16).Radius(8).Center().Shrink(0).TextColor(t.TextMuted).Label(T("Clear", "清除"))
			if clear.Hovered() {
				clear.Background(t.Text.Alpha(0.12))
			}
			clear.Children(func() { ui.Icon(c, iconClose).Size(9, 9) })
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
	if smallButton(c.Key(key+"-pick"), label).Clicked() {
		set := func(path string) {
			*value = path
			if changed != nil {
				changed()
			}
		}
		if kind == pathSave {
			go func() {
				path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: title, DefaultPath: "reference-cluster.nii.gz"})
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
	t := c.Theme()
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
			row.Background(t.Accent.Alpha(0.1))
		}
	}
	row.Children(func() {
		setting(c, title, desc, func() { a.pathControl(c, key, value, kind, title, filters, changed) })
	})
}

// runBar is the bar at the bottom of a page: what is missing or how the
// last run went on the left, and the button that starts the work, or the
// progress of the work and a button that stops it.
func (a *app) runBar(c *ui.Context, page, problem, runLabel string, st *status, run func()) {
	t := c.Theme()
	if j := a.job; j != nil && j.page == page {
		ui.Column(c).Grow(1).Gap(6).MinWidth(0).Children(func() {
			text := j.label
			value := -1.0
			if j.total > 0 {
				text += "  " + strconv.Itoa(j.done) + " / " + strconv.Itoa(j.total)
				value = float64(j.done) / float64(j.total)
			}
			ui.Text(c, text).FontSize(12).TextColor(t.TextMuted).SingleLine()
			ui.Progress(c, value).Label(j.label)
		})
		if ui.Button(c, T("Cancel", "取消")).Clicked() {
			j.cancel()
			j.label = T("Stopping…", "正在停止…")
		}
		return
	}
	message := func(ic *ui.SVG, color ui.Color, text string) {
		ui.Row(c).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, ic).Size(15, 15).TextColor(color).Shrink(0)
			ui.Text(c, text).FontSize(12.5).TextColor(color).MaxLines(2).Grow(1).MinWidth(0).Selectable()
		})
	}
	switch {
	case st.err != "":
		message(iconWarn, t.Danger, st.err)
	case problem != "":
		message(iconInfo, t.TextMuted, problem)
	case st.ok != "":
		message(iconCheck, t.Success, st.ok)
	case a.job != nil:
		message(iconInfo, t.TextMuted, T("Another task is running.", "另一项任务正在运行。"))
	default:
		message(iconCheck, t.TextMuted, T("Ready.", "准备就绪。"))
	}
	btn := ui.PrimaryButton(c, runLabel).Padding(7, 18).Disabled(problem != "" || a.job != nil)
	if btn.Clicked() {
		st.err, st.ok = "", ""
		run()
	}
}

// revealButton shows a file or folder in Finder, Explorer or the file manager.
func revealButton(c *ui.Context, key, label, path string) {
	b := ui.Button(c.Key(key), "").FontSize(12).Padding(3, 10).Gap(5)
	b.Children(func() {
		ui.Icon(c, iconReveal).Size(12, 12)
		ui.Text(c, label).FontSize(12).SingleLine()
	})
	if b.Clicked() {
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

// notes shows warnings below results.
func notesCard(c *ui.Context, lines []string) {
	if len(lines) == 0 {
		return
	}
	t := c.Theme()
	ui.Column(c).Radius(10).Padding(10, 14).Gap(4).Background(t.Warning.Alpha(0.1)).Border(1, t.Warning.Alpha(0.3)).Selectable().Children(func() {
		for i, line := range lines {
			if i == 8 {
				ui.Text(c, Tf("… and %d more", "……另有 %d 条", len(lines)-8)).FontSize(12).TextColor(t.TextMuted)
				break
			}
			ui.Text(c, line).FontSize(12)
		}
	})
}

// stat is a number with a caption, for the summary of a result.
func stat(c *ui.Context, value, caption string) {
	t := c.Theme()
	ui.Column(c).Gap(2).Grow(1).Basis(0).Padding(10, 14).Children(func() {
		ui.Text(c, value).FontSize(17).Bold().SingleLine()
		ui.Text(c, caption).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
	})
}

// stats is a card of numbers side by side.
func stats(c *ui.Context, items ...[2]string) {
	t := c.Theme()
	ui.Row(c).Radius(10).Background(cardFill(t)).Border(1, hairline(t)).AlignItems(ui.Stretch).Children(func() {
		for i, it := range items {
			if i > 0 {
				ui.Box(c).Width(1).Margin(10, 0).Background(hairline(t))
			}
			stat(c, it[0], it[1])
		}
	})
}
