package main

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

func openPath(path string) { mygo.Shell.OpenPath(path) }

// snapshots draws every page, and every section of the settings, without a
// window and saves the pictures, to check the look and for the
// documentation: sugar-pet --snapshot <folder> [en|zh] [height].  The window
// is 1060 points wide and, unless told otherwise, 740 high, as it first
// opens; the settings are as their window first opens.
func snapshots(args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	chinese = len(args) > 1 && args[1] == "zh"
	height := 740
	if len(args) > 2 {
		if h, err := strconv.Atoi(args[2]); err == nil && h >= 560 {
			height = h
		}
	}
	os.MkdirAll(dir, 0o755)
	save := func(tt *ui.Tester, name string) {
		file, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		png.Encode(file, tt.Image())
		file.Close()
	}
	// The settings, with updates as an installed app has them.
	checked := time.Now().Add(-2 * time.Hour)
	lastUpdateCheck, availableVersion = func() time.Time { return checked }, func() string { return "" }
	updateChecks, updateDownloads, canUpdate = func() bool { return true }, func() bool { return false }, func() bool { return true }
	for _, dark := range []bool{false, true} {
		for _, s := range settingsSections {
			a := newApp()
			a.demo()
			a.updates, a.settingsPage = true, s.id
			tt := ui.NewTesterWith(a.settingsView, settingsWidth, settingsHeight, ui.TesterOptions{Dark: dark})
			tt.SetScale(2)
			tt.Frame()
			name := "settings-" + s.id
			if dark {
				name += "-dark"
			}
			save(tt, name)
		}
	}
	for _, dark := range []bool{false, true} {
		for _, p := range pages {
			a := newApp()
			a.demo()
			a.page = p.id
			tt := ui.NewTesterWith(a.view, 1060, height, ui.TesterOptions{Dark: dark})
			tt.SetScale(2)
			tt.Frame()
			if dark {
				save(tt, p.id+"-dark")
				continue
			}
			save(tt, p.id)
			// The rest of the page, a screen at a time: page-2, page-3,
			// and the last of them once more as page-end.
			for n := 2; n <= 6; n++ {
				before := append([]byte(nil), tt.Image().Pix...)
				tt.Scroll(640, 400, 0, 560)
				tt.Frame()
				if string(before) == string(tt.Image().Pix) {
					break
				}
				save(tt, fmt.Sprintf("%s-%d", p.id, n))
			}
			save(tt, p.id+"-end")
		}
	}
}

