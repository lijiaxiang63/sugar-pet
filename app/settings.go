package main

import (
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

// The settings window: the language, the app's version and its updates, and
// the options of each analysis that the pages leave out.

const (
	settingsWidth   = 820
	settingsHeight  = 620
	settingsSidebar = 212
)

type settingsSection struct {
	id    string
	title label
	about label
	icon  *ui.SVG
}

var settingsSections = []settingsSection{
	{"general", label{"General", "通用"}, label{"Language, version and updates", "语言、版本与更新"}, iconGear},
	{"spatial", label{"Spatial Normalisation", "空间标准化"}, label{"SPM, MATLAB and the template of the warp", "SPM、MATLAB 与配准所用的模板"}, iconWarp},
	{"normalize", label{"Intensity Normalisation", "强度归一化"}, label{"How factors are estimated and what is written", "因子如何估计，输出哪些文件"}, iconSliders},
	{"reference", label{"Reference", "参考库"}, label{"How a new reference is built", "如何建立新的参考库"}, iconLayers},
	{"classify", label{"Classify Parkinsonism", "帕金森综合征分类"}, label{"The normalisation of the maps and the language of the report", "制图所用的归一化方法与报告语言"}, iconReport},
	{"compare", label{"Compare Groups", "组间比较"}, label{"The correction and the voxels of the t-test", "t 检验的校正方式与检验范围"}, iconCompare},
	{"cluster", label{"Reference Cluster", "参考簇"}, label{"The threshold of the reference cluster", "参考簇的阈值"}, iconTarget},
}

// shownSections are the sections of the settings that have something to
// show.  The options of the analyses are those of the project that is open:
// while none is, only what is of this computer is shown.
func (a *app) shownSections() []settingsSection {
	if a.proj != nil {
		return settingsSections
	}
	return settingsSections[:2]
}

// openSettings shows the settings window, at a section when one is named,
// opening it the first time.
func (a *app) openSettings(section string) {
	for _, s := range a.shownSections() {
		if s.id == section {
			a.settingsPage = section
		}
	}
	if a.win == nil {
		return // no app to show a window in, as in tests
	}
	if a.settingsWin != nil {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		a.settingsWin.Invalidate()
		return
	}
	a.settingsWin = mygo.NewWindow(mygo.WindowOptions{
		Title:           T("Settings", "设置"),
		Width:           settingsWidth,
		Height:          settingsHeight,
		MinWidth:        700,
		MinHeight:       440,
		StateKey:        "settings",
		TitleBarStyle:   mygo.TitleBarHiddenInset,
		Vibrancy:        mygo.VibrancySidebar,
		DisableMaximize: true,
		AutoHideMenuBar: true,
		Content:         ui.View(a.settingsView),
	})
	a.settingsWin.OnClosed(func() {
		a.settingsWin = nil
		a.tellNormSettings()
	})
	// Changed settings are told of once the user leaves the window: not
	// in the middle of typing a number.
	a.settingsWin.OnBlur(a.tellNormSettings)
}

// redraw draws the windows anew, after a change that was not made in them.
func (a *app) redraw() {
	if a.win != nil {
		a.win.Invalidate()
	}
	if a.settingsWin != nil {
		a.settingsWin.Invalidate()
	}
}

// setLanguage shows the app in another language from now on.
func (a *app) setLanguage(zh bool) {
	chinese = zh
	a.applyLanguage()
}

// applyLanguage tells what is not drawn by the views the language of the
// app: the update window, the menu and the titles of the windows.
func (a *app) applyLanguage() {
	if a.win == nil {
		return
	}
	tag := "en"
	if chinese {
		tag = "zh-Hans"
	}
	mygo.App.SetLocale(tag)
	a.setMenu()
	if a.settingsWin != nil {
		a.settingsWin.SetTitle(T("Settings", "设置"))
	}
}

// setMenu gives macOS, where the menu bar holds them, the items that open
// the settings and check for updates.  Windows and Linux show no menu bar:
// the sidebar and Ctrl+, open the settings there.
func (a *app) setMenu() {
	if runtime.GOOS != "darwin" {
		return
	}
	name := mygo.App.Name()
	appMenu := []*mygo.MenuItem{{Role: mygo.RoleAbout, Label: Tf("About %s", "关于 %s", name)}}
	if a.updates {
		appMenu = append(appMenu, &mygo.MenuItem{Label: T("Check for Updates…", "检查更新…"),
			Click: func(*mygo.MenuItem, *mygo.Window) { checkForUpdates() }})
	}
	appMenu = append(appMenu,
		mygo.Separator(),
		&mygo.MenuItem{Label: T("Settings…", "设置…"), Accelerator: "CmdOrCtrl+,",
			Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings("") }},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleServices, Label: T("Services", "服务")},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleHide, Label: Tf("Hide %s", "隐藏 %s", name)},
		&mygo.MenuItem{Role: mygo.RoleHideOthers, Label: T("Hide Others", "隐藏其他")},
		&mygo.MenuItem{Role: mygo.RoleUnhide, Label: T("Show All", "全部显示")},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleQuit, Label: Tf("Quit %s", "退出 %s", name)},
	)
	// The project: made, opened and left.  Asked in the main window.
	inMain := func(do func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) {
			a.win.Show()
			a.win.Focus()
			a.win.Update(func() {
				a.dialogs = a.win
				do()
			})
		}
	}
	fileMenu := []*mygo.MenuItem{
		{Label: T("New Project…", "新建项目…"), Accelerator: "CmdOrCtrl+N", Click: inMain(func() {
			if a.job == nil {
				a.closeProject()
				a.welcome.creating, a.welcome.err = true, ""
			}
		})},
		{Label: T("Open Project…", "打开项目…"), Accelerator: "CmdOrCtrl+O", Click: inMain(a.pickProject)},
		mygo.Separator(),
		{Label: T("Close Project", "关闭项目"), Accelerator: "CmdOrCtrl+Shift+W", Click: inMain(a.closeProject)},
	}
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Label: name, Submenu: appMenu},
		{Label: T("File", "文件"), Submenu: fileMenu},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu},
		{Role: mygo.RoleWindowMenu},
	}))
}

