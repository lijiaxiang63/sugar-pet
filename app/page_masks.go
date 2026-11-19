package main

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type masksPage struct {
	query    string
	table    ui.ListState
	selected map[int]bool // CerebrA label numbers picked to send to Intensity Normalisation
	st       status
	outDir   string
}

// maskVolumes are the volumes of the built-in masks in mL.
var maskVolumes = sync.OnceValue(func() map[string]float64 {
	out := map[string]float64{}
	for _, m := range core.Masks {
		vol, err := core.BuiltinMask(m.Name)
		if err != nil {
			continue
		}
		n := 0
		for _, v := range vol.Data {
			if v > 0 {
				n++
			}
		}
		out[m.Name] = float64(n) * core.VoxelVolumeML(vol.Aff)
	}
	return out
})

func (a *app) masksView(c *ui.Context) {
	p := &a.masks
	t := c.Theme()
	volumes := maskVolumes()
	a.frame(c, T("Masks & Atlas", "掩膜与图谱"),
		T("The regions built into the app, in MNI space", "应用内置的 MNI 空间区域"),
		func() {
			section(c, T("Built-in masks", "内置掩膜"), T("On a 2 mm grid in MNI space (ICBM 2009c symmetric), derived from the CerebrA atlas; resampled onto the grid of each image.",
				"位于 MNI 空间（ICBM 2009c 对称模板）的 2 mm 网格上，由 CerebrA 图谱生成；使用时自动重采样到图像的网格。"), func() {
				var list []func()
				for _, m := range core.Masks {
					list = append(list, func() {
						setting(c, maskName(m.Name), maskAbout(m.Name), func() {
							ui.Text(c, m.Name).FontSize(11.5).Font("monospace").TextColor(t.TextMuted).SingleLine()
							ui.Text(c, strconv.FormatFloat(volumes[m.Name], 'f', 0, 64)+" mL").FontSize(12.5).Width(64).TextAlign(ui.End).SingleLine()
						})
					})
				}
				rows(c, list...)
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					b := smallButton(c, T("Export All as NIfTI…", "全部导出为 NIfTI…")).Disabled(a.job != nil)
					if b.Clicked() {
						a.pick(pickOptions{Title: T("Export the built-in masks to", "将内置掩膜导出到"), Directory: true}, a.exportMasks)
					}
					switch {
					case p.st.err != "":
						ui.Text(c, p.st.err).FontSize(12).TextColor(t.Danger)
					case p.st.ok != "":
						ui.Text(c, p.st.ok).FontSize(12).TextColor(t.TextMuted)
						revealButton(c, "masks-reveal", revealLabel(), p.outDir)
					}
				})
			})
			section(c, T("CerebrA atlas labels", "CerebrA 图谱标签"), T("Combine label numbers into a custom region on the Intensity Normalisation page, e.g. 34,85 for the lateral occipital cortex of both sides.",
				"在“强度归一化”页面可用标签编号组合出自选区域，例如 34,85 为双侧外侧枕叶皮层。"), func() {
				all := core.AtlasLabels()
				shown := all
				if q := strings.ToLower(strings.TrimSpace(p.query)); q != "" {
					shown = nil
					for _, l := range all {
						if strings.Contains(strings.ToLower(l.Name), q) || strings.HasPrefix(strconv.Itoa(l.Index), q) {
							shown = append(shown, l)
						}
					}
				}
				if p.selected == nil {
					p.selected = map[int]bool{}
				}
				ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().GapY(6).Children(func() {
					ui.SearchField(c, &p.query).Placeholder(T("Search regions", "搜索脑区")).Width(260)
					ui.Text(c, T("Tick rows to build a custom region", "勾选脑区以快速组合自选区域")).FontSize(12).TextColor(t.TextMuted)
				})
				cols := []ui.TableColumn{
					{Title: "", Width: 36, Align: ui.Center},
					{Title: T("Number", "编号"), Width: 76, Align: ui.End},
					{Title: T("Region", "脑区")},
					{Title: T("Hemisphere", "半球"), Width: 96},
				}
				card(c).Children(func() {
					ui.Table(c, &p.table, cols, len(shown), func(row, col int) {
						l := shown[row]
						if col == 0 {
							on := p.selected[l.Index]
							check(c.Key("lbl-"+strconv.Itoa(l.Index)), &on, "")
							if on {
								p.selected[l.Index] = true
							} else {
								delete(p.selected, l.Index)
							}
							return
						}
						hemi := T("Right", "右")
						if l.Hemi == "L" {
							hemi = T("Left", "左")
						}
						ui.Text(c, [...]string{"", strconv.Itoa(l.Index), l.Name, hemi}[col]).FontSize(12.5).SingleLine()
					}).Height(330)
				})
				if len(p.selected) > 0 {
					var nums []int
					for id := range p.selected {
						nums = append(nums, id)
					}
					sort.Ints(nums)
					parts := make([]string, len(nums))
					for i, id := range nums {
						parts[i] = strconv.Itoa(id)
					}
					joined := strings.Join(parts, ",")
					card(c).Padding(10, 14).Children(func() {
						ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().GapY(6).Children(func() {
							pill(c, Tf("%d selected", "已选 %d 个", len(nums)), paletteFor(c).accent)
							ui.Text(c, joined).FontSize(12.5).Font("monospace").SingleLine().Ellipsis("…").Grow(1).MinWidth(0).Selectable()
							if button(c.Key("lbl-clear"), T("Clear", "清空"), nil, buttonGhost, 26).Clicked() {
								p.selected = map[int]bool{}
							}
							if button(c.Key("lbl-use"), T("Use as Custom Region", "用于自选区域归一化"), iconArrowRight, buttonPrimary, 26).Clicked() {
								// As the Reference Cluster page does with its region:
								// the region becomes the one method chosen.
								a.norm.methods = map[string]bool{"region": true}
								a.norm.region.kind, a.norm.region.labels = 2, joined
								a.page = "normalize"
							}
						})
					})
				}
			})
		}, nil)
}

func (a *app) exportMasks(paths []string) {
	p := &a.masks
	out := paths[0]
	p.st = status{}
	a.start("masks", T("Exporting", "正在导出"), func(ctx context.Context, step func(int, int)) (func(), error) {
		for i, m := range core.Masks {
			vol, err := core.BuiltinMask(m.Name)
			if err != nil {
				return nil, err
			}
			if err := core.SaveNifti(filepath.Join(out, m.Name+".nii.gz"), vol, true); err != nil {
				return nil, err
			}
			step(i+1, len(core.Masks))
		}
		return func() {
			p.outDir = out
			p.st.ok = Tf("%d masks exported.", "已导出 %d 个掩膜。", len(core.Masks))
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
