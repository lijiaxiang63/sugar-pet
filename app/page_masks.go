package main

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type masksPage struct {
	query  string
	table  ui.ListState
	st     status
	outDir string
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
				ui.SearchField(c, &p.query).Placeholder(T("Search regions", "搜索脑区")).Width(260)
				cols := []ui.TableColumn{
					{Title: T("Number", "编号"), Width: 80, Align: ui.End},
					{Title: T("Region", "脑区")},
					{Title: T("Hemisphere", "半球"), Width: 100},
				}
				card(c).Children(func() {
					ui.Table(c, &p.table, cols, len(shown), func(row, col int) {
						l := shown[row]
						hemi := T("Right", "右")
						if l.Hemi == "L" {
							hemi = T("Left", "左")
						}
						ui.Text(c, [...]string{strconv.Itoa(l.Index), l.Name, hemi}[col]).FontSize(12.5).SingleLine()
					}).Height(330)
				})
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