func (a *app) settingsView(c *ui.Context) {
	pal := paletteFor(c)
	c.SetTheme(themeFor(c, pal))
	a.dialogs = a.settingsWin
	wash := pal.wash
	if c.Vibrancy() {
		wash = wash.Alpha(0.86)
	}
	c.Root().Background(wash)
	if a.settingsWin != nil {
		c.OnShortcut(ui.Cmd, ui.KeyW, func() { a.settingsWin.Close() })
	}
	sections := a.shownSections()
	current := sections[0]
	for _, s := range sections {
		if s.id == a.settingsPage {
			current = s
		}
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(settingsSidebar).Shrink(0).Children(func() {
			bar := c.TitleBar()
			ui.Box(c).Height(max(bar.Bottom+10, titleBarHeight)).Shrink(0).DragWindow()
			page := current.id
			ui.Scroll(c.Key("settings-nav")).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Padding(0, 10, 8, 10).Gap(2).Children(func() {
					for i, s := range sections {
						if i == 1 {
							// The options below are kept in the project.
							heading := T("This computer", "本机")
							if a.proj != nil {
								heading = T("Analyses of this project", "本项目的分析选项")
							}
							ui.Text(c, heading).FontSize(11).FontWeight(600).TextColor(pal.faint).LetterSpacing(0.3).
								Padding(12, 10, 5, 10).SingleLine().Ellipsis("…")
						}
						a.settingsNavItem(c, &page, s)
					}
				})
			})
			a.settingsPage = page
			ui.Text(c, "SUGAR-PET "+appVersion()).FontSize(11).TextColor(pal.faint).Padding(8, 20, 14, 20).SingleLine().Shrink(0)
		})
		ui.Column(c).Grow(1).MinWidth(0).Padding(cardGap, cardGap, cardGap, 0).Children(func() {
			pane := ui.Column(c).Grow(1).MinHeight(0).Radius(12).Background(pal.card).Clip()
			raised(pane, pal).Children(func() {
				current = sections[0]
				for _, s := range sections {
					if s.id == a.settingsPage {
						current = s
					}
				}
				bar := c.TitleBar()
				ui.Column(c).Shrink(0).Padding(22, max(pagePad, bar.Right), 14, pagePad).Gap(3).DragWindow().Children(func() {
					ui.Text(c, current.title.String()).FontSize(21).FontWeight(700).LetterSpacing(-0.3).SingleLine()
					ui.Text(c, current.about.String()).FontSize(12.5).TextColor(pal.muted).SingleLine().Ellipsis("…")
				})
				ui.Scroll(c.Key("settings-" + current.id)).Grow(1).Children(func() {
					ui.Column(c.Key(current.id)).Padding(10, pagePad, 30, pagePad).Gap(24).Children(func() {
						switch current.id {
						case "general":
							a.settingsGeneral(c)
						case "spatial":
							a.settingsSpatial(c)
						case "normalize":
							a.settingsNormalize(c)
						case "reference":
							a.settingsReference(c)
						case "classify":
							a.settingsClassify(c)
						case "compare":
							a.settingsCompare(c)
						case "cluster":
							a.settingsCluster(c)
						}
					})
				})
			})
		})
	})
	// What changed here shows in the main window, and is kept.
	a.saveProject()
	if a.saveSettings() && a.win != nil {
		a.win.Invalidate()
	}
}

