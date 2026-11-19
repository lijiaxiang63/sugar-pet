package core

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Version is written into the references this program builds.
const Version = "0.8.0"

// Stat is the voxel-wise mean and standard deviation of the controls.
type Stat struct{ Mean, Std []float32 }

// Model is the regression of the normalised controls on their covariates:
// the coefficients (one volume per term) and the residual SD.
type Model struct {
	Beta [][]float32
	SD   []float32
}

// Reference is the voxel-wise summary of a group of healthy controls,
// needed by HN, iPS, iHN and by z-score maps.
type Reference struct {
	Template []float32 // mean of the smoothed controls after proportional scaling
	PSStd    []float32
	HNMean   []float32
	HNStd    []float32
	Mask     []bool // voxels used to estimate factors
	Dim      [3]int
	Aff      Affine
	N        int     // number of controls
	FWHM     float64 // smoothing (mm) applied to the controls

	MaskName       string
	SugarVersion   string
	ControlFactors []NamedFactors
	Terms          []Term
	Stats          map[string]Stat
	StatsFor       []string
	Models         map[string]Model
	ModelNames     []string

	gridMu sync.Mutex
	grid   *Reference // this reference on the grid it was last asked for
}

// NamedFactors holds the normalisation factors of the controls for one method.
type NamedFactors struct {
	Method  string
	Factors []float64
}

