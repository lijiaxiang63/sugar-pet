// SUGAR-PET is the desktop app of sugar-pet: intensity normalisation of
// brain FDG-PET images, with nothing to install beside it (only warping
// images into MNI space, which is SPM's work, needs MATLAB and SPM).
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

const (
	titleBarHeight = 52
	sidebarWidth   = 232
	contentWidth   = 720
	cardGap        = 8
)

// job is the task running in the background, if any.
type job struct {
	page        string
	label       string
	done, total int
	cancel      context.CancelFunc
	started     time.Time
}

// app is the state the window shows.
type app struct {
	win  *mygo.Window
	post func(func()) // runs a function on the main thread; the window's Update, but for tests
	page string
	job  *job
	// reveal is the page whose results are new and should come into view.
	reveal string
	// scrolls is how far each page is scrolled.
	scrolls map[string]*ui.ScrollState

	// opt are the options of the analyses, which the settings window changes.
	opt options
	// The settings window while it is open, and the section it shows.
	settingsWin  *mygo.Window
	settingsPage string
	// dialogs is the window that file dialogs and alerts belong to: the one
	// whose view was built last.
	dialogs *mygo.Window
	// allMethods is whether the pages list every method, the reference
	// regions too.
	allMethods bool
	// updates is whether the update plugin is used (useUpdater).
	updates bool

	spatial  spatialPage
	norm     normPage
	ref      refPage
	zmap     zmapPage
	classify classifyPage
	cmp      comparePage
	cluster  clusterPage
	masks    masksPage

	settingsPath string
	saved        string // the settings as last written
}

// settings is what the app remembers between launches.
type settings struct {
	Language   string   `json:"language"`
	Page       string   `json:"page"`
	Reference  string   `json:"reference"`
	Output     string   `json:"output"`
	SPM        string   `json:"spm,omitempty"`
	Matlab     string   `json:"matlab,omitempty"`
	AllMethods bool     `json:"all_methods,omitempty"`
	Options    *options `json:"options,omitempty"`
}

func newApp() *app {
	a := &app{page: "normalize", opt: defaultOptions()}
	a.norm.init()
	a.ref.init()
	a.zmap.init()
	a.classify.init()
	a.cmp.init()
	a.cluster.init()
	return a
}

func (a *app) loadSettings() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return
	}
	a.settingsPath = filepath.Join(dir, "settings.json")
	raw, err := os.ReadFile(a.settingsPath)
	if err != nil {
		return
	}
	// What the file leaves out stays as the app first has it.
	opt := defaultOptions()
	s := settings{Options: &opt}
	if json.Unmarshal(raw, &s) != nil {
		return
	}
	if s.Options != nil {
		a.opt = *s.Options
		a.opt.sound()
	}
	switch s.Language {
	case "zh":
		chinese = true
	case "en":
		chinese = false
	}
	for _, p := range pages {
		if p.id == s.Page {
			a.page = s.Page
		}
	}
	if s.Reference != "" {
		if _, err := os.Stat(filepath.Join(s.Reference, "reference.json")); err == nil {
			a.setReference(s.Reference)
		}
	}
	a.allMethods = s.AllMethods
	a.spatial.setSPM(s.SPM)
	if _, err := core.ResolveMatlab(s.Matlab); err == nil {
		a.spatial.matlab = s.Matlab
	}
	a.saved = a.settingsText()
}

// settingsText is what the app remembers between launches, as it is now.
func (a *app) settingsText() string {
	s := settings{Language: "en", Page: a.page, Reference: a.norm.reference, SPM: a.spatial.spm, Matlab: a.spatial.matlab,
		AllMethods: a.allMethods, Options: &a.opt}
	if chinese {
		s.Language = "zh"
	}
	raw, _ := json.MarshalIndent(s, "", "  ")
	return string(raw)
}

// saveSettings keeps what the app remembers between launches, and reports
// whether any of it changed since it was last kept.
func (a *app) saveSettings() bool {
	text := a.settingsText()
	if text == a.saved {
		return false
	}
	a.saved = text
	if a.settingsPath != "" {
		os.MkdirAll(filepath.Dir(a.settingsPath), 0o755)
		os.WriteFile(a.settingsPath, []byte(text), 0o644)
	}
	return true
}

