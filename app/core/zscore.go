package core

import (
	"context"
	"math"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

// ZMapOptions are the settings of ZMap.
type ZMapOptions struct {
	Covariates map[string]string // nil for plain z-scores; else W-scores adjusted for them
	PThreshold float64
	Exclude    string
	MaxIter    int
}

// DefaultZMapOptions returns the defaults of the Python package.
func DefaultZMapOptions() ZMapOptions {
	return ZMapOptions{PThreshold: 0.01, Exclude: "hypo", MaxIter: 1}
}

// ZMap makes the z-score map of one image against the controls of a
// reference: the image is intensity-normalised with method, smoothed like
// the controls were, and compared voxel by voxel with the controls
// normalised the same way.  Negative values mean lower uptake.
//
// With covariates the result is a W-score map: the image is compared with
// what the controls predict for a person of that age and sex.
func ZMap(img *Volume, method string, ref *Reference, opt ZMapOptions) (*Volume, []string, error) {
	parts, err := zmapScores(img, method, ref, opt)
	if err != nil {
		return nil, nil, err
	}
	return parts.volume(img, method, ref), parts.warnings, nil
}

// zmapParts is the work of ZMap: the scores, the voxels that were scored,
// and how they came about.
type zmapParts struct {
	scores   []float32
	brain    []bool
	res      *NormResult
	adjusted bool // W-scores, not z-scores
	warnings []string
}

// volume turns the scores into the image ZMap returns.
func (p *zmapParts) volume(img *Volume, method string, ref *Reference) *Volume {
	out := NewVolume(img.Dim, img.Aff)
	copy(out.Data, p.scores)
	kind := "z-score"
	if p.adjusted {
		kind = "w-score"
	}
	out.Descrip = "sugar-pet " + kind + " " + method + " factor=" + fmtG(p.res.Factor) + " n=" + itoa(ref.N)
	if len(out.Descrip) > 79 {
		out.Descrip = out.Descrip[:79]
	}
	return out
}

func zmapScores(img *Volume, method string, ref *Reference, opt ZMapOptions) (*zmapParts, error) {
	if _, err := ref.ControlStats(method); err != nil { // fail before doing any work
		return nil, err
	}
	var row []float64
	var warnings []string
	if opt.Covariates != nil {
		if _, err := ref.ControlModel(method); err != nil {
			return nil, err
		}
		var err error
		if row, warnings, err = DesignRow(ref.Terms, opt.Covariates); err != nil {
			return nil, err
		}
	}
	normOpt := DefaultNormOptions()
	normOpt.Reference = ref
	normOpt.PThreshold, normOpt.Exclude, normOpt.MaxIter = opt.PThreshold, opt.Exclude, opt.MaxIter
	res, err := Normalize(img, method, normOpt)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, res.Warnings...)
	grid := ref.OnGrid(img.Dim, img.Aff)
	nvox := img.NVox()
	var mean, std []float32
	if row == nil {
		stat, _ := grid.ControlStats(method)
		mean, std = stat.Mean, stat.Std
	} else {
		model, _ := grid.ControlModel(method)
		std = model.SD
		mean = make([]float32, nvox)
		for j, beta := range model.Beta {
			w := float32(row[j])
			for v := range mean {
				mean[v] += float32(w * beta[v])
			}
		}
	}
	data := res.Image.Data
	gm, err := GetMask(MaskSpec{Name: "gray-matter"}, img.Dim, img.Aff)
	if err != nil {
		return nil, err
	}
	par, err := GetMask(MaskSpec{Name: "parenchyma"}, img.Dim, img.Aff)
	if err != nil {
		return nil, err
	}
	valid := validVoxels(data)
	smoothed := Smooth(data, img.Dim, img.Aff, grid.FWHM)
	parts := &zmapParts{scores: make([]float32, nvox), brain: make([]bool, nvox), res: res, adjusted: row != nil, warnings: warnings}
	for v := 0; v < nvox; v++ {
		if (gm[v] || par[v]) && valid[v] && std[v] > 0 {
			parts.brain[v] = true
			parts.scores[v] = (smoothed[v] - mean[v]) / std[v]
		}
	}
	return parts, nil
}

