package core

import (
	"bytes"
	"image/png"
	"math"
	"testing"
)

// The regions of the classification: both sides alike, none overlapping
// where they must not.
func TestClassRegions(t *testing.T) {
	masks := regionMasks()
	a := atlasOnce()
	for _, r := range ClassRegions {
		both := masks[regionKey{r.Name, "both"}]
		if countTrue(both) < 100 {
			t.Errorf("%s has %d voxels", r.Name, countTrue(both))
		}
		if !r.Paired {
			continue
		}
		left, right := masks[regionKey{r.Name, "left"}], masks[regionKey{r.Name, "right"}]
		nl, nr := countTrue(left), countTrue(right)
		if d := nl - nr; d*d*100 > nl*nl {
			t.Errorf("%s: %d voxels on the left, %d on the right", r.Name, nl, nr)
		}
		var xl, xr float64
		for i := range left {
			if left[i] && right[i] {
				t.Fatalf("%s: the sides overlap", r.Name)
			}
			x := a.aff[0][0]*float64(i%a.dim[0]) + a.aff[0][3]
			if left[i] {
				xl += x / float64(nl)
			}
			if right[i] {
				xr += x / float64(nr)
			}
		}
		if !(xl < 0 && xr > 0) {
			t.Errorf("%s: the left side is at x = %.1f mm, the right at %.1f", r.Name, xl, xr)
		}
	}
	for i, in := range masks[regionKey{"midbrain", "both"}] {
		if in && masks[regionKey{"pons", "both"}][i] {
			t.Fatal("midbrain and pons overlap")
		}
	}
	for i, in := range masks[regionKey{"medial-frontal", "both"}] {
		if in && masks[regionKey{"lateral-frontal", "both"}][i] {
			t.Fatal("medial and lateral frontal cortex overlap")
		}
	}
	if _, err := RegionMask("putamen", "left"); err != nil {
		t.Error(err)
	}
	if _, err := RegionMask("pons", "left"); err == nil {
		t.Error("the pons has no left side")
	}
}

// A grid that ends inside a region holds a part of it, and two images may
// not share the name of their reports.
func TestRegionViewAndReportStems(t *testing.T) {
	a := atlasOnce()
	whole := regionView(a.dim, a.aff)
	for key := range regionMasks() {
		if whole[key] != 1 {
			t.Errorf("%v: %v of the region on the grid of the atlas itself", key, whole[key])
		}
	}
	// The same grid without its lower slices: the origin moves up with it.
	cut := 0
	for cut = 0; cut < a.dim[2]; cut++ {
		if z := a.aff[2][2]*float64(cut) + a.aff[2][3]; z > -30 {
			break
		}
	}
	dim, aff := a.dim, a.aff
	dim[2] -= cut
	for r := 0; r < 3; r++ {
		aff[r][3] += a.aff[r][2] * float64(cut)
	}
	view := regionView(dim, aff)
	if share := view[regionKey{"cerebellum", "both"}]; !(share > 0.05 && share < 0.6) {
		t.Errorf("%v of the cerebellum above z = -30 mm", share)
	}
	if view[regionKey{"putamen", "left"}] != 1 || view[regionKey{"occipital", "both"}] != 1 {
		t.Errorf("putamen %v, occipital %v", view[regionKey{"putamen", "left"}], view[regionKey{"occipital", "both"}])
	}
	// What is scored of a cut region counts against the whole of it.
	mask, brain := []bool{true, true, false}, []bool{true, true, true}
	if got := summarise([]float32{-3, -3, 0}, brain, mask, 0.3); got.Z != nil || math.Abs(got.Coverage-0.3) > 1e-12 {
		t.Errorf("a region 30%% inside the grid: %+v", got)
	}
	if got := summarise([]float32{-3, -3, 0}, brain, mask, 1); got.Z == nil || *got.Z != -3 || got.Coverage != 1 {
		t.Errorf("a region inside the grid: %+v", got)
	}

	if _, _, _, found := SharedReportStem([]string{"a/sub-01_pet.nii.gz", "a/sub-02_pet.nii.gz", "b/wFDG.nii"}); found {
		t.Error("three different names were taken for a clash")
	}
	first, second, stem, found := SharedReportStem([]string{"a/pet.nii.gz", "c/x.nii", "b/pet.nii.gz"})
	if !found || first != "a/pet.nii.gz" || second != "b/pet.nii.gz" || stem != "pet" {
		t.Errorf("a/pet and b/pet: %q %q %q %v", first, second, stem, found)
	}
	if _, _, _, found := SharedReportStem([]string{"sub-01.nii.gz", "Sub-01_pet.nii"}); !found {
		t.Error("sub-01.nii.gz and Sub-01_pet.nii share their report")
	}
}