// tellNormSettings says, once for a run of step 1, that the settings it ran
// with were changed: the steps after it wait until it is run again.  It is
// called when the settings window is left or closed.
func (a *app) tellNormSettings() {
	d := a.norm.done
	if d == nil || d.settings == a.opt.normSettings(d.methods) {
		a.normTold = nil // changed back, or run anew: a later change is told again
		return
	}
	if a.normTold == d {
		return
	}
	a.normTold = d
	if a.win != nil {
		a.dialogs = a.win // the steps are in the main window
		a.alert(T("Run Intensity Normalisation again", "需要重新运行强度归一化"),
			T("These settings were changed after step 1 ran. Z-score Maps and Classify Parkinsonism use what step 1 wrote, so they cannot go on until Intensity Normalisation is run again with the new settings.",
				"这些设置在第 1 步运行之后被改动了。Z 值图和帕金森综合征分类沿用第 1 步写出的结果，所以在按新设置重新运行强度归一化之前，它们无法继续。"))
	}
}

// settingsNavItem is a section of the settings in their sidebar.
func (a *app) settingsNavItem(c *ui.Context, page *string, s settingsSection) {
	pal := paletteFor(c)
	title := s.title.String()
	item := ui.RadioBase(c.Key("set-"+s.id), page, s.id).Height(32).Padding(0, 9, 0, 10).Gap(10).Radius(8).AlignItems(ui.Center).Label(title)
	chosen := *page == s.id
	bg, fg, tint := ui.Transparent, pal.text.Alpha(0.86), pal.muted
	switch {
	case chosen:
		bg, fg, tint = pal.rowSelected, pal.text, pal.accent
		face(item, pal)
	case item.Hovered():
		bg = pal.rowHover
	}
	item.Background(item.AnimateColor("bg", bg, quick)).Children(func() {
		ui.Icon(c, s.icon).Size(16, 16).TextColor(tint).Shrink(0)
		ui.Text(c, title).FontSize(13).TextColor(fg).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
		if groups := sectionGroups[s.id]; len(groups) > 0 && len(a.opt.changed(groups...)) > 0 {
			ui.Box(c).Size(6, 6).Radius(3).Background(pal.accent).Shrink(0)
		}
	})
}

