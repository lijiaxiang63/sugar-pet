package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// Spatial normalisation: warping native-space PET (no MRI) onto a PET
// template in MNI space with SPM's "Old Normalise".  It is the one thing
// here that needs MATLAB and SPM12/SPM25; the images are prepared and
// collected in Go, the warp itself is SPM's.

var (
	// Rough centre of the brain in MNI coordinates (mm).  The head's centre
	// of mass is moved here so that SPM's affine search starts close to the
	// answer.
	mniBrainCentre = [3]float64{0, -18, 8}
	// Half-size (mm) of the box kept around the head before registration.
	cropHalfSize = [3]float64{110, 130, 110}
)

const matlabTemplate = `addpath('%[1]s');
addpath(fullfile('%[1]s', 'toolbox', 'OldNorm'));
spm('defaults', 'PET');
spm_jobman('initcfg');
spm_get_defaults('cmdline', true);
files = {
%[4]s
};
for i = 1:numel(files)
    try
        clear matlabbatch
        matlabbatch{1}.spm.tools.oldnorm.estwrite.subj.source = files(i);
        matlabbatch{1}.spm.tools.oldnorm.estwrite.subj.wtsrc = '';
        matlabbatch{1}.spm.tools.oldnorm.estwrite.subj.resample = files(i);
        matlabbatch{1}.spm.tools.oldnorm.estwrite.eoptions.template = {'%[2]s,1'};
        matlabbatch{1}.spm.tools.oldnorm.estwrite.eoptions.weight = '';
        matlabbatch{1}.spm.tools.oldnorm.estwrite.eoptions.smosrc = 8;
        matlabbatch{1}.spm.tools.oldnorm.estwrite.eoptions.smoref = %[3]d;
        matlabbatch{1}.spm.tools.oldnorm.estwrite.eoptions.regtype = 'mni';
        matlabbatch{1}.spm.tools.oldnorm.estwrite.eoptions.cutoff = 25;
        matlabbatch{1}.spm.tools.oldnorm.estwrite.eoptions.nits = 16;
        matlabbatch{1}.spm.tools.oldnorm.estwrite.eoptions.reg = 1;
        matlabbatch{1}.spm.tools.oldnorm.estwrite.roptions.preserve = 0;
        matlabbatch{1}.spm.tools.oldnorm.estwrite.roptions.bb = [-90 -126 -72; 90 90 108];
        matlabbatch{1}.spm.tools.oldnorm.estwrite.roptions.vox = [2 2 2];
        matlabbatch{1}.spm.tools.oldnorm.estwrite.roptions.interp = 1;
        matlabbatch{1}.spm.tools.oldnorm.estwrite.roptions.wrap = [0 0 0];
        matlabbatch{1}.spm.tools.oldnorm.estwrite.roptions.prefix = 'w';
        spm_jobman('run', matlabbatch);
    catch err
        fprintf(2, 'FAILED %%s: %%s\n', files{i}, err.message);
    end
end
`

// matlabQuote makes a path safe inside a MATLAB string in single quotes.
func matlabQuote(path string) string { return strings.ReplaceAll(path, "'", "''") }

