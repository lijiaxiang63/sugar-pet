package core

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// The z-score of a strong t-statistic is finite, and that of an everyday one
// is what the quantile of the upper tail gives.
func TestTToZ(t *testing.T) {
	for _, c := range []struct{ t, dof, want float64 }{
		{20, 100, 12.658857948227011}, // SciPy: -norm.ppf(t.sf(20, 100))
		{-20, 100, -12.658857948227011},
		{9, 100, 7.684489973879415},
		{60, 30, 11.929457765408362},
	} {
		got := tToZ(c.t, c.dof)
		if math.IsInf(got, 0) || math.Abs(got-c.want) > 1e-4*math.Abs(c.want) {
			t.Errorf("tToZ(%g, %g) = %g, want %g", c.t, c.dof, got, c.want)
		}
	}
	// Where 1 - p loses nothing, it agrees with GrouVox's way.
	for _, tv := range []float64{0.5, 2, 3.3, 5} {
		got, old := tToZ(tv, 26), normPPF(1-tSF(tv, 26))
		if math.Abs(got-old) > 1e-9 {
			t.Errorf("tToZ(%g, 26) = %.12g, GrouVox gives %.12g", tv, got, old)
		}
	}
}

// A slope of 0 in the header means the values are not scaled, so the
// intercept is not added either (as nibabel reads it).
func TestNiftiScaling(t *testing.T) {
	_, aff := AtlasGrid()
	vol := &Volume{Dim: [3]int{2, 2, 2}, NT: 1, Data: []float32{0, 1, 2, 3, 4, 5, 6, 7}, Aff: aff}
	path := filepath.Join(t.TempDir(), "image.nii")
	if err := SaveNifti(path, vol, false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	read := func(slope, inter float32) []float32 {
		t.Helper()
		binary.LittleEndian.PutUint32(raw[112:], math.Float32bits(slope))
		binary.LittleEndian.PutUint32(raw[116:], math.Float32bits(inter))
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := LoadNifti(path)
		if err != nil {
			t.Fatal(err)
		}
		return got.Data
	}
	for _, slope := range []float32{0, float32(math.NaN()), float32(math.Inf(1))} {
		if got := read(slope, 100); got[5] != 5 || got[0] != 0 {
			t.Errorf("slope %g, intercept 100: read %v, want the stored values", slope, got)
		}
	}
	if got := read(2, 100); got[5] != 110 || got[0] != 100 {
		t.Errorf("slope 2, intercept 100: read %v", got)
	}
}
