package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

// makeImages writes made-up FDG-PET images in MNI space: controls, and
// patients whose cortex takes up 15% less than their cerebellum.  It returns
// their paths and a table of their age and sex.
func makeImages(t *testing.T, dir string, controls, patients int) (hc, pt []string, table string) {
	t.Helper()
	dim, aff := core.AtlasGrid()
	mask := func(name string) []float32 {
		vol, err := core.BuiltinMask(name)
		if err != nil {
			t.Fatal(err)
		}
		return vol.Data
	}
	brain, gray, cerebellum := mask("parenchyma"), mask("gray-matter"), mask("cerebellum")
	rng := rand.New(rand.NewSource(1))
	rows := "participant_id\tage\tsex\n"
	write := func(name string, patient bool) string {
		scale := float32(4 + 6*rng.Float64()) // the dose and the weight differ between people
		data := make([]float32, len(brain))
		for i := range data {
			if brain[i] == 0 {
				continue
			}
			v := float32(3)
			if gray[i] > 0 {
				v = 6
			}
			if patient && cerebellum[i] == 0 {
				v *= 0.85
			}
			data[i] = scale * v * float32(1+0.05*rng.NormFloat64())
		}
		path := filepath.Join(dir, name+"_pet.nii.gz")
		if err := core.SaveNifti(path, &core.Volume{Dim: dim, NT: 1, Data: data, Aff: aff}, false); err != nil {
			t.Fatal(err)
		}
		rows += fmt.Sprintf("%s\t%d\t%s\n", name, 50+rng.Intn(30), []string{"F", "M"}[rng.Intn(2)])
		return path
	}
	for i := range controls {
		hc = append(hc, write(fmt.Sprintf("sub-hc%02d", i), false))
	}
	for i := range patients {
		pt = append(pt, write(fmt.Sprintf("sub-pt%02d", i), true))
	}
	table = filepath.Join(dir, "participants.tsv")
	if err := os.WriteFile(table, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	return hc, pt, table
}

// TestWorkflow goes through the app as a user does, pressing its buttons:
// build a reference, normalise, make z-score maps, classify patients,
// compare two groups and find a reference cluster.
func TestWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("works on whole images")
	}
	chinese = true
	defer func() { chinese = false }()
	dir := t.TempDir()
	hc, pt, table := makeImages(t, filepath.Join(dir, "data"), 12, 8)

	a := newApp()
	queue := make(chan func(), 256)
	a.post = func(fn func()) { queue <- fn }
	tt := ui.NewTesterWith(a.view, 1060, 740, ui.TesterOptions{})
	wait := func() {
		t.Helper()
		timeout := time.After(10 * time.Minute)
		for a.job != nil {
			select {
			case fn := <-queue:
				fn()
				tt.Frame()
			case <-timeout:
				t.Fatal("the task did not finish")
			}
		}
		tt.Frame()
	}
	press := func(page, button string, st *status) {
		t.Helper()
		a.page = page
		tt.Frame()
		if err := tt.Click(button); err != nil {
			t.Fatalf("%s: %v (shown: %q)", page, err, tt.Texts())
		}
		tt.Frame()
		if a.job == nil && st.ok == "" {
			t.Fatalf("%s: nothing started: %s", page, st.err)
		}
		wait()
		tt.Advance(time.Second) // the results glide into view
		if st.err != "" {
			t.Fatalf("%s: %s", page, st.err)
		}
		if st.ok == "" || !tt.HasText(st.ok) {
			t.Fatalf("%s: no message of success shown", page)
		}
	}
	// drawn waits for the picture of a page's results, which is made in the
	// background from the files the run wrote.
	drawn := func(pic *picture) {
		t.Helper()
		tt.Frame()
		timeout := time.After(time.Minute)
		for pic.loading {
			select {
			case fn := <-queue:
				fn()
				tt.Frame()
			case <-timeout:
				t.Fatal("the picture was not drawn")
			}
		}
		if pic.bitmap == nil {
			t.Fatalf("no picture of %s: %s", pic.key, pic.err)
		}
	}
	exists := func(path string) {
		t.Helper()
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}

	// The reference, with the cerebellum's statistics and W-scores.
	ref := filepath.Join(dir, "reference")
	a.ref.files.files, a.ref.output = hc, ref
	a.opt.RefStatsFor["cerebellum"] = true
	a.ref.cov.path = table
	a.ref.cov.load()
	press("reference", "建立参考库", &a.ref.st)
	exists(filepath.Join(ref, "reference.json"))
	if a.norm.reference != ref || a.cmp.reference != ref {
		t.Fatal("the new reference is not the one in use")
	}
	tt.Frame()
	if !tt.HasText("age、sex") {
		t.Fatalf("the reference's covariates are not shown: %q", tt.Texts())
	}
	drawn(&a.ref.pic)

	// Normalisation by two methods.
	out := filepath.Join(dir, "normalised")
	a.norm.files.files, a.norm.output = pt, out
	a.norm.methods = map[string]bool{"hn": true, "cerebellum": true}
	press("normalize", "开始强度归一化", &a.norm.st)
	if len(a.norm.rows) != 2*len(pt) {
		t.Fatalf("%d rows of factors, want %d", len(a.norm.rows), 2*len(pt))
	}
	exists(filepath.Join(out, "factors.tsv"))
	exists(filepath.Join(out, core.OutputName(pt[0], "hn", "pet")))

	// Z-score and W-score maps.
	zdir := filepath.Join(dir, "zmaps")
	a.zmap.files.files, a.zmap.output, a.zmap.covTable = pt[:3], zdir, table
	a.zmap.methods = map[string]bool{"hn": true, "cerebellum": true}
	press("zmap", "生成 Z 值图", &a.zmap.st)
	if len(a.zmap.rows) != 6 {
		t.Fatalf("%d z-score maps, want 6", len(a.zmap.rows))
	}
	maps, _ := filepath.Glob(filepath.Join(zdir, "*.nii.gz"))
	if len(maps) != 6 {
		t.Fatalf("%d files in %s, want 6", len(maps), zdir)
	}
	drawn(&a.zmap.pic)

	// A report for each of two patients, in the language of the app.
	reports := filepath.Join(dir, "reports")
	a.classify.files.files, a.classify.output, a.classify.covTable = pt[:2], reports, table
	press("classify", "开始分类", &a.classify.st)
	if len(a.classify.rows) != 2 || a.classify.rows[0].reading == "" {
		t.Fatalf("classified %+v, notes %v", a.classify.rows, a.classify.notes)
	}
	page, err := os.ReadFile(a.classify.rows[1].report)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<!doctype html>", "帕金森综合征代谢模式分析报告", "W 值", "data:image/png;base64,", filepath.Base(pt[1])} {
		if !strings.Contains(string(page), want) {
			t.Fatalf("the report lacks %q", want)
		}
	}
	summary, err := os.ReadFile(filepath.Join(reports, "classification.tsv"))
	if err != nil || len(strings.Split(strings.TrimSpace(string(summary)), "\n")) != 3 {
		t.Fatalf("classification.tsv: %v, %q", err, summary)
	}
	exists(filepath.Join(reports, "sub-pt00_classification.json"))
	exists(filepath.Join(reports, "sub-pt00_desc-hn_wmap.nii.gz"))
	if !tt.HasText("打开报告") {
		t.Fatalf("no button opens the report: %q", tt.Texts())
	}
	drawn(&a.classify.pic)
	if got := a.classify.rows[0].scores; len(got) != len(core.Patterns) {
		t.Fatalf("pattern scores %v", got)
	}
	// The cerebellum's statistics are in the reference, so the map can rest on it.
	if got := a.classify.methods(); got[len(got)-1] != "cerebellum" || len(a.classify.checks("hn")) != 1 {
		t.Fatalf("methods %v, checks %v", got, a.classify.checks("hn"))
	}

	// The patients against the controls: the cortex takes up less.
	cmp := filepath.Join(dir, "compare")
	a.cmp.g1.files, a.cmp.g2.files, a.cmp.output, a.cmp.method = pt, hc, cmp, "cerebellum"
	a.cmp.cov.path = table
	a.cmp.cov.load()
	press("compare", "开始比较", &a.cmp.st)
	res := a.cmp.result
	if res == nil || !res.Corrected || res.NClusters == 0 || len(res.Clusters) == 0 {
		t.Fatalf("no clusters found: %+v", res)
	}
	if res.Clusters[0].Peak >= 0 {
		t.Fatalf("the largest cluster has peak %g, want hypometabolism", res.Clusters[0].Peak)
	}
	if res.DOF != float64(len(pt)+len(hc)-4) {
		t.Fatalf("dof = %g with two covariates", res.DOF)
	}
	exists(res.TMap)
	exists(res.File("cluster_report"))
	drawn(&a.cmp.pic)
	if a.cmp.pic.key != res.File("thresholded") {
		t.Fatalf("the picture is of %q, want the thresholded map", a.cmp.pic.key)
	}
	// A second run writes files of the same names: the picture is not the
	// one of the first run.
	first := a.cmp.pic.bitmap
	press("compare", "开始比较", &a.cmp.st)
	drawn(&a.cmp.pic)
	if a.cmp.pic.bitmap == first || a.cmp.pic.key != a.cmp.result.File("thresholded") {
		t.Fatal("the picture of the first run is shown for the second")
	}

	// The reference cluster: the cerebellum, which the disease spares.
	mask := filepath.Join(dir, "cluster.nii.gz")
	a.cluster.files.files, a.cluster.output = pt, mask
	press("cluster", "查找参考簇", &a.cluster.st)
	if a.cluster.volume < 20 {
		t.Fatalf("the reference cluster has %g mL", a.cluster.volume)
	}
	exists(mask)
	drawn(&a.cluster.pic)
	tt.Scroll(640, 400, 0, 900)
	tt.Frame()
	if err := tt.Click("用该区域做强度归一化"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != "normalize" || a.norm.region.file != mask || !a.norm.methods["region"] {
		t.Fatal("the cluster was not handed to the Intensity Normalisation page")
	}
	press("normalize", "开始强度归一化", &a.norm.st)
	if len(a.norm.rows) != len(pt) || a.norm.rows[0].voxels == "" || a.norm.rows[0].mask != "custom" {
		t.Fatalf("normalising by the cluster gave %+v", a.norm.rows)
	}

	// The built-in masks.
	a.page = "masks"
	a.exportMasks([]string{filepath.Join(dir, "masks")})
	wait()
	exported, _ := filepath.Glob(filepath.Join(dir, "masks", "*.nii.gz"))
	if len(exported) != len(core.Masks) {
		t.Fatalf("%d masks exported, want %d", len(exported), len(core.Masks))
	}
}