// PrepareNative crops a native PET around the head and puts its origin near
// the brain centre, writing the result to dst.
//
// Scanner images often have their origin far outside the head, which makes
// SPM's registration fail.  Only the header and the field of view change;
// voxel values are untouched (a 4D image is averaged over its frames).
func PrepareNative(src, dst string) error {
	vol, err := LoadNifti(src)
	if err != nil {
		return err
	}
	n := vol.NVox()
	data := make([]float32, n)
	if vol.NT == 1 {
		copy(data, vol.Data)
	} else {
		parallelFor(n, func(lo, hi int) {
			for i := lo; i < hi; i++ {
				sum := 0.0
				for t := 0; t < vol.NT; t++ {
					sum += float64(vol.Data[t*n+i])
				}
				data[i] = float32(sum / float64(vol.NT))
			}
		})
	}
	for i, v := range data {
		if f := float64(v); math.IsNaN(f) || math.IsInf(f, 0) {
			data[i] = 0
		}
	}

	// Brain uptake is far higher than anything else in a head scan, so a
	// high threshold isolates it well enough for a centre of mass.
	sorted := slices.Clone(data)
	slices.Sort(sorted)
	pos := 0.995 * float64(n-1)
	k := int(math.Floor(pos))
	p := float64(sorted[n-1])
	if k < n-1 {
		p = float64(sorted[k]) + (float64(sorted[k+1])-float64(sorted[k]))*(pos-float64(k))
	}
	thr := float32(0.4) * float32(p)
	var sum [3]float64
	count := 0
	dim := vol.Dim
	for z := 0; z < dim[2]; z++ {
		for y := 0; y < dim[1]; y++ {
			row := (z*dim[1] + y) * dim[0]
			for x := 0; x < dim[0]; x++ {
				if data[row+x] > thr {
					sum[0] += float64(x)
					sum[1] += float64(y)
					sum[2] += float64(z)
					count++
				}
			}
		}
	}
	if count == 0 {
		return fmt.Errorf("%s: image looks empty", src)
	}
	apply := func(v [3]float64) (out [3]float64) {
		for r := 0; r < 3; r++ {
			out[r] = vol.Aff[r][0]*v[0] + vol.Aff[r][1]*v[1] + vol.Aff[r][2]*v[2] + vol.Aff[r][3]
		}
		return out
	}
	var centre [3]float64
	for i := range centre {
		centre[i] = sum[i] / float64(count)
	}
	centreMM := apply(centre)

	sizes := VoxelSizes(vol.Aff)
	var lo, hi [3]int
	for i := 0; i < 3; i++ {
		if sizes[i] == 0 {
			return fmt.Errorf("%s: the header gives no voxel size", src)
		}
		half := int(math.Ceil(cropHalfSize[i] / sizes[i]))
		c := int(math.RoundToEven(centre[i]))
		lo[i] = max(c-half, 0)
		hi[i] = min(c+half+1, dim[i])
	}
	out := NewVolume([3]int{hi[0] - lo[0], hi[1] - lo[1], hi[2] - lo[2]}, vol.Aff)
	i := 0
	for z := lo[2]; z < hi[2]; z++ {
		for y := lo[1]; y < hi[1]; y++ {
			row := (z*dim[1]+y)*dim[0] + lo[0]
			i += copy(out.Data[i:], data[row:row+hi[0]-lo[0]])
		}
	}
	// Shift the affine for the crop, then move the brain centre to MNI space.
	corner := apply([3]float64{float64(lo[0]), float64(lo[1]), float64(lo[2])})
	for r := 0; r < 3; r++ {
		out.Aff[r][3] = corner[r] + mniBrainCentre[r] - centreMM[r]
	}
	return SaveNifti(dst, out, false)
}

// SpatialOutputName names the warped image of a native one:
// sub-01_pet.nii.gz -> sub-01_space-MNI_pet.nii.gz.
func SpatialOutputName(path string) string {
	stem := StripExt(baseName(path))
	if strings.HasSuffix(stem, "_pet") {
		return stem[:len(stem)-4] + "_space-MNI_pet.nii.gz"
	}
	return stem + "_space-MNI.nii.gz"
}

// FindMatlab locates a MATLAB executable: on the PATH first, then where
// MATLAB installs itself.  It returns "" when there is none.
func FindMatlab() string {
	if exe, err := exec.LookPath("matlab"); err == nil {
		return exe
	}
	var patterns []string
	switch runtime.GOOS {
	case "darwin":
		patterns = []string{"/Applications/MATLAB_R*.app/bin/matlab"}
	case "windows":
		for _, env := range []string{"ProgramFiles", "ProgramW6432"} {
			if dir := os.Getenv(env); dir != "" {
				patterns = append(patterns, filepath.Join(dir, "MATLAB", "R*", "bin", "matlab.exe"))
			}
		}
	default:
		patterns = []string{"/usr/local/MATLAB/R*/bin/matlab"}
	}
	var found []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		found = append(found, matches...)
	}
	if len(found) == 0 {
		return ""
	}
	// The newest release: R2025b sorts after R2024a.
	sort.Slice(found, func(i, j int) bool {
		return filepath.Base(filepath.Dir(filepath.Dir(found[i]))) < filepath.Base(filepath.Dir(filepath.Dir(found[j])))
	})
	return found[len(found)-1]
}

