package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

// options are the settings of the analyses that most runs leave alone.
// They are changed in the settings window and kept between launches; the
// pages hold only what a run cannot do without, and say in a line which of
// these are in effect.
type options struct {
	// Spatial normalisation
	Template string  `json:"template,omitempty"` // a PET template of the user's; "" is SPM's
	Jobs     float64 `json:"matlab_processes"`

	// Intensity normalisation
	Statistic int     `json:"statistic"` // 0 mean, 1 median
	Target    float64 `json:"target"`
	FWHMAuto  bool    `json:"smoothing_automatic"`
	FWHM      float64 `json:"smoothing_mm"`
	SaveMasks bool    `json:"save_masks"`

	// The iterative methods, iPS and iHN, wherever they are used
	PThreshold float64 `json:"iterative_p"`
	Exclude    int     `json:"iterative_exclude"` // 0 hypo, 1 both
	MaxIter    float64 `json:"iterative_rounds"`
	LiteralIPS bool    `json:"ips_plain_mean"`

	// Building a reference
	RefMask     string          `json:"reference_mask"` // a built-in mask, or "file"
	RefMaskFile string          `json:"reference_mask_file,omitempty"`
	RefFWHM     float64         `json:"reference_smoothing_mm"`
	RefStatsFor map[string]bool `json:"reference_stats_for"`

	// Classification
	ReportLanguage int `json:"report_language"` // 0 as the app, 1 English, 2 Chinese
	// CrossCheck: the findings are also read on maps made with other
	// normalisations, which are computed for it.  Off, the classification
	// reads the maps of step 2 alone.
	CrossCheck bool `json:"cross_check"`

	// Group comparison
	Correction   int     `json:"correction"` // 0 GRF, 1 FDR, 2 none
	VoxelP       float64 `json:"voxel_p"`
	ClusterP     float64 `json:"cluster_p"`
	Q            float64 `json:"fdr_q"`
	Analysis     string  `json:"analysis_mask"` // a built-in mask, or "file"
	AnalysisFile string  `json:"analysis_mask_file,omitempty"`
	CmpFWHMAuto  bool    `json:"compare_smoothing_automatic"`
	CmpFWHM      float64 `json:"compare_smoothing_mm"`
	KeepImages   bool    `json:"keep_images"`

	// Reference cluster
	ClusterThreshold float64 `json:"cluster_threshold_p"`
	MinCluster       float64 `json:"cluster_min_ml"`
}

func defaultOptions() options {
	return options{
		Jobs:   float64(min(max(runtime.NumCPU()/2, 1), 4)),
		Target: 1, FWHMAuto: true, FWHM: 8,
		PThreshold: 0.01, MaxIter: 1,
		RefMask: "gray-matter", RefFWHM: 8, RefStatsFor: map[string]bool{},
		VoxelP: 0.001, ClusterP: 0.05, Q: 0.05, Analysis: "gray-matter", CmpFWHMAuto: true, CmpFWHM: 8,
		ClusterThreshold: 0.001, MinCluster: 1,
	}
}

// sound puts values a settings file should not hold back to their defaults.
func (o *options) sound() {
	d := defaultOptions()
	positive := func(v *float64, def float64) {
		if !(*v > 0) {
			*v = def
		}
	}
	positive(&o.Jobs, d.Jobs)
	o.Jobs = min(o.Jobs, float64(max(runtime.NumCPU(), 1)))
	positive(&o.Target, d.Target)
	positive(&o.PThreshold, d.PThreshold)
	positive(&o.VoxelP, d.VoxelP)
	positive(&o.ClusterP, d.ClusterP)
	positive(&o.Q, d.Q)
	positive(&o.ClusterThreshold, d.ClusterThreshold)
	o.FWHM, o.RefFWHM, o.CmpFWHM = max(o.FWHM, 0), max(o.RefFWHM, 0), max(o.CmpFWHM, 0)
	o.MaxIter, o.MinCluster = max(o.MaxIter, 0), max(o.MinCluster, 0)
	o.Statistic, o.Exclude = min(max(o.Statistic, 0), 1), min(max(o.Exclude, 0), 1)
	o.ReportLanguage, o.Correction = min(max(o.ReportLanguage, 0), 2), min(max(o.Correction, 0), 2)
	mask := func(name *string, file, def string) {
		if *name == "file" && file != "" {
			return
		}
		if !core.IsMask(*name) {
			*name = def
		}
	}
	mask(&o.RefMask, o.RefMaskFile, d.RefMask)
	mask(&o.Analysis, o.AnalysisFile, d.Analysis)
	if o.RefStatsFor == nil {
		o.RefStatsFor = map[string]bool{}
	}
}