// StatsMethods are the methods whose control statistics can be stored on
// request (PS/iPS and HN/iHN statistics are always part of a reference).
func StatsMethods() []string {
	out := make([]string, 0, len(ReferenceRegions)+1)
	for _, r := range ReferenceRegions {
		out = append(out, r.Name)
	}
	return append(out, "spm-global")
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

// BuildOptions are the settings of BuildReference.
type BuildOptions struct {
	Mask       MaskSpec // voxels used to estimate factors [gray-matter]
	FWHM       float64  // smoothing in mm before computing statistics (0 = already smoothed)
	StatsFor   []string // also store control statistics for these methods
	Covariates []Covariate
	Progress   func(done, total int)
}

// meanStd returns the voxel-wise mean and SD (n-1) of a stack of volumes.
func meanStd(vols [][]float32) ([]float32, []float32) {
	n, nvox := len(vols), len(vols[0])
	mean, std := make([]float32, nvox), make([]float32, nvox)
	parallelFor(nvox, func(lo, hi int) {
		sum := make([]float64, hi-lo)
		for _, vol := range vols {
			for v := lo; v < hi; v++ {
				sum[v-lo] += float64(vol[v])
			}
		}
		for i := range sum {
			sum[i] /= float64(n)
		}
		sq := make([]float64, hi-lo)
		for _, vol := range vols {
			for v := lo; v < hi; v++ {
				d := float64(vol[v]) - sum[v-lo]
				sq[v-lo] += d * d
			}
		}
		for v := lo; v < hi; v++ {
			mean[v] = float32(sum[v-lo])
			std[v] = float32(math.Sqrt(sq[v-lo] / float64(n-1)))
		}
	})
	return mean, std
}

// fitModel is the voxel-wise least squares of the control images (times an
// optional factor per image) on the design matrix.
func fitModel(vols [][]float32, ratio []float32, design [][]float64) (Model, error) {
	n, p := len(design), len(design[0])
	pin, ok := pinv(design)
	if !ok {
		return Model{}, errorf("the covariates are redundant (one can be predicted exactly from the others)")
	}
	nvox := len(vols[0])
	model := Model{Beta: make([][]float32, p), SD: make([]float32, nvox)}
	for j := range model.Beta {
		model.Beta[j] = make([]float32, nvox)
	}
	parallelFor(nvox, func(lo, hi int) {
		width := hi - lo
		beta := make([]float64, p*width)
		xty := make([]float64, p*width)
		squares := make([]float64, width)
		for i, vol := range vols {
			for v := lo; v < hi; v++ {
				x := vol[v]
				if ratio != nil {
					x *= ratio[i]
				}
				y := float64(x)
				k := v - lo
				for j := 0; j < p; j++ {
					beta[j*width+k] += float64(pin[j][i] * y)
					xty[j*width+k] += float64(design[i][j] * y)
				}
				squares[k] += y * y
			}
		}
		for k := 0; k < width; k++ {
			explained := 0.0
			for j := 0; j < p; j++ {
				explained += float64(beta[j*width+k] * xty[j*width+k])
				model.Beta[j][lo+k] = float32(beta[j*width+k])
			}
			residual := math.Max(squares[k]-explained, 0)
			model.SD[lo+k] = float32(math.Sqrt(residual / float64(n-p)))
		}
	})
	return model, nil
}

// BuildReference builds a reference from spatially normalised
// healthy-control images, all on the same grid.
func BuildReference(ctx context.Context, controls []string, opt BuildOptions) (*Reference, error) {
	if len(controls) < 3 {
		return nil, errorf("at least 3 control images are needed to build a reference")
	}
	var statsFor []string
	for _, m := range opt.StatsFor {
		if !contains(statsFor, m) {
			statsFor = append(statsFor, m)
		}
	}
	var unknown []string
	for _, m := range statsFor {
		if !contains(StatsMethods(), m) {
			unknown = append(unknown, m)
		}
	}
	if len(unknown) > 0 {
		return nil, errorf("cannot store statistics for %s; choose from: %s", pyList(unknown), strings.Join(StatsMethods(), ", "))
	}
	var design [][]float64
	var terms []Term
	if len(opt.Covariates) > 0 {
		var err error
		if design, terms, err = BuildDesign(opt.Covariates); err != nil {
			return nil, err
		}
		if len(design) != len(controls) {
			return nil, errorf("%d covariate values for %d controls", len(design), len(controls))
		}
	}
	maskSpec := opt.Mask
	if maskSpec.IsZero() {
		maskSpec = MaskSpec{Name: "gray-matter"}
	}
	first, err := LoadImage(controls[0])
	if err != nil {
		return nil, err
	}
	n, nvox := len(controls), first.NVox()
	mask0, err := GetMask(maskSpec, first.Dim, first.Aff)
	if err != nil {
		return nil, err
	}
	regions := map[string][]bool{}
	for _, m := range statsFor {
		if m != "spm-global" {
			if regions[m], err = GetMask(MaskSpec{Name: regionMask(m)}, first.Dim, first.Aff); err != nil {
				return nil, err
			}
		}
	}
	other := map[string][]float64{}
	maskIdx := make([]int, 0, nvox)
	for i, m := range mask0 {
		if m {
			maskIdx = append(maskIdx, i)
		}
	}
	scaled := make([][]float32, n)
	unsmoothed := make([][]float32, n)
	inFOV := make([]int32, nvox)
	for i, path := range controls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		img := first
		if i > 0 {
			if img, err = LoadImage(path); err != nil {
				return nil, err
			}
		}
		if !SameGrid(img.Dim, img.Aff, first.Dim, first.Aff) {
			return nil, errorf("control %d is on a different grid than the first control", i+1)
		}
		valid := validVoxels(img.Data)
		for v, ok := range valid {
			if ok {
				inFOV[v]++
			}
		}
		unsmoothed[i] = make([]float32, len(maskIdx))
		for k, v := range maskIdx {
			unsmoothed[i][k] = img.Data[v]
		}
		for _, m := range statsFor {
			var f float64
			if m == "spm-global" {
				f = spmGlobal(img.Data)
			} else {
				region := make([]bool, nvox)
				for v := range region {
					region[v] = regions[m][v] && valid[v]
				}
				if f, err = regionMean(img.Data, region); err != nil {
					return nil, err
				}
			}
			other[m] = append(other[m], f)
		}
		scaled[i] = Smooth(img.Data, img.Dim, img.Aff, opt.FWHM)
		if opt.FWHM == 0 {
			scaled[i] = append([]float32(nil), img.Data...)
		}
		if opt.Progress != nil {
			opt.Progress(i+1, n)
		}
	}

	// Keep only voxels that every control actually covers.
	final := make([]bool, nvox)
	kept := 0
	for v := range final {
		final[v] = mask0[v] && int(inFOV[v]) == n
		if final[v] {
			kept++
		}
	}
	if kept < 1000 {
		return nil, errorf("the mask barely overlaps the controls; are they in MNI space?")
	}
	// Proportional scaling is defined on the images as given (before
	// smoothing), so that all data-driven methods share one scale.
	ps := make([]float64, n)
	for i := range controls {
		var sum float64
		for k, v := range maskIdx {
			if final[v] {
				sum += float64(unsmoothed[i][k])
			}
		}
		ps[i] = sum / float64(kept)
		unsmoothed[i] = nil
	}
	mask0 = final
	for i := range scaled {
		vol, f := scaled[i], ps[i]
		parallelFor(nvox, func(lo, hi int) {
			for v := lo; v < hi; v++ {
				vol[v] = float32(float64(vol[v]) / f)
			}
		})
	}
	ref := &Reference{Mask: mask0, Dim: first.Dim, Aff: first.Aff, N: n, FWHM: opt.FWHM,
		SugarVersion: Version, Terms: terms, Stats: map[string]Stat{}, Models: map[string]Model{}}
	ref.Template, ref.PSStd = meanStd(scaled)
	if design != nil {
		model, err := fitModel(scaled, nil, design)
		if err != nil {
			return nil, err
		}
		ref.Models["ps"] = model
		ref.ModelNames = append(ref.ModelNames, "ps")
	}

	// HN factor of each control against the template, then HN statistics.
	hn := make([]float64, n)
	ratioBuf := make([]float32, 0, kept)
	for i := range scaled {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ratioBuf = ratioBuf[:0]
		for v, m := range mask0 {
			if m {
				ratioBuf = append(ratioBuf, scaled[i][v]/ref.Template[v])
			}
		}
		mode, err := histogramMode(ratioBuf)
		if err != nil {
			return nil, err
		}
		hn[i] = ps[i] * mode
		vol, f := scaled[i], float32(mode)
		parallelFor(nvox, func(lo, hi int) {
			for v := lo; v < hi; v++ {
				vol[v] /= f
			}
		})
	}
	ref.HNMean, ref.HNStd = meanStd(scaled)
	if design != nil {
		model, err := fitModel(scaled, nil, design)
		if err != nil {
			return nil, err
		}
		ref.Models["hn"] = model
		ref.ModelNames = append(ref.ModelNames, "hn")
	}

	// Control statistics under the other methods: scaled now holds
	// smoothed / HN factor, so rescale it by HN factor / method factor.
	for _, m := range statsFor {
		ratio := make([]float32, n)
		for i := range ratio {
			ratio[i] = float32(hn[i] / other[m][i])
		}
		stat := Stat{Mean: make([]float32, nvox), Std: make([]float32, nvox)}
		parallelFor(nvox, func(lo, hi int) {
			total := make([]float64, hi-lo)
			squares := make([]float64, hi-lo)
			for i, vol := range scaled {
				for v := lo; v < hi; v++ {
					x := float64(vol[v] * ratio[i])
					total[v-lo] += x
					squares[v-lo] += x * x
				}
			}
			for v := lo; v < hi; v++ {
				mean := total[v-lo] / float64(n)
				variance := math.Max(squares[v-lo]-float64(float64(n)*mean*mean), 0) / float64(n-1)
				stat.Mean[v] = float32(mean)
				stat.Std[v] = float32(math.Sqrt(variance))
			}
		})
		ref.Stats[m] = stat
		ref.StatsFor = append(ref.StatsFor, m)
		if design != nil {
			model, err := fitModel(scaled, ratio, design)
			if err != nil {
				return nil, err
			}
			ref.Models[m] = model
			ref.ModelNames = append(ref.ModelNames, m)
		}
	}

	ref.MaskName = "custom"
	if opt.Mask.Vol == nil {
		ref.MaskName = maskSpec.Name
	}
	ref.ControlFactors = []NamedFactors{{"ps", ps}, {"hn", hn}}
	for _, m := range statsFor {
		ref.ControlFactors = append(ref.ControlFactors, NamedFactors{m, other[m]})
	}
	return ref, nil
}

