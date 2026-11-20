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
	// The page the app opens with, and the form of a new project.
	for _, dark := range []bool{false, true} {
		for _, creating := range []bool{false, true} {
			a := newApp()
			a.recent = []string{"/Users/ada/study", "/Users/ada/projects/msa-2025", "/Volumes/lab/pet/dlb-pilot"}
			a.seen = map[seenKey]seenFile{}
			for _, dir := range a.recent[:2] { // made up: nothing is asked of the disk
				a.seen[seenKey{dir, true}] = seenFile{true, time.Now().Add(time.Hour)}
				a.seen[seenKey{filepath.Join(dir, projectFileName), false}] = seenFile{true, time.Now().Add(time.Hour)}
			}
			a.seen[seenKey{a.recent[2], true}] = seenFile{false, time.Now().Add(time.Hour)}
			name := "welcome"
			if creating {
				a.welcome.creating, a.welcome.name, a.welcome.parent = true, "parkinsonism-2026", "/Users/ada/projects"
				a.seen[seenKey{"/Users/ada/projects/parkinsonism-2026/" + projectFileName, false}] = seenFile{false, time.Now().Add(time.Hour)}
				name = "welcome-new"
			}
			tt := ui.NewTesterWith(a.view, 1060, height, ui.TesterOptions{Dark: dark})
			tt.SetScale(2)
			tt.Frame()
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
	// A project of images in native space, all of them warped: every page
	// has something to show.
	a.proj = &project{dir: "/Users/ada/study", name: "Parkinsonism 2026", created: "2026-03-02T09:30:00+01:00", unsaved: true, native: true}
	a.projectOutputs()
	patient := func(i int) string { return fmt.Sprintf("/Users/ada/study/patients/sub-pt%02d_space-MNI_pet.nii.gz", i) }
	control := func(i int) string { return fmt.Sprintf("/Users/ada/study/controls/sub-hc%02d_space-MNI_pet.nii.gz", i) }
	sp := &a.spatial
	sp.spm, sp.matlab, sp.outDir = "/Users/ada/spm12", "/Applications/MATLAB_R2025b.app", a.proj.dir
	sp.warped = map[string]string{}
	warp := func(list *fileList, name string, mni string) {
		native := fmt.Sprintf("/Users/ada/study/raw/%s_pet.nii.gz", name)
		list.files = append(list.files, native)
		sp.warped[native] = mni
		sp.rows = append(sp.rows, spatialRow{image: filepath.Base(native), output: core.SpatialOutputName(native), path: mni})
		sp.written = append(sp.written, mni)
	}
	for i := 1; i <= 6; i++ {
		warp(&a.proj.patients, fmt.Sprintf("sub-pt%02d", i), patient(i))
	}
	for i := 1; i <= 16; i++ {
		warp(&a.proj.controls, fmt.Sprintf("sub-hc%02d", i), control(i))
	}
	sp.st.ok = Tf("%d image(s) warped to MNI space.", "已完成：%d 幅图像已配准到 MNI 空间。", 22)
	a.norm.methods["cerebellum"] = true
	a.setReference("/Users/ada/study/hc_reference")
	a.norm.outDir = a.norm.output
	for i, f := range [][2]string{{"5.81437", "5.42206"}, {"6.20915", "5.96384"}, {"4.97203", "4.51870"}, {"5.53866", "5.30129"}, {"6.74052", "6.18845"}, {"5.12790", "4.89317"}} {
		image := filepath.Base(patient(i + 1))
		hn, _ := strconv.ParseFloat(f[0], 64)
		cerebellum, _ := strconv.ParseFloat(f[1], 64)
		a.norm.rows = append(a.norm.rows,
			normRow{image: image, group: "patients", method: "hn", factor: f[0], voxels: "193204", value: hn},
			normRow{image: image, group: "patients", method: "cerebellum", factor: f[1], voxels: "22750", value: cerebellum})
	}
	// The controls are normalised with the patients: their factors lie
	// around the same value.
	for i := 1; i <= 16; i++ {
		image := filepath.Base(control(i))
		hn := 5.2 + 0.11*float64(i*7%13)
		cerebellum := hn * (0.93 + 0.004*float64(i*5%7))
		a.norm.rows = append(a.norm.rows,
			normRow{image: image, group: "controls", method: "hn", factor: strconv.FormatFloat(hn, 'g', 6, 64), voxels: "193204", value: hn},
			normRow{image: image, group: "controls", method: "cerebellum", factor: strconv.FormatFloat(cerebellum, 'g', 6, 64), voxels: "22750", value: cerebellum})
	}
	a.norm.st.ok = Tf("%d patient and %d control image(s) intensity-normalised with %d method(s).", "强度归一化已完成：%d 幅患者图像、%d 幅对照图像 × %d 种方法。", 6, 16, 2)
	// The steps of the individual analysis, each done on what is before it.
	patients, controls := a.images()
	a.norm.done = &normDone{asked: a.normImages(), files: a.normImages(), controls: controls, methods: a.norm.chosen(),
		reference: a.reference, settings: a.opt.normSettings(a.norm.chosen())}

	info := &refInfo{N: 16, FWHM: 8, Mask: "gray-matter", StatsFor: []string{"cerebellum", "white-matter"}}
	info.Covariates = []struct {
		Name string `json:"name"`
	}{{"age"}, {"sex"}}
	a.refRead.of, a.refRead.info = a.reference, info
	a.opt.RefStatsFor["cerebellum"], a.opt.RefStatsFor["white-matter"] = true, true
	a.cov.path = "/Users/ada/study/participants.tsv"
	a.cov.table = &core.Table{Path: a.cov.path, Header: []string{"participant_id", "age", "sex"}, Rows: make([]map[string]string, 22)}
	a.cov.adjust = map[string]bool{"age": true, "sex": true}
	a.cov.columns = a.cov.table.Columns()
	a.cov.stamp = stampOf(a.cov.path) // the table is made up: nothing is to be read
	// The reference is the one step 1 built from the controls.
	a.ref.built = &refBuilt{sig: a.refSignature(), statsFor: a.refStatsFor(), controls: map[string]fileStamp{}}
	for _, path := range controls {
		a.ref.built.controls[path] = fileStamp{}
	}
	a.norm.done.refSig = a.refFactorSignature()
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
	a.zmap.done = &zmapDone{from: a.norm.done, files: patients, methods: a.norm.chosen(), table: a.cov.path, refSig: a.refSignature()}

	cl := &a.classify
	cl.outDir = cl.output
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

	a.cluster.saved, a.cluster.volume = a.cluster.output, 58
	a.cluster.st.ok = Tf("Reference cluster of %.0f mL saved.", "已保存参考簇（%.0f mL）。", 58.0)

	// The pictures, of a made-up patient.
	pics := demoPictures()
	show := func(pic *picture, key string, bitmap *ui.Bitmap) { pic.key, pic.bitmap = key, bitmap }
	show(&sp.pic, sp.rows[0].path, pics.warped)
	show(&a.ref.pic, a.reference, pics.mean)
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