// statsFor are the methods a new reference also keeps control statistics
// for, in the order of the methods.
func (o *options) statsFor() []string {
	var out []string
	for _, m := range core.StatsMethods() {
		if o.RefStatsFor[m] {
			out = append(out, m)
		}
	}
	return out
}

// normSettings says which of the options a run of Intensity Normalisation by
// these methods depends on, and how they are set: two runs with the same
// text give the same images.
func (o *options) normSettings(methods []string) string {
	text := fmt.Sprint(o.Statistic, o.Target, o.FWHMAuto)
	if !o.FWHMAuto {
		text += fmt.Sprint(" ", o.FWHM)
	}
	if slices.Contains(methods, "ips") || slices.Contains(methods, "ihn") {
		text += fmt.Sprint(" ", o.PThreshold, o.Exclude, o.MaxIter, o.LiteralIPS)
	}
	return text
}

// The groups of options, as the sections of the settings window that hold
// them.
const (
	groupSpatial   = "spatial"
	groupNormalize = "normalize"
	groupIterative = "iterative"
	groupReference = "reference"
	groupClassify  = "classify"
	groupCompare   = "compare"
	groupCluster   = "cluster"
)

// The names of the options, as the settings window and the pages show them.
var (
	lblTemplate   = label{"PET template", "PET 模板"}
	lblJobs       = label{"MATLAB processes", "MATLAB 进程数"}
	lblStatistic  = label{"Region statistic", "区域统计量"}
	lblTarget     = label{"Target value", "目标值"}
	lblSmoothing  = label{"Smoothing before estimating", "估计因子前的平滑"}
	lblSaveMasks  = label{"Save masks", "保存掩膜"}
	lblIterP      = label{"Exclusion p-value", "剔除体素的 p 值"}
	lblIterSide   = label{"Exclude", "剔除范围"}
	lblIterRounds = label{"Re-estimations", "重新估计次数"}
	lblLiteral    = label{"Plain mean for iPS", "iPS 用普通均值"}
	lblRefMask    = label{"Mask", "掩膜"}
	lblRefFWHM    = label{"Smoothing (FWHM, mm)", "平滑 (FWHM, mm)"}
	lblRefStats   = label{"Control statistics of other methods", "其他方法的对照统计"}
	lblClsLang    = label{"Language of the report", "报告语言"}
	lblClsCheck   = label{"Cross-check with another normalisation", "用另一种归一化方法交叉验证"}
	lblCorrection = label{"Multiple-comparison correction", "多重比较校正"}
	lblVoxelP     = label{"Voxel-level p", "体素水平 p 值"}
	lblClusterP   = label{"Cluster-level p", "团块水平 p 值"}
	lblQ          = label{"q value", "q 值"}
	lblAnalysis   = label{"Voxels to test", "检验的体素"}
	lblCmpFWHM    = label{"Smoothing before the test", "检验前的平滑"}
	lblKeep       = label{"Keep the images that were tested", "保留参与检验的图像"}
	lblClusterThr = label{"p-value", "p 值"}
	lblMinCluster = label{"Smallest cluster (mL)", "最小团块体积 (mL)"}
)

