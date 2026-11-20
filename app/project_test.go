package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

// testProject is an app with a new, empty project open, in a folder of its own.
func testProject(t *testing.T) *app {
	t.Helper()
	a := newApp()
	if err := a.createProject(filepath.Join(t.TempDir(), "project"), "Test", false); err != nil {
		t.Fatal(err)
	}
	return a
}

// writeFile writes a small file that passes for an image.
func writeFile(t *testing.T, path, text string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestProject goes through the life of a project as a user does: made on
// the page the app opens with, given images that stay where they are or are
// copied in, closed, opened again from the list, and moved.
func TestProject(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(root, "elsewhere")
	pt := []string{writeFile(t, filepath.Join(elsewhere, "sub-pt01_pet.nii.gz"), "one"), writeFile(t, filepath.Join(elsewhere, "sub-pt02_pet.nii.gz"), "two")}
	hc := []string{writeFile(t, filepath.Join(elsewhere, "sub-hc01_pet.nii.gz"), "three"), writeFile(t, filepath.Join(elsewhere, "sub-hc02_pet.nii"), "four")}

	a := newApp()
	queue := make(chan func(), 64)
	a.post = func(fn func()) { queue <- fn }
	wait := func(tt *ui.Tester) {
		t.Helper()
		timeout := time.After(time.Minute)
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
	tt := ui.NewTesterWith(a.view, 1060, 900, ui.TesterOptions{})
	click := func(label string) {
		t.Helper()
		tt.Frame()
		if err := tt.Click(label); err != nil {
			t.Fatalf("%v (shown: %q)", err, tt.Texts())
		}
		tt.Frame()
	}
	tt.Frame()
	// Without a project there is nothing to analyse: only where to get one.
	if a.proj != nil || !tt.HasText("New Project…") || !tt.HasText("Open Project…") || tt.HasText("Intensity Normalisation") {
		t.Fatalf("the app does not open with the projects: %q", tt.Texts())
	}
	click("New Project…")
	if !tt.HasText("Create Project") || a.welcome.problem() == "" {
		t.Fatalf("the form of a new project: %q", tt.Texts())
	}
	a.welcome.name, a.welcome.parent = "trial", root
	click("Create Project")
	dir := filepath.Join(root, "trial")
	p := a.proj
	if p == nil || p.dir != dir || p.name != "trial" || a.page != "data" || !tt.HasText("Close Project") {
		t.Fatalf("no project was made: %+v, error %q (shown: %q)", p, a.welcome.err, tt.Texts())
	}
	for _, e := range core.StudyLayout {
		if info, err := os.Stat(filepath.Join(dir, e.Name)); err != nil || info.IsDir() != e.Dir {
			t.Errorf("%s was not created", e.Name)
		}
	}
	file := filepath.Join(dir, projectFileName)
	read := func() string {
		t.Helper()
		tt.Frame() // what changed is written after the frame
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	if text := read(); !strings.Contains(text, `"name": "trial"`) || strings.Contains(text, dir) {
		t.Fatalf("the project file: %s", text)
	}
	if a.zmap.output != filepath.Join(dir, "zmaps") || a.ref.output != filepath.Join(dir, "hc_reference") {
		t.Errorf("the pages do not write into the project: %q, %q", a.zmap.output, a.ref.output)
	}
	var kept settings
	if err := json.Unmarshal([]byte(a.settingsText()), &kept); err != nil || kept.Project != dir || !slices.Equal(kept.Recent, []string{dir}) || kept.Projects != root {
		t.Fatalf("the app does not remember the project: %+v (%v)", kept, err)
	}
	// A project of that name is there now.
	a.welcome.name = "trial"
	if a.welcome.problem() == "" || a.createProject(dir, "again", false) == nil {
		t.Error("a second project was made in the folder of the first")
	}

	// Images that stay where they are: the project notes where.
	a.addImages(&p.patients, "patients", pt)
	if !slices.Equal(p.patients.files, pt) || a.job != nil {
		t.Fatalf("the patients were not taken as they are: %v", p.patients.files)
	}
	if text := read(); !strings.Contains(text, filepath.ToSlash(pt[0])) && !strings.Contains(text, strings.ReplaceAll(pt[0], `\`, `\\`)) {
		t.Fatalf("the project file does not say where the patients are: %s", text)
	}
	if !slices.Equal(a.norm.files.files, pt) || !slices.Equal(a.cmp.g1.files, pt) || !slices.Equal(a.cluster.files.files, pt) {
		t.Errorf("the pages do not take the patients of the project: %v", a.norm.files.files)
	}
	// Images that are copied in, the same twice: one copy, listed once.
	click("Copy into Project")
	if !p.copies {
		t.Fatal("the project does not copy its images")
	}
	for range 2 {
		a.addImages(&p.controls, "controls", []string{elsewhere})
		wait(tt)
	}
	copied := []string{filepath.Join(dir, "controls", "sub-hc01_pet.nii.gz"), filepath.Join(dir, "controls", "sub-hc02_pet.nii"),
		filepath.Join(dir, "controls", "sub-pt01_pet.nii.gz"), filepath.Join(dir, "controls", "sub-pt02_pet.nii.gz")}
	if !slices.Equal(p.controls.files, copied) || p.st.err != "" || !tt.HasText(p.st.ok) {
		t.Fatalf("the controls were not copied in: %v, %+v, notes %v", p.controls.files, p.st, p.notes)
	}
	if text, _ := os.ReadFile(copied[0]); string(text) != "three" {
		t.Fatalf("the copy holds %q", text)
	}
	if text, _ := os.ReadFile(hc[0]); string(text) != "three" {
		t.Fatal("the image that was copied is gone from where it was")
	}
	// Another image of a name that is taken is not written over it.
	other := writeFile(t, filepath.Join(root, "other", "sub-hc01_pet.nii.gz"), "another")
	a.addImages(&p.controls, "controls", []string{other})
	wait(tt)
	if len(p.controls.files) != 4 || len(p.notes) != 1 || p.st.err == "" {
		t.Fatalf("an image was copied over another: %v, notes %v", p.controls.files, p.notes)
	}
	if text, _ := os.ReadFile(copied[0]); string(text) != "three" {
		t.Fatal("the first copy was written over")
	}
	// What lies in the project is written by where it is in it.
	text := read()
	if !strings.Contains(text, `"$PROJECT/controls/sub-hc01_pet.nii.gz"`) || strings.Contains(text, filepath.ToSlash(dir)+"/controls") {
		t.Fatalf("the copied images are not written as of the project: %s", text)
	}
	if !slices.Equal(a.ref.files.files, copied) {
		t.Errorf("the Reference page does not take the controls of the project: %v", a.ref.files.files)
	}

	// Some choices of the pages, and a run that is noted.
	a.norm.methods = map[string]bool{"cerebellum": true, "pons": true}
	a.cmp.name, a.cmp.swapped, a.opt.Target, a.page = "pd_vs_hc", true, 50, "compare"
	mapOf := writeFile(t, filepath.Join(dir, "normalised", "sub-pt01_desc-cerebellum_pet.nii.gz"), "normalised")
	a.norm.done = &normDone{asked: pt, files: pt[:1], methods: a.norm.chosen(), settings: a.opt.normSettings(a.norm.chosen()),
		images: map[string]map[string]string{pt[0]: {"cerebellum": mapOf}}, factors: map[string]map[string]float64{pt[0]: {"cerebellum": 5.5}}, target: 50}
	a.norm.done.stamp()
	a.norm.st.ok, a.norm.outDir = "normalised", filepath.Join(dir, "normalised")
	a.norm.rows = []normRow{{image: "sub-pt01_pet.nii.gz", method: "cerebellum", factor: "5.5", value: 5.5}}
	if !a.stepDone("normalize") {
		t.Fatalf("the made-up run of step 1 does not stand: %q", a.normChanged())
	}
	text = read()
	for _, want := range []string{`"$PROJECT/normalised/sub-pt01_desc-cerebellum_pet.nii.gz"`, `"swapped": true`, `"target": 50`, `"page": "compare"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("the project file lacks %s: %s", want, text)
		}
	}

	// Closed: the app is back at the projects, with this one listed.
	a.page = "data"
	click("Close Project")
	tt.Frame()
	if a.proj != nil || !tt.HasText("Recent projects") || !tt.HasText("trial") || a.opt.Target == 50 || len(a.norm.files.files) != 0 {
		t.Fatalf("the project was not left: %q", tt.Texts())
	}
	kept = settings{}
	if err := json.Unmarshal([]byte(a.settingsText()), &kept); err != nil || kept.Project != "" || len(kept.Recent) != 1 {
		t.Fatalf("after closing the app remembers %+v", kept)
	}
	// Opened from the list: where it was left.
	click("trial")
	p = a.proj
	if p == nil || !p.copies || a.page != "data" || !slices.Equal(p.patients.files, pt) || !slices.Equal(p.controls.files, copied) {
		t.Fatalf("the project was not opened as it was left: %+v (%q)", p, a.welcome.err)
	}
	if !a.norm.methods["pons"] || a.norm.methods["hn"] || a.cmp.name != "pd_vs_hc" || !a.cmp.swapped || a.opt.Target != 50 {
		t.Errorf("the choices of the pages were not kept: %v, %q, %v, %g", a.norm.methods, a.cmp.name, a.cmp.swapped, a.opt.Target)
	}
	if !a.stepDone("normalize") || a.norm.done.images[pt[0]]["cerebellum"] != mapOf || a.norm.done.factors[pt[0]]["cerebellum"] != 5.5 || len(a.norm.rows) != 1 {
		t.Fatalf("the run of step 1 was not kept: %+v (changed: %q)", a.norm.done, a.normChanged())
	}
	if !slices.Equal(a.cmp.g1.files, copied) || !slices.Equal(a.cmp.g2.files, pt) {
		t.Errorf("the groups are not swapped as they were: %v", a.cmp.g1.files)
	}
	// Opening it writes nothing: the file is as it was.
	before, _ := os.Stat(file)
	tt.Frame()
	if after, _ := os.Stat(file); !after.ModTime().Equal(before.ModTime()) {
		t.Error("the project file was written again on opening it")
	}

	// Moved: what lies in the project is found where the project is now.
	a.page = "data"
	click("Close Project")
	moved := filepath.Join(root, "moved")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	a.seen = nil // a second later: what is there is asked anew
	tt.Frame()
	if !tt.HasText("Cannot be reached") {
		t.Errorf("a project that was moved away is offered as if it were there: %q", tt.Texts())
	}
	if err := a.openProject(moved); err != nil {
		t.Fatal(err)
	}
	if got := a.proj.controls.files[0]; got != filepath.Join(moved, "controls", "sub-hc01_pet.nii.gz") || !slices.Equal(a.proj.patients.files, pt) {
		t.Fatalf("in the moved project the controls are at %q, the patients at %v", got, a.proj.patients.files)
	}
	if got := a.norm.done.images[pt[0]]["cerebellum"]; got != filepath.Join(moved, "normalised", filepath.Base(mapOf)) || !a.stepDone("normalize") {
		t.Fatalf("in the moved project step 1 wrote %q (changed: %q)", got, a.normChanged())
	}
	// What a step wrote is gone: the step is to be run again.
	os.Remove(a.norm.done.images[pt[0]]["cerebellum"])
	b := newApp()
	if err := b.openProject(moved); err != nil {
		t.Fatal(err)
	}
	if b.norm.done != nil || b.norm.st.err == "" || b.stepDone("normalize") {
		t.Fatalf("a step whose files are gone is still done: %+v", b.norm.st)
	}

	// A file of a later version, and one that is no project file.
	raw, _ := os.ReadFile(filepath.Join(moved, projectFileName))
	later := strings.Replace(string(raw), `"format": 1`, `"format": 99`, 1)
	os.WriteFile(filepath.Join(moved, projectFileName), []byte(later), 0o644)
	if err := b.openProject(moved); err == nil || !strings.Contains(err.Error(), "newer version") {
		t.Errorf("a project of a later version: %v", err)
	}
	os.WriteFile(filepath.Join(moved, projectFileName), []byte("{"), 0o644)
	if err := b.openProject(moved); err == nil || b.proj == nil || b.proj.dir != moved {
		t.Errorf("a file that cannot be read: %v, and the project that was open is %v", err, b.proj)
	}
	if err := b.openProject(elsewhere); err == nil {
		t.Error("a folder without a project was opened as one")
	}
}

// TestStudyFolder opens a study folder of an earlier version, which holds
// images, a reference and a table but no project: it is made a project that
// has them.
func TestStudyFolder(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	study := filepath.Join(dir, "study")
	if _, err := core.InitStudy(study); err != nil {
		t.Fatal(err)
	}
	pt := writeFile(t, filepath.Join(study, "patients", "sub-pt01_pet.nii.gz"), "one")
	hc := writeFile(t, filepath.Join(study, "controls", "sub-hc01_pet.nii.gz"), "two")
	writeFile(t, filepath.Join(study, "hc_reference", "reference.json"), `{"n_controls": 12, "fwhm_mm": 8}`)
	table := writeFile(t, filepath.Join(study, "participants.tsv"), core.ParticipantsHeader+"sub-pt01\t60\tF\nsub-hc01\t58\tM\n")

	a := newApp()
	a.recent = []string{study}
	tt := ui.NewTesterWith(a.view, 1060, 900, ui.TesterOptions{})
	tt.Frame()
	if !tt.HasText("Study folder") {
		t.Fatalf("a study folder is not told from a project: %q", tt.Texts())
	}
	if err := tt.Click("study"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	p := a.proj
	if p == nil || p.dir != study || p.name != "study" || p.copies || !isProject(study) {
		t.Fatalf("the study was not made a project: %+v (%q)", p, a.welcome.err)
	}
	if !slices.Equal(p.patients.files, []string{pt}) || !slices.Equal(p.controls.files, []string{hc}) {
		t.Errorf("the images of the study were not taken: %v, %v", p.patients.files, p.controls.files)
	}
	if a.norm.reference != filepath.Join(study, "hc_reference") || !a.ownReference() || !a.done("reference") {
		t.Errorf("the reference of the study is not in use: %q", a.norm.reference)
	}
	if a.cov.path != table || !slices.Equal(a.cov.names(), []string{"age", "sex"}) {
		t.Errorf("the table of the study is not used: %q, %v", a.cov.path, a.cov.names())
	}
	// A reference from elsewhere is used in its place, and the project's own
	// can be taken back.
	other := filepath.Join(dir, "lab_reference")
	writeFile(t, filepath.Join(other, "reference.json"), `{"n_controls": 40, "fwhm_mm": 8}`)
	a.setReference(other)
	a.page = "reference"
	tt.Frame()
	if a.ownReference() || !tt.HasText("Imported from "+shortDir(dir)) {
		t.Fatalf("an imported reference is not shown as one: %q", tt.Texts())
	}
	tt.Frame()
	b := newApp()
	if err := b.openProject(study); err != nil || b.norm.reference != other {
		t.Fatalf("the imported reference was not kept: %v, %q", err, b.norm.reference)
	}
	if err := tt.Click("Use the Project's Own"); err != nil {
		t.Fatalf("%v (shown: %q)", err, tt.Texts())
	}
	tt.Frame()
	if !a.ownReference() {
		t.Error("the project's own reference was not taken back")
	}
	// What is there is asked of the file system once, and again after a
	// second: until then a folder that has gone is still seen.
	patients := seenKey{filepath.Join(study, "patients"), true}
	if !a.onDisk(patients.path, true) || a.onDisk(patients.path, false) {
		t.Error("the folder of patients is not seen as a folder")
	}
	os.RemoveAll(patients.path)
	a.seen[patients] = seenFile{true, time.Now().Add(time.Hour)} // just seen, however slow the test
	if !a.onDisk(patients.path, true) {
		t.Error("the file system is asked on every frame")
	}
	a.seen[patients] = seenFile{true, time.Now().Add(-2 * time.Second)}
	if a.onDisk(patients.path, true) {
		t.Error("a folder that is gone is seen for good")
	}
}

// fill gives every field of a value something that is not its zero value,
// those that are not exported too.
func fill(v reflect.Value) {
	if !v.CanSet() {
		v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString("text")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int64:
		v.SetInt(7)
	case reflect.Float64:
		v.SetFloat(1.5)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fill(v.Elem())
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fill(v.Index(0))
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		key, value := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
		fill(key)
		fill(value)
		v.SetMapIndex(key, value)
	case reflect.Struct:
		for i := range v.NumField() {
			fill(v.Field(i))
		}
	}
}

// TestProjectRecords checks that what a page holds of its run is written
// into the project file whole: a field added to one of these types and not
// to how it is written fails here.
func TestProjectRecords(t *testing.T) {
	check := func(value, back any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, back); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(value, back) {
			t.Errorf("%T is not read back as it was written:\n%+v\n%+v\n%s", value, value, back, raw)
		}
	}
	whole := func(value any) any {
		fill(reflect.ValueOf(value).Elem())
		return value
	}
	check(whole(&status{}), &status{})
	check(whole(&fileStamp{}), &fileStamp{})
	check(whole(&normRow{}), &normRow{})
	check(whole(&zmapRow{}), &zmapRow{})
	check(whole(&classifyRow{}), &classifyRow{})
	check(whole(&spatialRow{}), &spatialRow{})
	norm := whole(&normDone{}).(*normDone)
	norm.changed = false // found when a project is opened, not written
	check(norm, &normDone{})
	zmap := whole(&zmapDone{}).(*zmapDone)
	zmap.from = nil // the file says whether it is the run step 1 holds
	check(zmap, &zmapDone{})

	// The made-up project of the snapshots, as a whole.
	a := newApp()
	a.demo()
	raw, err := json.MarshalIndent(a.projectState(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// Paths in the project are written by where they are in it, and read
	// back as they were.
	there := portable(raw, a.proj.dir)
	if strings.Contains(string(there), `"`+a.proj.dir) || !strings.Contains(string(there), `"$PROJECT/zmaps/`) {
		t.Fatalf("the paths of the project are not written as of it: %s", there)
	}
	if back := local(there, a.proj.dir); string(back) != string(raw) {
		t.Fatal("the paths of the project are not read back as they were")
	}
	if elsewhere := local(there, filepath.FromSlash("/mnt/shared/study")); !strings.Contains(string(elsewhere), filepath.Join(filepath.FromSlash("/mnt/shared/study"), "zmaps")) {
		t.Fatal("the paths of a project that was moved do not follow it")
	}
	var s projectState
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	again, err := json.MarshalIndent(&s, "", "  ")
	if err != nil || string(again) != string(raw) {
		t.Fatalf("the project is not read back as it was written (%v)", err)
	}
	if s.Steps.Normalize == nil || s.Steps.ZMap == nil || !s.Steps.ZMap.Current || s.Steps.Classify == nil || !s.Steps.Classify.Current ||
		s.Steps.Compare == nil || s.Steps.Compare.Result == nil || s.Steps.Cluster == nil || s.Steps.Spatial == nil {
		t.Fatalf("a step of the made-up project is not recorded: %+v", s.Steps)
	}
}