// sectionGroups are the groups of options each section of the settings
// holds.
var sectionGroups = map[string][]string{
	"spatial":   {groupSpatial},
	"normalize": {groupNormalize, groupIterative},
	"reference": {groupReference},
	"classify":  {groupClassify},
	"compare":   {groupCompare},
	"cluster":   {groupCluster},
}

// restoreDefaults is the end of a section: what in it is not as the app
// first has it, and a button that puts it back.
func (a *app) restoreDefaults(c *ui.Context, section string) {
	pal := paletteFor(c)
	groups := sectionGroups[section]
	changed := a.opt.changed(groups...)
	ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(0, 2).Children(func() {
		text := T("Everything here is as the app first has it.", "本页设置均为默认值。")
		if len(changed) > 0 {
			text = T("Changed: ", "已修改：") + strings.Join(changed, T(", ", "、"))
		}
		ui.Text(c, text).FontSize(12).TextColor(pal.muted).LineHeight(1.35).Grow(1).MinWidth(0)
		if button(c.Key("restore"), T("Restore Defaults", "恢复默认设置"), nil, buttonPlain, 26).Disabled(len(changed) == 0).Clicked() {
			a.opt.restore(groups...)
		}
	})
}

// selfUpdating is whether this copy of the app can replace itself with a
// new version.  It tries to write where the app is, so it is asked once.
var selfUpdating = sync.OnceValue(func() bool { return canUpdate() })

func (a *app) settingsGeneral(c *ui.Context) {
	pal := paletteFor(c)
	section(c, T("Language", "语言"), "", func() {
		rows(c, func() {
			setting(c, T("Language of the app", "界面语言"), T("Reports can be written in another one: see Classify Parkinsonism.", "报告可使用其他语言，见“帕金森综合征分类”。"), func() {
				lang := 0
				if chinese {
					lang = 1
				}
				if segmented(c.Key("lang"), &lang, "English", "中文") {
					a.setLanguage(lang == 1)
				}
			})
		})
	})
	section(c, T("Software update", "软件更新"), "", func() {
		var list []func()
		list = append(list, func() {
			desc := T("New versions are published on GitHub.", "新版本发布在 GitHub 上。")
			if a.updates {
				if t := lastUpdateCheck(); !t.IsZero() {
					desc = T("Last checked: ", "上次检查：") + ago(t, time.Now())
				}
			}
			row := ui.Row(c).Padding(12, 14).Gap(14).AlignItems(ui.Center)
			row.Children(func() {
				ui.Image(c, appLogo).Size(40, 40).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, Tf("SUGAR-PET %s", "SUGAR-PET %s", appVersion())).FontSize(14.5).FontWeight(650).SingleLine().Selectable()
						if a.updates {
							if v := availableVersion(); v != "" {
								pill(c, Tf("%s is available", "可更新至 %s", v), pal.accent)
							}
						}
					})
					ui.Text(c, desc).FontSize(12).TextColor(pal.muted).LineHeight(1.35)
				})
				if a.updates {
					if button(c.Key("check-updates"), T("Check for Updates…", "检查更新…"), iconRefresh, buttonPrimary, 28).Clicked() {
						checkForUpdates()
					}
				} else if button(c.Key("releases"), T("Releases Page", "发布页面"), iconReveal, buttonPlain, 28).Clicked() {
					mygo.Shell.OpenExternal(releasesURL)
				}
			})
		})
		if a.updates && selfUpdating() {
			list = append(list,
				func() {
					setting(c, T("Check for updates automatically", "自动检查更新"), T("Soon after the app opens, then once a day.", "应用启动后不久检查一次，此后每天一次。"), func() {
						on := updateChecks()
						if toggle(c.Key("update-checks"), &on, T("Check for updates automatically", "自动检查更新")).Changed() {
							setUpdateChecks(on)
						}
					})
				},
				func() {
					setting(c, T("Download and install automatically", "自动下载并安装"), T("New versions are downloaded in the background and take effect the next time the app opens.", "在后台下载新版本，下次打开应用时生效。"), func() {
						on := updateDownloads()
						if toggle(c.Key("update-downloads"), &on, T("Download and install automatically", "自动下载并安装")).Disabled(!updateChecks()).Changed() {
							setUpdateDownloads(on)
						}
					})
				})
		} else if a.updates {
			list = append(list, func() {
				setting(c, T("This copy does not update itself", "此副本无法自行更新"),
					T("It was installed by a package manager, sits where it cannot be replaced, or is a development build. New versions are on the releases page.",
						"它由包管理器安装、所在位置不可写，或为开发版本。新版本请到发布页面下载。"), func() {
						if smallButton(c, T("Releases Page", "发布页面")).Clicked() {
							mygo.Shell.OpenExternal(releasesURL)
						}
					})
			})
		}
		rows(c, list...)
	})
	section(c, T("About", "关于"), "", func() {
		rows(c,
			func() {
				setting(c, T("User guide", "用户指南"), T("Every page of the app, step by step, with the methods behind it.", "逐步讲解应用的每个页面及其方法。"), func() {
					if smallButton(c.Key("guide"), T("Open", "打开")).Clicked() {
						mygo.Shell.OpenExternal(guideURL)
					}
				})
			},
			func() {
				setting(c, T("Source code", "源代码"), T("MIT licence. Questions and bug reports are welcome on GitHub.", "MIT 许可证。欢迎在 GitHub 上提问或报告问题。"), func() {
					if smallButton(c.Key("source"), "GitHub").Clicked() {
						mygo.Shell.OpenExternal(sourceURL)
					}
				})
			},
		)
	})
}