// Covariates returns the names of the covariates W-scores are adjusted for.
func (r *Reference) Covariates() []string {
	names := make([]string, len(r.Terms))
	for i, t := range r.Terms {
		names[i] = t.Name
	}
	return names
}

// ControlStats returns the mean and SD of the controls normalised with method.
func (r *Reference) ControlStats(method string) (Stat, error) {
	switch method {
	case "ps", "ips":
		return Stat{r.Template, r.PSStd}, nil
	case "hn", "ihn":
		return Stat{r.HNMean, r.HNStd}, nil
	}
	if s, ok := r.Stats[method]; ok {
		return s, nil
	}
	if contains(StatsMethods(), method) {
		return Stat{}, errorf("this reference holds no control statistics for '%s'; rebuild it with stats_for=['%s'] (--stats-for %s)",
			method, method, method)
	}
	return Stat{}, errorf("control statistics are not available for method '%s'", method)
}

// ControlModel returns the covariate model of the controls normalised with method.
func (r *Reference) ControlModel(method string) (Model, error) {
	if len(r.Models) == 0 {
		return Model{}, errorf("this reference was built without covariates; rebuild it with covariates (--covariates)")
	}
	key := method
	switch method {
	case "ips":
		key = "ps"
	case "ihn":
		key = "hn"
	}
	if m, ok := r.Models[key]; ok {
		return m, nil
	}
	if _, err := r.ControlStats(method); err != nil {
		return Model{}, err
	}
	return Model{}, errorf("this reference holds no covariate model for '%s'", method)
}