// ResolveMatlab turns what a user points at (the program, the MATLAB_R*.app
// of macOS or MATLAB's folder) into the MATLAB executable.
func ResolveMatlab(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("MATLAB not found: %s", path)
	}
	if !info.IsDir() {
		return path, nil
	}
	name := "matlab"
	if runtime.GOOS == "windows" {
		name = "matlab.exe"
	}
	exe := filepath.Join(path, "bin", name)
	if info, err := os.Stat(exe); err != nil || info.IsDir() {
		return "", fmt.Errorf("no MATLAB program in %s", path)
	}
	return exe, nil
}

// CheckSPM reports what is wrong with a folder given as an SPM12 or SPM25
// installation.
func CheckSPM(path string) error {
	if _, err := os.Stat(filepath.Join(path, "spm_jobman.m")); err != nil {
		return fmt.Errorf("not an SPM folder (no spm_jobman.m): %s", path)
	}
	if _, err := os.Stat(filepath.Join(path, "toolbox", "OldNorm")); err != nil {
		return fmt.Errorf("this SPM has no toolbox/OldNorm; SPM12 or SPM25 is needed: %s", path)
	}
	return nil
}

// SpatialOptions are the settings of SpatialNormalize.
type SpatialOptions struct {
	SPMPath  string // folder of an SPM12 or SPM25 installation
	Template string // PET template in MNI space; "" is SPM's toolbox/OldNorm/PET.nii
	Matlab   string // MATLAB executable; "" searches for it
	Jobs     int    // MATLAB processes run side by side
	WorkDir  string // kept when given; "" works in a temporary folder
	Progress func(done, total int)
}

// SpatialResult is what became of one image.
type SpatialResult struct {
	Input  string
	Output string
	Err    error // nil when Output was written
}

