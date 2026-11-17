package core

import (
	"math"
	"sort"
	"strings"
)

// Methods lists the normalisation methods, in the order they are shown.
var Methods = func() []Named {
	var out []Named
	for _, r := range ReferenceRegions {
		out = append(out, Named{r.Name, "Reference region: " + maskText(r.Text)})
	}
	return append(out,
		Named{"region", "Reference region: any built-in mask, mask file or set of atlas labels (needs mask=...)"},
		Named{"ps", "Proportional scaling: mean uptake of the gray-matter mask"},
		Named{"spm-global", "SPM-style global mean (voxels above 1/8 of the image mean)"},
		Named{"hn", "Histogram normalisation: peak of the subject/template ratio histogram [needs reference]"},
		Named{"ips", "Iterative PS: PS, then re-estimated without hypometabolic voxels [needs reference]"},
		Named{"ihn", "Iterative HN: HN, then re-estimated without hypometabolic voxels [needs reference]"},
	)
}()

// IsMethod reports whether name is a normalisation method.
func IsMethod(name string) bool {
	for _, m := range Methods {
		if m.Name == name {
			return true
		}
	}
	return false
}

// NeedsReference reports whether a method needs a healthy-control reference.
func NeedsReference(method string) bool {
	return method == "hn" || method == "ips" || method == "ihn"
}

func methodNames() string {
	names := make([]string, len(Methods))
	for i, m := range Methods {
		names[i] = m.Name
	}
	return strings.Join(names, ", ")
}

// validVoxels marks the voxels that hold real signal (finite and above zero).
func validVoxels(data []float32) []bool {
	out := make([]bool, len(data))
	for i, v := range data {
		f := float64(v)
		out[i] = f > 0 && !math.IsInf(f, 0)
	}
	return out
}

const noOverlap = "the mask does not overlap the image; is the image in MNI space?"

// regionMean is the mean uptake inside a mask.
func regionMean(data []float32, mask []bool) (float64, error) {
	var sum float64
	n := 0
	for i, m := range mask {
		if m {
			sum += float64(data[i])
			n++
		}
	}
	if n == 0 {
		return 0, errorf(noOverlap)
	}
	return sum / float64(n), nil
}