type factorsJSON []NamedFactors

func (f factorsJSON) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, item := range f {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, _ := json.Marshal(item.Method)
		val, err := json.Marshal(item.Factors)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(val)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (f *factorsJSON) UnmarshalJSON(data []byte) error {
	var raw map[string][]float64
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rank := func(name string) int {
		switch name {
		case "ps":
			return 0
		case "hn":
			return 1
		}
		return 2
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if rank(keys[i]) != rank(keys[j]) {
			return rank(keys[i]) < rank(keys[j])
		}
		return keys[i] < keys[j]
	})
	*f = nil
	for _, k := range keys {
		*f = append(*f, NamedFactors{k, raw[k]})
	}
	return nil
}

type referenceJSON struct {
	NControls      int         `json:"n_controls"`
	FWHM           float64     `json:"fwhm_mm"`
	Version        string      `json:"sugar_pet_version"`
	Mask           string      `json:"mask"`
	ControlFactors factorsJSON `json:"control_factors"`
	Covariates     []Term      `json:"covariates,omitempty"`
	StatsFor       []string    `json:"stats_for"`
	Models         []string    `json:"models"`
}

func (r *Reference) volume(data []float32) *Volume {
	return &Volume{Dim: r.Dim, NT: 1, Data: data, Aff: r.Aff}
}

// Save writes the reference as a folder of NIfTI files plus reference.json,
// in the format of the Python package.
func (r *Reference) Save(folder string) error {
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	write := func(name string, data []float32) error {
		return SaveNifti(filepath.Join(folder, name+".nii.gz"), r.volume(data), false)
	}
	for name, data := range map[string][]float32{
		"template": r.Template, "ps_std": r.PSStd, "hn_mean": r.HNMean, "hn_std": r.HNStd,
	} {
		if err := write(name, data); err != nil {
			return err
		}
	}
	if err := SaveMask(filepath.Join(folder, "mask.nii.gz"), r.Mask, r.Dim, r.Aff, ""); err != nil {
		return err
	}
	for _, method := range r.StatsFor {
		if err := write("stats-"+method+"_mean", r.Stats[method].Mean); err != nil {
			return err
		}
		if err := write("stats-"+method+"_std", r.Stats[method].Std); err != nil {
			return err
		}
	}
	for _, method := range r.ModelNames {
		model := r.Models[method]
		beta := &Volume{Dim: r.Dim, NT: len(model.Beta), Aff: r.Aff}
		for _, frame := range model.Beta {
			beta.Data = append(beta.Data, frame...)
		}
		if err := SaveNifti(filepath.Join(folder, "model-"+method+"_beta.nii.gz"), beta, false); err != nil {
			return err
		}
		if err := write("model-"+method+"_sd", model.SD); err != nil {
			return err
		}
	}
	info := referenceJSON{r.N, r.FWHM, r.SugarVersion, r.MaskName, r.ControlFactors, r.Terms, r.StatsFor, r.ModelNames}
	if info.StatsFor == nil {
		info.StatsFor = []string{}
	}
	if info.Models == nil {
		info.Models = []string{}
	}
	text, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(folder, "reference.json"), text, 0o644)
}

// IsReferenceFolder reports whether a folder holds a saved reference.
func IsReferenceFolder(folder string) bool {
	_, err := os.Stat(filepath.Join(folder, "reference.json"))
	return err == nil
}

