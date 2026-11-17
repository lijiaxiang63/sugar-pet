package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
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
// build a reference, normalise, make z-score maps, compare two groups and
// find a reference cluster.
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
	exists := func(path string) {
		t.Helper()
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}

	// The reference, with the cerebellum's statistics and W-scores.
	ref := filepath.Join(dir, "reference")
	a.ref.files.files, a.ref.output = hc, ref
	a.ref.statsFor["cerebellum"] = true
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

	// Normalisation by two methods.
	out := filepath.Join(dir, "normalised")
	a.norm.files.files, a.norm.output = pt, out
	a.norm.methods = map[string]bool{"hn": true, "cerebellum": true}
	press("normalize", "开始归一化", &a.norm.st)
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

	// The reference cluster: the cerebellum, which the disease spares.
	mask := filepath.Join(dir, "cluster.nii.gz")
	a.cluster.files.files, a.cluster.output = pt, mask
	press("cluster", "查找参考簇", &a.cluster.st)
	if a.cluster.volume < 20 {
		t.Fatalf("the reference cluster has %g mL", a.cluster.volume)
	}
	exists(mask)
	if err := tt.Click("用该区域做归一化"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != "normalize" || a.norm.region.file != mask || !a.norm.methods["region"] {
		t.Fatal("the cluster was not handed to the Normalise page")
	}
	press("normalize", "开始归一化", &a.norm.st)
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
		if got := p.smoothing(); got != 8 {
			t.Errorf("%s without a reference: %g mm, want 8", p.method, got)
		}
		write(6)
		a.setReference(ref)
		if got := p.smoothing(); got != 6 {
			t.Errorf("%s with a reference smoothed by 6 mm: %g mm", p.method, got)
		}
		write(0)
		a.setReference(ref) // as after building it anew
		if got := p.smoothing(); got != 0 {
			t.Errorf("%s with an unsmoothed reference: %g mm", p.method, got)
		}
		p.fwhmAuto, p.fwhm = false, 10
		if got := p.smoothing(); got != 10 {
			t.Errorf("%s with 10 mm asked for: %g mm", p.method, got)
		}
		p.fwhmAuto = true
	}
	a.setReference(filepath.Join(ref, "missing"))
	if got := p.smoothing(); got != 8 {
		t.Errorf("with a reference that cannot be read: %g mm, want 8", got)
	}
}