// setReference makes a reference folder the one every page uses.
func (a *app) setReference(path string) {
	a.norm.reference, a.zmap.reference, a.cmp.reference, a.cluster.reference = path, path, path, path
	a.classify.reference = path
	a.zmap.loadedFrom, a.classify.loadedFrom = "", ""
	a.cmp.fwhmKnown = false
}

type pageInfo struct {
	id     string
	en, zh string
	icon   *ui.SVG
}

var pages = []pageInfo{
	{"spatial", "Spatial Normalisation", "空间标准化", iconWarp},
	{"normalize", "Intensity Normalisation", "强度归一化", iconSliders},
	{"reference", "Reference", "参考库", iconLayers},
	{"zmap", "Z-score Maps", "Z 值图", iconCurve},
	{"classify", "Classify Parkinsonism", "帕金森综合征分类", iconReport},
	{"compare", "Compare Groups", "组间比较", iconCompare},
	{"cluster", "Reference Cluster", "参考簇", iconTarget},
	{"masks", "Masks & Atlas", "掩膜与图谱", iconGrid},
}

func (a *app) view(c *ui.Context) {
	pal := paletteFor(c)
	c.SetTheme(themeFor(c, pal))
	a.dialogs = a.win
	if runtime.GOOS != "darwin" { // on macOS the menu bar has the item
		c.OnShortcut(ui.Cmd, ui.KeyComma, func() { a.openSettings("") })
	}
	// The wash lies over the window's material, which shows through a
	// little where there is one.
	wash := pal.wash
	if c.Vibrancy() {
		wash = wash.Alpha(0.86)
	}
	c.Root().Background(wash)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(sidebarWidth).Shrink(0).Children(func() { a.sidebar(c) })
		ui.Column(c).Grow(1).MinWidth(0).Padding(cardGap, cardGap, cardGap, 0).Children(func() {
			pane := ui.Column(c).Grow(1).MinHeight(0).Radius(12).Background(pal.card).Clip()
			raised(pane, pal).Children(func() {
				switch a.page {
				case "spatial":
					a.spatialView(c)
				case "normalize":
					a.normalizeView(c)
				case "reference":
					a.referenceView(c)
				case "zmap":
					a.zmapView(c)
				case "classify":
					a.classifyView(c)
				case "compare":
					a.compareView(c)
				case "cluster":
					a.clusterView(c)
				case "masks":
					a.masksView(c)
				}
			})
		})
	})
	// What changed is kept, and shows in the settings window.
	if a.saveSettings() && a.settingsWin != nil {
		a.settingsWin.Invalidate()
	}
}

// done reports whether the last run of a page went well, which the
// sidebar marks.
func (a *app) done(page string) bool {
	switch page {
	case "spatial":
		return a.spatial.st.ok != ""
	case "normalize":
		return a.norm.st.ok != ""
	case "reference":
		return a.ref.st.ok != ""
	case "zmap":
		return a.zmap.st.ok != ""
	case "classify":
		return a.classify.st.ok != ""
	case "compare":
		return a.cmp.st.ok != ""
	case "cluster":
		return a.cluster.st.ok != ""
	}
	return false
}

// navItem is a page of the sidebar: the chosen one a face raised off the
// wash, with its icon in the accent.
func (a *app) navItem(c *ui.Context, page *string, p pageInfo) {
	pal := paletteFor(c)
	title := T(p.en, p.zh)
	item := ui.RadioBase(c.Key("nav-"+p.id), page, p.id).Height(32).Padding(0, 9, 0, 10).Gap(10).Radius(8).AlignItems(ui.Center).Label(title)
	chosen := *page == p.id
	bg, fg, tint := ui.Transparent, pal.text.Alpha(0.86), pal.muted
	switch {
	case chosen:
		bg, fg, tint = pal.rowSelected, pal.text, pal.accent
		face(item, pal)
	case item.Hovered():
		bg = pal.rowHover
	}
	item.Background(item.AnimateColor("bg", bg, quick)).Children(func() {
		ui.Icon(c, p.icon).Size(16, 16).TextColor(tint).Shrink(0)
		ui.Text(c, title).FontSize(13).TextColor(fg).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
		switch {
		case a.job != nil && a.job.page == p.id:
			ui.Spinner(c).Size(12, 12)
		case a.done(p.id):
			ui.Icon(c, iconTick).Size(11, 11).TextColor(pal.success).Shrink(0)
		}
	})
}

