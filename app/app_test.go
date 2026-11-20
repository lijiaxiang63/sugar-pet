package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

// jsonApart says where two JSON texts differ by more than tol in a number, or
// at all in anything else; "" when they say the same.
func jsonApart(a, b []byte, tol float64) string {
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		return err.Error()
	}
	if err := json.Unmarshal(b, &y); err != nil {
		return err.Error()
	}
	var apart func(at string, x, y any) string
	apart = func(at string, x, y any) string {
		switch x := x.(type) {
		case map[string]any:
			y, ok := y.(map[string]any)
			if !ok || len(x) != len(y) {
				return at
			}
			for k, v := range x {
				if where := apart(at+"."+k, v, y[k]); where != "" {
					return where
				}
			}
		case []any:
			y, ok := y.([]any)
			if !ok || len(x) != len(y) {
				return at
			}
			for i, v := range x {
				if where := apart(at+"["+strconv.Itoa(i)+"]", v, y[i]); where != "" {
					return where
				}
			}
		case float64:
			if y, ok := y.(float64); !ok || math.Abs(x-y) > tol {
				return fmt.Sprint(at, ": ", x, " and ", y)
			}
		default:
			if !reflect.DeepEqual(x, y) {
				return fmt.Sprint(at, ": ", x, " and ", y)
			}
		}
		return ""
	}
	return apart("", x, y)
}