// ClusterOptions are the settings of ReferenceCluster.
type ClusterOptions struct {
	PThreshold   float64 // one-sided, uncorrected p-value of the t-test [0.001]
	MinClusterML float64 // connected clusters smaller than this are dropped [1]
	Progress     func(done, total int)
}

// ReferenceCluster finds the voxels that are best preserved in a group of
// patients (Yakushev et al., 2009).  All images are scaled by their global
// mean; regions spared by the disease then look hypermetabolic, and a
// voxel-wise two-sample t-test (patients > controls) picks them out.
func ReferenceCluster(ctx context.Context, patients []string, reference *Reference, opt ClusterOptions) (*Volume, error) {
	if len(patients) < 2 {
		return nil, errorf("at least 2 patient images are needed")
	}
	first, err := LoadImage(patients[0])
	if err != nil {
		return nil, err
	}
	ref := reference.OnGrid(first.Dim, first.Aff)
	nvox := first.NVox()
	total := make([]float64, nvox)
	squares := make([]float64, nvox)
	covered := make([]int32, nvox)
	for i, path := range patients {
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
			return nil, errorf("patient %d is on a different grid than the first patient", i+1)
		}
		valid := validVoxels(img.Data)
		region := make([]bool, nvox)
		for v := range region {
			region[v] = ref.Mask[v] && valid[v]
			if valid[v] {
				covered[v]++
			}
		}
		// Same proportional scaling as the controls of the reference.
		ps, err := regionMean(img.Data, region)
		if err != nil {
			return nil, err
		}
		smoothed := Smooth(img.Data, img.Dim, img.Aff, ref.FWHM)
		scale := float32(ps)
		parallelFor(nvox, func(lo, hi int) {
			for v := lo; v < hi; v++ {
				x := float64(smoothed[v] / scale)
				total[v] += x
				squares[v] += x * x
			}
		})
		if opt.Progress != nil {
			opt.Progress(i+1, len(patients))
		}
	}
	n1, n2 := float64(len(patients)), float64(ref.N)
	limit := tPPF(1.0-opt.PThreshold, n1+n2-2)
	cluster := make([]bool, nvox)
	for v := 0; v < nvox; v++ {
		mean1 := total[v] / n1
		var1 := math.Max(squares[v]-float64(n1*mean1*mean1), 0) / (n1 - 1)
		sd2 := float64(ref.PSStd[v])
		pooled := ((n1-1)*var1 + (n2-1)*sd2*sd2) / (n1 + n2 - 2)
		if ref.Mask[v] && int(covered[v]) == len(patients) && pooled > 0 {
			t := (mean1 - float64(ref.Template[v])) / math.Sqrt(pooled*(1.0/n1+1.0/n2))
			cluster[v] = t > limit
		}
	}
	// Drop small clusters (18-connectivity, as in SPM).
	labels, count := labelComponents(cluster, first.Dim, 18)
	if count > 0 {
		voxelML := VoxelVolumeML(first.Aff)
		sizes := make([]int, count+1)
		for _, l := range labels {
			sizes[l]++
		}
		for v, l := range labels {
			cluster[v] = l > 0 && float64(sizes[l])*voxelML >= opt.MinClusterML
		}
	}
	if countTrue(cluster) == 0 {
		return nil, errorf("no voxels are relatively preserved at this threshold; try a larger p-threshold or a smaller minimum cluster size")
	}
	out := boolVolume(cluster, first.Dim, first.Aff)
	out.Descrip = "sugar-pet reference cluster p<" + fmtG(opt.PThreshold) + " n=" + itoa(len(patients)) + " vs " + itoa(ref.N)
	if len(out.Descrip) > 79 {
		out.Descrip = out.Descrip[:79]
	}
	return out, nil
}