// SpatialNormalize warps native-space PET images to MNI space (2 mm,
// 91x109x91), writing inputs[i] to outputs[i].  An image that fails is
// reported in its result and does not stop the others; the error returned
// is for what stops them all.
//
// A study-specific FDG template (for example the mean of your own
// normalised controls) usually gives a better fit than SPM's.
func SpatialNormalize(ctx context.Context, inputs, outputs []string, opt SpatialOptions) ([]SpatialResult, error) {
	if len(inputs) != len(outputs) {
		return nil, errors.New("inputs and outputs must have the same length")
	}
	spm, err := filepath.Abs(opt.SPMPath)
	if err != nil {
		return nil, err
	}
	if err := CheckSPM(spm); err != nil {
		return nil, err
	}
	custom := opt.Template != ""
	template := filepath.Join(spm, "toolbox", "OldNorm", "PET.nii")
	if custom {
		if template, err = filepath.Abs(opt.Template); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(template); err != nil {
		return nil, fmt.Errorf("PET template not found: %s", template)
	}
	matlab := opt.Matlab
	if matlab == "" {
		matlab = FindMatlab()
	}
	if matlab == "" {
		return nil, errors.New("MATLAB was not found; give its path")
	}
	if matlab, err = ResolveMatlab(matlab); err != nil {
		return nil, err
	}

	work := opt.WorkDir
	if work == "" {
		if work, err = os.MkdirTemp("", "sugarpet_"); err != nil {
			return nil, err
		}
		defer os.RemoveAll(work)
	} else if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	if work, err = filepath.Abs(work); err != nil {
		return nil, err
	}

	if strings.HasSuffix(strings.ToLower(template), ".gz") {
		raw, err := readMaybeGzip(template)
		if err != nil {
			return nil, err
		}
		template = filepath.Join(work, "template.nii")
		if err := os.WriteFile(template, raw, 0o644); err != nil {
			return nil, err
		}
	}

	results := make([]SpatialResult, len(inputs))
	native := make([]string, len(inputs))
	var ready []int // the images MATLAB is given
	for i, src := range inputs {
		results[i] = SpatialResult{Input: src, Output: outputs[i]}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		native[i] = filepath.Join(work, fmt.Sprintf("img%05d.nii", i))
		if err := PrepareNative(src, native[i]); err != nil {
			results[i].Err = err
			continue
		}
		ready = append(ready, i)
	}
	warped := func(i int) string { return filepath.Join(work, "w"+filepath.Base(native[i])) }
	total := len(inputs)
	shown := -1
	progress := func() {
		if opt.Progress == nil {
			return
		}
		done := total - len(ready)
		for _, i := range ready {
			if _, err := os.Stat(warped(i)); err == nil {
				done++
			}
		}
		// The last image is done once MATLAB has left and it is saved.
		if done = min(done, total-1); done != shown {
			shown = done
			opt.Progress(done, total)
		}
	}
	if len(ready) == 0 {
		return results, nil
	}
	progress()

	jobs := max(1, min(opt.Jobs, len(ready)))
	chunks := make([][]int, jobs)
	for k, i := range ready {
		chunks[k%jobs] = append(chunks[k%jobs], i)
	}
	smoref := 0 // SPM's own template is already smooth; a custom mean image is smoothed to match the source
	if custom {
		smoref = 8
	}
	logs := make([]string, jobs)
	exits := make([]error, jobs)
	var wg sync.WaitGroup
	for j, chunk := range chunks {
		list := make([]string, len(chunk))
		for k, i := range chunk {
			list[k] = "'" + matlabQuote(native[i]) + ",1'"
		}
		script := filepath.Join(work, fmt.Sprintf("sugarpet_job%d.m", j))
		text := fmt.Sprintf(matlabTemplate, matlabQuote(spm), matlabQuote(template), smoref, strings.Join(list, "\n"))
		if err := os.WriteFile(script, []byte(text), 0o644); err != nil {
			return nil, err
		}
		logs[j] = strings.TrimSuffix(script, ".m") + ".log"
		wg.Add(1)
		go func() {
			defer wg.Done()
			exits[j] = runMatlab(ctx, matlab, script, logs[j])
		}()
	}
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	tick := time.NewTicker(500 * time.Millisecond)
	for running := true; running; {
		select {
		case <-finished:
			running = false
		case <-tick.C:
			progress()
		}
	}
	tick.Stop()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Why an image has no warped file: SPM's own words, else the end of
	// what MATLAB printed.
	reasons := map[string]string{}
	tails := make([]string, jobs)
	for j, log := range logs {
		raw, _ := os.ReadFile(log)
		var last []string
		for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r", ""), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if rest, ok := strings.CutPrefix(line, "FAILED "); ok {
				if file, msg, ok := strings.Cut(rest, ",1: "); ok {
					reasons[file] = msg
				}
				continue
			}
			last = append(last, line)
		}
		if len(last) > 3 {
			last = last[len(last)-3:]
		}
		tails[j] = strings.Join(last, " ")
		if tails[j] == "" && exits[j] != nil {
			tails[j] = exits[j].Error()
		}
	}
	for j, chunk := range chunks {
		for _, i := range chunk {
			res := &results[i]
			vol, err := LoadNifti(warped(i))
			if err != nil {
				switch {
				case reasons[native[i]] != "":
					res.Err = errors.New("SPM: " + reasons[native[i]])
				case tails[j] != "":
					res.Err = errors.New("MATLAB: " + tails[j])
				default:
					res.Err = errors.New("SPM wrote no image")
				}
				continue
			}
			for k, v := range vol.Data {
				if f := float64(v); math.IsNaN(f) || math.IsInf(f, 0) {
					vol.Data[k] = 0
				}
			}
			vol.Descrip = ""
			res.Err = SaveNifti(res.Output, vol, false)
		}
	}
	if opt.Progress != nil {
		opt.Progress(total, total)
	}
	return results, nil
}

// runMatlab runs a script and waits for it.  MATLAB leaves helper processes
// that keep pipes open, so its output goes to a file rather than a pipe.
func runMatlab(ctx context.Context, matlab, script, log string) error {
	out, err := os.Create(log)
	if err != nil {
		return err
	}
	defer out.Close()
	cmd := exec.CommandContext(ctx, matlab, "-batch", "run('"+matlabQuote(script)+"')")
	cmd.Dir = filepath.Dir(script)
	cmd.Stdout, cmd.Stderr = out, out
	ownProcessGroup(cmd)
	return cmd.Run()
}