// changed names the options of the groups that are not as the app first
// has them.
func (o *options) changed(groups ...string) []string {
	d := defaultOptions()
	var out []string
	add := func(group string, differs bool, name label) {
		if differs && slices.Contains(groups, group) {
			out = append(out, name.String())
		}
	}
	add(groupSpatial, o.Template != "", lblTemplate)
	add(groupSpatial, o.Jobs != d.Jobs, lblJobs)
	add(groupNormalize, o.Statistic != d.Statistic, lblStatistic)
	add(groupNormalize, o.Target != d.Target, lblTarget)
	add(groupNormalize, !o.FWHMAuto, lblSmoothing)
	add(groupNormalize, o.SaveMasks != d.SaveMasks, lblSaveMasks)
	add(groupIterative, o.PThreshold != d.PThreshold, lblIterP)
	add(groupIterative, o.Exclude != d.Exclude, lblIterSide)
	add(groupIterative, o.MaxIter != d.MaxIter, lblIterRounds)
	add(groupIterative, o.LiteralIPS != d.LiteralIPS, lblLiteral)
	add(groupReference, o.RefMask != d.RefMask, lblRefMask)
	add(groupReference, o.RefFWHM != d.RefFWHM, lblRefFWHM)
	add(groupReference, len(o.statsFor()) > 0, lblRefStats)
	add(groupClassify, o.ReportLanguage != d.ReportLanguage, lblClsLang)
	add(groupClassify, o.CrossCheck != d.CrossCheck, lblClsCheck)
	add(groupCompare, o.Correction != d.Correction, lblCorrection)
	add(groupCompare, o.Correction == 0 && o.VoxelP != d.VoxelP, lblVoxelP)
	add(groupCompare, o.Correction == 0 && o.ClusterP != d.ClusterP, lblClusterP)
	add(groupCompare, o.Correction == 1 && o.Q != d.Q, lblQ)
	add(groupCompare, o.Analysis != d.Analysis, lblAnalysis)
	add(groupCompare, !o.CmpFWHMAuto, lblCmpFWHM)
	add(groupCompare, o.KeepImages != d.KeepImages, lblKeep)
	add(groupCluster, o.ClusterThreshold != d.ClusterThreshold, lblClusterThr)
	add(groupCluster, o.MinCluster != d.MinCluster, lblMinCluster)
	return out
}

// restore puts the options of the groups back as the app first has them.
func (o *options) restore(groups ...string) {
	d := defaultOptions()
	for _, group := range groups {
		switch group {
		case groupSpatial:
			o.Template, o.Jobs = d.Template, d.Jobs
		case groupNormalize:
			o.Statistic, o.Target, o.FWHMAuto, o.FWHM = d.Statistic, d.Target, d.FWHMAuto, d.FWHM
			o.SaveMasks = d.SaveMasks
		case groupIterative:
			o.PThreshold, o.Exclude, o.MaxIter, o.LiteralIPS = d.PThreshold, d.Exclude, d.MaxIter, d.LiteralIPS
		case groupReference:
			o.RefMask, o.RefMaskFile, o.RefFWHM, o.RefStatsFor = d.RefMask, d.RefMaskFile, d.RefFWHM, d.RefStatsFor
		case groupClassify:
			o.ReportLanguage, o.CrossCheck = d.ReportLanguage, d.CrossCheck
		case groupCompare:
			o.Correction, o.VoxelP, o.ClusterP, o.Q = d.Correction, d.VoxelP, d.ClusterP, d.Q
			o.Analysis, o.AnalysisFile, o.CmpFWHMAuto, o.CmpFWHM, o.KeepImages = d.Analysis, d.AnalysisFile, d.CmpFWHMAuto, d.CmpFWHM, d.KeepImages
		case groupCluster:
			o.ClusterThreshold, o.MinCluster = d.ClusterThreshold, d.MinCluster
		}
	}
}

func num(v float64) string { return strconv.FormatFloat(v, 'g', 6, 64) }