// demo fills the pages with made-up input and results, for the snapshots.
func (a *app) demo() {
	patient := func(i int) string { return fmt.Sprintf("/Users/ada/study/patients/sub-pt%02d_space-MNI_pet.nii.gz", i) }
	control := func(i int) string { return fmt.Sprintf("/Users/ada/study/controls/sub-hc%02d_space-MNI_pet.nii.gz", i) }
	for i := 1; i <= 6; i++ {
		a.norm.files.files = append(a.norm.files.files, patient(i))
	}
	for i := 1; i <= 12; i++ {
		a.cmp.g1.files = append(a.cmp.g1.files, patient(i))
		a.cluster.files.files = append(a.cluster.files.files, patient(i))
	}
	for i := 1; i <= 16; i++ {
		a.cmp.g2.files = append(a.cmp.g2.files, control(i))
		a.ref.files.files = append(a.ref.files.files, control(i))
	}
	a.study.parent, a.study.made, a.study.statusOf = "/Users/ada", "/Users/ada/study", "/Users/ada/study"
	a.study.created = make([]bool, len(core.StudyLayout))
	for i := range a.study.created {
		a.study.created[i] = true
	}
	a.study.st.ok = Tf("Study folder ready: %d created, %d already there. The pages now write into it.",
		"研究文件夹已就绪：新建 %d 项，%d 项原已存在。各页面的输出已指向这里。", len(core.StudyLayout), 0)
	sp := &a.spatial
	sp.spm, sp.matlab, sp.output = "/Users/ada/spm12", "/Applications/MATLAB_R2025b.app", "/Users/ada/study/patients"
	sp.outDir = sp.output
	for i := 1; i <= 6; i++ {
		native := fmt.Sprintf("/Users/ada/study/raw/sub-pt%02d_pet.nii.gz", i)
		sp.files.files = append(sp.files.files, native)
		sp.rows = append(sp.rows, spatialRow{image: filepath.Base(native), output: core.SpatialOutputName(native), path: patient(i)})
		sp.written = append(sp.written, patient(i))
	}
	sp.st.ok = Tf("%d image(s) warped to MNI space.", "已完成：%d 幅图像已配准到 MNI 空间。", 6)
	a.norm.methods["cerebellum"] = true
	a.setReference("/Users/ada/study/hc_reference")
	a.norm.output = "/Users/ada/study/normalised"
	a.norm.outDir = a.norm.output
	for i, f := range [][2]string{{"5.81437", "5.42206"}, {"6.20915", "5.96384"}, {"4.97203", "4.51870"}, {"5.53866", "5.30129"}, {"6.74052", "6.18845"}, {"5.12790", "4.89317"}} {
		image := filepath.Base(patient(i + 1))
		hn, _ := strconv.ParseFloat(f[0], 64)
		cerebellum, _ := strconv.ParseFloat(f[1], 64)
		a.norm.rows = append(a.norm.rows,
			normRow{image: image, method: "hn", factor: f[0], voxels: "193204", value: hn},
			normRow{image: image, method: "cerebellum", factor: f[1], voxels: "22750", value: cerebellum})
	}
	a.norm.st.ok = Tf("%d image(s) intensity-normalised with %d method(s).", "强度归一化已完成：%d 幅图像 × %d 种方法。", 6, 2)
	// The steps of the individual analysis, each done on what is before it.
	patients := a.norm.files.files
	a.norm.done = &normDone{asked: patients, files: patients, methods: a.norm.chosen(), reference: a.norm.reference,
		settings: a.opt.normSettings(a.norm.chosen())}

	info := &refInfo{N: 16, FWHM: 8, Mask: "gray-matter", StatsFor: []string{"cerebellum", "white-matter"}}
	info.Covariates = []struct {
		Name string `json:"name"`
	}{{"age"}, {"sex"}}
	a.ref.current, a.ref.info = a.norm.reference, info
	a.ref.output = "/Users/ada/study/hc_reference"
	a.opt.RefStatsFor["cerebellum"], a.opt.RefStatsFor["white-matter"] = true, true
	a.ref.cov.path = "/Users/ada/study/participants.tsv"
	a.ref.cov.table = &core.Table{Path: a.ref.cov.path, Header: []string{"participant_id", "age", "sex"}, Rows: make([]map[string]string, 28)}
	a.ref.cov.adjust = map[string]bool{"age": true, "sex": true}
	a.ref.cov.stamp = stampOf(a.ref.cov.path) // the table is made up: nothing is to be read
	// Compare Groups takes the table of the reference, as it does when shown.
	a.cmp.cov, a.cmp.covFrom = a.ref.cov, a.ref.cov.path
	a.cmp.cov.columns = a.ref.cov.table.Columns()
	a.zmap.loadedFrom, a.zmap.info = a.zmap.reference, info
	a.zmap.covTable = "/Users/ada/study/participants.tsv"
	a.zmap.output = "/Users/ada/study/zmaps"
	a.zmap.outDir = a.zmap.output
	// A map of each image by each method of step 1, the first of them HN's.
	for i, r := range [][4]string{{"-6.41", "9.8%", "-6.02", "8.9%"}, {"-5.07", "6.3%", "-4.71", "5.6%"}, {"-3.12", "2.6%", "-2.95", "2.4%"},
		{"-7.85", "12.4%", "-7.36", "11.2%"}, {"-4.66", "5.1%", "-4.38", "4.7%"}, {"-5.93", "8.7%", "-5.60", "7.9%"}} {
		image := filepath.Base(patient(i + 1))
		for n, method := range []string{"hn", "cerebellum"} {
			file := core.OutputName(image, method, "wmap")
			a.zmap.rows = append(a.zmap.rows, zmapRow{image: image, method: method, file: file, lowest: r[2*n], share: r[2*n+1],
				source: patient(i + 1), path: filepath.Join(a.zmap.output, file), wscore: true})
		}
	}
	a.zmap.st.ok = Tf("%d map(s) written.", "已输出 %d 幅图。", 12)
	a.zmap.done = &zmapDone{from: a.norm.done, files: patients, methods: a.norm.chosen(), table: a.zmap.covTable}

	cl := &a.classify
	cl.loadedFrom, cl.info = cl.reference, info
	cl.output, cl.outDir = "/Users/ada/study/reports", "/Users/ada/study/reports"
	cl.from, cl.method = a.zmap.done, "hn"
	// A reading for each image of step 2.
	scores := [][]int{{84, 31, 22, 18, 12, 9}, {24, 12, 62, 55, 20, 14}, {11, 8, 14, 10, 16, 58}, {15, 10, 21, 26, 73, 12}, {19, 47, 16, 11, 9, 22}, {9, 12, 10, 14, 13, 61}}
	leading := []string{"msa-p", "psp", "", "dlb", "msa-c", ""}
	cats := [][2]string{{"pattern", "high"}, {"indeterminate", ""}, {"none", ""}, {"pattern", "moderate"}, {"possible", ""}, {"none", ""}}
	none := [2]string{T("No pattern of atypical parkinsonism", "未见非典型帕金森综合征的模式"), T("Favours Parkinson's disease in a patient with parkinsonism", "对帕金森综合征患者而言更倾向于帕金森病")}
	for i, r := range [][2]string{
		{core.ReportText("name-msa-p", uiLang()), Tf("Pattern match %d/100", "模式匹配度 %d/100", 84)},
		{Tf("%s or %s", "%s 或 %s", core.ReportText("name-psp", uiLang()), core.ReportText("name-cbd", uiLang())),
			Tf("Cannot be told apart: %d/100 and %d/100", "无法可靠区分：%d/100 与 %d/100", 62, 55)},
		none,
		{core.ReportText("name-dlb", uiLang()), Tf("Pattern match %d/100", "模式匹配度 %d/100", 73)},
		{Tf("Partial features of %s", "%s的部分特征", core.ReportText("name-msa-c", uiLang())),
			Tf("Pattern match %d/100: not enough to assign a pattern", "模式匹配度 %d/100：不足以判定", 47)},
		none,
	} {
		image := filepath.Base(patients[i])
		row := classifyRow{image: image, reading: r[0], detail: r[1], report: filepath.Join(cl.output, core.ReportStem(image)+"_report.html"),
			source: patients[i], zmap: filepath.Join(cl.output, core.OutputName(image, "hn", "wmap")), wscore: true, leading: leading[i],
			category: cats[i][0], confidence: cats[i][1]}
		row.badge, _ = classifyBadge(row.category, row.confidence, &lightPalette)
		for n, id := range core.Patterns {
			row.scores = append(row.scores, patternScore{id, scores[i][n]})
		}
		cl.rows = append(cl.rows, row)
	}
	cl.st.ok = Tf("%d report(s) written.", "已生成 %d 份报告。", len(cl.rows))

	a.cmp.output = "/Users/ada/study/compare"
	a.cmp.name = "patients_vs_controls"
	a.cmp.how = Tf("GRF correction: voxel p < %g, cluster p < %g, clusters of at least %d voxels (|z| > %.2f).",
		"GRF 校正：体素 p < %g，团块 p < %g，团块至少 %d 个体素（|z| > %.2f）。", 0.001, 0.05, 187, 3.29)
	a.cmp.st.ok = Tf("%d cluster(s) survive the correction.", "有 %d 个团块通过校正。", 3)
	a.cmp.result = &core.GroupComparison{
		TMap: "/Users/ada/study/compare/patients_vs_controls_T.nii.gz", DOF: 26, FWHM: [3]float64{11.8, 12.4, 11.1},
		Corrected: true, NClusters: 3, ZThreshold: 3.29, MinClusterVoxels: 187,
		Files: []core.NamedFile{{Role: "thresholded", Path: "/Users/ada/study/compare/ClusterThresholded_patients_vs_controls_T.nii.gz"}},
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

	// The pictures, of a made-up patient.
	pics := demoPictures()
	show := func(pic *picture, key string, bitmap *ui.Bitmap) { pic.key, pic.bitmap = key, bitmap }
	show(&sp.pic, sp.rows[0].path, pics.warped)
	show(&a.ref.pic, a.ref.current, pics.mean)
	show(&a.zmap.pic, a.zmap.rows[0].path, pics.scores)
	show(&cl.pic, cl.rows[0].zmap, pics.scores)
	show(&a.cmp.pic, a.cmp.result.File("thresholded"), pics.tmap)
	show(&a.cluster.pic, a.cluster.saved, pics.cluster)
}

// demoPictures are the pictures of the snapshots: a made-up image, with
// the parietal cortex and the precuneus taking up less and the cerebellum
// more.
var demoPictures = sync.OnceValue(func() (pics struct{ warped, mean, scores, tmap, cluster *ui.Bitmap }) {
	img := core.TemplateImage()
	img.Data = core.Smooth(img.Data, img.Dim, img.Aff, 6)
	gray, _ := core.BuiltinMask("gray-matter")
	scores := core.NewVolume(img.Dim, img.Aff)
	blobs := []struct{ x, y, z, radius, peak float64 }{
		{-44, -62, 38, 20, -6}, {46, -60, 38, 17, -5}, {2, -54, 30, 18, -6.5}, {-52, -40, -8, 14, -3.6}, {0, -60, -30, 22, 3.4},
	}
	nx, ny := img.Dim[0], img.Dim[1]
	for i := range scores.Data {
		if gray.Data[i] == 0 {
			continue
		}
		x, y, z := float64(i%nx), float64(i/nx%ny), float64(i/(nx*ny))
		var at [3]float64
		for r := range at {
			at[r] = img.Aff[r][0]*x + img.Aff[r][1]*y + img.Aff[r][2]*z + img.Aff[r][3]
		}
		for _, b := range blobs {
			d2 := (at[0]-b.x)*(at[0]-b.x) + (at[1]-b.y)*(at[1]-b.y) + (at[2]-b.z)*(at[2]-b.z)
			scores.Data[i] += float32(b.peak * math.Exp(-d2/(b.radius*b.radius)))
		}
	}
	tmap := core.NewVolume(img.Dim, img.Aff)
	for i, v := range scores.Data {
		if v < -3 || v > 3 {
			tmap.Data[i] = 1.3 * v
		}
	}
	cerebellum, _ := core.BuiltinMask("cerebellar-gm")
	pics.warped = ui.NewBitmap(core.Picture(img, nil, core.PictureOptions{Outline: true}))
	pics.mean = ui.NewBitmap(core.Picture(img, nil, core.PictureOptions{}))
	pics.scores = ui.NewBitmap(core.Picture(img, scores, core.PictureOptions{}))
	pics.tmap = ui.NewBitmap(core.Picture(nil, tmap, core.PictureOptions{From: 1e-6, Full: 8}))
	pics.cluster = ui.NewBitmap(core.Picture(nil, cerebellum, core.PictureOptions{Solid: true}))
	return pics
})
