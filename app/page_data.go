package main

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

// welcomePage is what the window shows while no project is open: a project
// to make, one to open, and those opened lately.
type welcomePage struct {
	creating bool   // the form of a new project is shown
	name     string // of the project to make, and of its folder
	parent   string // the folder that holds the project folder
	copies   int    // 0: images stay where they are, 1: they are copied in
	err      string
}

func (w *welcomePage) init() {}

// target is the folder of the project to make, or "" while it is not named.
func (w *welcomePage) target() string {
	name := strings.TrimSpace(w.name)
	if w.parent == "" || name == "" {
		return ""
	}
	return filepath.Join(w.parent, name)
}

func (w *welcomePage) problem() string {
	name := strings.TrimSpace(w.name)
	switch {
	case name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:`):
		return T(`Give the project a name, without / \ or :.`, `请填写项目名称，不能含 / \ 或 :。`)
	case w.parent == "":
		return T("Choose where to create the project folder.", "请选择项目文件夹建在哪里。")
	case isProject(w.target()):
		return T("A project of this name is there already: open it, or choose another name.", "这里已有同名项目：请直接打开，或换一个名称。")
	}
	return ""
}

// copyChoice is the control that says what becomes of images added from
// outside the project folder.
func copyChoice(c *ui.Context, key string, copies *int) {
	segmented(c.Key(key), copies, T("Keep in Place", "留在原处"), T("Copy into Project", "复制进项目"))
}

func copyAbout() string {
	return T("Kept in place, the project only notes where an image is: nothing is duplicated, but the image must stay there. Copied, the project folder holds everything and can be moved or shared as a whole.",
		"留在原处：项目只记录图像的位置，不占额外空间，但图像不能移走。复制进项目：项目文件夹自成一体，可以整体移动或共享。")
}

func (a *app) welcomeView(c *ui.Context) {
	pal := paletteFor(c)
	bar := c.TitleBar()
	w := &a.welcome
	root := ui.Column(c.Key("welcome")).Fill()
	// A project folder dropped on the window is opened.
	if dropped := root.DroppedFiles(); len(dropped) > 0 {
		a.openFolder(dropped[0])
	}
	root.Children(func() {
		ui.Box(c).Height(max(bar.Bottom, titleBarHeight)).Shrink(0).DragWindow()
		ui.Scroll(c.Key("welcome-scroll")).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(600).Margin(0, ui.Auto).Padding(8, pagePad, 28, pagePad).Gap(22).Children(func() {
				ui.Column(c).Gap(6).AlignItems(ui.Center).Padding(0, 0, 4, 0).Children(func() {
					ui.Image(c, appLogo).Size(68, 68)
					ui.Text(c, "SUGAR-PET").FontSize(24).FontWeight(750).LetterSpacing(0.3).Margin(6, 0, 0, 0).SingleLine()
					ui.Text(c, T("Intensity normalisation and analysis of brain FDG-PET, one project at a time",
						"脑 FDG-PET 的强度归一化与分析，以项目为单位")).FontSize(13).TextColor(pal.muted).TextAlign(ui.Center).LineHeight(1.4)
				})
				pane := ui.Column(c).Radius(14).Background(pal.card).Padding(20, 20, 22, 20).Gap(20)
				raised(pane, pal).Children(func() {
					if w.creating {
						a.newProject(c)
						return
					}
					section(c, T("Start", "开始"), T("A project is a folder: your images, everything the analyses write, and a file that records how far each step has come. Open it again and go on where you stopped.",
						"项目就是一个文件夹：里面有你的图像、各项分析写出的全部结果，以及一个记录每一步进度的项目文件。再次打开即可从上次停下的地方继续。"), func() {
						ui.Row(c).Gap(10).Wrap().GapY(8).Children(func() {
							if button(c.Key("new-project"), T("New Project…", "新建项目…"), iconPlus, buttonPrimary, 36).Clicked() {
								w.creating, w.err = true, ""
							}
							if button(c.Key("open-project"), T("Open Project…", "打开项目…"), iconFolder, buttonPlain, 36).Clicked() {
								a.pickProject()
							}
						})
						if w.err != "" {
							ui.Text(c, w.err).FontSize(12.5).TextColor(pal.danger).LineHeight(1.4).Selectable()
						}
					})
					a.recentProjects(c)
				})
				ui.Row(c).Justify(ui.Center).Gap(14).AlignItems(ui.Center).Children(func() {
					if button(c.Key("welcome-settings"), T("Settings…", "设置…"), iconGear, buttonGhost, 28).Clicked() {
						a.openSettings("general")
					}
					ui.Text(c, "SUGAR-PET "+appVersion()).FontSize(11.5).TextColor(pal.faint).SingleLine()
				})
			})
		})
	})
}

// pickProject asks for the folder of a project and opens it.
func (a *app) pickProject() {
	a.pick(pickOptions{Title: T("Open a project folder", "打开项目文件夹"), Directory: true}, func(paths []string) {
		a.openFolder(paths[0])
	})
}

// newProject is the form that makes a project.
func (a *app) newProject(c *ui.Context) {
	pal := paletteFor(c)
	w := &a.welcome
	section(c, T("New project", "新建项目"), T("A folder of the project's name is created, with a folder each for the patients, the controls and what the analyses write.",
		"将以项目名称新建一个文件夹，其中为患者、对照和各项分析的结果各建一个子文件夹。"), func() {
		where := T("Also the name of the project folder.", "同时也是项目文件夹的名称。")
		if target := w.target(); target != "" {
			where = Tf("The project folder is %s", "项目文件夹为 %s", shortDir(target))
		}
		rows(c,
			func() {
				setting(c, T("Name", "名称"), where, func() {
					ui.TextInput(c.Key("new-name"), &w.name).Placeholder(T("e.g. parkinsonism-2026", "例如 parkinsonism-2026")).Width(230)
				})
			},
			func() {
				a.pathSetting(c, "new-parent", T("Location", "存放位置"), T("The project folder is created in this folder.", "项目文件夹建在这个文件夹里。"),
					&w.parent, pathFolder, nil, nil)
			},
			func() {
				setting(c, T("Images from outside the project", "项目文件夹之外的图像"), copyAbout(), func() { copyChoice(c, "new-copies", &w.copies) })
			},
		)
		if w.err != "" {
			ui.Text(c, w.err).FontSize(12.5).TextColor(pal.danger).LineHeight(1.4).Selectable()
		}
		problem := w.problem()
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, problem).FontSize(12.5).TextColor(pal.muted).LineHeight(1.35).Grow(1).MinWidth(0)
			if button(c.Key("new-cancel"), T("Cancel", "取消"), nil, buttonPlain, 34).Clicked() {
				w.creating, w.err = false, ""
			}
			if button(c.Key("new-create"), T("Create Project", "创建项目"), nil, buttonPrimary, 34).Disabled(problem != "").Clicked() {
				if err := a.createProject(w.target(), w.name, w.copies == 1); err != nil {
					w.err = err.Error()
				} else {
					w.creating, w.name, w.err = false, "", ""
				}
			}
		})
	})
}

// recentProjects lists the projects opened lately.
func (a *app) recentProjects(c *ui.Context) {
	if len(a.recent) == 0 {
		return
	}
	pal := paletteFor(c)
	section(c, T("Recent projects", "最近的项目"), "", func() {
		var list []func()
		for _, dir := range a.recent {
			list = append(list, func() {
				there := a.onDisk(dir, true)
				row := ui.ButtonBase(c.Key("recent-"+dir)).Padding(9, 8, 9, 14).Gap(12).AlignItems(ui.Center).Label(filepath.Base(dir)).Tooltip(dir)
				if row.Hovered() && there {
					row.Background(pal.rowHover)
				}
				if row.Clicked() && there {
					a.openFolder(dir)
				}
				row.Children(func() {
					tint := pal.accent
					if !there {
						tint = pal.faint
					}
					ui.Box(c).Size(30, 30).Radius(8).Center().Shrink(0).Background(tint.Alpha(0.13)).Children(func() {
						ui.Icon(c, iconFolder).Size(15, 15).TextColor(tint)
					})
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, filepath.Base(dir)).FontSize(13.5).FontWeight(550).SingleLine().Ellipsis("…")
						ui.Text(c, shortDir(filepath.Dir(dir))).FontSize(11.5).TextColor(pal.muted).SingleLine().Ellipsis("…")
					})
					switch {
					case !there:
						pill(c, T("Cannot be reached", "无法访问"), pal.muted)
					case !a.onDisk(filepath.Join(dir, projectFileName), false):
						pill(c, T("Study folder", "研究文件夹"), pal.muted).
							Tooltip(T("Laid out by an earlier version: it is made a project when you open it.", "由旧版本建立：打开时会把它建成项目。"))
					}
					del := iconButton(c, iconClose, T("Remove from the list", "从列表中移除")).Tooltip(T("Remove from the list", "从列表中移除"))
					del.OnClick(func() { a.forget(dir) })
				})
			})
		}
		rows(c, list...)
	})
}

// projectChip is the project that is open, at the top of the sidebar.  It
// leads to the Data page, where the project is described and left.
func (a *app) projectChip(c *ui.Context) {
	pal := paletteFor(c)
	p := a.proj
	name := strings.TrimSpace(p.name)
	if name == "" {
		name = filepath.Base(p.dir)
	}
	ui.Column(c).Padding(0, 10, 6, 10).Shrink(0).Children(func() {
		box := ui.ButtonBase(c.Key("project-chip")).Padding(7, 10).Radius(10).Gap(9).AlignItems(ui.Center).Label(T("Project", "项目")).Tooltip(p.dir)
		bg := pal.text.Alpha(0.045)
		if box.Hovered() {
			bg = pal.text.Alpha(0.08)
		}
		box.Background(box.AnimateColor("bg", bg, quick)).InsetShadow(0, 0, 0, 1, pal.border).Children(func() {
			tint := pal.accent
			if p.saveErr != "" {
				tint = pal.warning
			}
			ui.Box(c).Size(24, 24).Radius(6).Center().Shrink(0).Background(tint.Alpha(0.14)).Children(func() {
				ic := iconFolder
				if p.saveErr != "" {
					ic = iconWarn
				}
				ui.Icon(c, ic).Size(13, 13).TextColor(tint)
			})
			ui.Column(c).Gap(0.5).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, T("Project", "项目")).FontSize(10).TextColor(pal.muted).SingleLine()
				ui.Text(c, name).FontSize(12).FontWeight(550).SingleLine().Ellipsis("…")
			})
		})
		if box.Clicked() {
			a.page = "data"
		}
	})
}

// dataProblem is what the project lacks before any analysis: its images,
// and among them the 3 healthy controls it takes at least.  Every patient
// is compared with the controls of the project, and no others: the maps and
// the group comparison rest on the same ones.
func (a *app) dataProblem() string {
	p := a.proj
	if len(p.patients.files)+len(p.controls.files) == 0 {
		return T("Add the images of the project: the patients, and at least 3 healthy controls.", "请添加项目的图像：患者，以及至少 3 例健康对照。")
	}
	// An image is of one group: it is normalised once, and then compared
	// with the other.
	for _, path := range p.controls.files {
		if slices.Contains(p.patients.files, path) {
			return Tf("%s is among the patients and among the controls: remove it from one of the two.", "%s 同时出现在患者和对照中：请从其中一组移除。", filepath.Base(path))
		}
	}
	if n := len(p.controls.files); n < 3 {
		return Tf("Add at least 3 healthy controls: every patient is compared with them. The project has %d.", "请至少添加 3 例健康对照：每位患者都要与它们比较。项目目前有 %d 例。", n)
	}
	return ""
}

// needImages is what a page says while the project has none of the images
// it works on, which its run bar offers to mend (remedy).
func (a *app) needImages(group string) string {
	p := a.proj
	which, more := T("the patients' images", "患者的图像"), len(p.patients.files) > 0
	if group == "controls" {
		which, more = T("the images of the healthy controls", "健康对照的图像"), len(p.controls.files) > 0
	}
	if p.native && more { // they are there, and not warped yet
		return Tf("Warp %s into MNI space first, on the Spatial Normalisation page.", "请先在“空间标准化”页面把%s配准到 MNI 空间。", which)
	}
	return Tf("Add %s on the Data page first.", "请先在“数据”页面添加%s。", which)
}

// dataSummary says in a line what the project holds.
func (a *app) dataSummary() string {
	p := a.proj
	text := Tf("%d patient image(s) and %d control image(s) in the project.", "项目中有 %d 幅患者图像、%d 幅对照图像。", len(p.patients.files), len(p.controls.files))
	if p.native {
		text += T(" They are warped into MNI space next.", "下一步把它们配准到 MNI 空间。")
	}
	return text
}

// spatialDone reports whether every image of a project in native space has
// been warped into MNI space.
func (a *app) spatialDone() bool {
	files := a.toWarp()
	for _, f := range files {
		if a.spatial.warped[f] == "" {
			return false
		}
	}
	return len(files) > 0
}

// unwarped counts the images of a project in native space that Spatial
// Normalisation has not warped, of all it holds: the pages after it leave
// them out.
func (a *app) unwarped() (missing, total int) {
	if a.proj == nil || !a.proj.native {
		return 0, 0
	}
	for _, f := range append(slices.Clone(a.proj.patients.files), a.proj.controls.files...) {
		total++
		if a.spatial.warped[f] == "" {
			missing++
		}
	}
	return missing, total
}

// source is a card of what a page takes from another page of the project:
// the facts, and the way to where they are chosen.
func (a *app) source(c *ui.Context, key, page, label string, facts ...func()) {
	facts = append(facts, func() {
		ui.Row(c).Padding(7, 10).Justify(ui.End).Children(func() {
			if button(c.Key(key), label, nil, buttonGhost, 26).Clicked() {
				a.page = page
			}
		})
	})
	rows(c, facts...)
}

// The labels of the ways back to where a page's input is chosen.
func changeInData() string { return T("Change in Data", "到“数据”页面修改") }

// countText says how many images a group holds and where they lie.
func countText(files []string) string {
	if len(files) == 0 {
		return T("None yet", "尚未添加")
	}
	return imagesText(files, 0)
}

// writesTo names the folder of the project that a page writes into.
func (a *app) writesTo(path string) string {
	if rel, err := filepath.Rel(a.proj.dir, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return shortDir(path)
}

// next is the button, at the end of a page's bar, that leads to the page
// that comes after it in the project.
func (a *app) next(c *ui.Context, page string, ready bool) {
	if !ready || a.job != nil {
		return
	}
	info := pageInfoOf(page)
	if button(c.Key("next-page"), Tf("Next: %s", "下一步：%s", T(info.en, info.zh)), iconArrowRight, buttonSoft, 34).Clicked() {
		a.page = page
	}
}

func (a *app) dataView(c *ui.Context) {
	p := a.proj
	busy := a.job != nil
	a.frame(c, T("Data", "数据"),
		T("The project and the images it works on", "项目以及它所分析的图像"),
		func() {
			if p.saveErr != "" {
				caution(c, Tf("The project cannot be saved into its folder (%s). What you do now is lost when the app closes, unless the folder can be written again.",
					"项目无法保存到它的文件夹（%s）。除非该文件夹恢复可写，否则现在所做的操作在关闭应用后会丢失。", p.saveErr))
			}
			section(c, T("Project", "项目"), "", func() {
				rows(c,
					func() {
						setting(c, T("Name", "名称"), "", func() { ui.TextInput(c.Key("project-name"), &p.name).Width(230) })
					},
					func() {
						setting(c, T("Project folder", "项目文件夹"), Tf("%s · %s in it records the project and how far each step has come.", "%s · 其中的 %s 记录项目信息与每一步的进度。", shortDir(p.dir), projectFileName), func() {
							revealButton(c, "project-reveal", revealLabel(), p.dir)
							if button(c.Key("project-close"), T("Close Project", "关闭项目"), nil, buttonPlain, 26).Disabled(busy).Clicked() {
								a.leaving = true // once the frame is drawn
							}
						})
					},
				)
			})
			section(c, T("Images", "图像"), T("One FDG-PET scan per subject. Every page of the project takes its images from here: both groups are normalised, the controls are summarised for the patients to be compared with, the patients are mapped and classified, and the two groups are compared.",
				"每位受试者一幅 FDG-PET 图像。项目的每个页面都从这里取图像：两组都做强度归一化，对照汇总为对照统计供患者比较，患者用于制图和分类，两组一起用于组间比较。"), func() {
				rows(c,
					func() {
						setting(c, T("The images are in", "图像所在的空间"), T("Images in native space are warped into MNI space by Spatial Normalisation before anything else reads them; that needs MATLAB and SPM.",
							"原始空间的图像先由“空间标准化”配准到 MNI 空间，其他步骤才会读取；这一步需要 MATLAB 和 SPM。"), func() {
							space := 0
							if p.native {
								space = 1
							}
							if busy {
								pill(c, [...]string{T("MNI space", "MNI 空间"), T("Native space", "原始空间")}[space], paletteFor(c).muted)
								return
							}
							segmented(c.Key("space"), &space, T("MNI Space", "MNI 空间"), T("Native Space", "原始空间"))
							p.native = space == 1
						})
					},
					func() {
						setting(c, T("Images from outside the project", "项目文件夹之外的图像"), copyAbout(), func() {
							copies := 0
							if p.copies {
								copies = 1
							}
							copyChoice(c, "copies", &copies)
							p.copies = copies == 1
						})
					},
				)
				p.patients.adder = func(paths []string) { a.addImages(&p.patients, "patients", paths) }
				p.controls.adder = func(paths []string) { a.addImages(&p.controls, "controls", paths) }
				hint := T(".nii and .nii.gz files, or whole folders", "支持 .nii 与 .nii.gz 文件，也可添加整个文件夹")
				folders := [2]string{"patients", "controls"}
				if p.native { // the folders of the groups hold what is warped, not what is added
					folders = [2]string{}
				}
				ui.Row(c).Gap(14).AlignItems(ui.Start).Children(func() {
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
						a.filesCardFromStudy(c, "data-patients", &p.patients, T("Patients", "患者"), hint, folders[0])
					})
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
						a.filesCardFromStudy(c, "data-controls", &p.controls, T("Healthy controls (at least 3)", "健康对照（至少 3 例）"), hint, folders[1])
					})
				})
				notesCard(c, p.notes)
			})
			section(c, T("Age and sex (covariates)", "年龄和性别（协变量）"),
				T("Optional. With a table, the control statistics model what age and sex do to the uptake of the controls (W-scores), and the group comparison adjusts for them. It must list every image of the project: rows are matched by an “image” or “participant_id” column (sub-01 matches sub-01_…_pet.nii.gz), as in a BIDS participants.tsv.",
					"可选。提供表格后，对照统计会对年龄和性别对对照摄取的影响建模（W 值），组间比较也会校正它们。表格须包含项目的全部图像：通过 image 或 participant_id 列将行与图像对应（sub-01 匹配 sub-01_…_pet.nii.gz），与 BIDS 的 participants.tsv 一致。"), func() {
					rows(c, a.covRows(c, "cov", &a.cov, T("A TSV or CSV file with the covariates of the patients and the controls.", "包含患者和对照协变量的 TSV 或 CSV 文件。"))...)
				})
		},
		func() {
			a.runBar(c, "data", a.dataProblem(), "", &p.st, nil)
			to := "normalize"
			if p.native {
				to = "spatial"
			}
			a.next(c, to, a.dataProblem() == "")
		})
}