// regionStat is the mean or median uptake inside a mask.
func regionStat(data []float32, mask []bool, statistic string) (float64, error) {
	if statistic == "mean" {
		return regionMean(data, mask)
	}
	if statistic != "median" {
		return 0, errorf("statistic must be 'mean' or 'median'")
	}
	var vals []float32
	for i, m := range mask {
		if m {
			vals = append(vals, data[i])
		}
	}
	if len(vals) == 0 {
		return 0, errorf(noOverlap)
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	n := len(vals)
	if n%2 == 1 {
		return float64(vals[n/2]), nil
	}
	return float64((vals[n/2-1] + vals[n/2]) / 2), nil
}

// spmGlobal is SPM's global mean: the mean of all voxels brighter than 1/8
// of the image mean.
func spmGlobal(data []float32) float64 {
	var sum float64
	n := 0
	for _, v := range data {
		if f := float64(v); !math.IsNaN(f) && !math.IsInf(f, 0) {
			sum += f
			n++
		}
	}
	threshold := sum / float64(n) / 8.0
	sum, n = 0, 0
	for _, v := range data {
		if f := float64(v); !math.IsNaN(f) && !math.IsInf(f, 0) && f > threshold {
			sum += f
			n++
		}
	}
	return sum / float64(n)
}

// NormOptions are the settings of Normalize.  Start from DefaultNormOptions.
type NormOptions struct {
	Reference   *Reference
	Mask        MaskSpec // replaces the default region of a method
	FWHM        *float64 // smoothing (mm) before the factor is estimated; nil = the method's default
	Statistic   string   // "mean" or "median" of the region
	Target      float64  // value the reference takes in the output
	PThreshold  float64  // ips/ihn: p-value for excluding voxels
	Exclude     string   // ips/ihn: "hypo" or "both"
	MaxIter     int      // ips/ihn: number of re-estimations
	MinFraction float64  // ips/ihn: never let the mask shrink below this fraction
	LiteralIPS  bool     // ips: plain mean of the remaining voxels
}

// DefaultNormOptions returns the defaults of the Python package.
func DefaultNormOptions() NormOptions {
	return NormOptions{Statistic: "mean", Target: 1, PThreshold: 0.01, Exclude: "hypo", MaxIter: 1, MinFraction: 0.2}
}

// NormResult is the outcome of normalising one image.
type NormResult struct {
	Method   string
	Factor   float64 // the normalisation factor, in the units of the input image
	Image    *Volume // input / factor * target
	Mask     []bool  // the voxels the factor was finally estimated from (nil for spm-global)
	Warnings []string

	FWHM             float64
	MaskName         string // "reference", a built-in mask name, "custom" or ""
	Statistic        string
	InitialFactor    float64
	Iterations       int
	ExcludedFraction float64
	Iterative        bool
	NVoxels          int
	Target           float64
}

// singleSubjectT is the t statistic of one subject against n controls
// (Crawford & Howell, 1998).  Negative: lower uptake than the controls.
func singleSubjectT(value, mean, std float32, scale float64) float32 {
	if !(std > 0) {
		return 0
	}
	diff := value - mean
	return float32(float64(diff) / float64(float64(std)*scale))
}

// Normalize intensity-normalises one spatially normalised FDG-PET image.
func Normalize(img *Volume, method string, opt NormOptions) (*NormResult, error) {
	if !IsMethod(method) {
		hint := ""
		if IsMask(method) {
			hint = " ('" + method + "' is a built-in mask: use method 'region' with mask '" + method + "')"
		}
		return nil, errorf("unknown method '%s'%s; choose from: %s", method, hint, methodNames())
	}
	if opt.Statistic != "mean" && opt.Statistic != "median" {
		return nil, errorf("statistic must be 'mean' or 'median'")
	}
	if math.IsNaN(opt.Target) || math.IsInf(opt.Target, 0) || opt.Target <= 0 {
		return nil, errorf("target must be a positive number")
	}
	if opt.Exclude != "hypo" && opt.Exclude != "both" {
		return nil, errorf("exclude must be 'hypo' or 'both'")
	}
	if NeedsReference(method) && opt.Reference == nil {
		return nil, errorf("method '%s' needs a reference built from healthy controls", method)
	}
	raw := img.Data[:img.NVox()]
	res := &NormResult{Method: method, Target: opt.Target}
	var ref *Reference
	fwhm := 0.0
	if opt.FWHM != nil {
		fwhm = *opt.FWHM
	}
	if opt.Reference != nil {
		ref = opt.Reference.OnGrid(img.Dim, img.Aff)
		if opt.FWHM == nil && NeedsReference(method) {
			fwhm = ref.FWHM
		}
	}
	data := Smooth(raw, img.Dim, img.Aff, fwhm)
	valid := validVoxels(raw)
	res.FWHM = fwhm

	var factor float64
	var finalMask []bool
	var err error
	switch {
	case method == "spm-global":
		factor = spmGlobal(data)
	case !NeedsReference(method):
		if method == "region" && opt.Mask.IsZero() {
			return nil, errorf("method 'region' needs a mask")
		}
		if opt.Mask.IsZero() && method == "ps" && ref != nil {
			// Same voxels as the reference-based methods, so that PS, iPS,
			// HN and iHN are directly comparable.
			finalMask = append([]bool(nil), ref.Mask...)
			res.MaskName = "reference"
		} else {
			spec := opt.Mask
			if spec.IsZero() {
				name := regionMask(method)
				if name == "" {
					name = "gray-matter"
				}
				spec = MaskSpec{Name: name}
			}
			res.MaskName = "custom"
			if spec.Vol == nil && IsMask(spec.Name) {
				res.MaskName = spec.Name
			}
			if finalMask, err = GetMask(spec, img.Dim, img.Aff); err != nil {
				return nil, err
			}
		}
		for i := range finalMask {
			finalMask[i] = finalMask[i] && valid[i]
		}
		if factor, err = regionStat(data, finalMask, opt.Statistic); err != nil {
			return nil, err
		}
		res.Statistic = opt.Statistic
	default:
		mask0 := make([]bool, len(raw))
		n0, nRef := 0, 0
		for i := range mask0 {
			mask0[i] = ref.Mask[i] && valid[i] && ref.Template[i] > 0
			if mask0[i] {
				n0++
			}
			if ref.Mask[i] {
				nRef++
			}
		}
		if float64(n0) < 0.5*float64(nRef) {
			res.Warnings = append(res.Warnings,
				"less than half of the reference mask is covered by the image; check that it is in MNI space")
		}
		base := "hn"
		if method == "ips" {
			base = "ps"
		}
		if factor, err = baseFactor(base, data, ref, mask0, false); err != nil {
			return nil, err
		}
		finalMask = mask0
		if method != "hn" {
			res.Iterative = true
			res.InitialFactor = factor
			factor, finalMask, err = iterate(base, data, ref, mask0, n0, factor, opt, res)
			if err != nil {
				return nil, err
			}
			res.ExcludedFraction = 1 - float64(countTrue(finalMask))/float64(n0)
		}
	}
	if math.IsNaN(factor) || math.IsInf(factor, 0) || factor <= 0 {
		return nil, errorf("method '%s' gave an unusable factor (%v)", method, factor)
	}
	if finalMask != nil {
		res.NVoxels = countTrue(finalMask)
	}
	out := NewVolume(img.Dim, img.Aff)
	scale := float32(opt.Target / factor)
	parallelFor(len(raw), func(lo, hi int) {
		for i := lo; i < hi; i++ {
			out.Data[i] = raw[i] * scale
		}
	})
	out.Descrip = "sugar-pet " + method + " factor=" + fmtG(factor)
	if opt.Target != 1 {
		out.Descrip += " target=" + fmtG(opt.Target)
	}
	if len(out.Descrip) > 79 {
		out.Descrip = out.Descrip[:79]
	}
	res.Factor, res.Image, res.Mask = factor, out, finalMask
	return res, nil
}

// baseFactor estimates a PS or HN factor from the voxels of a mask.
func baseFactor(base string, data []float32, ref *Reference, mask []bool, literal bool) (float64, error) {
	if base == "hn" {
		ratio := make([]float32, 0, len(data)/4)
		for i, m := range mask {
			if m {
				ratio = append(ratio, data[i]/ref.Template[i])
			}
		}
		return histogramMode(ratio)
	}
	num, err := regionMean(data, mask)
	if err != nil || literal {
		return num, err
	}
	// Dividing by the control mean of the same voxels keeps the scale
	// comparable to the controls whatever the mask has lost.
	den, err := regionMean(ref.Template, mask)
	if err != nil {
		return 0, err
	}
	return num / den, nil
}

// iterate refines a PS or HN factor by leaving out voxels that look
// abnormal: the subject, scaled by the current factor, is compared with the
// controls voxel by voxel; significant voxels are dropped from the mask and
// the factor is estimated again from what is left.
func iterate(base string, data []float32, ref *Reference, mask0 []bool, n0 int, factor0 float64,
	opt NormOptions, res *NormResult) (float64, []bool, error) {
	mean, std := ref.HNMean, ref.HNStd
	if base == "ps" {
		mean, std = ref.Template, ref.PSStd
	}
	df := float64(ref.N - 1)
	scale := math.Sqrt(1.0 + 1.0/float64(ref.N))
	var limit float64
	if opt.Exclude == "hypo" {
		limit = tPPF(opt.PThreshold, df)
	} else {
		limit = tPPF(1.0-opt.PThreshold/2.0, df)
	}
	factor, mask := factor0, mask0
	for iter := 0; iter < opt.MaxIter; iter++ {
		f := float32(factor)
		next := make([]bool, len(mask0))
		kept := 0
		same := true
		for i, m := range mask0 {
			if m && std[i] > 0 {
				t := float64(singleSubjectT(data[i]/f, mean[i], std[i], scale))
				if opt.Exclude == "hypo" {
					next[i] = !(t < limit)
				} else {
					next[i] = !(math.Abs(t) > limit)
				}
			}
			if next[i] {
				kept++
			}
			same = same && next[i] == mask[i]
		}
		if float64(kept) < opt.MinFraction*float64(n0) {
			res.Warnings = append(res.Warnings, "iterative step would keep under "+
				fmtG(math.Round(opt.MinFraction*100))+"% of the mask; stopping with the previous factor")
			break
		}
		if res.Iterations > 0 && same {
			break
		}
		mask = next
		res.Iterations++
		var err error
		if factor, err = baseFactor(base, data, ref, mask, opt.LiteralIPS); err != nil {
			return 0, nil, err
		}
	}
	return factor, mask, nil
}

// NormalizeFile loads an image and normalises it.
func NormalizeFile(path, method string, opt NormOptions) (*NormResult, error) {
	img, err := LoadImage(path)
	if err != nil {
		return nil, err
	}
	return Normalize(img, method, opt)
}

// OutputName builds a BIDS-style output name:
// sub-01_pet.nii.gz -> sub-01_desc-hn_pet.nii.gz.
func OutputName(path, method, suffix string) string {
	stem := StripExt(baseName(path))
	label := "desc-" + strings.ReplaceAll(method, "-", "")
	if strings.HasSuffix(stem, "_pet") {
		return stem[:len(stem)-4] + "_" + label + "_" + suffix + ".nii.gz"
	}
	if suffix == "pet" {
		return stem + "_" + label + ".nii.gz"
	}
	return stem + "_" + label + "_" + suffix + ".nii.gz"
}

func baseName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}