// spmRows are the settings of SPM and MATLAB.
func (a *app) spmRows(c *ui.Context) []func() {
	p := &a.spatial
	return []func(){
		func() {
			desc := T("The folder of SPM12 or SPM25, the one holding spm.m.", "SPM12 或 SPM25 的文件夹，即 spm.m 所在的文件夹。")
			if p.spmBad {
				desc = T("This folder is not SPM12 or SPM25: it has no spm_jobman.m or no toolbox/OldNorm.", "该文件夹不是 SPM12 或 SPM25：其中没有 spm_jobman.m 或 toolbox/OldNorm。")
			}
			a.pathSetting(c, "sp-spm", T("SPM folder", "SPM 文件夹"), desc, &p.spm, pathFolder, nil, func() { p.setSPM(p.spm) })
		},
		func() {
			desc := T("Not found on this computer: choose the MATLAB program.", "未在本机找到：请选择 MATLAB 程序。")
			if p.matlab != "" {
				desc = ""
			} else if found := foundMatlab(); found != "" {
				desc = T("Found: ", "已找到：") + shortDir(found)
			}
			a.pathSetting(c, "sp-matlab", "MATLAB", desc, &p.matlab, pathFile, nil, func() {
				if _, err := core.ResolveMatlab(p.matlab); p.matlab != "" && err != nil {
					p.matlab = ""
					a.alert(T("Not MATLAB", "不是 MATLAB"), T("Choose the MATLAB program: the MATLAB app on macOS, bin/matlab elsewhere.",
						"请选择 MATLAB 程序：macOS 上为 MATLAB 应用，其他系统为 bin/matlab。"))
				}
			})
		},
	}
}