// TestCovariateTable fills in a table of covariates after it was chosen, as
// a participants.tsv made with only its header is: the page shows the file as
// it is now and keeps the ticks of the columns that are still there, and a
// run reads the file anew.
func TestCovariateTable(t *testing.T) {
	if testing.Short() {
		t.Skip("works on whole images")
	}
	chinese = true
	defer func() { chinese = false }()
	dir := t.TempDir()
	hc, pt, _ := makeImages(t, filepath.Join(dir, "data"), 6, 3)
	var people []string
	for _, image := range append(append([]string(nil), hc...), pt...) {
		people = append(people, strings.TrimSuffix(filepath.Base(image), "_pet.nii.gz"))
	}
	// but lists everyone except one person.
	but := func(skip string) []string {
		return slices.DeleteFunc(slices.Clone(people), func(id string) bool { return id == skip })
	}
	table := filepath.Join(dir, "covariates.tsv")
	write := func(columns []string, ids []string) {
		t.Helper()
		text := "participant_id\t" + strings.Join(columns, "\t") + "\n"
		for i, id := range ids {
			text += id
			for _, col := range columns {
				text += "\t" + map[string]string{
					"age":  fmt.Sprint(50 + i*7%23),
					"sex":  []string{"F", "M"}[i%2],
					"site": []string{"A", "B"}[i/2%2],
					"bmi":  fmt.Sprint(20.5 + float64(i*5%11)),
				}[col]
			}
			text += "\n"
		}
		if err := os.WriteFile(table, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	a := newApp()
	queue := make(chan func(), 256)
	a.post = func(fn func()) { queue <- fn }
	tt := ui.NewTesterWith(a.view, 1060, 740, ui.TesterOptions{})
	click := func(label string) {
		t.Helper()
		tt.Frame()
		if err := tt.Click(label); err != nil {
			t.Fatalf("%v (shown: %q)", err, tt.Texts())
		}
		tt.Frame()
	}
	// later is the user coming back to the page after a while.
	later := func() {
		tt.Advance(2 * covCheckEvery)
		tt.Frame()
	}
	// run presses the button of the page and waits for the work to end.
	run := func(button string, st *status) {
		t.Helper()
		click(button)
		timeout := time.After(5 * time.Minute)
		for a.job != nil {
			select {
			case fn := <-queue:
				fn()
				tt.Frame()
			case <-timeout:
				t.Fatal("the task did not finish")
			}
		}
		tt.Frame()
		if st.err != "" || st.ok == "" {
			t.Fatalf("the run failed: %q", st.err)
		}
	}
	// refused presses the button and expects the run not to start.
	refused := func(button string, st *status, why string) {
		t.Helper()
		click(button)
		if a.job != nil || !strings.Contains(st.err, why) || !tt.HasText(why) {
			t.Fatalf("the run was not refused with %q: job %v, %q", why, a.job != nil, st.err)
		}
	}
	ticks := func(cv *covChoice, want map[string]bool) {
		t.Helper()
		if fmt.Sprint(cv.adjust) != fmt.Sprint(want) {
			t.Fatalf("ticks %v, want %v", cv.adjust, want)
		}
	}

	// The table as first chosen: other people, and a column for the site.
	cv := &a.ref.cov
	write([]string{"age", "sex", "site"}, []string{"sub-xx00", "sub-xx01", "sub-xx02", "sub-xx03"})
	a.page, a.ref.files.files, a.ref.output = "reference", hc, filepath.Join(dir, "reference")
	cv.path = table
	cv.load()
	tt.Frame()
	tt.Scroll(640, 400, 0, 900) // down to the table
	click("sex")
	click("site")
	ticks(cv, map[string]bool{"age": true, "sex": false, "site": true})
	if !tt.HasText("共 4 行") {
		t.Fatalf("the rows are not counted: %q", tt.Texts())
	}

	// Filled in with another program: the page shows the new rows and the
	// new column; age and sex keep their ticks, that of the site is gone.
	write([]string{"age", "sex", "bmi"}, but("sub-hc05"))
	tt.Frame()
	if len(cv.table.Rows) != 4 {
		t.Fatal("the file is read on every frame")
	}
	later()
	ticks(cv, map[string]bool{"age": true, "sex": false, "bmi": false})
	if _, shown := tt.Find("site"); shown || !tt.HasText("bmi") || !tt.HasText(fmt.Sprintf("共 %d 行", len(people)-1)) {
		t.Fatalf("the page does not show the table as it is now: %q", tt.Texts())
	}
	click("bmi")

	// The last control is added just before the run, and the columns put
	// in another order, which the page has not seen: the run reads the
	// file itself.
	write([]string{"bmi", "sex", "age"}, people)
	tt.Frame()
	if len(cv.table.Rows) != len(people)-1 {
		t.Fatal("the page has read the file already")
	}
	run("建立参考库", &a.ref.st)
	if got := readRefInfo(a.ref.output).covariateNames(); fmt.Sprint(got) != "[bmi age]" {
		t.Fatalf("the reference adjusts for %v", got)
	}
	ticks(cv, map[string]bool{"age": true, "sex": false, "bmi": true})

	// A ticked column leaves the file before the page shows it: the run
	// does not go on with fewer columns than were ticked.
	write([]string{"age", "sex"}, people)
	refused("建立参考库", &a.ref.st, "列已发生变化")
	ticks(cv, map[string]bool{"age": true, "sex": false})

	// The file is gone: the run does not use the rows it read before, and
	// the ticks wait for the file to be back.
	if err := os.Remove(table); err != nil {
		t.Fatal(err)
	}
	refused("建立参考库", &a.ref.st, "无法读取协变量表")
	later()
	if cv.table != nil || cv.err == "" || !tt.HasText(cv.err) {
		t.Fatalf("the page does not say that the file is gone: %q, %q", cv.err, tt.Texts())
	}
	write([]string{"sex", "age", "bmi"}, people)
	later()
	ticks(cv, map[string]bool{"age": true, "sex": false, "bmi": false})

	// A file that could not be read is tried again though it looks the
	// same, as when the permission to read it is given back: here a table
	// without rows is mended at the same size and time.
	rewrite := func(text string) {
		t.Helper()
		if err := os.WriteFile(table, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rewrite("participant_id\tage\tsub-xx00\t61\n")
	later()
	if cv.table != nil || cv.err == "" {
		t.Fatal("a table without rows was read")
	}
	stamp := cv.stamp
	rewrite("participant_id\tage\nsub-xx00\t61\n")
	if err := os.Chtimes(table, time.Time{}, time.Unix(0, stamp.modified)); err != nil {
		t.Fatal(err)
	}
	if stampOf(table) != stamp {
		t.Fatal("the mended file does not look as it did")
	}
	later()
	if cv.table == nil {
		t.Fatalf("the file was not read again: %s", cv.err)
	}
	ticks(cv, map[string]bool{"age": true})

	// The Compare Groups page keeps its table the same way.
	cv = &a.cmp.cov
	write([]string{"age", "sex"}, but("sub-pt02"))
	a.page, a.cmp.g1.files, a.cmp.g2.files, a.cmp.output, a.cmp.method = "compare", pt, hc, filepath.Join(dir, "compare"), "cerebellum"
	cv.path = table
	cv.load()
	tt.Frame()
	tt.Scroll(640, 400, 0, 900)
	click("sex")
	write([]string{"age", "sex", "bmi"}, people)
	tt.Frame()
	if len(cv.table.Rows) != len(people)-1 {
		t.Fatal("the page has read the file already")
	}
	run("开始比较", &a.cmp.st)
	if res := a.cmp.result; res == nil || res.DOF != float64(len(people)-3) {
		t.Fatalf("the comparison is not adjusted for age alone: %+v", res)
	}
	ticks(cv, map[string]bool{"age": true, "sex": false, "bmi": false})

	// The made-up table of the screenshots has no file, and stays.
	a = newApp()
	a.demo()
	a.page = "reference"
	tt = ui.NewTesterWith(a.view, 1060, 740, ui.TesterOptions{})
	later()
	if a.ref.cov.table == nil || !tt.HasText("共 28 行") {
		t.Fatalf("the table of the screenshots is gone: %q", tt.Texts())
	}
}

// fakeSPM makes a folder that passes for SPM and a program that passes for
// MATLAB: it "warps" the images of the script it is given by copying them.
func fakeSPM(t *testing.T, dir string) (spm, matlab string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in for MATLAB is a shell script")
	}
	spm = filepath.Join(dir, "spm12")
	if err := os.MkdirAll(filepath.Join(spm, "toolbox", "OldNorm"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"spm_jobman.m", "toolbox/OldNorm/PET.nii"} {
		if err := os.WriteFile(filepath.Join(spm, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	matlab = filepath.Join(dir, "matlab")
	script := `#!/bin/sh
script=$(printf '%s' "$2" | sed "s/^run('//; s/')$//")
grep "^'.*,1'$" "$script" | sed "s/^'//; s/,1'$//" | while read -r f; do
  case "$f" in
    *img00001.nii) echo "FAILED $f,1: the image would not fit" >&2 ;;
    *) cp "$f" "$(dirname "$f")/w$(basename "$f")" ;;
  esac
done
`
	if err := os.WriteFile(matlab, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return spm, matlab
}

// TestSpatial warps images on the Spatial Normalisation page, with a
// stand-in for MATLAB, and hands them to the Intensity Normalisation page.
func TestSpatial(t *testing.T) {
	dir := t.TempDir()
	spm, matlab := fakeSPM(t, dir)
	_, pt, _ := makeImages(t, filepath.Join(dir, "raw"), 0, 3)

	a := newApp()
	queue := make(chan func(), 256)
	a.post = func(fn func()) { queue <- fn }
	a.page = "spatial"
	tt := ui.NewTesterWith(a.view, 1060, 740, ui.TesterOptions{})
	tt.Frame()
	p := &a.spatial
	out := filepath.Join(dir, "mni")
	p.files.files, p.output, p.matlab, a.opt.Jobs = pt, out, matlab, 2
	p.setSPM(dir)
	if p.problem() == "" {
		t.Fatal("a folder that is not SPM was taken for SPM")
	}
	p.setSPM(spm)
	if msg := p.problem(); msg != "" {
		t.Fatal(msg)
	}
	tt.Frame()
	if err := tt.Click("Warp to MNI Space"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	timeout := time.After(2 * time.Minute)
	for a.job != nil {
		select {
		case fn := <-queue:
			fn()
			tt.Frame()
		case <-timeout:
			t.Fatal("the task did not finish")
		}
	}
	tt.Advance(time.Second)
	if len(p.rows) != 3 || len(p.written) != 2 || !p.rows[1].failed || len(p.notes) != 1 {
		t.Fatalf("rows %+v, written %v, notes %v, status %+v", p.rows, p.written, p.notes, p.st)
	}
	if want := "sub-pt01_pet.nii.gz: SPM: the image would not fit"; p.notes[0] != want {
		t.Fatalf("note %q, want %q", p.notes[0], want)
	}
	if want := filepath.Join(out, "sub-pt00_space-MNI_pet.nii.gz"); p.written[0] != want {
		t.Fatalf("written %q, want %q", p.written[0], want)
	}
	// The stand-in does not warp: the image is the prepared one, cropped
	// around the head with the brain's centre moved to that of MNI space.
	vol, err := core.LoadImage(p.written[0])
	if err != nil {
		t.Fatal(err)
	}
	dim, _ := core.AtlasGrid()
	if vol.Dim != dim { // the made-up head is smaller than the box kept around it
		t.Fatalf("prepared image of %v voxels, want %v", vol.Dim, dim)
	}
	var sum [3]float64
	n := 0.0
	for i, v := range vol.Data {
		if v > 0 {
			x, y, z := float64(i%dim[0]), float64(i/dim[0]%dim[1]), float64(i/(dim[0]*dim[1]))
			for r := range sum {
				sum[r] += vol.Aff[r][0]*x + vol.Aff[r][1]*y + vol.Aff[r][2]*z + vol.Aff[r][3]
			}
			n++
		}
	}
	for r, want := range []float64{0, -18, 8} {
		if got := sum[r] / n; got < want-6 || got > want+6 {
			t.Fatalf("the brain's centre is at %g mm on axis %d, want about %g", got, r, want)
		}
	}
	// The picture of the first image that was warped, then, further down the
	// results, the button that hands them on.
	if p.shown != 0 || p.pic.key != p.written[0] {
		t.Fatalf("the picture is of row %d, %q", p.shown, p.pic.key)
	}
	tt.Scroll(640, 400, 0, 900)
	tt.Frame()
	if err := tt.Click("Use for Intensity Normalisation"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != "normalize" || len(a.norm.files.files) != 2 || a.norm.files.files[0] != p.written[0] {
		t.Fatalf("the warped images were not handed on: page %s, files %v", a.page, a.norm.files.files)
	}

	// Two images that would be written under one name.
	a.page = "spatial"
	other := filepath.Join(dir, "other", filepath.Base(pt[0]))
	p.files.files = []string{pt[0], other}
	if p.problem() == "" {
		t.Fatal("two images with one output name were accepted")
	}
}

// TestPages draws every page, empty and filled, in both languages and both
// appearances.
func TestPages(t *testing.T) {
	defer func() { chinese = false }()
	for _, chinese = range []bool{false, true} {
		for _, filled := range []bool{false, true} {
			for _, dark := range []bool{false, true} {
				for _, p := range pages {
					a := newApp()
					if filled {
						a.demo()
					}
					a.page = p.id
					tt := ui.NewTesterWith(a.view, 860, 560, ui.TesterOptions{Dark: dark})
					tt.Frame()
					title := T(p.en, p.zh)
					if !tt.HasText(title) {
						t.Errorf("page %s does not show %q", p.id, title)
					}
				}
			}
		}
	}
}

// TestCompareSmoothing checks that, left to the app, the smoothing before
// the test is that of the chosen reference, also for a method that does not
// need the reference.
func TestCompareSmoothing(t *testing.T) {
	ref := t.TempDir()
	write := func(fwhm float64) {
		t.Helper()
		raw := fmt.Sprintf(`{"n_controls": 12, "fwhm_mm": %g}`, fwhm)
		if err := os.WriteFile(filepath.Join(ref, "reference.json"), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := newApp()
	p := &a.cmp
	for _, p.method = range []string{"cerebellum", "hn"} {
		a.setReference("")
		if got := p.smoothing(&a.opt); got != 8 {
			t.Errorf("%s without a reference: %g mm, want 8", p.method, got)
		}
		write(6)
		a.setReference(ref)
		if got := p.smoothing(&a.opt); got != 6 {
			t.Errorf("%s with a reference smoothed by 6 mm: %g mm", p.method, got)
		}
		write(0)
		a.setReference(ref) // as after building it anew
		if got := p.smoothing(&a.opt); got != 0 {
			t.Errorf("%s with an unsmoothed reference: %g mm", p.method, got)
		}
		a.opt.CmpFWHMAuto, a.opt.CmpFWHM = false, 10
		if got := p.smoothing(&a.opt); got != 10 {
			t.Errorf("%s with 10 mm asked for: %g mm", p.method, got)
		}
		a.opt.CmpFWHMAuto = true
	}
	a.setReference(filepath.Join(ref, "missing"))
	if got := p.smoothing(&a.opt); got != 8 {
		t.Errorf("with a reference that cannot be read: %g mm, want 8", got)
	}
}

// TestSettings draws every section of the settings window, in both
// languages and both appearances, and changes options in it as a user does.
func TestSettings(t *testing.T) {
	defer func() { chinese = false }()
	for _, chinese = range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			for _, s := range settingsSections {
				a := newApp()
				a.demo()
				a.settingsPage = s.id
				tt := ui.NewTesterWith(a.settingsView, settingsWidth, settingsHeight, ui.TesterOptions{Dark: dark})
				tt.Frame()
				if title := s.title.String(); !tt.HasText(title) {
					t.Errorf("section %s does not show %q", s.id, title)
				}
			}
		}
	}
	chinese = false

	a := newApp()
	a.openSettings("compare") // without a window it only chooses the section
	if a.settingsPage != "compare" {
		t.Fatalf("section %q, want compare", a.settingsPage)
	}
	tt := ui.NewTesterWith(a.settingsView, settingsWidth, settingsHeight, ui.TesterOptions{})
	tt.Frame()
	if err := tt.Click("FDR"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.opt.Correction != 1 || !tt.HasText("q value") {
		t.Fatalf("correction %d after choosing FDR; shown %q", a.opt.Correction, tt.Texts())
	}
	if got := a.opt.changed(groupCompare); len(got) != 1 {
		t.Fatalf("changed %v, want the correction alone", got)
	}
	if err := tt.Click("Restore Defaults"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.opt.Correction != 0 || len(a.opt.changed(groupCompare)) != 0 {
		t.Fatalf("the defaults were not restored: %+v", a.opt)
	}

	// The language, from the first section.
	a.settingsPage = "general"
	tt.Frame()
	if err := tt.Click("中文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !chinese || !tt.HasText("软件更新") {
		t.Fatalf("the app does not speak Chinese: %q", tt.Texts())
	}
	chinese = false
}

// TestOptionsKept checks that the options survive a launch, that a file of
// an older version leaves the rest as the app first has it, and that
// values no run could use are not taken.
func TestOptionsKept(t *testing.T) {
	a := newApp()
	a.opt.Target, a.opt.Statistic, a.opt.Correction, a.opt.Q = 50, 1, 1, 0.01
	a.opt.RefStatsFor["white-matter"] = true
	text := a.settingsText()

	opt := defaultOptions()
	s := settings{Options: &opt}
	if err := json.Unmarshal([]byte(text), &s); err != nil {
		t.Fatal(err)
	}
	opt.sound()
	if opt.Target != 50 || opt.Statistic != 1 || opt.Correction != 1 || opt.Q != 0.01 || !opt.RefStatsFor["white-matter"] {
		t.Fatalf("read back %+v", opt)
	}
	if got := opt.changed(groupNormalize, groupCompare, groupReference); len(got) != 5 {
		t.Fatalf("changed %v, want five options", got)
	}
	opt.restore(groupNormalize, groupCompare, groupReference)
	if got := opt.changed(groupNormalize, groupCompare, groupReference, groupIterative, groupSpatial, groupClassify, groupCluster); len(got) != 0 {
		t.Fatalf("still changed after restoring: %v", got)
	}

	// A file written before the options were kept, and one with nonsense.
	opt = defaultOptions()
	s = settings{Options: &opt}
	if err := json.Unmarshal([]byte(`{"language": "zh", "options": {"target": -3, "reference_mask": "nowhere", "voxel_p": 0, "classify_method": "x"}}`), &s); err != nil {
		t.Fatal(err)
	}
	opt.sound()
	d := defaultOptions()
	if opt.Target != d.Target || opt.RefMask != d.RefMask || opt.VoxelP != d.VoxelP || opt.ClassMethod != d.ClassMethod || opt.FWHM != d.FWHM || !opt.Images {
		t.Fatalf("unsound options were taken: %+v", opt)
	}

	// Every page that has options says which are in effect.
	for _, p := range pages {
		if p.id == "study" || p.id == "masks" {
			continue
		}
		if d.summary(p.id) == "" {
			t.Errorf("page %s has no summary of its options", p.id)
		}
	}
}

// TestPicture draws an image with a map over it.
func TestPicture(t *testing.T) {
	img := core.TemplateImage()
	scores := core.NewVolume(img.Dim, img.Aff)
	centre := img.Dim[0]/2 + img.Dim[0]*(img.Dim[1]/2+img.Dim[1]*40)
	scores.Data[centre], scores.Data[centre+3] = -6, 6
	pic := core.Picture(img, scores, core.PictureOptions{})
	bounds := pic.Bounds()
	if bounds.Dx() < 5*img.Dim[0]*2 || bounds.Dy() < 2*img.Dim[1]*2 {
		t.Fatalf("picture of %v", bounds)
	}
	cool, warm, gray := 0, 0, 0
	for i := 0; i < len(pic.Pix); i += 4 {
		r, g, b := pic.Pix[i], pic.Pix[i+1], pic.Pix[i+2]
		switch {
		case r == 0 && b == 255:
			cool++
		case r == 255 && b == 0:
			warm++
		case r == g && g == b && r > 60:
			gray++
		}
	}
	if cool == 0 || warm == 0 || gray < 1000 {
		t.Fatalf("%d cool, %d warm and %d gray pixels", cool, warm, gray)
	}
	mask, err := core.BuiltinMask("cerebellum")
	if err != nil {
		t.Fatal(err)
	}
	if got := core.Picture(nil, mask, core.PictureOptions{Solid: true, Outline: true}).Bounds(); got != bounds {
		t.Fatalf("a mask on the template is %v, the image %v", got, bounds)
	}
}

// TestStudy lays out a study folder with the button of the page, twice:
// what is there the second time is kept.
func TestStudy(t *testing.T) {
	// The folder itself: the pages are given no links.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp()
	a.page = "study"
	tt := ui.NewTesterWith(a.view, 1060, 900, ui.TesterOptions{})
	tt.Frame()
	if tt.HasText("Created") || a.study.problem() == "" {
		t.Fatal("the page is ready before a place is chosen")
	}
	a.study.parent = dir
	tt.Frame()
	if err := tt.Click("Create Folders"); err != nil {
		t.Fatalf("%v (shown: %q)", err, tt.Texts())
	}
	tt.Frame()
	study := filepath.Join(dir, "study")
	for _, e := range core.StudyLayout {
		if info, err := os.Stat(filepath.Join(study, e.Name)); err != nil || info.IsDir() != e.Dir {
			t.Errorf("%s was not created", e.Name)
		}
	}
	if a.study.st.err != "" || !tt.HasText(a.study.st.ok) || !tt.HasText("Created") || tt.HasText("Already there") {
		t.Fatalf("after creating: %q, shown: %q", a.study.st, tt.Texts())
	}
	// The pages now write into the study; Spatial Normalisation is left alone.
	outputs := map[string]string{"hc_reference": a.ref.output, "normalised": a.norm.output, "zmaps": a.zmap.output,
		"reports": a.classify.output, "compare": a.cmp.output, "reference_cluster.nii.gz": a.cluster.output}
	for name, got := range outputs {
		if got != filepath.Join(study, name) {
			t.Errorf("the output for %s is %q", name, got)
		}
	}
	if a.spatial.output != "" || a.norm.reference != "" {
		t.Errorf("set although the study says nothing of it: %q, %q", a.spatial.output, a.norm.reference)
	}
	// The study is kept between launches, and used again while it is there.
	var kept settings
	if err := json.Unmarshal([]byte(a.settingsText()), &kept); err != nil || kept.Study != study {
		t.Fatalf("kept the study as %q (%v)", kept.Study, err)
	}
	// The reference is the one the app kept, also when that is none: the
	// study's own is not put in its place.
	os.WriteFile(filepath.Join(study, "hc_reference", "reference.json"), []byte("{}"), 0o644)
	b := newApp()
	b.restoreStudy(kept.Study)
	if b.zmap.output != filepath.Join(study, "zmaps") || b.study.parent != dir || b.study.name != "study" || b.norm.reference != "" {
		t.Errorf("after a new launch: %q, %q, %q, %q", b.zmap.output, b.study.parent, b.study.name, b.norm.reference)
	}
	// A study that cannot be reached is not used, but kept in mind: it is
	// used again once it can be, and what the settings hold of it goes when
	// another study is used.
	gone := filepath.Join(dir, "gone")
	b = newApp()
	b.restoreStudy(gone)
	if b.zmap.output != "" || b.study.inUse != "" {
		t.Error("a study folder that is gone is used")
	}
	if err := json.Unmarshal([]byte(b.settingsText()), &kept); err != nil || kept.Study != gone {
		t.Errorf("a study that cannot be reached is forgotten: kept %q (%v)", kept.Study, err)
	}
	b.opt.RefMaskFile, b.opt.AnalysisFile = filepath.Join(gone, "mask.nii.gz"), filepath.Join(dir, "elsewhere", "mask.nii.gz")
	b.useStudy(study)
	if b.opt.RefMaskFile != "" || b.opt.AnalysisFile == "" || b.study.last != "" {
		t.Errorf("after leaving a study that cannot be reached: masks %q (its own), %q (from elsewhere)", b.opt.RefMaskFile, b.opt.AnalysisFile)
	}
	os.Remove(filepath.Join(study, "hc_reference", "reference.json"))
	table := filepath.Join(study, "participants.tsv")
	os.WriteFile(table, []byte(core.ParticipantsHeader+"sub-01\t60\tF\n"), 0o644)
	if err := tt.Click("Create Folders"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if text, _ := os.ReadFile(table); !strings.Contains(string(text), "sub-01") {
		t.Error("the table that was filled was written over")
	}
	if !tt.HasText("Already there") || tt.HasText("Created") {
		t.Errorf("after creating again, shown: %q", tt.Texts())
	}
	// Another name is another folder: nothing is said of it yet, and the
	// page says where the pages still write.
	ready := a.study.st.ok
	a.study.name = "other"
	tt.Frame()
	if tt.HasText("Already there") || tt.HasText(ready) || a.study.st.ok != "" {
		t.Errorf("a folder not laid out yet is spoken of as ready, shown: %q", tt.Texts())
	}
	if a.zmap.output != filepath.Join(study, "zmaps") || !strings.Contains(strings.Join(tt.Texts(), "\n"), "The pages still write into "+shortDir(study)+".") {
		t.Errorf("not said where the pages still write, shown: %q", tt.Texts())
	}
	// With no folder chosen, it is still said where the pages write.
	a.study.name = ""
	tt.Frame()
	if !tt.HasText("The pages write into " + shortDir(study) + ".") {
		t.Errorf("with no folder chosen, not said where the pages write, shown: %q", tt.Texts())
	}
	// A run that fails takes back what an earlier run of the folder showed.
	a.study.name = "study"
	tt.Frame()
	os.Remove(filepath.Join(study, "zmaps"))
	os.WriteFile(filepath.Join(study, "zmaps"), nil, 0o644)
	if err := tt.Click("Create Folders"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.study.st.err == "" || tt.HasText("Already there") || tt.HasText("Created") {
		t.Errorf("after a run that failed (%q), shown: %q", a.study.st.err, tt.Texts())
	}
	os.Remove(filepath.Join(study, "zmaps"))

	// The reference follows the study: with the study's own in use, another
	// study brings its own or none; one chosen from elsewhere is kept.
	reference := func(dir string) string {
		ref := filepath.Join(dir, "hc_reference")
		os.MkdirAll(ref, 0o755)
		os.WriteFile(filepath.Join(ref, "reference.json"), []byte("{}"), 0o644)
		return ref
	}
	one, two, three := filepath.Join(dir, "one"), filepath.Join(dir, "two"), filepath.Join(dir, "three")
	refOne, refTwo := reference(one), reference(two)
	os.MkdirAll(three, 0o755)
	c := newApp()
	c.useStudy(one)
	if c.norm.reference != refOne {
		t.Fatalf("the reference of the study is not used: %q", c.norm.reference)
	}
	c.useStudy(two)
	if c.norm.reference != refTwo || c.classify.reference != refTwo {
		t.Errorf("study two is compared with the controls of %q", c.norm.reference)
	}
	c.useStudy(three)
	if c.norm.reference != "" {
		t.Errorf("study three, which has no reference, is compared with the controls of %q", c.norm.reference)
	}
	outside := reference(filepath.Join(dir, "outside"))
	c.setReference(outside)
	c.useStudy(one)
	if c.norm.reference != outside {
		t.Errorf("a reference chosen from elsewhere was replaced by %q", c.norm.reference)
	}
	// What a page read of a reference goes with the reference.
	c.zmap.info, c.classify.info = &refInfo{N: 16}, &refInfo{N: 16}
	c.setReference("")
	if c.zmap.info != nil || c.classify.info != nil {
		t.Error("the pages still describe a reference that is no longer in use")
	}

	pages0 := []string{"spatial", "normalize", "reference", "zmap", "classify", "compare", "cluster"}
	// Leaving a study takes its images and tables off the pages; those of
	// elsewhere stay, and so does everything when the same study is used again.
	mine, theirs := filepath.Join(one, "controls", "sub-hc01_pet.nii.gz"), filepath.Join(dir, "elsewhere", "sub-x_pet.nii.gz")
	covTable := filepath.Join(one, "participants.tsv")
	fill := func() {
		for _, list := range []*fileList{&c.spatial.files, &c.norm.files, &c.ref.files, &c.zmap.files, &c.classify.files, &c.cmp.g1, &c.cmp.g2, &c.cluster.files} {
			list.files = []string{mine, theirs}
		}
		c.zmap.covTable, c.classify.covTable, c.cmp.covExtra, c.ref.cov.path, c.cmp.cov.path = covTable, covTable, covTable, covTable, covTable
		c.norm.region.kind, c.norm.region.file = 1, filepath.Join(one, "reference_cluster.nii.gz")
		c.cmp.region.kind, c.cmp.region.file = 1, filepath.Join(dir, "elsewhere", "mask.nii.gz")
		c.spatial.output = filepath.Join(one, "patients")
		// The files of the settings too.
		c.opt.Template, c.opt.RefMaskFile = filepath.Join(one, "template.nii"), filepath.Join(one, "mask.nii.gz")
		c.opt.AnalysisFile = filepath.Join(dir, "elsewhere", "mask.nii.gz")
	}
	// The results of the runs that wrote into the study, and what the pages
	// offer to do with them.
	results := func(study string) {
		out := func(name string) string { return filepath.Join(study, name) }
		c.spatial.rows, c.spatial.written = []spatialRow{{image: "sub-pt01_pet.nii.gz"}}, []string{mine}
		c.spatial.outDir, c.spatial.st.ok = out("patients"), "warped"
		c.norm.rows, c.norm.outDir, c.norm.st.ok = []normRow{{image: "sub-pt01_pet.nii.gz"}}, out("normalised"), "normalised"
		c.ref.built, c.ref.st.ok = out("hc_reference"), "built"
		c.masks.outDir, c.masks.st.ok = out("masks"), "exported"
		c.zmap.rows, c.zmap.outDir, c.zmap.st.ok = []zmapRow{{image: "sub-pt01_pet.nii.gz"}}, out("zmaps"), "mapped"
		c.classify.rows, c.classify.outDir, c.classify.st.ok = []classifyRow{{image: "sub-pt01_pet.nii.gz"}}, out("reports"), "classified"
		c.cmp.result, c.cmp.how, c.cmp.st.ok = &core.GroupComparison{TMap: filepath.Join(out("compare"), "a_vs_b_T.nii.gz")}, "corrected", "compared"
		c.cluster.saved, c.cluster.volume, c.cluster.st.ok = out("reference_cluster.nii.gz"), 58, "found"
	}
	done := func() (pages []string) {
		for _, p := range pages0 {
			if c.done(p) {
				pages = append(pages, p)
			}
		}
		return pages
	}
	fill()
	results(one)
	c.useStudy(one)
	if len(c.ref.files.files) != 2 || c.zmap.covTable != covTable || c.spatial.output == "" || len(done()) != len(pages0) || len(c.zmap.rows) != 1 {
		t.Errorf("using the same study again took its images, tables or results off the pages (done: %q)", done())
	}
	c.useStudy(two)
	if got := done(); len(got) != 0 {
		t.Errorf("pages still marked as done after their study was left: %q", got)
	}
	if len(c.spatial.rows)+len(c.spatial.written)+len(c.norm.rows)+len(c.zmap.rows)+len(c.classify.rows) != 0 || c.cmp.result != nil || c.cmp.how != "" || c.cluster.saved != "" {
		t.Error("results of the study left behind are still on the pages")
	}
	if c.masks.outDir != "" || c.masks.st.ok != "" {
		t.Errorf("the masks exported into the study left behind are still shown: %q", c.masks.outDir)
	}
	// Results written elsewhere are no part of the study, and stay.
	results(filepath.Join(dir, "elsewhere"))
	c.spatial.output = ""
	c.useStudy(three)
	if got := done(); len(got) != len(pages0) || len(c.zmap.rows) != 1 || c.cmp.result == nil || c.masks.outDir == "" {
		t.Errorf("results written elsewhere were taken off the pages (done: %q)", got)
	}
	// A reference built into the study is of the study, wherever the next
	// one is to be saved.
	c.useStudy(one)
	c.ref.built, c.ref.st.ok, c.ref.output = refOne, "built", filepath.Join(dir, "elsewhere", "hc_reference")
	c.useStudy(three)
	if c.ref.st.ok != "" {
		t.Error("the reference built into the study left behind is still marked as done")
	}
	results(two)
	fill()
	c.useStudy(one)
	c.useStudy(two)
	for name, list := range map[string]*fileList{"spatial": &c.spatial.files, "normalize": &c.norm.files, "reference": &c.ref.files, "zmap": &c.zmap.files,
		"classify": &c.classify.files, "group 1": &c.cmp.g1, "group 2": &c.cmp.g2, "cluster": &c.cluster.files} {
		if len(list.files) != 1 || list.files[0] != theirs {
			t.Errorf("%s still holds %q after its study was left", name, list.files)
		}
	}
	if c.zmap.covTable != "" || c.classify.covTable != "" || c.cmp.covExtra != "" || c.ref.cov.path != "" || c.cmp.cov.path != "" || c.spatial.output != "" {
		t.Error("a table or an output of the study left behind is still chosen")
	}
	if c.norm.region.file != "" || c.cmp.region.file == "" {
		t.Errorf("the masks after leaving the study: %q (its own), %q (from elsewhere)", c.norm.region.file, c.cmp.region.file)
	}
	if c.opt.Template != "" || c.opt.RefMaskFile != "" || c.opt.AnalysisFile == "" {
		t.Errorf("the files of the settings after leaving the study: template %q, masks %q (its own), %q (from elsewhere)",
			c.opt.Template, c.opt.RefMaskFile, c.opt.AnalysisFile)
	}
	if c.ref.output != filepath.Join(two, "hc_reference") || c.norm.reference != refTwo {
		t.Errorf("after going over to study two: output %q, reference %q", c.ref.output, c.norm.reference)
	}
	// The same folder under another spelling is the same study.
	if runtime.GOOS != "windows" {
		alias := filepath.Join(dir, "alias")
		if err := os.Symlink(two, alias); err != nil {
			t.Fatal(err)
		}
		c.ref.files.files = []string{filepath.Join(two, "controls", "sub-hc01_pet.nii.gz")}
		if !c.study.chosenInUse(alias) || c.study.chosenInUse(three) {
			t.Error("the page does not know the study in use under another spelling, or takes another for it")
		}
		c.useStudy(alias)
		if len(c.ref.files.files) != 1 {
			t.Error("the same study under another spelling was taken for another study")
		}
		c.setReference(filepath.Join(alias, "hc_reference"))
		c.useStudy(three)
		if c.norm.reference != "" || len(c.ref.files.files) != 0 {
			t.Errorf("the study left under another spelling kept its reference %q or its images", c.norm.reference)
		}
		// Links elsewhere that lead into a study belong to it.
		refs := filepath.Join(dir, "refs")
		os.MkdirAll(refs, 0o755)
		current, image := filepath.Join(refs, "current"), filepath.Join(refs, "sub-hc01_pet.nii.gz")
		os.WriteFile(filepath.Join(one, "sub-hc01_pet.nii.gz"), nil, 0o644)
		os.Symlink(refOne, current)
		os.Symlink(filepath.Join(one, "sub-hc01_pet.nii.gz"), image)
		c.useStudy(one)
		c.setReference(current)
		c.ref.files.files = []string{image, theirs}
		c.useStudy(two)
		if c.norm.reference != refTwo || len(c.ref.files.files) != 1 || c.ref.files.files[0] != theirs {
			t.Errorf("links that lead into the study left behind were kept: reference %q, images %q", c.norm.reference, c.ref.files.files)
		}
		// So do links that spell the study in other letter case, where that
		// is the same folder.
		if shout := filepath.Join(dir, "ONE", "hc_reference"); sameDir(shout, refOne) {
			os.Remove(current)
			os.Symlink(shout, current)
			c.useStudy(one)
			c.setReference(current)
			c.useStudy(two)
			if c.norm.reference != refTwo {
				t.Errorf("a link into the study left behind, spelt in other case, was kept: %q", c.norm.reference)
			}
		}
		// A study in use is the folder it was then: the pages write into
		// it, not into the study a link to it leads to by now, and that
		// study is another.
		moving := filepath.Join(dir, "moving")
		os.Symlink(one, moving)
		c.useStudy(moving)
		c.ref.files.files = []string{mine, theirs}
		os.Remove(moving)
		os.Symlink(three, moving)
		if c.zmap.output != filepath.Join(one, "zmaps") || c.study.chosenInUse(moving) {
			t.Errorf("the pages write through a link, which leads to another study by now: %q", c.zmap.output)
		}
		c.useStudy(moving)
		if len(c.ref.files.files) != 1 || c.ref.files.files[0] != theirs || c.zmap.output != filepath.Join(three, "zmaps") {
			t.Errorf("the images of the study a link led to before were kept: %q", c.ref.files.files)
		}
	}
	// A folder made anew where the study was is another study.
	c.useStudy(one)
	fill()
	results(one)
	if err := os.Rename(one, filepath.Join(dir, "one-before")); err != nil {
		t.Fatal(err)
	}
	if c.study.chosenInUse(one) {
		t.Error("a study that was moved away is taken for the one in use")
	}
	os.MkdirAll(one, 0o755)
	c.useStudy(one)
	if got := done(); len(got) != 0 || len(c.ref.files.files) != 1 || c.zmap.covTable != "" {
		t.Errorf("a study made anew where another was kept that one's images or results (done: %q, images %q)", got, c.ref.files.files)
	}
	os.Remove(one)
	os.Rename(filepath.Join(dir, "one-before"), one)
	// A study given another name is the same folder, but what the pages
	// hold of it is named as before, and goes; its reference is found again.
	c.useStudy(one)
	fill()
	c.setReference(refOne)
	c.ref.current, c.ref.info = refOne, &refInfo{N: 16}
	renamed := filepath.Join(dir, "one-renamed")
	if err := os.Rename(one, renamed); err != nil {
		t.Fatal(err)
	}
	c.useStudy(renamed)
	if len(c.ref.files.files) != 1 || c.zmap.covTable != "" || c.norm.reference != filepath.Join(renamed, "hc_reference") {
		t.Errorf("after the study was given another name: images %q, table %q, reference %q", c.ref.files.files, c.zmap.covTable, c.norm.reference)
	}
	if c.ref.current != "" {
		t.Error("the Reference page still describes the reference it read before")
	}
	os.Rename(renamed, one)
	// How a run failed is not said of the next study, wherever it was to write.
	c.useStudy(one)
	results(filepath.Join(dir, "elsewhere"))
	c.norm.st, c.cmp.st = status{err: "no reference"}, status{err: "too few images"}
	c.useStudy(three)
	if c.norm.st.err != "" || c.cmp.st.err != "" || len(c.norm.rows) != 1 || c.zmap.st.ok == "" {
		t.Errorf("after leaving a study: errors %q, %q; results written elsewhere: %d", c.norm.st.err, c.cmp.st.err, len(c.norm.rows))
	}
	// The study's own reference that is gone is not kept when the study is
	// laid out again; one that is still there is.
	c.useStudy(two)
	c.setReference(refTwo)
	c.useStudy(two)
	if c.norm.reference != refTwo {
		t.Errorf("laying the study out again let go of its reference: %q", c.norm.reference)
	}
	os.Remove(filepath.Join(refTwo, "reference.json"))
	c.useStudy(two)
	if c.norm.reference != "" {
		t.Errorf("the reference that is gone is still in use: %q", c.norm.reference)
	}
}

// TestStudyChosen checks that choosing a study folder that is there already
// only fills in where it is: nothing is written into it, and the pages keep
// the study they have until the folders are created.
func TestStudyChosen(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	old, other := filepath.Join(dir, "old"), filepath.Join(dir, "other")
	os.MkdirAll(filepath.Join(other, "patients"), 0o755)
	a := newApp()
	a.study.parent, a.study.name = dir, "old"
	a.runStudy()
	if a.study.inUse != old {
		t.Fatalf("the study in use is %q", a.study.inUse)
	}
	a.zmap.files.files = []string{filepath.Join(old, "patients", "sub-01_pet.nii.gz")}
	a.study.choose(other)
	if a.study.target() != other {
		t.Errorf("the folder chosen is %q", a.study.target())
	}
	if entries, _ := os.ReadDir(other); len(entries) != 1 {
		t.Errorf("choosing the folder wrote into it: %d entries", len(entries))
	}
	if a.study.inUse != old || a.zmap.output != filepath.Join(old, "zmaps") || len(a.zmap.files.files) != 1 {
		t.Errorf("choosing the folder went over to it: in use %q, output %q, %d images", a.study.inUse, a.zmap.output, len(a.zmap.files.files))
	}
	// The page says so, and what going over takes off the pages, before
	// Create Folders does it.
	a.page = "study"
	tt := ui.NewTesterWith(a.view, 1060, 900, ui.TesterOptions{})
	tt.Frame()
	if !tt.HasText("Choose Study Folder…") {
		t.Errorf("no button to choose a study folder (shown: %q)", tt.Texts())
	}
	if err := tt.Click("Create Folders"); err != nil {
		t.Fatal(err)
	}
	if a.study.inUse != other || len(a.zmap.files.files) != 0 {
		t.Errorf("after creating: in use %q, %d images of the study left", a.study.inUse, len(a.zmap.files.files))
	}
}

// TestRemedy checks that the run bar offers the way out of a problem for
// that very problem only, in both languages.
func TestRemedy(t *testing.T) {
	defer func() { chinese = false }()
	for _, chinese = range []bool{false, true} {
		a := newApp()
		// No images yet: that is what the page says, though it has no
		// reference either.
		if got := a.remedy("normalize", a.norm.problem()); got != "" {
			t.Errorf("normalize without images: remedy %q for %q", got, a.norm.problem())
		}
		file := []string{"/study/patients/sub-01_pet.nii.gz"}
		a.norm.files.files, a.zmap.files.files, a.classify.files.files = file, file, file
		a.cluster.files.files = append(file, "/study/patients/sub-02_pet.nii.gz")
		a.cmp.g1.files, a.cmp.g2.files, a.cmp.method = a.cluster.files.files, a.cluster.files.files, "hn"
		problems := map[string]string{"normalize": a.norm.problem(), "zmap": a.zmap.problem(), "classify": a.classify.problem(),
			"cluster": a.cluster.problem(), "compare": a.cmp.problem(&a.opt)}
		for page, problem := range problems {
			if got := a.remedy(page, problem); got != "reference" {
				t.Errorf("%s without a reference: remedy %q for %q", page, got, problem)
			}
		}
		// A method that needs none, and a page that has one.
		a.norm.methods = map[string]bool{"cerebellum": true}
		a.zmap.reference = "/study/hc_reference"
		if got := a.remedy("normalize", a.norm.problem()); got != "" {
			t.Errorf("normalize by the cerebellum: remedy %q for %q", got, a.norm.problem())
		}
		if got := a.remedy("zmap", a.zmap.problem()); got != "" {
			t.Errorf("zmap with a reference: remedy %q for %q", got, a.zmap.problem())
		}
		// Spatial Normalisation: the settings mend SPM, not the output folder.
		a.spatial.files.files = file
		if a.spatial.spm == "" {
			if got := a.remedy("spatial", a.spatial.problem()); got != "spm" {
				t.Errorf("spatial without SPM: remedy %q for %q", got, a.spatial.problem())
			}
		}
		if got := a.remedy("spatial", "Choose the output folder."); got != "" {
			t.Errorf("spatial without an output folder: remedy %q", got)
		}
	}
}

// TestClassifyBadge checks that only a reading that names a pattern has a
// pill, which says its confidence once.
func TestClassifyBadge(t *testing.T) {
	defer func() { chinese = false }()
	pal := &lightPalette
	for _, chinese = range []bool{false, true} {
		for _, category := range []string{"none", "indeterminate", "possible", ""} {
			if badge, _ := classifyBadge(category, "low", pal); badge != "" {
				t.Errorf("a reading of category %q has the pill %q", category, badge)
			}
		}
		for _, category := range []string{"pattern", "pd"} {
			for _, confidence := range []string{"high", "moderate", "low"} {
				if badge, _ := classifyBadge(category, confidence, pal); badge == "" {
					t.Errorf("no pill for %s of %s confidence", category, confidence)
				}
			}
		}
		res := &core.Classification{Patterns: []core.Pattern{{ID: "msa-p", Score: 0.84}},
			Conclusion: core.Conclusion{Category: "pattern", Pattern: "msa-p", Confidence: "high"}}
		if _, detail := reading(res); strings.Contains(strings.ToLower(detail), "confidence") || strings.Contains(detail, "置信度") {
			t.Errorf("the detail repeats the confidence of the pill: %q", detail)
		}
	}
}

// TestSidebar checks that every page is in one group of the sidebar, and
// that every method has a line under its name in both languages that is
// not the name again.
func TestSidebar(t *testing.T) {
	defer func() { chinese = false }()
	count := map[string]int{}
	for _, g := range navGroups {
		for _, id := range g.pages {
			count[id]++
			if pageInfoOf(id).id != id {
				t.Errorf("the sidebar lists %q, which is no page", id)
			}
		}
	}
	for _, p := range pages {
		if count[p.id] != 1 {
			t.Errorf("page %s is in %d groups of the sidebar", p.id, count[p.id])
		}
	}
	for _, chinese = range []bool{false, true} {
		for _, g := range methodGroups {
			for _, id := range g.methods {
				if hint := methodHint(id); hint == "" || strings.EqualFold(hint, methodName(id)) {
					t.Errorf("method %s: hint %q under the name %q", id, hint, methodName(id))
				}
			}
		}
	}
}

// TestStudyShortcuts checks the shortcuts to what the study in use holds:
// its folder of patients on a list of images, asked of the file system at
// most once a second.
func TestStudyShortcuts(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	one, other := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if got := commonDir([]string{filepath.Join(one, "x.nii"), filepath.Join(one, "y.nii")}); got != one {
		t.Errorf("the folder of two files in %s is %q", one, got)
	}
	if got := commonDir([]string{filepath.Join(one, "x.nii"), filepath.Join(other, "y.nii")}); got != "" || commonDir(nil) != "" {
		t.Errorf("files apart, or none, share the folder %q", got)
	}
	a := newApp()
	a.page = "zmap"
	tt := ui.NewTesterWith(a.view, 1060, 900, ui.TesterOptions{})
	tt.Frame()
	if tt.HasText("Load patients/") {
		t.Fatal("a shortcut to a study while none is in use")
	}
	a.study.parent = dir
	a.runStudy()
	study := filepath.Join(dir, "study")
	_, pt, _ := makeImages(t, filepath.Join(study, "patients"), 0, 2)
	tt.Frame()
	if err := tt.Click("Load patients/"); err != nil {
		t.Fatalf("%v (shown: %q)", err, tt.Texts())
	}
	tt.Frame()
	if !slices.Equal(a.zmap.files.files, pt) {
		t.Errorf("loaded %q, not %q", a.zmap.files.files, pt)
	}
	// What is there is asked of the file system once, and again after a
	// second: until then a folder that has gone is still seen.
	patients := seenKey{filepath.Join(study, "patients"), true}
	if !a.onDisk(patients.path, true) || a.onDisk(patients.path, false) {
		t.Error("the folder of patients is not seen as a folder")
	}
	os.RemoveAll(patients.path)
	if !a.onDisk(patients.path, true) {
		t.Error("the file system is asked on every frame")
	}
	a.seen[patients] = seenFile{true, time.Now().Add(-2 * time.Second)}
	if a.onDisk(patients.path, true) {
		t.Error("a folder that is gone is seen for good")
	}
}

// TestSwapGroups checks that swapping the groups of a comparison takes the
// result off the page, whose T-map has the sign of the groups as they were,
// and that the folders of the study follow the groups.
func TestSwapGroups(t *testing.T) {
	a := newApp()
	a.demo()
	p := &a.cmp
	g1, g2 := slices.Clone(p.g1.files), slices.Clone(p.g2.files)
	if p.result == nil || p.st.ok == "" {
		t.Fatal("the made-up comparison has no result")
	}
	p.swap()
	if !slices.Equal(p.g1.files, g2) || !slices.Equal(p.g2.files, g1) || !p.swapped {
		t.Error("the groups are not swapped")
	}
	if p.result != nil || p.st.ok != "" || p.how != "" {
		t.Errorf("the result of the groups as they were is still shown: %q", p.st.ok)
	}
	// Group 1, emptied, is where the controls go now: its hint says so.
	a.page = "compare"
	p.g1.files = nil
	tt := ui.NewTesterWith(a.view, 1060, 900, ui.TesterOptions{})
	tt.Frame()
	if !tt.HasText("e.g. the controls") || tt.HasText("e.g. the patients") || !p.swapped {
		t.Errorf("the hints do not follow the groups (shown: %q)", tt.Texts())
	}
	// With both lists empty the groups are as at first.
	p.g2.files = nil
	tt.Frame()
	tt.Frame()
	if p.swapped || !tt.HasText("e.g. the patients") {
		t.Error("empty groups are still swapped")
	}
}

// TestStudyLinks checks what a study folder may hold in place of the usual:
// a folder of patients that is a link is loaded where it leads, under the
// names of the link; a table that is no regular file is not offered; and a
// study folder chosen is the target to the letter.
func TestStudyLinks(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp()
	if a.onDisk(dir, false) || !a.onDisk(dir, true) {
		t.Error("a folder is seen as a file")
	}
	_, pt, _ := makeImages(t, filepath.Join(dir, "elsewhere"), 0, 2)
	link := filepath.Join(dir, "patients")
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), link); err != nil {
		t.Skipf("no links here: %v", err)
	}
	want := []string{filepath.Join(link, filepath.Base(pt[0])), filepath.Join(link, filepath.Base(pt[1]))}
	if got := expandPaths([]string{link}); !slices.Equal(got, want) {
		t.Errorf("a linked folder holds %q, not %q", got, want)
	}
	if got := expandPaths([]string{filepath.Join(dir, "elsewhere")}); !slices.Equal(got, pt) {
		t.Errorf("the folder itself holds %q, not %q", got, pt)
	}
	// A name with a space at its end is no name to type, but may be picked.
	spaced := filepath.Join(dir, "cohort ")
	if err := os.Mkdir(spaced, 0o755); err == nil {
		a.study.choose(spaced)
		if a.study.target() != spaced {
			t.Errorf("chose %q, the target is %q", spaced, a.study.target())
		}
		a.study.name = "cohort"
		if a.study.target() != filepath.Join(dir, "cohort") {
			t.Errorf("a name typed afterwards gives the target %q", a.study.target())
		}
	}
}