func (a *app) sidebar(c *ui.Context) {
	pal := paletteFor(c)
	bar := c.TitleBar()
	ui.Box(c).Height(max(bar.Bottom+14, titleBarHeight)).Shrink(0).DragWindow()
	ui.Row(c).Padding(0, 16, 14, 16).Gap(10).AlignItems(ui.Center).Shrink(0).DragWindow().Children(func() {
		ui.Image(c, appLogo).Size(32, 32)
		ui.Column(c).Gap(1).MinWidth(0).Children(func() {
			ui.Text(c, "SUGAR-PET").FontSize(14.5).FontWeight(700).LetterSpacing(0.2).SingleLine()
			ui.Text(c, T("FDG-PET normalisation", "FDG-PET 强度归一化")).FontSize(11).TextColor(pal.muted).SingleLine()
		})
	})
	page := a.page
	group := func(title string, items []pageInfo) {
		ui.Text(c, title).FontSize(11).FontWeight(600).TextColor(pal.faint).LetterSpacing(0.3).Padding(10, 10, 5, 10).SingleLine()
		for _, p := range items {
			a.navItem(c, &page, p)
		}
	}
	ui.Scroll(c.Key("nav")).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(0, 10, 8, 10).Gap(2).Children(func() {
			group(T("Workflow", "工作流程"), pages[:len(pages)-1])
			group(T("Resources", "资源"), pages[len(pages)-1:])
		})
	})
	a.page = page

	// The reference in use, and the settings.
	ui.Column(c).Padding(8, 10, 10, 10).Gap(6).Shrink(0).Children(func() {
		ref := a.norm.reference
		box := ui.ButtonBase(c.Key("ref-chip")).Padding(8, 10).Radius(10).Gap(9).AlignItems(ui.Center).
			Label(T("Reference in use", "当前参考库"))
		bg := pal.text.Alpha(0.045)
		if box.Hovered() {
			bg = pal.text.Alpha(0.08)
		}
		box.Background(box.AnimateColor("bg", bg, quick)).Children(func() {
			tint := pal.faint
			if ref != "" {
				tint = pal.accent
			}
			ui.Box(c).Size(26, 26).Radius(7).Center().Shrink(0).Background(tint.Alpha(0.14)).Children(func() {
				ui.Icon(c, iconLayers).Size(14, 14).TextColor(tint)
			})
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, T("Reference", "参考库")).FontSize(10.5).TextColor(pal.muted).SingleLine()
				if ref != "" {
					ui.Text(c, filepath.Base(ref)).FontSize(12.5).FontWeight(500).SingleLine().Ellipsis("…")
				} else {
					ui.Text(c, T("Choose…", "点击选择…")).FontSize(12.5).FontWeight(500).TextColor(pal.muted).SingleLine()
				}
			})
		})
		if ref != "" {
			box.Tooltip(ref)
		} else {
			box.Tooltip(T("Choose a reference folder built from healthy controls", "选择由健康对照建立的参考库文件夹"))
		}
		if box.Clicked() {
			a.chooseReference()
		}
		// The settings, with the version, or the new one there is.
		title := T("Settings", "设置")
		gear := ui.ButtonBase(c.Key("settings")).Height(32).Padding(0, 9, 0, 10).Gap(10).Radius(8).AlignItems(ui.Center).Label(title)
		if gear.Hovered() {
			gear.Background(pal.rowHover)
		}
		newer := a.updates && availableVersion() != ""
		gear.Children(func() {
			ui.Icon(c, iconGear).Size(16, 16).TextColor(pal.muted).Shrink(0)
			ui.Text(c, title).FontSize(13).TextColor(pal.text.Alpha(0.86)).SingleLine().Grow(1).MinWidth(0)
			if newer {
				pill(c, T("Update", "有新版本"), pal.accent)
			} else {
				ui.Text(c, appVersion()).FontSize(11).TextColor(pal.faint).SingleLine()
			}
		})
		if gear.Clicked() {
			section := ""
			if newer {
				section = "general"
			}
			a.openSettings(section)
		}
	})
}