func (a *app) settingsSpatial(c *ui.Context) {
	o := &a.opt
	section(c, T("SPM and MATLAB", "SPM 与 MATLAB"), T("The warp is SPM's “Old Normalise” onto a PET template, run by MATLAB in the background. No other page needs them.",
		"配准使用 SPM 的 “Old Normalise” 和 PET 模板，由 MATLAB 在后台运行。其他页面都不需要它们。"), func() {
		rows(c, a.spmRows(c)...)
	})
	if a.proj == nil { // the rest is of a project
		return
	}
	section(c, T("Warp", "配准"), "", func() {
		rows(c,
			func() {
				a.pathSetting(c, "sp-template", lblTemplate.String(),
					T("In MNI space. SPM's own PET template unless you choose one; the mean of your own warped controls usually fits better.",
						"须在 MNI 空间。不选则用 SPM 自带的 PET 模板；用自己已配准对照的均值图通常配得更好。"),
					&o.Template, pathFile, niftiFilter, nil)
			},
			func() {
				setting(c, lblJobs.String(), T("Images are shared out among them; each is a MATLAB of its own.", "图像分给各进程同时处理；每个进程都是一个独立的 MATLAB。"),
					func() { number(c.Key("sp-jobs"), &o.Jobs, 1, float64(max(runtime.NumCPU(), 1)), 1) })
			},
		)
	})
	a.restoreDefaults(c, "spatial")
}

func (a *app) settingsNormalize(c *ui.Context) {
	o := &a.opt
	// Step 1 of the individual analysis ran with other settings than these:
	// the steps after it wait until it is run again.
	if d := a.norm.done; d != nil && d.settings != o.normSettings(d.methods) {
		caution(c, T("These are no longer the settings Intensity Normalisation last ran with. Run it again: until then Z-score Maps and Classify Parkinsonism cannot go on.",
			"这些设置已与上次运行强度归一化时不同。请重新运行强度归一化：在此之前，Z 值图和帕金森综合征分类无法继续。"))
	}
	section(c, T("Factors", "因子"), T("The factors of the controls are estimated the same way when a reference is built: step 1 and the reference must agree for Z-score maps.",
		"建立参考库时，对照的因子也按这里的方式估计：要做 Z 值图，第 1 步与参考库须保持一致。"), func() {
		rows(c,
			func() {
				setting(c, lblStatistic.String(), T("For the reference regions and PS.", "用于参考区法和 PS。"), func() {
					segmented(c.Key("statistic"), &o.Statistic, T("Mean", "均值"), T("Median", "中位数"))
				})
			},
			func() {
				setting(c, lblTarget.String(), T("The reference takes this value in the output, e.g. 50 for “global mean = 50”.", "输出图像中参考值等于该数，例如 50 即“全局均值 = 50”。"),
					func() { number(c.Key("target"), &o.Target, 0.001, 100000, 1) })
			},
			func() {
				setting(c, lblSmoothing.String(),
					T("Automatic: that of the reference for HN, iPS and iHN, none otherwise. The output is never smoothed.", "自动：HN、iPS、iHN 采用参考库的平滑，其他方法不平滑。输出图像不会被平滑。"),
					func() {
						check(c.Key("fwhm-auto"), &o.FWHMAuto, T("Automatic", "自动"))
						number(c.Key("fwhm"), &o.FWHM, 0, 30, 1).Disabled(o.FWHMAuto).Tooltip("FWHM, mm")
					})
			},
		)
	})
	section(c, T("Output", "输出"), "", func() {
		rows(c,
			func() {
				setting(c, lblSaveMasks.String(), T("Also save the voxels each factor came from.", "同时保存估计每个因子所用的体素。"),
					func() { toggle(c.Key("masks"), &o.SaveMasks, lblSaveMasks.String()) })
			},
		)
	})
	section(c, T("Iterative methods", "迭代方法"), T("For iPS and iHN.", "用于 iPS 与 iHN。"), func() {
		rows(c,
			func() {
				setting(c, lblIterP.String(),
					T("Voxels that differ from the controls at this uncorrected p are left out.", "与对照相比达到该未校正 p 值的体素被剔除。"),
					func() { number(c.Key("iter-p"), &o.PThreshold, 0.0001, 0.5, 0.001) })
			},
			func() {
				setting(c, lblIterSide.String(), "", func() {
					segmented(c.Key("iter-excl"), &o.Exclude, T("Hypometabolic only", "仅低代谢"), T("Hypo- and hypermetabolic", "低代谢和高代谢"))
				})
			},
			func() {
				setting(c, lblIterRounds.String(), T("The paper uses 1.", "文章中为 1 次。"),
					func() { number(c.Key("iter-rounds"), &o.MaxIter, 0, 20, 1) })
			},
			func() {
				setting(c, lblLiteral.String(), T("As written in the paper, instead of the mean relative to the control template.", "按文章字面实现，而非相对对照模板的均值。"),
					func() { toggle(c.Key("literal"), &o.LiteralIPS, lblLiteral.String()) })
			},
		)
	})
	a.restoreDefaults(c, "normalize")
}

