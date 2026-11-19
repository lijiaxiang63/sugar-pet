package core

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The z-score of a t-statistic is finite however strong it is, and agrees
// with GrouVox 0.2.1.
func TestTToZ(t *testing.T) {
	for _, c := range []struct{ t, dof, want float64 }{ // grouvox.correction._t_to_z
		{0.5, 26, 0.4940361145648001},
		{5, 26, 4.147552585313663},
		{9, 100, 7.684489973879415},
		{20, 100, 12.658857948227011},
		{-20, 100, -12.658857948227011},
		{60, 30, 11.929457765408362},
		{200, 30, 14.626273931113698},
		{1e4, 30, 21.163491551675346},
		{1e6, 26, 25.110230015800322},
		{45, 1000, 33.2627165452198},
		{-300, 500, -50.96661157065154}, // the tail is below the smallest number
		{3e38, 26, 67.2100194031716},    // the largest t a T-map can hold
	} {
		got := tToZ(c.t, c.dof)
		if math.IsInf(got, 0) || math.IsNaN(got) || math.Abs(got-c.want) > 1e-12*math.Abs(c.want) {
			t.Errorf("tToZ(%g, %g) = %.17g, want %.17g", c.t, c.dof, got, c.want)
		}
	}
	// Where 1 - p loses nothing, it agrees with the older way.
	for _, tv := range []float64{0.5, 2, 3.3, 5} {
		got, old := tToZ(tv, 26), normPPF(1-tSF(tv, 26))
		if math.Abs(got-old) > 1e-9 {
			t.Errorf("tToZ(%g, 26) = %.12g, the older way gives %.12g", tv, got, old)
		}
	}
	// The log of the tail, on both sides of where the two ways meet.
	for _, c := range []struct{ t, dof, want float64 }{ // grouvox.correction._t_logsf
		{0.5, 26, -1.1691195080630312},
		{5, 26, -10.993987929230094},
		{45, 1000, -557.6284334557529},
		{300, 500, -1303.64824137489},
		{3e38, 26, -2263.7203362095725},
	} {
		if got := tLogSF(c.t, c.dof); math.Abs(got-c.want) > 1e-12*math.Abs(c.want) {
			t.Errorf("tLogSF(%g, %g) = %.17g, want %.17g", c.t, c.dof, got, c.want)
		}
	}
}

