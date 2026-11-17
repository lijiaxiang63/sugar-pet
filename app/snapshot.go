package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

func openPath(path string) { mygo.Shell.OpenPath(path) }

// snapshots draws every page without a window and saves the pictures, to
// check the look: sugar-pet --snapshot <folder> [zh].
func snapshots(args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	chinese = len(args) > 1 && args[1] == "zh"
	os.MkdirAll(dir, 0o755)
	for _, dark := range []bool{false, true} {
		for _, p := range pages {
			a := newApp()
			a.demo()
			a.page = p.id
			tt := ui.NewTesterWith(a.view, 1060, 740, ui.TesterOptions{Dark: dark})
			tt.SetScale(2)
			tt.Frame()
			name := p.id
			if dark {
				name += "-dark"
			}
			file, err := os.Create(filepath.Join(dir, name+".png"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			png.Encode(file, tt.Image())
			file.Close()
			if dark {
				continue
			}
			// The rest of the page, a screen at a time.
			for n := 2; n <= 5; n++ {
				before := tt.Image().Pix
				before = append([]byte(nil), before...)
				tt.Scroll(1052, 400, 0, 560)
				tt.Frame()
				if string(before) == string(tt.Image().Pix) {
					break
				}
				file, _ := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%d.png", p.id, n)))
				png.Encode(file, tt.Image())
				file.Close()
			}
		}
	}
}

// demo fills the pages with made-up input and results, for the snapshots.
func (a *app) demo() {
	patient := func(i int) string { return fmt.Sprintf("/Users/ada/study/patients/sub-pt%02d_space-MNI_pet.nii.gz", i) }
	control := func(i int) string { return fmt.Sprintf("/Users/ada/study/controls/sub-hc%02d_space-MNI_pet.nii.gz", i) }
	for i := 1; i <= 6; i++ {
		a.norm.files.files = append(a.norm.files.files, patient(i))
		a.zmap.files.files = append(a.zmap.files.files, patient(i))
	}
	for i := 1; i <= 12; i++ {
		a.cmp.g1.files = append(a.cmp.g1.files, patient(i))
		a.cluster.files.files = append(a.cluster.files.files, patient(i))
	}
	for i := 1; i <= 16; i++ {
		a.cmp.g2.files = append(a.cmp.g2.files, control(i))
		a.ref.files.files = append(a.ref.files.files, control(i))
	}
	sp := &a.spatial
	sp.spm, sp.matlab, sp.output = "/Users/ada/spm12", "/Applications/MATLAB_R2025b.app", "/Users/ada/study/patients"
	sp.outDir = sp.output
	for i := 1; i <= 6; i++ {
		native := fmt.Sprintf("/Users/ada/study/raw/sub-pt%02d_pet.nii.gz", i)
		sp.files.files = append(sp.files.files, native)
		sp.rows = append(sp.rows, spatialRow{image: filepath.Base(native), output: core.SpatialOutputName(native)})
		sp.written = append(sp.written, patient(i))
	}
	sp.st.ok = Tf("%d image(s) warped to MNI space.", "已完成：%d 幅图像已配准到 MNI 空间。", 6)
	a.norm.methods["cerebellum"] = true
	a.setReference("/Users/ada/study/hc_reference")
	a.norm.output = "/Users/ada/study/normalised"

	info := &refInfo{N: 16, FWHM: 8, Mask: "gray-matter", StatsFor: []string{"cerebellum"}}
	info.Covariates = []struct {
		Name string `json:"name"`
	}{{"age"}, {"sex"}}
	a.ref.current, a.ref.info = a.norm.reference, info
	a.ref.output = "/Users/ada/study/hc_reference"
	a.zmap.loadedFrom, a.zmap.info = a.zmap.reference, info
	a.zmap.covTable = "/Users/ada/study/participants.tsv"
	a.zmap.output = "/Users/ada/study/zmaps"

	cl := &a.classify
	cl.files.files = a.zmap.files.files[:3]
	cl.loadedFrom, cl.info = cl.reference, info
	cl.covTable, cl.output, cl.outDir = a.zmap.covTable, "/Users/ada/study/reports", "/Users/ada/study/reports"
	for i, r := range [][2]string{
		{core.ReportText("name-msa-p", uiLang()), Tf("Pattern match %d/100, confidence %s", "模式匹配度 %d/100，置信度%s", 84, core.ReportText("confidence-high", uiLang()))},
		{Tf("%s or %s", "%s 或 %s", core.ReportText("name-psp", uiLang()), core.ReportText("name-cbd", uiLang())),
			Tf("Cannot be told apart: %d/100 and %d/100", "无法可靠区分：%d/100 与 %d/100", 62, 55)},
		{T("No pattern of atypical parkinsonism", "未见非典型帕金森综合征的模式"), T("Favours Parkinson's disease in a patient with parkinsonism", "对帕金森综合征患者而言更倾向于帕金森病")},
	} {
		image := filepath.Base(cl.files.files[i])
		cl.rows = append(cl.rows, classifyRow{image: image, reading: r[0], detail: r[1], report: filepath.Join(cl.output, core.ReportStem(image)+"_report.html")})
	}
	cl.st.ok = Tf("%d report(s) written.", "已生成 %d 份报告。", 3)

	a.cmp.output = "/Users/ada/study/compare"
	a.cmp.name = "patients_vs_controls"
	a.cmp.how = Tf("GRF correction: voxel p < %g, cluster p < %g, clusters of at least %d voxels (|z| > %.2f).",
		"GRF 校正：体素 p < %g，团块 p < %g，团块至少 %d 个体素（|z| > %.2f）。", 0.001, 0.05, 187, 3.29)
	a.cmp.st.ok = Tf("%d cluster(s) survive the correction.", "有 %d 个团块通过校正。", 3)
	a.cmp.result = &core.GroupComparison{
		TMap: "/Users/ada/study/compare/patients_vs_controls_T.nii.gz", DOF: 26, FWHM: [3]float64{11.8, 12.4, 11.1},
		Corrected: true, NClusters: 3, ZThreshold: 3.29, MinClusterVoxels: 187,
		Clusters: []core.Cluster{
			{Size: 2841, Peak: -7.92, MNI: [3]float64{-44, -62, 38}, PeakAtlas: [3]string{"Angular_L"},
				Regions: [3][]core.Region{{{Name: "Angular_L", Voxels: 902, Pct: 31.7}, {Name: "Parietal_Inf_L", Voxels: 744, Pct: 26.2}, {Name: "Temporal_Mid_L", Voxels: 510, Pct: 18}}}},
			{Size: 1975, Peak: -6.48, MNI: [3]float64{2, -54, 30}, PeakAtlas: [3]string{"Cingulum_Post_R"},
				Regions: [3][]core.Region{{{Name: "Precuneus_L", Voxels: 801, Pct: 40.6}, {Name: "Precuneus_R", Voxels: 655, Pct: 33.2}, {Name: "Cingulum_Post_R", Voxels: 190, Pct: 9.6}}}},
			{Size: 412, Peak: 5.31, MNI: [3]float64{-2, -28, -34}, PeakAtlas: [3]string{"Pons"},
				Regions: [3][]core.Region{{{Name: "Pons", Voxels: 300, Pct: 72.8}, {Name: "Cerebelum_9_L", Voxels: 61, Pct: 14.8}}}},
		},
	}

	a.cluster.output = "/Users/ada/study/reference_cluster.nii.gz"
	a.cluster.saved, a.cluster.volume = a.cluster.output, 58
	a.cluster.st.ok = Tf("Reference cluster of %.0f mL saved.", "已保存参考簇（%.0f mL）。", 58.0)
}