// maskChoices are the built-in masks and, last, a file of the user's.
func maskChoices() (ids, names []string) {
	for _, m := range core.Masks {
		ids, names = append(ids, m.Name), append(names, maskName(m.Name))
	}
	return append(ids, "file"), append(names, T("Mask file…", "掩膜文件…"))
}

func (a *app) settingsReference(c *ui.Context) {
	o := &a.opt
	section(c, T("Building a reference", "建立参考库"), T("These apply to the next reference built on the Reference page; one already built keeps its own.", "对之后在“参考库”页面新建的参考库生效；已建好的参考库不受影响。"), func() {
		ids, names := maskChoices()
		var maskRow func()
		if o.RefMask == "file" {
			maskRow = func() {
				a.pathSetting(c, "ref-maskfile", T("Mask file", "掩膜文件"), T("A NIfTI image in MNI space.", "MNI 空间的 NIfTI 图像。"),
					&o.RefMaskFile, pathFile, niftiFilter, nil)
			}
		}
		rows(c,
			func() {
				setting(c, lblRefMask.String(), T("The voxels the factors are estimated from. The paper uses gray matter.", "用于估计因子的体素。文章使用灰质。"),
					func() { selectOf(c.Key("ref-mask"), &o.RefMask, ids, names).Width(190) })
			},
			maskRow,
			func() {
				setting(c, lblRefFWHM.String(), T("Applied before the statistics are computed. Use 0 if the images are already smoothed.", "计算统计量之前施加。图像已平滑时填 0。"),
					func() { number(c.Key("ref-fwhm"), &o.RefFWHM, 0, 30, 1) })
			},
		)
	})
	section(c, lblRefStats.String(),
		T("PS, iPS, HN and iHN never need this. Tick the methods you also want z-score maps for; White matter adds a check to the classification that does not rest on gray matter.",
			"PS、iPS、HN、iHN 无需勾选。若还要用以下方法制作 Z 值图，请勾选；勾选“深部白质”可为分类增加一种不依赖灰质的核对方式。"), func() {
			card(c).Padding(8, 11).Children(func() {
				ui.Row(c).Wrap().Children(func() {
					for _, m := range core.StatsMethods() {
						on := o.RefStatsFor[m]
						tile(c, m, 2, func() { checkTile(c, &on, methodName(m), nil).Tooltip(methodAbout(m)) })
						if on {
							o.RefStatsFor[m] = true
						} else {
							delete(o.RefStatsFor, m)
						}
					}
				})
			})
		})
	a.restoreDefaults(c, "reference")
}

func (a *app) settingsClassify(c *ui.Context) {
	o := &a.opt
	// The method is not chosen here: the classification rests on the maps
	// of the step before it.
	section(c, T("Report", "报告"), "", func() {
		rows(c, func() {
			setting(c, lblClsLang.String(), "", func() {
				segmented(c.Key("cls-lang"), &o.ReportLanguage, T("As the app", "与界面一致"), "English", "中文")
			})
		})
	})
	a.restoreDefaults(c, "classify")
}