// LoadReference reads a reference saved by Save or by the Python package.
func LoadReference(folder string) (*Reference, error) {
	raw, err := os.ReadFile(filepath.Join(folder, "reference.json"))
	if err != nil {
		return nil, errorf("%s is not a sugar-pet reference (no reference.json)", folder)
	}
	var info referenceJSON
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, errorf("%s: reference.json cannot be read (%v)", folder, err)
	}
	r := &Reference{N: info.NControls, FWHM: info.FWHM, SugarVersion: info.Version, MaskName: info.Mask,
		ControlFactors: info.ControlFactors, Terms: info.Covariates,
		Stats: map[string]Stat{}, StatsFor: info.StatsFor, Models: map[string]Model{}, ModelNames: info.Models}
	first := true
	read := func(name string) (*Volume, error) {
		vol, err := LoadNifti(filepath.Join(folder, name+".nii.gz"))
		if err != nil {
			return nil, err
		}
		if first {
			r.Dim, first = vol.Dim, false
		} else if vol.Dim != r.Dim {
			return nil, errorf("%s: %s.nii.gz does not match the other volumes of the reference", folder, name)
		}
		r.Aff = vol.Aff
		return vol, nil
	}
	targets := []*[]float32{&r.Template, &r.PSStd, &r.HNMean, &r.HNStd}
	for i, name := range []string{"template", "ps_std", "hn_mean", "hn_std"} {
		vol, err := read(name)
		if err != nil {
			return nil, err
		}
		*targets[i] = vol.Frame(0)
	}
	maskVol, err := read("mask")
	if err != nil {
		return nil, err
	}
	r.Mask = make([]bool, maskVol.NVox())
	for i, v := range maskVol.Frame(0) {
		r.Mask[i] = v > 0.5
	}
	for _, method := range r.StatsFor {
		mean, err := read("stats-" + method + "_mean")
		if err != nil {
			return nil, err
		}
		std, err := read("stats-" + method + "_std")
		if err != nil {
			return nil, err
		}
		r.Stats[method] = Stat{mean.Frame(0), std.Frame(0)}
	}
	for _, method := range r.ModelNames {
		beta, err := read("model-" + method + "_beta")
		if err != nil {
			return nil, err
		}
		sd, err := read("model-" + method + "_sd")
		if err != nil {
			return nil, err
		}
		model := Model{SD: sd.Frame(0)}
		for t := 0; t < beta.NT; t++ {
			model.Beta = append(model.Beta, beta.Frame(t))
		}
		r.Models[method] = model
	}
	return r, nil
}

// OnGrid returns this reference resampled onto another grid.
func (r *Reference) OnGrid(dim [3]int, aff Affine) *Reference {
	if SameGrid(r.Dim, r.Aff, dim, aff) {
		return r
	}
	r.gridMu.Lock()
	defer r.gridMu.Unlock()
	if r.grid != nil && SameGrid(r.grid.Dim, r.grid.Aff, dim, aff) {
		return r.grid
	}
	move := func(data []float32) []float32 { return resampleLinear(data, r.Dim, r.Aff, dim, aff, false) }
	out := &Reference{Dim: dim, Aff: aff, N: r.N, FWHM: r.FWHM, MaskName: r.MaskName, SugarVersion: r.SugarVersion,
		ControlFactors: r.ControlFactors, Terms: r.Terms, StatsFor: r.StatsFor, ModelNames: r.ModelNames,
		Stats: map[string]Stat{}, Models: map[string]Model{}}
	out.Template, out.PSStd, out.HNMean, out.HNStd = move(r.Template), move(r.PSStd), move(r.HNMean), move(r.HNStd)
	maskFloat := make([]float32, len(r.Mask))
	for i, m := range r.Mask {
		if m {
			maskFloat[i] = 1
		}
	}
	moved := move(maskFloat)
	out.Mask = make([]bool, len(moved))
	for i, v := range moved {
		out.Mask[i] = v > 0.5
	}
	for method, s := range r.Stats {
		out.Stats[method] = Stat{move(s.Mean), move(s.Std)}
	}
	for method, m := range r.Models {
		model := Model{SD: move(m.SD)}
		for _, b := range m.Beta {
			model.Beta = append(model.Beta, move(b))
		}
		out.Models[method] = model
	}
	r.grid = out
	return out
}