// The quantiles of the normal distribution agree with SciPy over the whole
// range, both tails.
func TestNormPPF(t *testing.T) {
	for _, c := range [][2]float64{ // scipy.stats.norm.ppf
		{0.5, 0.0},
		{0.3, -0.5244005127080409},
		{0.1, -1.2815515655446004},
		{0.075, -1.4395314709384563},
		{0.05, -1.6448536269514729},
		{0.01, -2.3263478740408408},
		{0.001, -3.090232306167813},
		{1e-05, -4.264890793922825},
		{1e-08, -5.612001244174789},
		{1e-10, -6.361340902404056},
		{1e-11, -6.706023155495138},
		{1e-12, -7.034483825301131},
		{1e-15, -7.941345326170998},
		{1e-20, -9.262340089798409},
		{1e-50, -14.933337534788487},
		{1e-100, -21.273453560965322},
		{1e-200, -30.20559417957964},
		{1e-290, -36.420731673207996},
		{1e-300, -37.0470962993612},
	} {
		if got := normPPF(c[0]); math.Abs(got-c[1]) > 1e-13*math.Max(1, math.Abs(c[1])) {
			t.Errorf("normPPF(%g) = %.17g, want %.17g", c[0], got, c[1])
		}
		if c[0] > 1e-10 {
			if got := normPPF(1 - c[0]); math.Abs(got+c[1]) > 1e-6 {
				t.Errorf("normPPF(1 - %g) = %.17g, want %.17g", c[0], got, -c[1])
			}
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

// spatialSetup makes, in dir, a folder that passes for SPM, a shell script
// that passes for MATLAB and a made-up image of a head.
func spatialSetup(t *testing.T, dir, script string) (spm, matlab, image string) {
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
	if err := os.WriteFile(matlab, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	dim, aff := AtlasGrid()
	vol := NewVolume(dim, aff)
	brain, _ := BuiltinMask("parenchyma")
	copy(vol.Data, brain.Data)
	image = filepath.Join(dir, "sub-01_pet.nii.gz")
	if err := SaveNifti(image, vol, false); err != nil {
		t.Fatal(err)
	}
	return spm, matlab, image
}

// copyingMatlab "warps" the images of the script it is given by copying them.
const copyingMatlab = `script=$(printf '%s' "$2" | sed "s/^run('//; s/')$//")
grep "^'.*,1'$" "$script" | sed "s/^'//; s/,1'$//" | while read -r f; do
  cp "$f" "$(dirname "$f")/w$(basename "$f")"
done
`

// TestSpatialStaleWorkDir checks that a warped image left in a work folder
// by an earlier run is not taken for the result of a run that failed.
func TestSpatialStaleWorkDir(t *testing.T) {
	dir := t.TempDir()
	spm, matlab, src := spatialSetup(t, dir, "echo 'no licence' >&2\nexit 1\n")
	work := filepath.Join(dir, "work")
	stale := NewVolume([3]int{4, 4, 4}, Affine{{2, 0, 0, 0}, {0, 2, 0, 0}, {0, 0, 2, 0}, {0, 0, 0, 1}})
	if err := SaveNifti(filepath.Join(work, "wimg00000.nii"), stale, false); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.nii.gz")
	results, err := SpatialNormalize(context.Background(), []string{src}, []string{out},
		SpatialOptions{SPMPath: spm, Matlab: matlab, WorkDir: work})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Err == nil || !strings.Contains(results[0].Err.Error(), "no licence") {
		t.Fatalf("the failed run reported %v, want what MATLAB printed", results[0].Err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("the image of an earlier run was written as the result")
	}
}

// TestSpatialSameOutput checks that two images that would be written to one
// file are refused before anything runs.
func TestSpatialSameOutput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "pet_space-MNI.nii.gz")
	_, err := SpatialNormalize(context.Background(), []string{"a/pet.nii.gz", "b/pet.nii.gz"},
		[]string{out, filepath.Join(filepath.Dir(out), ".", filepath.Base(out))}, SpatialOptions{})
	if err == nil || !strings.Contains(err.Error(), "a/pet.nii.gz and b/pet.nii.gz would both be written") {
		t.Fatalf("error %v, want that of the shared output", err)
	}
}

// TestSpatialRelativeMatlab checks that MATLAB given by a path relative to
// the working folder is still found once it runs in the work folder.
func TestSpatialRelativeMatlab(t *testing.T) {
	dir := t.TempDir()
	spm, _, src := spatialSetup(t, dir, copyingMatlab)
	t.Chdir(dir)
	out := filepath.Join(dir, "out.nii.gz")
	results, err := SpatialNormalize(context.Background(), []string{src}, []string{out},
		SpatialOptions{SPMPath: spm, Matlab: "./matlab"})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

// TestSpatialCancel checks that cancelling a spatial normalisation returns
// at once and takes with it what the stand-in for MATLAB started.
func TestSpatialCancel(t *testing.T) {
	dir := t.TempDir()
	// As MATLAB's launcher, it starts another process and waits for it.
	pidFile := filepath.Join(dir, "child.pid")
	spm, matlab, src := spatialSetup(t, dir, "sleep 300 &\necho $! > '"+pidFile+"'\nwait\n")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for { // once the stand-in runs
			if _, err := os.Stat(pidFile); err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := SpatialNormalize(ctx, []string{src}, []string{filepath.Join(dir, "out.nii.gz")},
		SpatialOptions{SPMPath: spm, Matlab: matlab})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want that of the cancelled context", err)
	}
	if took := time.Since(start); took > 30*time.Second {
		t.Fatalf("cancelling took %v", took)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	for i := 0; i < 100; i++ {
		if exec.Command("kill", "-0", strconv.Itoa(pid)).Run() != nil {
			return // gone
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("process %d outlived the cancelled run", pid)
}

// TestInitStudy checks that a study folder is laid out as the guide
// recommends, and that what is there already is kept.
func TestInitStudy(t *testing.T) {
	study := filepath.Join(t.TempDir(), "study")
	created, err := InitStudy(study)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range StudyLayout {
		info, err := os.Stat(filepath.Join(study, e.Name))
		if err != nil || info.IsDir() != e.Dir || !created[i] {
			t.Errorf("%s: not created as it should be", e.Name)
		}
	}
	table := filepath.Join(study, "participants.tsv")
	if text, _ := os.ReadFile(table); string(text) != ParticipantsHeader {
		t.Errorf("participants.tsv holds %q", text)
	}

	// Again: the table that was filled is kept, the folder that went is made.
	filled := ParticipantsHeader + "sub-01\t60\tF\n"
	os.WriteFile(table, []byte(filled), 0o644)
	os.Remove(filepath.Join(study, "zmaps"))
	created, err = InitStudy(study)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range StudyLayout {
		if created[i] != (e.Name == "zmaps") {
			t.Errorf("%s: created is %v", e.Name, created[i])
		}
	}
	if text, _ := os.ReadFile(table); string(text) != filled {
		t.Errorf("participants.tsv was changed to %q", text)
	}

	// A file in the way is reported, and nothing is made.
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, "patients"), nil, 0o644)
	if _, err := InitStudy(other); err == nil {
		t.Error("a file named patients was accepted as the folder")
	}
	if _, err := os.Stat(filepath.Join(other, "raw")); err == nil {
		t.Error("folders were made although the study could not be laid out")
	}

	// A link that leads nowhere is not written through, and nothing is made;
	// a link to a real folder is the folder.
	if runtime.GOOS == "windows" {
		return // making links needs a privilege there
	}
	base := t.TempDir()
	outside := filepath.Join(base, "outside.tsv")
	linked := filepath.Join(base, "linked")
	os.Mkdir(linked, 0o755)
	if err := os.Symlink(outside, filepath.Join(linked, "participants.tsv")); err != nil {
		t.Fatal(err)
	}
	if _, err := InitStudy(linked); err == nil || !strings.Contains(err.Error(), "leads nowhere") {
		t.Errorf("a link that leads nowhere gave %v", err)
	}
	if _, err := os.Stat(outside); err == nil {
		t.Error("the table was written through the link")
	}
	if _, err := os.Stat(filepath.Join(linked, "raw")); err == nil {
		t.Error("folders were made beside a link that leads nowhere")
	}
	elsewhere, real := filepath.Join(base, "elsewhere"), filepath.Join(base, "real")
	os.Mkdir(elsewhere, 0o755)
	os.Mkdir(real, 0o755)
	os.Symlink(elsewhere, filepath.Join(real, "patients"))
	created, err = InitStudy(real)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range StudyLayout {
		if created[i] == (e.Name == "patients") {
			t.Errorf("%s beside a linked folder: created is %v", e.Name, created[i])
		}
	}

	// A path through a link and back is the folder the system reads it as,
	// not the one its spelling shortens to.
	site, here := filepath.Join(base, "data", "site"), filepath.Join(base, "here")
	os.MkdirAll(site, 0o755)
	os.MkdirAll(filepath.Join(here, "study"), 0o755)
	os.Symlink(site, filepath.Join(here, "link"))
	through := here + string(filepath.Separator) + "link" + string(filepath.Separator) + ".." + string(filepath.Separator) + "study"
	if _, err := InitStudy(through); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "data", "study", "patients")); err != nil {
		t.Error("the study was not laid out where the path leads")
	}
	if _, err := os.Stat(filepath.Join(here, "study", "patients")); err == nil {
		t.Error("the study was laid out in the folder the path's spelling shortens to")
	}
}