// TestReferenceFactors builds a reference with the factors of its controls
// estimated another way than the usual one, and checks that it is the way
// Normalize estimates the factor of an image with the same settings: only
// then is a map of that image one against these controls.
func TestReferenceFactors(t *testing.T) {
	dir := t.TempDir()
	hc, pt, _ := makeImages(t, filepath.Join(dir, "data"), 6, 1)
	six := 6.0
	build := core.BuildOptions{FWHM: 8, StatsFor: []string{"cerebellum"}, Statistic: "median", FactorFWHM: &six}
	built, err := core.BuildReference(context.Background(), hc, build)
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "ref")
	if err := built.Save(folder); err != nil {
		t.Fatal(err)
	}
	ref, err := core.LoadReference(folder)
	if err != nil {
		t.Fatal(err)
	}
	info := readRefInfo(folder)
	if ref.Statistic != "median" || ref.FactorFWHM == nil || *ref.FactorFWHM != 6 || info.Statistic != "median" || info.FactorFWHM == nil {
		t.Fatalf("the reference does not say how its factors were estimated: %+v, %+v", ref.Statistic, info)
	}
	opt := core.DefaultNormOptions()
	opt.Reference, opt.Statistic, opt.FWHM = ref, "median", &six
	for _, factors := range ref.ControlFactors {
		for i, path := range hc[:2] {
			res, err := core.NormalizeFile(path, factors.Method, opt)
			if err != nil {
				t.Fatal(err)
			}
			if want := factors.Factors[i]; math.Abs(res.Factor-want) > 1e-4*want {
				t.Errorf("%s of control %d: Normalize gives %g, the reference has %g", factors.Method, i, res.Factor, want)
			}
		}
	}
	// A map of an image normalised so is the map the reference makes itself.
	raw, err := core.LoadImage(pt[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"hn", "ps", "cerebellum"} {
		want, _, err := core.ZMap(raw, method, ref, core.DefaultZMapOptions())
		if err != nil {
			t.Fatal(err)
		}
		res, err := core.Normalize(raw, method, opt)
		if err != nil {
			t.Fatal(err)
		}
		got, _, err := core.ZMapOf(res.Image, method, res.Factor, 1, ref, nil)
		if err != nil {
			t.Fatal(err)
		}
		worst := 0.0
		for i, v := range want.Data {
			worst = math.Max(worst, math.Abs(float64(v-got.Data[i])))
		}
		if worst > 1e-3 {
			t.Errorf("%s: the map of the normalised image is up to %g from the reference's own", method, worst)
		}
	}
	// The usual way is not written down: the file stays what it was.
	build.Statistic, build.FactorFWHM = "", nil
	if built, err = core.BuildReference(context.Background(), hc, build); err != nil {
		t.Fatal(err)
	}
	if err := built.Save(folder); err != nil {
		t.Fatal(err)
	}
	text, _ := os.ReadFile(filepath.Join(folder, "reference.json"))
	if strings.Contains(string(text), "statistic") || strings.Contains(string(text), "factor_fwhm_mm") {
		t.Fatalf("a reference built the usual way says how: %s", text)
	}
	if ref, err = core.LoadReference(folder); err != nil || ref.Statistic != "mean" || ref.FactorFWHM != nil {
		t.Fatalf("read back: %v, %+v", err, ref.Statistic)
	}
}

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
// normalise the patients and the controls, which builds the reference, make
// z-score maps, classify patients, compare the two groups and find a
// reference cluster.
func TestWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("works on whole images")
	}
	chinese = true
	defer func() { chinese = false }()
	hc, pt, table := makeImages(t, filepath.Join(t.TempDir(), "data"), 12, 8)

	// A project, with the images where they are.
	a := testProject(t)
	dir := a.proj.dir
	a.proj.patients.files, a.proj.controls.files = pt, hc
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

	// The reference is the controls, and no step of its own: its page has
	// nothing to run, and says that step 1 builds it.
	ref := filepath.Join(dir, "hc_reference")
	a.cov.path = table
	a.cov.load()
	a.page = "reference"
	tt.Frame()
	if _, shown := tt.Find("建立参考库"); shown || !tt.HasText("还没有参考库") || !tt.HasText("如何建立") || a.norm.reference != "" || !a.buildsReference() {
		t.Fatalf("the Reference page before step 1: %q", tt.Texts())
	}

	// The pages that rest on step 1 wait for it, and say so.
	for _, page := range []string{"zmap", "classify", "compare"} {
		a.page = page
		tt.Frame()
		if step, why := a.lock(page); step != "normalize" || !tt.HasText(why) || !tt.HasText("前往第 1 步") {
			t.Fatalf("%s before step 1: waits for %q (shown: %q)", page, step, tt.Texts())
		}
	}

	// Step 1: the reference is built from the controls, with W-scores and
	// the statistics of the methods chosen, and then the patients and the
	// controls are normalised by the two methods.
	out := filepath.Join(dir, "normalised")
	a.norm.methods = map[string]bool{"hn": true, "cerebellum": true}
	a.page = "normalize"
	tt.Frame()
	if !tt.HasText(fmt.Sprintf("运行强度归一化时由 %d 例对照自动建立", len(hc))) || !tt.HasText("主方法") {
		t.Fatalf("step 1 does not say that it builds the reference, or which method is primary: %q", tt.Texts())
	}
	press("normalize", "开始强度归一化", &a.norm.st)
	exists(filepath.Join(ref, "reference.json"))
	if a.norm.reference != ref || a.cmp.reference != ref || a.ref.built == nil || a.refStale() || !strings.HasPrefix(a.norm.st.ok, "已用 12 例对照建立参考库") {
		t.Fatalf("step 1 did not build the reference of the project: %q, %q", a.norm.reference, a.norm.st.ok)
	}
	if info := readRefInfo(ref); !slices.Contains(info.StatsFor, "cerebellum") || fmt.Sprint(info.covariateNames()) != "[age sex]" || info.N != len(hc) {
		t.Fatalf("the reference built: %+v", info)
	}
	if len(a.norm.rows) != 2*(len(pt)+len(hc)) || a.norm.rows[0].group != "patients" || a.norm.rows[len(a.norm.rows)-1].group != "controls" {
		t.Fatalf("%d rows of factors, want %d", len(a.norm.rows), 2*(len(pt)+len(hc)))
	}
	exists(filepath.Join(out, "factors.tsv"))
	exists(filepath.Join(out, "patients", core.OutputName(pt[0], "hn", "pet")))
	exists(filepath.Join(out, "controls", core.OutputName(hc[0], "cerebellum", "pet")))
	if got, _ := a.norm.done.of(true); !slices.Equal(got, hc) {
		t.Fatalf("the controls step 1 normalised: %v", got)
	}
	// The Reference page shows what was built.
	a.page = "reference"
	tt.Frame()
	if !tt.HasText("age、sex") || !tt.HasText("由本项目的健康对照建立") {
		t.Fatalf("the reference is not shown: %q", tt.Texts())
	}
	drawn(&a.ref.pic)
	// What the reference is built with changed: it is built anew by the next
	// run, and step 1 is not what it would do now.
	a.opt.RefFWHM += 2
	if !a.refStale() || a.stepDone("normalize") || !strings.Contains(a.normChanged(), "参考库设置") {
		t.Fatalf("with another smoothing of the reference: stale %v, changed %q", a.refStale(), a.normChanged())
	}
	a.opt.RefFWHM -= 2
	a.page = "normalize"
	tt.Frame()

	if !a.stepDone("normalize") || a.stepDone("zmap") {
		t.Fatal("step 1 is not marked as done, or step 2 is")
	}
	// A setting of the normalisation changed after the run: the images are
	// not what it would write now, and the steps after it wait for another
	// run, of which the user is told once.
	target := a.opt.Target
	a.opt.Target = 50
	if step, why := a.lock("zmap"); a.stepDone("normalize") || step != "normalize" || !strings.Contains(why, "设置") {
		t.Fatalf("with the target changed step 2 waits for %q: %q", step, why)
	}
	if a.tellNormSettings(); a.normTold != a.norm.done {
		t.Fatal("the changed settings were not told of")
	}
	a.opt.Target = target
	if a.tellNormSettings(); a.normTold != nil || !a.stepDone("normalize") {
		t.Fatal("with the settings as they were step 1 is not done again")
	}
	// The iterative settings are no part of a run without iPS or iHN.
	a.opt.PThreshold /= 2
	if !a.stepDone("normalize") {
		t.Fatal("a setting of the iterative methods undid a run without them")
	}
	a.opt.PThreshold *= 2
	// Factors estimated unlike those of the controls give no map.
	a.norm.done.median = true
	if why := a.zmapProblem(); !strings.Contains(why, "区域中位数") || !strings.Contains(why, "区域均值") || a.remedy("zmap", why) != "factors" {
		t.Fatalf("with factors from the median: %q, remedy %q", why, a.remedy("zmap", why))
	}
	// A reference whose controls were scaled by the median is one to
	// compare them with; of HN alone the statistic is no part.
	a.zmap.info.Statistic = "median"
	if why := a.zmapProblem(); why != "" {
		t.Fatalf("step 1 and the reference both by the median: %q", why)
	}
	a.zmap.info.Statistic = "mean"
	if why := a.norm.done.unfit([]string{"hn"}, a.zmap.info); why != "" {
		t.Fatalf("HN with another statistic than the reference: %q", why)
	}
	a.norm.done.median = false
	six := 6.0
	a.norm.done.fwhm = &six
	if why := a.norm.done.unfit([]string{"hn"}, a.zmap.info); !strings.Contains(why, "6 mm") || !strings.Contains(why, "自动平滑") {
		t.Fatalf("with a smoothing of its own: %q", why)
	}
	a.zmap.info.FactorFWHM = &six
	if why := a.zmapProblem(); why != "" {
		t.Fatalf("step 1 and the reference both after 6 mm: %q", why)
	}
	a.norm.done.fwhm, a.zmap.info.FactorFWHM = nil, nil
	// The reference built anew in its folder is not the one step 1 used.
	a.norm.done.rebuilt = true
	if _, why := a.lock("zmap"); a.stepDone("normalize") || !strings.Contains(why, "参考库") {
		t.Fatalf("with the reference rebuilt: %q", why)
	}
	a.norm.done.rebuilt = false

	// The button beside the run leads to the next step.
	if err := tt.Click("下一步：Z 值图"); err != nil {
		t.Fatalf("%v (shown: %q)", err, tt.Texts())
	}
	tt.Frame()
	if a.page != "zmap" {
		t.Fatalf("the next step led to %q", a.page)
	}

	// Step 2: W-score maps of the images of step 1, by its methods, which
	// are shown and not asked for again.
	zdir := filepath.Join(dir, "zmaps")
	tt.Frame()
	if !tt.HasText("来自第 1 步") || !tt.HasText("返回第 1 步修改") || tt.HasText("添加文件…") {
		t.Fatalf("step 2 does not show what it takes from step 1: %q", tt.Texts())
	}
	press("zmap", "生成 Z 值图", &a.zmap.st)
	if len(a.zmap.rows) != 2*len(pt) {
		t.Fatalf("%d z-score maps, want %d", len(a.zmap.rows), 2*len(pt))
	}
	maps, _ := filepath.Glob(filepath.Join(zdir, "*.nii.gz"))
	if len(maps) != 2*len(pt) {
		t.Fatalf("%d files in %s, want %d", len(maps), zdir, 2*len(pt))
	}
	drawn(&a.zmap.pic)
	// The maps were made of the images step 1 wrote, and are those the
	// original images give.
	ref0, err := core.LoadReference(ref)
	if err != nil {
		t.Fatal(err)
	}
	cov, err := core.LoadTable(table)
	if err != nil {
		t.Fatal(err)
	}
	values, err := cov.Lookup([]string{pt[0]}, ref0.Covariates())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := core.LoadImage(pt[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"hn", "cerebellum"} {
		zopt := core.DefaultZMapOptions()
		zopt.Covariates = values[0]
		want, _, err := core.ZMap(raw, method, ref0, zopt)
		if err != nil {
			t.Fatal(err)
		}
		got, err := core.LoadImage(a.zmap.done.maps[pt[0]][method])
		if err != nil {
			t.Fatal(err)
		}
		worst := 0.0
		for i, v := range want.Data {
			worst = math.Max(worst, math.Abs(float64(v-got.Data[i])))
		}
		if worst > 1e-3 {
			t.Fatalf("the %s map of step 2 is up to %g from the one of the original image", method, worst)
		}
	}
	// Another covariate table is not what the maps were made with.
	a.zmap.covTable = ""
	if a.stepDone("zmap") {
		t.Fatal("step 2 is done though its covariate table was taken away")
	}
	if step, _ := a.lock("classify"); step != "zmap" {
		t.Fatalf("step 3 waits for %q", step)
	}
	a.zmap.covTable = table
	if !a.stepDone("zmap") {
		t.Fatal("step 2 is not marked as done")
	}

	// Step 3: a report for each patient, in the language of the app.
	reports := filepath.Join(dir, "reports")
	a.page = "classify"
	tt.Frame()
	if !tt.HasText("来自前两步") || tt.HasText("分类所依据的图") || tt.HasText("添加文件…") {
		t.Fatalf("step 3 does not show what it takes from the steps before: %q", tt.Texts())
	}
	press("classify", "开始分类", &a.classify.st)
	if len(a.classify.rows) != len(pt) || a.classify.rows[0].reading == "" || !a.stepDone("classify") {
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
	if err != nil || len(strings.Split(strings.TrimSpace(string(summary)), "\n")) != len(pt)+1 {
		t.Fatalf("classification.tsv: %v, %q", err, summary)
	}
	exists(filepath.Join(reports, "sub-pt00_classification.json"))
	// The classification read the maps of step 2 and, with no cross-check
	// asked for, made none itself: it says what one from the original image
	// says that is not cross-checked either.
	copt := core.DefaultClassifyOptions()
	copt.Covariates, copt.Checks = values[0], []string{}
	whole, err := core.Classify(raw, filepath.Base(pt[0]), "hn", ref0, copt)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := whole.JSON()
	gotJSON, err := os.ReadFile(filepath.Join(reports, "sub-pt00_classification.json"))
	if err != nil {
		t.Fatal(err)
	}
	if where := jsonApart(wantJSON, gotJSON, 2e-3); where != "" {
		t.Fatalf("the classification from the maps of step 2 is not the one from the image: %s", where)
	}
	// The map is the one of step 2: no second copy lies with the reports.
	if again, _ := filepath.Glob(filepath.Join(reports, "*.nii.gz")); len(again) != 0 {
		t.Fatalf("classification wrote maps of its own: %v", again)
	}
	if got, want := a.classify.rows[0].zmap, filepath.Join(zdir, core.OutputName(pt[0], "hn", "wmap")); got != want {
		t.Fatalf("the classification shows the map %q, want %q", got, want)
	}
	if !tt.HasText("打开报告") {
		t.Fatalf("no button opens the report: %q", tt.Texts())
	}
	drawn(&a.classify.pic)
	if got := a.classify.rows[0].scores; len(got) != len(core.Patterns) {
		t.Fatalf("pattern scores %v", got)
	}
	// The classification rests on HN, the primary method of the two of step
	// 1; the cerebellum's statistics are in the reference, so it can rest on
	// that, once made primary in step 1.
	if got := a.norm.primary(); got != "hn" || a.classify.method != "hn" || len(a.classifyChecks("hn")) != 0 || a.classify.checked {
		t.Fatalf("classified by %q (%q), checks %v", got, a.classify.method, a.classifyChecks("hn"))
	}
	if strings.Contains(string(gotJSON), `"ps"`) {
		t.Fatal("without the cross-check a finding was checked by global scaling")
	}
	a.norm.main = "cerebellum"
	if a.stepDone("classify") || !a.stepDone("normalize") || !a.stepDone("zmap") || a.classifyProblem() != "" {
		t.Fatalf("with the cerebellum made primary: step 3 done %v, problem %q", a.stepDone("classify"), a.classifyProblem())
	}
	a.norm.main = ""
	// The cross-check asked for in the settings: the reports are not what
	// step 3 writes now.  Run again, it checks its findings by global
	// scaling, as a classification of the original image does.
	a.opt.CrossCheck = true
	if a.stepDone("classify") || fmt.Sprint(a.classifyChecks("hn")) != "[ps]" {
		t.Fatalf("with the cross-check asked for: step 3 done %v, checks %v", a.stepDone("classify"), a.classifyChecks("hn"))
	}
	a.page = "classify"
	tt.Frame()
	if !strings.Contains(fmt.Sprint(tt.Texts()), "复核") {
		t.Fatalf("step 3 does not say what it checks with: %q", tt.Texts())
	}
	press("classify", "开始分类", &a.classify.st)
	copt.Checks = nil
	if whole, err = core.Classify(raw, filepath.Base(pt[0]), "hn", ref0, copt); err != nil {
		t.Fatal(err)
	}
	wantJSON, _ = whole.JSON()
	if gotJSON, err = os.ReadFile(filepath.Join(reports, "sub-pt00_classification.json")); err != nil {
		t.Fatal(err)
	}
	if where := jsonApart(wantJSON, gotJSON, 2e-3); where != "" || !a.classify.checked || !a.stepDone("classify") {
		t.Fatalf("the cross-checked classification is not the one from the image: %s", where)
	}

	// The project opened again is where it was left: the three steps done,
	// with what they showed, and nothing to run anew.
	tt.Frame()
	again := newApp()
	if err := again.openProject(dir); err != nil {
		t.Fatal(err)
	}
	for _, step := range flowSteps {
		if !again.stepDone(step) || !again.done(step) {
			t.Fatalf("in the project opened again step %s is not done (step 1 changed: %q)", step, again.normChanged())
		}
	}
	if again.ref.built == nil || again.refStale() {
		t.Fatal("the project opened again would build its reference anew")
	}
	if build, problem := again.referenceBuild(); build != nil || problem != "" {
		t.Fatalf("the project opened again builds its reference anew: %q", problem)
	}
	if len(again.norm.rows) != len(a.norm.rows) || len(again.zmap.rows) != len(a.zmap.rows) || len(again.classify.rows) != len(pt) ||
		again.norm.reference != ref || again.classify.rows[0].reading != a.classify.rows[0].reading {
		t.Fatalf("the project opened again does not show what the steps did: %d, %d and %d rows, reference %q",
			len(again.norm.rows), len(again.zmap.rows), len(again.classify.rows), again.norm.reference)
	}
	// An image of step 1 changed on disk since: it is not what was normalised.
	if err := os.Chtimes(pt[0], time.Time{}, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	changed := newApp()
	if err := changed.openProject(dir); err != nil {
		t.Fatal(err)
	}
	if changed.stepDone("normalize") || !strings.Contains(changed.normChanged(), "图像") {
		t.Fatalf("with an image changed since step 1 ran, step 1 is still done: %q", changed.normChanged())
	}
	if err := os.Chtimes(pt[0], time.Time{}, time.Unix(0, a.norm.done.stamps[pt[0]].modified)); err != nil {
		t.Fatal(err)
	}

	// Sex is no longer adjusted for.  No factor rests on the covariates:
	// step 1 still stands, and step 2, which rests on the model of the
	// reference, waits for a run that builds the reference anew.
	a.cov.adjust["sex"] = false
	if !a.stepDone("normalize") || a.stepDone("zmap") || !a.refStale() || a.refBuilder() != "zmap" {
		t.Fatalf("with sex left out: step 1 done %v, step 2 done %v, stale %v (step 1 changed: %q)",
			a.stepDone("normalize"), a.stepDone("zmap"), a.refStale(), a.normChanged())
	}
	if step, why := a.lock("classify"); step != "zmap" || !strings.Contains(why, "协变量") {
		t.Fatalf("with sex left out step 3 waits for %q: %q", step, why)
	}
	if step, _ := a.lock("compare"); step != "" {
		t.Fatalf("with sex left out the comparison waits for %q", step)
	}
	a.page = "reference"
	tt.Frame()
	if !tt.HasText("下次运行Z 值图时会重新建立") {
		t.Fatalf("the Reference page does not say that step 2 builds it: %q", tt.Texts())
	}
	normalised := a.norm.done
	press("zmap", "生成 Z 值图", &a.zmap.st)
	if got := readRefInfo(ref).covariateNames(); fmt.Sprint(got) != "[age]" || a.refStale() || !strings.HasPrefix(a.zmap.st.ok, "已用 12 例对照重新建立参考库") {
		t.Fatalf("step 2 did not build the reference anew: %v, %q", got, a.zmap.st.ok)
	}
	if a.norm.done != normalised || !a.stepDone("normalize") || !a.stepDone("zmap") {
		t.Fatalf("after step 2 built the reference anew: step 1 done %v (changed %q), step 2 done %v",
			a.stepDone("normalize"), a.normChanged(), a.stepDone("zmap"))
	}
	// Opened again, step 1 still rests on the reference built anew.
	tt.Frame()
	if again := newApp(); again.openProject(dir) != nil || !again.stepDone("normalize") || !again.stepDone("zmap") {
		t.Fatal("in the project opened again the reference built by step 2 undid a step")
	}
	a.cov.adjust["sex"] = true
	press("zmap", "生成 Z 值图", &a.zmap.st)

	// The patients against the controls: the cortex takes up less.
	a.norm.main = "cerebellum"
	a.page = "compare"
	tt.Frame()
	if !tt.HasText("全小脑 · 主方法") {
		t.Fatalf("the comparison does not show the primary method of step 1: %q", tt.Texts())
	}
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
	// The comparison read the images step 1 wrote: its T-map is the one of
	// a comparison that normalises the original images itself.
	own := core.DefaultCompareOptions()
	own.Method, own.Reference, own.Correction, own.Name = "cerebellum", ref0, "none", "check"
	own.FWHM = &ref0.FWHM
	both := append(append([]string(nil), pt...), hc...)
	covRows, err := cov.Lookup(both, []string{"age", "sex"})
	if err != nil {
		t.Fatal(err)
	}
	own.Covariates = core.CovariatesFromRows(covRows, []string{"age", "sex"})
	check, err := core.CompareGroups(context.Background(), pt, hc, t.TempDir(), own)
	if err != nil {
		t.Fatal(err)
	}
	wantT, err := core.LoadImage(check.TMap)
	if err != nil {
		t.Fatal(err)
	}
	gotT, err := core.LoadImage(res.TMap)
	if err != nil {
		t.Fatal(err)
	}
	apart := 0.0
	for i, v := range wantT.Data {
		apart = math.Max(apart, math.Abs(float64(v-gotT.Data[i])))
	}
	if apart > 1e-2 {
		t.Fatalf("the T-map from the images of step 1 is up to %g from the one of the original images", apart)
	}
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
	mask := filepath.Join(dir, "reference_cluster.nii.gz")
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
	// The method of step 1 was changed: the steps after it wait for a new run.
	if step, _ := a.lock("classify"); step != "normalize" || a.stepDone("zmap") {
		t.Fatalf("after another method was chosen in step 1, step 3 waits for %q", step)
	}
	// The reference is as its controls and the settings make it: this run
	// does not build it again.
	built := a.ref.built
	press("normalize", "开始强度归一化", &a.norm.st)
	if len(a.norm.rows) != len(pt)+len(hc) || a.norm.rows[0].voxels == "" || a.norm.rows[0].mask != "custom" {
		t.Fatalf("normalising by the cluster gave %+v", a.norm.rows)
	}
	if a.ref.built != built || strings.Contains(a.norm.st.ok, "参考库") {
		t.Fatalf("the reference was built again though nothing of it changed: %q", a.norm.st.ok)
	}
	// No map can be made with a custom region: step 2 says so.
	a.page = "zmap"
	tt.Frame()
	if got := a.zmapProblem(); got != noMethod([]string{"region"}) || !tt.HasText(got) {
		t.Fatalf("step 2 after a custom region: %q", got)
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

	a := testProject(t)
	a.proj.patients.files, a.proj.controls.files = pt, hc
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

	// The table as first chosen, on the Data page: other people, and a
	// column for the site.
	cv := &a.cov
	write([]string{"age", "sex", "site"}, []string{"sub-xx00", "sub-xx01", "sub-xx02", "sub-xx03"})
	a.page = "data"
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
	a.page = "normalize"
	run("开始强度归一化", &a.norm.st)
	if got := readRefInfo(a.ref.output).covariateNames(); fmt.Sprint(got) != "[bmi age]" {
		t.Fatalf("the reference adjusts for %v", got)
	}
	ticks(cv, map[string]bool{"age": true, "sex": false, "bmi": true})

	// A ticked column leaves the file before the page shows it: the run
	// does not go on with fewer columns than were ticked.
	write([]string{"age", "sex"}, people)
	refused("开始强度归一化", &a.norm.st, "列已发生变化")
	ticks(cv, map[string]bool{"age": true, "sex": false})

	// The file is gone: the run does not use the rows it read before, and
	// the ticks wait for the file to be back.
	if err := os.Remove(table); err != nil {
		t.Fatal(err)
	}
	refused("开始强度归一化", &a.norm.st, "无法读取协变量表")
	a.page = "data"
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

	// The comparison of the groups adjusts by the same table, read anew.
	write([]string{"age", "sex"}, but("sub-pt02"))
	later()
	ticks(cv, map[string]bool{"age": true, "sex": true})
	tt.Scroll(640, 400, 0, 900)
	click("sex")
	// The covariates are not those the reference was built with: it is to
	// be built anew, but no factor rests on them, so step 1 still stands and
	// the comparison, which adjusts by the table itself, waits for nothing.
	a.page = "compare"
	tt.Frame()
	if step, why := a.lock("compare"); step != "" || !a.refStale() || !a.stepDone("normalize") || a.refBuilder() != "zmap" {
		t.Fatalf("with other covariates the comparison waits for %q: %q (step 1 changed: %q)", step, why, a.normChanged())
	}
	// Another method for step 1: its run builds the reference anew.
	a.norm.methods = map[string]bool{"cerebellum": true}
	if a.refBuilder() != "normalize" {
		t.Fatal("with step 1 to be run again, Z-score Maps would build the reference")
	}
	a.page = "normalize"
	run("开始强度归一化", &a.norm.st)
	if got := readRefInfo(a.ref.output).covariateNames(); fmt.Sprint(got) != "[age]" || a.refStale() {
		t.Fatalf("the reference built anew adjusts for %v", got)
	}
	write([]string{"age", "sex", "bmi"}, people)
	tt.Frame()
	if len(cv.table.Rows) != len(people)-1 {
		t.Fatal("the page has read the file already")
	}
	a.page = "compare"
	run("开始比较", &a.cmp.st)
	if res := a.cmp.result; res == nil || res.DOF != float64(len(people)-3) {
		t.Fatalf("the comparison is not adjusted for age alone: %+v", res)
	}
	ticks(cv, map[string]bool{"age": true, "sex": false, "bmi": false})

	// The made-up table of the screenshots has no file, and stays.
	a = newApp()
	a.demo()
	a.page = "data"
	tt = ui.NewTesterWith(a.view, 1060, 740, ui.TesterOptions{})
	later()
	tt.Scroll(640, 400, 0, 900)
	tt.Frame()
	if a.cov.table == nil || !tt.HasText("共 22 行") {
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

// TestSpatial warps the images of a project in native space on the Spatial
// Normalisation page, with a stand-in for MATLAB: what is warped is written
// into the folders of the groups, and is what the pages after it read.
func TestSpatial(t *testing.T) {
	dir := t.TempDir()
	spm, matlab := fakeSPM(t, dir)
	_, pt, _ := makeImages(t, filepath.Join(dir, "raw"), 0, 3)

	a := testProject(t)
	queue := make(chan func(), 256)
	a.post = func(fn func()) { queue <- fn }
	a.page = "spatial"
	tt := ui.NewTesterWith(a.view, 1060, 740, ui.TesterOptions{})
	tt.Frame()
	p := &a.spatial
	// Images in MNI space need no warp: the page says so, and leads back.
	a.proj.patients.files, a.proj.controls.files = pt[:2], pt[2:]
	tt.Frame()
	if msg := a.spatialProblem(); msg != notNative() || a.remedy("spatial", msg) != "data" || len(p.files.files) != 0 {
		t.Fatalf("a project of images in MNI space: %q, %d images to warp", msg, len(p.files.files))
	}
	a.proj.native = true
	p.matlab, a.opt.Jobs = matlab, 2
	tt.Frame()
	// Nothing is read by the pages after this one until it is warped.
	if len(a.norm.files.files) != 0 || len(a.ref.files.files) != 0 || a.normProblem() == "" || a.remedy("normalize", a.normProblem()) != "spatial" {
		t.Fatalf("images in native space are read before they are warped: %v, %q", a.norm.files.files, a.normProblem())
	}
	p.setSPM(dir)
	if a.spatialProblem() == "" {
		t.Fatal("a folder that is not SPM was taken for SPM")
	}
	p.setSPM(spm)
	if msg := a.spatialProblem(); msg != "" {
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
	if want := a.proj.in(filepath.Join("patients", "sub-pt00_space-MNI_pet.nii.gz")); p.written[0] != want {
		t.Fatalf("written %q, want %q", p.written[0], want)
	}
	if want := a.proj.in(filepath.Join("controls", "sub-pt02_space-MNI_pet.nii.gz")); p.written[1] != want {
		t.Fatalf("the control was written as %q, want %q", p.written[1], want)
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
	// The picture is of the first image that was warped.
	if p.shown != 0 || p.pic.key != p.written[0] {
		t.Fatalf("the picture is of row %d, %q", p.shown, p.pic.key)
	}
	// The pages after this one read what was warped, each of its group: the
	// patient that failed is left out, and the page is not done.
	tt.Frame()
	if !slices.Equal(a.norm.files.files, p.written) || !slices.Equal(a.norm.controls, p.written[1:]) || !slices.Equal(a.ref.files.files, p.written[1:]) || a.spatialDone() || a.done("spatial") {
		t.Fatalf("the warped images were not handed on: patients %v, controls %v", a.norm.files.files, a.ref.files.files)
	}
	// The project opened again knows what was warped.
	again := newApp()
	if err := again.openProject(a.proj.dir); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(again.norm.files.files, p.written) || len(again.spatial.rows) != 3 || !again.spatial.rows[1].failed {
		t.Fatalf("the project opened again reads %v, rows %+v", again.norm.files.files, again.spatial.rows)
	}

	// Two images of a group that would be written under one name.
	other := filepath.Join(dir, "other", filepath.Base(pt[0]))
	a.proj.patients.files = []string{pt[0], other}
	a.feed()
	if a.spatialProblem() == "" {
		t.Fatal("two images with one output name were accepted")
	}
}

// TestPages draws every page, empty and filled, in both languages and both
// appearances.
func TestPages(t *testing.T) {
	defer func() { chinese = false }()
	empty := t.TempDir()
	for _, chinese = range []bool{false, true} {
		for _, filled := range []bool{false, true} {
			for _, dark := range []bool{false, true} {
				for _, p := range pages {
					a := newApp()
					if filled {
						a.demo()
					} else {
						a.proj = &project{dir: empty, name: "Empty", unsaved: true}
						a.projectOutputs()
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
	for _, a.norm.main = range []string{"cerebellum", "hn"} {
		a.setReference("")
		if got := p.smoothing(&a.opt); got != 8 {
			t.Errorf("%s without a reference: %g mm, want 8", a.norm.main, got)
		}
		write(6)
		a.setReference(ref)
		if got := p.smoothing(&a.opt); got != 6 {
			t.Errorf("%s with a reference smoothed by 6 mm: %g mm", a.norm.main, got)
		}
		write(0)
		a.setReference(ref) // as after building it anew
		if got := p.smoothing(&a.opt); got != 0 {
			t.Errorf("%s with an unsmoothed reference: %g mm", a.norm.main, got)
		}
		a.opt.CmpFWHMAuto, a.opt.CmpFWHM = false, 10
		if got := p.smoothing(&a.opt); got != 10 {
			t.Errorf("%s with 10 mm asked for: %g mm", a.norm.main, got)
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

	a := testProject(t)
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

// TestOptionsKept checks that the options are kept in the project, that a
// file of an older version leaves the rest as the app first has it, and that
// values no run could use are not taken.
func TestOptionsKept(t *testing.T) {
	a := testProject(t)
	a.opt.Target, a.opt.Statistic, a.opt.Correction, a.opt.Q = 50, 1, 1, 0.01
	a.opt.RefStatsFor["white-matter"] = true
	a.saveProject()

	b := newApp()
	if err := b.openProject(a.proj.dir); err != nil {
		t.Fatal(err)
	}
	opt := b.opt
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
	// Another project has the options the app first has, not those of this one.
	c := testProject(t)
	if got := c.opt.changed(groupNormalize, groupCompare, groupReference); len(got) != 0 {
		t.Fatalf("a new project has the options of another: %v", got)
	}

	// A file that says little of the options, and one with nonsense.
	s := projectState{Options: defaultOptions()}
	if err := json.Unmarshal([]byte(`{"name": "old", "options": {"target": -3, "reference_mask": "nowhere", "voxel_p": 0}}`), &s); err != nil {
		t.Fatal(err)
	}
	opt = s.Options
	opt.sound()
	d := defaultOptions()
	if opt.Target != d.Target || opt.RefMask != d.RefMask || opt.VoxelP != d.VoxelP || opt.FWHM != d.FWHM {
		t.Fatalf("unsound options were taken: %+v", opt)
	}
	// Every page that has options says which are in effect.
	for _, p := range pages {
		// Z-score Maps has none of its own: it takes what step 1 normalised.
		if p.id == "data" || p.id == "masks" || p.id == "zmap" {
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

// TestRemedy checks that the run bar offers the way out of a problem for
// that very problem only, in both languages.
func TestRemedy(t *testing.T) {
	defer func() { chinese = false }()
	for _, chinese = range []bool{false, true} {
		a := newApp()
		a.proj = &project{dir: "/study", unsaved: true}
		a.projectOutputs()
		// No images yet: that is what the page says, though it has no
		// reference either, and the Data page is where they are added.
		if got := a.remedy("normalize", a.normProblem()); got != "data" {
			t.Errorf("normalize without images: remedy %q for %q", got, a.normProblem())
		}
		file := []string{"/study/patients/sub-01_pet.nii.gz"}
		a.proj.patients.files, a.norm.files.files = file, file
		a.cluster.files.files = append(file, "/study/patients/sub-02_pet.nii.gz")
		a.cmp.g1.files, a.cmp.g2.files = a.cluster.files.files, a.cluster.files.files
		// The pages that rest on step 1 wait for it, and lead there.
		for page, step := range map[string]string{"zmap": "normalize", "classify": "normalize", "compare": "normalize"} {
			problem := map[string]string{"zmap": a.zmapProblem(), "classify": a.classifyProblem(), "compare": a.cmpProblem()}[page]
			if got := a.remedy(page, problem); got != step {
				t.Errorf("%s before step 1 is done: remedy %q for %q", page, got, problem)
			}
		}
		a.norm.methods = map[string]bool{"cerebellum": true}
		a.norm.done = &normDone{asked: file, files: file, methods: a.norm.chosen(), settings: a.opt.normSettings(a.norm.chosen())}
		if got := a.remedy("classify", a.classifyProblem()); got != "zmap" {
			t.Errorf("classify before step 2 is done: remedy %q for %q", got, a.classifyProblem())
		}
		a.zmap.done = &zmapDone{from: a.norm.done, files: file, methods: []string{"hn"}}
		// With the steps before them done, they lack a reference.
		a.norm.methods["hn"] = true
		a.norm.done.methods = a.norm.chosen()
		problems := map[string]string{"normalize": a.normProblem(), "zmap": a.zmapProblem(), "classify": a.classifyProblem(),
			"cluster": a.clusterProblem()}
		for page, problem := range problems {
			if got := a.remedy(page, problem); got != "reference" {
				t.Errorf("%s without a reference: remedy %q for %q", page, got, problem)
			}
		}
		// A method that needs none, and a page that has one.
		a.norm.methods = map[string]bool{"cerebellum": true}
		a.norm.done.methods = a.norm.chosen()
		a.zmap.reference = "/study/hc_reference"
		if got := a.remedy("normalize", a.normProblem()); got != "" {
			t.Errorf("normalize by the cerebellum: remedy %q for %q", got, a.normProblem())
		}
		if got := a.remedy("zmap", a.zmapProblem()); got != "" {
			t.Errorf("zmap with a reference: remedy %q for %q", got, a.zmapProblem())
		}
		// What step 1 ran with was changed since: the steps after it wait again.
		a.norm.files.files = append(file, "/study/patients/sub-03_pet.nii.gz")
		if got := a.remedy("zmap", a.zmapProblem()); got != "normalize" || a.stepDone("normalize") {
			t.Errorf("zmap after step 1 was changed: remedy %q for %q", got, a.zmapProblem())
		}
		// Spatial Normalisation: the settings mend SPM, not the output folder.
		a.proj.native = true
		a.spatial.files.files = file
		if a.spatial.spm == "" {
			if got := a.remedy("spatial", a.spatialProblem()); got != "spm" {
				t.Errorf("spatial without SPM: remedy %q for %q", got, a.spatialProblem())
			}
		}
		if got := a.remedy("spatial", "Choose the output folder."); got != "" {
			t.Errorf("spatial without an output folder: remedy %q", got)
		}
	}
}

// TestFollowCompare checks that Compare Groups compares by one of the
// methods step 1 normalised with: the one chosen on its page while it is
// among them, else the leading one.
func TestPrimaryMethod(t *testing.T) {
	a := newApp()
	if got := a.norm.primary(); got != "hn" {
		t.Fatalf("of the methods a new project chooses: %q", got)
	}
	choose := func(methods ...string) string {
		a.norm.methods = map[string]bool{}
		for _, m := range methods {
			a.norm.methods[m] = true
		}
		return a.norm.primary()
	}
	if got := choose("cerebellum"); got != "cerebellum" {
		t.Errorf("of one method: %q", got)
	}
	// HN before the others, and a custom region only when there is no other.
	if got := choose("hn", "pons", "region"); got != "hn" {
		t.Errorf("of several methods %q was taken", got)
	}
	if got := choose("region"); got != "region" {
		t.Errorf("with a custom region alone: %q", got)
	}
	if got := choose(); got != "" {
		t.Errorf("of none: %q", got)
	}
	// Made primary in step 1, while it is chosen.
	a.norm.main = "pons"
	if got := choose("hn", "pons"); got != "pons" {
		t.Errorf("the method made primary was replaced by %q", got)
	}
	if got := choose("hn", "cerebellum"); got != "hn" {
		t.Errorf("a method no longer chosen stayed primary: %q", got)
	}
	// The classification waits for a primary method step 2 made maps with.
	if why := primaryWithoutMap("region"); !strings.Contains(why, methodName("region")) {
		t.Errorf("the problem does not name the method: %q", why)
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

// TestSwapGroups checks that swapping the groups of a comparison takes the
// result off the page, whose T-map has the sign of the groups as they were.
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
	// The groups stay swapped when the page takes the images of the project
	// again, and the page says which group is which.
	a.page = "compare"
	tt := ui.NewTesterWith(a.view, 1060, 900, ui.TesterOptions{})
	tt.Frame()
	if !slices.Equal(p.g1.files, g2) || !slices.Equal(p.g2.files, g1) || !tt.HasText("Group 1 · Healthy controls") || !tt.HasText("Group 2 · Patients") {
		t.Errorf("the groups do not stay swapped (shown: %q)", tt.Texts())
	}
}