func (a *app) settingsCompare(c *ui.Context) {
	o := &a.opt
	p := &a.cmp
	section(c, T("Statistics", "统计"), "", func() {
		list := []func(){func() {
			setting(c, lblCorrection.String(), [...]string{
				T("Cluster-level, by Gaussian random field theory; two-tailed.", "团块水平，基于高斯随机场理论；双侧。"),
				T("Voxel-level false discovery rate; two-tailed.", "体素水平的错误发现率；双侧。"),
				T("Only the unthresholded T-map is written.", "仅输出未阈值化的 T 图。"),
			}[o.Correction], func() { segmented(c.Key("correction"), &o.Correction, "GRF", "FDR", T("None", "不校正")) })
		}}
		switch o.Correction {
		case 0:
			list = append(list,
				func() {
					setting(c, lblVoxelP.String(), T("Two-tailed threshold that forms the clusters.", "形成团块的双侧阈值。"),
						func() { number(c.Key("voxel-p"), &o.VoxelP, 0.00001, 0.1, 0.001) })
				},
				func() {
					setting(c, lblClusterP.String(), "", func() { number(c.Key("cluster-p"), &o.ClusterP, 0.0001, 0.5, 0.01) })
				})
		case 1:
			list = append(list, func() { setting(c, lblQ.String(), "", func() { number(c.Key("q"), &o.Q, 0.0001, 0.5, 0.01) }) })
		}
		rows(c, list...)
	})
	section(c, T("Images", "图像"), "", func() {
		ids, names := maskChoices()
		var fileRow func()
		if o.Analysis == "file" {
			fileRow = func() {
				a.pathSetting(c, "cmp-mask", T("Mask file", "掩膜文件"), "", &o.AnalysisFile, pathFile, niftiFilter, nil)
			}
		}
		rows(c,
			func() {
				setting(c, lblAnalysis.String(), T("Voxels that any image does not cover are left out.", "任何一幅图像未覆盖的体素会被排除。"),
					func() { selectOf(c.Key("analysis"), &o.Analysis, ids, names).Width(190) })
			},
			fileRow,
			func() {
				about := T("Automatic: 8 mm, as no reference is chosen. Use 0 if the images are already smoothed.", "自动：未选择参考库，采用 8 mm。图像已平滑时填 0。")
				if fwhm, ok := p.referenceFWHM(); ok {
					about = Tf("Automatic: %g mm, that of the reference. Use 0 if the images are already smoothed.", "自动：采用参考库的平滑 %g mm。图像已平滑时填 0。", fwhm)
				}
				setting(c, lblCmpFWHM.String(), about,
					func() {
						check(c.Key("cmp-fwhm-auto"), &o.CmpFWHMAuto, T("Automatic", "自动"))
						number(c.Key("cmp-fwhm"), &o.CmpFWHM, 0, 30, 1).Disabled(o.CmpFWHMAuto).Tooltip("FWHM, mm")
					})
			},
			func() {
				setting(c, lblKeep.String(), T("The intensity-normalised, smoothed images of both groups.", "两组强度归一化并平滑后的图像。"),
					func() { toggle(c.Key("keep"), &o.KeepImages, T("Keep images", "保留图像")) })
			},
		)
	})
	a.restoreDefaults(c, "compare")
}

func (a *app) settingsCluster(c *ui.Context) {
	o := &a.opt
	section(c, T("Threshold", "阈值"), T("The t-test of the patient group against the controls that picks the cluster out.", "患者组与对照的 t 检验，用于找出参考簇。"), func() {
		rows(c,
			func() {
				setting(c, lblClusterThr.String(), T("One-sided, uncorrected.", "单侧，未校正。"), func() { number(c.Key("cl-p"), &o.ClusterThreshold, 0.00001, 0.2, 0.001) })
			},
			func() {
				setting(c, lblMinCluster.String(), T("Smaller connected clusters are dropped.", "更小的连通团块会被舍弃。"),
					func() { number(c.Key("cl-min"), &o.MinCluster, 0, 100, 0.5) })
			},
		)
	})
	a.restoreDefaults(c, "cluster")
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