func join(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// smoothingText says how much an analysis smooths.
func smoothingText(auto bool, fwhm float64) string {
	if auto {
		return T("smoothing automatic", "平滑自动")
	}
	if fwhm == 0 {
		return T("no smoothing", "不平滑")
	}
	return Tf("smoothing %s mm", "平滑 %s mm", num(fwhm))
}

// iterativeText says in a line how iPS and iHN leave voxels out.
func (o *options) iterativeText() string {
	side := T("hypometabolic voxels", "低代谢体素")
	if o.Exclude == 1 {
		side = T("hypo- and hypermetabolic voxels", "低代谢和高代谢体素")
	}
	return Tf("iPS and iHN leave out %s at p < %s, re-estimating %s×", "iPS 与 iHN 剔除 p < %[2]s 的%[1]s，重新估计 %[3]s 次",
		side, num(o.PThreshold), num(o.MaxIter))
}

// summary says in a line which options are in effect on a page.
func (o *options) summary(page string) string {
	maskOf := func(name, file string) string {
		if name == "file" {
			return filepath.Base(file)
		}
		return maskName(name)
	}
	switch page {
	case "spatial":
		template := T("SPM's PET template", "SPM 自带的 PET 模板")
		if o.Template != "" {
			template = T("Template ", "模板 ") + filepath.Base(o.Template)
		}
		return join(template, Tf("%s MATLAB process(es)", "%s 个 MATLAB 进程", num(o.Jobs)))
	case "normalize":
		statistic := T("Mean of the region", "区域均值")
		if o.Statistic == 1 {
			statistic = T("Median of the region", "区域中位数")
		}
		written := ""
		if o.SaveMasks {
			written = T("masks saved", "保存掩膜")
		}
		return join(statistic, Tf("target %s", "目标值 %s", num(o.Target)), smoothingText(o.FWHMAuto, o.FWHM), written)
	case "reference":
		also := ""
		if names := o.statsFor(); len(names) > 0 {
			for i, m := range names {
				names[i] = methodName(m)
			}
			sort.Strings(names)
			also = T("also keeps the statistics of ", "另保留以下方法的对照统计：") + strings.Join(names, T(", ", "、"))
		}
		// The factors of the controls follow Intensity Normalisation.
		factors := ""
		if o.Statistic == 1 {
			factors = T("factors by the median", "因子取中位数")
		}
		if !o.FWHMAuto {
			factors = join(factors, Tf("factors after %s mm", "估计因子前平滑 %s mm", num(o.FWHM)))
		}
		return join(Tf("%s mask", "%s掩膜", maskOf(o.RefMask, o.RefMaskFile)), Tf("smoothing %s mm", "平滑 %s mm", num(o.RefFWHM)), factors, also)
	case "classify":
		language := [...]string{T("Report in the app's language", "报告语言与界面一致"), T("Report in English", "英文报告"), T("Report in Chinese", "中文报告")}[o.ReportLanguage]
		check := T("no cross-check", "不做交叉验证")
		if o.CrossCheck {
			check = T("cross-checked with another normalisation", "用另一种归一化方法交叉验证")
		}
		return join(language, check)
	case "compare":
		correction := T("No correction", "不校正")
		switch o.Correction {
		case 0:
			correction = Tf("GRF correction: voxel p < %s, cluster p < %s", "GRF 校正：体素 p < %s，团块 p < %s", num(o.VoxelP), num(o.ClusterP))
		case 1:
			correction = Tf("FDR correction: q < %s", "FDR 校正：q < %s", num(o.Q))
		}
		kept := ""
		if o.KeepImages {
			kept = T("tested images kept", "保留参与检验的图像")
		}
		return join(correction, T("within ", "范围：")+maskOf(o.Analysis, o.AnalysisFile), smoothingText(o.CmpFWHMAuto, o.CmpFWHM), kept)
	case "cluster":
		return join(Tf("p < %s, one-sided", "单侧 p < %s", num(o.ClusterThreshold)), Tf("clusters of at least %s mL", "团块不小于 %s mL", num(o.MinCluster)))
	}
	return ""
}

// optionsLine is the line of a page that says which options are in effect
// and opens the settings that hold them.
func (a *app) optionsLine(c *ui.Context, page, section string, groups ...string) {
	pal := paletteFor(c)
	changed := a.opt.changed(groups...)
	text := a.opt.summary(page)
	row := ui.Row(c.Key("options-"+page)).Padding(0, 4, 0, 12).MinHeight(40).Gap(10).Radius(12).AlignItems(ui.Center).
		Background(pal.panel).InsetShadow(0, 0, 0, 1, pal.border)
	row.Children(func() {
		ui.Icon(c, iconSliders).Size(15, 15).TextColor(pal.muted).Shrink(0)
		ui.Text(c, text).FontSize(12.5).TextColor(pal.muted).SingleLine().Ellipsis("…").Grow(1).MinWidth(0).Tooltip(text)
		if len(changed) > 0 {
			pill(c, Tf("%d changed", "已修改 %d 项", len(changed)), pal.accent).
				Tooltip(T("Not as the app first has them: ", "与默认设置不同：") + strings.Join(changed, T(", ", "、")))
		}
		if button(c.Key("open"), T("Options…", "选项…"), nil, buttonGhost, 28).Clicked() {
			a.openSettings(section)
		}
	})
}