// chooseReference asks for a reference folder and uses it everywhere.
func (a *app) chooseReference() {
	a.pick(pickOptions{Title: T("Choose a reference folder", "选择参考库文件夹"), Directory: true}, func(paths []string) {
		if _, err := os.Stat(filepath.Join(paths[0], "reference.json")); err != nil {
			a.alert(T("Not a reference folder", "不是参考库文件夹"),
				T("The folder has no reference.json. Build a reference from healthy controls on the Reference page.",
					"该文件夹中没有 reference.json。请先在“参考库”页面用健康对照建立参考库。"))
			return
		}
		a.setReference(paths[0])
	})
}

// update changes the state from another goroutine and draws the window anew.
func (a *app) update(fn func()) {
	if a.post != nil {
		a.post(fn)
		return
	}
	a.win.Update(func() {
		fn()
		if a.settingsWin != nil {
			a.settingsWin.Invalidate()
		}
	})
}

type pickOptions struct {
	Title     string
	Directory bool
	Multiple  bool
	Filters   []mygo.FileFilter
}

var (
	niftiFilter = []mygo.FileFilter{{Name: "NIfTI", Extensions: []string{"nii", "gz", "img", "hdr"}}}
	tableFilter = []mygo.FileFilter{{Name: "TSV / CSV", Extensions: []string{"tsv", "csv", "txt"}}}
)

// pick shows the system's file dialog and gives what was chosen to done, on
// the main thread.
func (a *app) pick(opt pickOptions, done func(paths []string)) {
	parent := a.dialogs
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: parent, Title: opt.Title, Directory: opt.Directory, Multiple: opt.Multiple,
			Filters: opt.Filters, CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.update(func() { done(paths) })
	}()
}

func (a *app) alert(title, detail string) {
	go mygo.Dialog.Message(mygo.MessageOptions{Parent: a.dialogs, Type: mygo.MessageWarning, Message: title, Detail: detail,
		Buttons: []string{T("OK", "好")}})
}

// start runs work in the background.  work reports its progress through
// step and returns what to do on the main thread once it is done.
func (a *app) start(page, label string, work func(ctx context.Context, step func(done, total int)) (finish func(), err error), failed func(err error)) {
	if a.job != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{page: page, label: label, cancel: cancel, started: time.Now()}
	a.job = j
	step := func(done, total int) {
		a.update(func() { j.done, j.total = done, total })
	}
	go func() {
		var finish func()
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = errorString(T("unexpected error: ", "意外错误：") + strings.TrimSpace(toString(r)))
				}
			}()
			finish, err = work(ctx, step)
		}()
		a.update(func() {
			a.job = nil
			cancelled := ctx.Err() != nil
			cancel()
			// However the run ended, it may have written over the files the
			// page's picture was drawn from.
			if pic := a.pictureOf(page); pic != nil {
				pic.forget()
			}
			switch {
			case cancelled:
				failed(errorString(T("Cancelled.", "已取消。")))
			case err != nil:
				failed(err)
			default:
				finish()
				a.reveal = page
			}
		})
	}()
}

type errorString string

func (e errorString) Error() string { return string(e) }

func main() {
	a := newApp()
	if len(os.Args) > 1 && os.Args[1] == "--snapshot" {
		snapshots(os.Args[2:])
		return
	}
	a.useUpdater()
	mygo.App.WhenReady(func() {
		chinese = strings.HasPrefix(strings.ToLower(mygo.App.Locale()), "zh")
		for _, tag := range mygo.App.PreferredLanguages() {
			chinese = strings.HasPrefix(strings.ToLower(tag), "zh")
			break
		}
		a.loadSettings()
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "SUGAR-PET",
			Width:         1060,
			Height:        740,
			MinWidth:      880,
			MinHeight:     560,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHiddenInset,
			Vibrancy:      mygo.VibrancySidebar,
			Content:       ui.View(a.view),
		})
		a.applyLanguage()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