// The reading of the pattern scores, as tests/test_classify.py has it.
func TestConclude(t *testing.T) {
	// robust: what is left of a pattern without the unstable findings (all of it, if negative).
	readRobust := func(scores map[string]float64, robust float64) Conclusion {
		var patterns []Pattern
		for _, id := range Patterns {
			p := Pattern{ID: id, Score: scores[id], Robust: scores[id]}
			if robust >= 0 && p.Score != 0 {
				p.Robust = robust
			}
			patterns = append(patterns, p)
		}
		return conclude(patterns, "left")
	}
	read := func(scores map[string]float64) Conclusion { return readRobust(scores, -1) }
	for _, c := range []struct {
		scores map[string]float64
		want   Conclusion
	}{
		{map[string]float64{"psp": 0.8, "cbd": 0.4}, Conclusion{"pattern", "psp", "cbd", "high", "left"}},
		{map[string]float64{"psp": 0.8, "cbd": 0.6}, Conclusion{"pattern", "psp", "cbd", "moderate", "left"}},
		{map[string]float64{"psp": 0.6, "dlb": 0.5}, Conclusion{"indeterminate", "psp", "dlb", "low", ""}},
		// The two forms of MSA are one disease: the other form is not a rival.
		{map[string]float64{"msa-p": 0.9, "msa-c": 0.9}, Conclusion{"pattern", "msa-p", "", "high", ""}},
		{map[string]float64{"dlb": 0.4, "pd": 0.9}, Conclusion{"possible", "dlb", "", "low", ""}},
		{map[string]float64{"dlb": 0.2, "pd": 0.6}, Conclusion{"pd", "pd", "", "moderate", ""}},
		{map[string]float64{"pd": 0.35}, Conclusion{"pd", "pd", "", "low", ""}},
		{map[string]float64{"pd": 0.2}, Conclusion{Category: "none"}},
	} {
		if got := read(c.scores); got != c.want {
			t.Errorf("%v: %+v, want %+v", c.scores, got, c.want)
		}
	}
	// A pattern that rests on findings which change with the normalisation is assigned with less confidence.
	for robust, want := range map[float64]string{0.8: "high", 0.6: "moderate", 0.2: "low"} {
		got := readRobust(map[string]float64{"psp": 0.8}, robust)
		if got != (Conclusion{Category: "pattern", Pattern: "psp", Confidence: want}) {
			t.Errorf("robust %v: %+v, want confidence %s", robust, got, want)
		}
	}
	raw, _ := Conclusion{Category: "none"}.MarshalJSON()
	if want := `{"category":"none","pattern":null,"alternative":null,"confidence":null,"side":null}`; string(raw) != want {
		t.Errorf("a conclusion is written as %s", raw)
	}
}

// The texts of the report are there in both languages, numbers read as in
// the Python package, and the figure is a PNG other programs can read.
func TestReportParts(t *testing.T) {
	for key, entry := range reportTexts() {
		if entry["en"] == "" || entry["zh"] == "" || len(entry) != 2 {
			t.Errorf("text %q: %v", key, entry)
		}
	}
	for _, r := range ClassRegions {
		if regionName(r.Name, "zh") == "" {
			t.Errorf("region %s has no name", r.Name)
		}
	}
	for _, id := range Patterns {
		for _, kind := range []string{"name-", "expect-", "caveat-"} {
			if ReportText(kind+id, "en") == "" {
				t.Errorf("no text %s%s", kind, id)
			}
		}
	}
	if got := ReportText("item-both", "zh", "壳核", "−3.1", "−2.8"); got != "双侧壳核（左 −3.1，右 −2.8）" {
		t.Error(got)
	}
	values := []float64{-2.25, -0.04, 0.04, 3}
	for i, want := range []string{"−2.2", "0.0", "0.0", "3.0"} {
		if got := ScoreText(&values[i], 1); got != want {
			t.Errorf("ScoreText(%g) = %s, want %s", values[i], got, want)
		}
	}
	if level := -37.5; ScoreText(nil, 1) != "–" || ScoreText(&level, 0) != "−38" {
		t.Error("a missing score or a slice level is printed wrongly")
	}
	if Percent(0.745) != 75 || Percent(0.004) != 0 {
		t.Error("Percent does not round to the nearest")
	}

	rgb := make([]byte, 4*5*3)
	for i := range rgb {
		rgb[i] = byte(7 * i)
	}
	img, err := png.Decode(bytes.NewReader(EncodePNG(rgb, 4, 5)))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 5 || b.Dy() != 4 {
		t.Fatalf("the picture is %v", b)
	}
	if r, g, b, _ := img.At(2, 3).RGBA(); byte(r>>8) != rgb[51] || byte(g>>8) != rgb[52] || byte(b>>8) != rgb[53] {
		t.Error("the picture does not come back as it was")
	}
}
