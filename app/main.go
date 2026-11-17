// SUGAR-PET is the desktop app of sugar-pet: intensity normalisation of
// brain FDG-PET images, with nothing to install beside it.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	titleBarHeight = 52
	sidebarWidth   = 216
	contentWidth   = 720
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

	norm    normPage
	ref     refPage
	zmap    zmapPage
	cmp     comparePage
	cluster clusterPage
	masks   masksPage

	settingsPath string
}

// settings is what the app remembers between launches.
type settings struct {
	Language  string `json:"language"`
	Page      string `json:"page"`
	Reference string `json:"reference"`
	Output    string `json:"output"`
}

func newApp() *app {
	a := &app{page: "normalize"}
	a.norm.init()
	a.ref.init()
	a.zmap.init()
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
	var s settings
	if json.Unmarshal(raw, &s) != nil {
		return
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
}

func (a *app) saveSettings() {
	if a.settingsPath == "" {
		return
	}
	s := settings{Language: "en", Page: a.page, Reference: a.norm.reference}
	if chinese {
		s.Language = "zh"
	}
	raw, _ := json.MarshalIndent(s, "", "  ")
	os.MkdirAll(filepath.Dir(a.settingsPath), 0o755)
	os.WriteFile(a.settingsPath, raw, 0o644)
}

// setReference makes a reference folder the one every page uses.
func (a *app) setReference(path string) {
	a.norm.reference, a.zmap.reference, a.cmp.reference, a.cluster.reference = path, path, path, path
	a.zmap.loadedFrom = ""
	a.cmp.fwhmKnown = false
}

type pageInfo struct {
	id     string
	en, zh string
	icon   *ui.SVG
}

var pages = []pageInfo{
	{"normalize", "Normalise", "强度归一化", iconSliders},
	{"reference", "Reference", "参考库", iconLayers},
	{"zmap", "Z-score Maps", "Z 值图", iconCurve},
	{"compare", "Compare Groups", "组间比较", iconCompare},
	{"cluster", "Reference Cluster", "参考簇", iconTarget},
	{"masks", "Masks & Atlas", "掩膜与图谱", iconGrid},
}

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	vibrant := c.Vibrancy()
	if vibrant {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		side := ui.Column(c).Width(sidebarWidth).Shrink(0)
		if !vibrant {
			side.Background(t.Surface)
		}
		side.Children(func() { a.sidebar(c, vibrant) })
		ui.Box(c).Width(1).Background(hairline(t))
		pane := ui.Column(c).Grow(1).MinWidth(0).Background(t.Background)
		pane.Children(func() {
			switch a.page {
			case "normalize":
				a.normalizeView(c)
			case "reference":
				a.referenceView(c)
			case "zmap":
				a.zmapView(c)
			case "compare":
				a.compareView(c)
			case "cluster":
				a.clusterView(c)
			case "masks":
				a.masksView(c)
			}
		})
	})
}

func (a *app) sidebar(c *ui.Context, vibrant bool) {
	t := c.Theme()
	bar := c.TitleBar()
	ui.Box(c).Height(max(bar.Bottom+14, titleBarHeight)).Shrink(0).DragWindow()
	ui.Row(c).Padding(0, 16, 10, 16).Gap(9).AlignItems(ui.Center).Shrink(0).DragWindow().Children(func() {
		ui.Image(c, appLogo).Size(30, 30)
		ui.Column(c).Gap(0).Children(func() {
			ui.Text(c, "SUGAR-PET").FontSize(14).Bold().SingleLine()
			ui.Text(c, T("FDG-PET normalisation", "FDG-PET 强度归一化")).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
	})
	page := a.page
	list := ui.Sidebar(c, &page, func() {
		ui.SidebarSection(c, T("Workflow", "工作流程"), nil, func() {
			for _, p := range pages[:5] {
				item := ui.SidebarItem(c, p.id, p.icon, T(p.en, p.zh))
				if a.job != nil && a.job.page == p.id {
					item.Children(func() { ui.Spinner(c).Size(12, 12) })
				}
			}
		})
		ui.SidebarSection(c, T("Resources", "资源"), nil, func() {
			p := pages[5]
			ui.SidebarItem(c, p.id, p.icon, T(p.en, p.zh))
		})
	}).Grow(1).Label(T("Pages", "页面"))
	if vibrant {
		list.Background(ui.Transparent)
	}
	if page != a.page {
		a.page = page
		a.saveSettings()
	}

	// The reference in use, and the language.
	ui.Column(c).Padding(10, 14, 14, 14).Gap(10).Shrink(0).Children(func() {
		ref := a.norm.reference
		box := ui.ButtonBase(c.Key("ref-chip")).Padding(8, 10).Radius(8).Gap(8).AlignItems(ui.Center).
			Background(t.Text.Alpha(0.05)).Label(T("Reference in use", "当前参考库"))
		if box.Hovered() {
			box.Background(t.Text.Alpha(0.09))
		}
		box.Children(func() {
			ui.Icon(c, iconLayers).Size(14, 14).TextColor(t.TextMuted)
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, T("Reference", "参考库")).FontSize(10.5).TextColor(t.TextMuted).SingleLine()
				name := T("None chosen", "尚未选择")
				if ref != "" {
					name = filepath.Base(ref)
				}
				ui.Text(c, name).FontSize(12).SingleLine()
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
		lang := 0
		if chinese {
			lang = 1
		}
		if ui.Segmented(c, &lang, "English", "中文").FillEqually().Label("Language").Changed() {
			chinese = lang == 1
			a.saveSettings()
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
		a.saveSettings()
	})
}

// update changes the state from another goroutine and draws the window anew.
func (a *app) update(fn func()) {
	if a.post != nil {
		a.post(fn)
		return
	}
	a.win.Update(fn)
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
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win, Title: opt.Title, Directory: opt.Directory, Multiple: opt.Multiple,
			Filters: opt.Filters, CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.update(func() { done(paths) })
	}()
}

func (a *app) alert(title, detail string) {
	go mygo.Dialog.Message(mygo.MessageOptions{Parent: a.win, Type: mygo.MessageWarning, Message: title, Detail: detail,
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
			MinWidth:      860,
			MinHeight:     560,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHiddenInset,
			Vibrancy:      mygo.VibrancySidebar,
			Content:       ui.View(a.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
