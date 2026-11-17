package core

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

// This file is the Go port of the parts of GrouVox that sugar-pet uses:
// the two-sample t-test with covariates, the smoothness estimate, GRF
// cluster-level and FDR correction, and the atlas-annotated cluster report.

// CompareOptions are the settings of CompareGroups.
type CompareOptions struct {
	Method       string
	Reference    *Reference
	Covariates   []Covariate // one value per image, group 1 first, then group 2
	Mask         MaskSpec    // replaces the default region of the method
	FWHM         *float64    // smoothing (mm) of the normalised images [that of the reference, else 8]
	AnalysisMask MaskSpec    // voxels to test [gray-matter]
	Correction   string      // "grf", "fdr" or "none"
	VoxelP       float64     // grf: voxel-level p, two-tailed
	ClusterP     float64     // grf: cluster-level p
	Q            float64     // fdr: q value
	Name         string      // prefix of the result files
	KeepImages   bool        // keep the normalised, smoothed images
	Progress     func(done, total int)
}

// DefaultCompareOptions returns the defaults of the Python package.
func DefaultCompareOptions() CompareOptions {
	return CompareOptions{Method: "hn", Correction: "grf", VoxelP: 0.001, ClusterP: 0.05, Q: 0.05, Name: "group1_vs_group2"}
}

// Region is the part of a cluster that lies in one atlas region.
type Region struct {
	Name   string
	Voxels int
	Pct    float64
}

// Cluster is one cluster that survived the correction.
type Cluster struct {
	Size      int
	Peak      float64 // peak z (GRF) or t (FDR) value
	PeakVoxel [3]int
	MNI       [3]float64
	PeakAtlas [3]string   // AAL, Harvard-Oxford cortical, Harvard-Oxford subcortical
	Regions   [3][]Region // overlap with the same atlases
	voxels    []int32
}

// NamedFile is a result file and its role.
type NamedFile struct{ Role, Path string }

// GroupComparison is the outcome of CompareGroups.
type GroupComparison struct {
	TMap             string // T-map; positive: group 1 higher than group 2
	DOF              float64
	FWHM             [3]float64 // estimated smoothness (mm) of the residuals
	Corrected        bool
	NClusters        int
	Clusters         []Cluster
	ZThreshold       float64
	MinClusterVoxels int
	PThreshold       float64
	Files            []NamedFile
	Factors          []float64 // normalisation factor of each image, group 1 first
}

func (g *GroupComparison) file(role string) string {
	for _, f := range g.Files {
		if f.Role == role {
			return f.Path
		}
	}
	return ""
}

// File returns the path of a result file by role, or "".
func (g *GroupComparison) File(role string) string { return g.file(role) }

// CompareGroups compares two groups voxel by voxel (two-sample t-test,
// group 1 minus group 2).  Every image is intensity-normalised, smoothed,
// and entered into a general linear model together with the covariates.
func CompareGroups(ctx context.Context, group1, group2 []string, output string, opt CompareOptions) (*GroupComparison, error) {
	if opt.Correction != "grf" && opt.Correction != "fdr" && opt.Correction != "none" {
		return nil, errorf("correction must be 'grf', 'fdr' or 'none'")
	}
	if !IsMethod(opt.Method) {
		return nil, errorf("unknown method '%s'; choose from: %s", opt.Method, methodNames())
	}
	if min(len(group1), len(group2)) < 2 {
		return nil, errorf("each group needs at least 2 images")
	}
	images := append(append([]string(nil), group1...), group2...)
	n1, n := len(group1), len(images)
	var covColumns [][]float64
	if len(opt.Covariates) > 0 {
		design, _, err := BuildDesign(opt.Covariates)
		if err != nil {
			return nil, err
		}
		if len(design) != n {
			return nil, errorf("%d covariate values for %d images (group 1 first, then group 2)", len(design), n)
		}
		covColumns = design
	}
	ref := opt.Reference
	fwhm := 8.0
	if opt.FWHM != nil {
		fwhm = *opt.FWHM
	} else if ref != nil {
		fwhm = ref.FWHM
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return nil, err
	}
	prefix := filepath.Join(output, opt.Name)
	imageDir := prefix + "_images"
	if opt.KeepImages {
		if err := os.MkdirAll(imageDir, 0o755); err != nil {
			return nil, err
		}
	}

	normOpt := DefaultNormOptions()
	normOpt.Reference, normOpt.Mask = ref, opt.Mask
	var first *Volume
	var covered []bool
	smoothed := make([][]float32, n)
	factors := make([]float64, n)
	for i, path := range images {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		res, err := NormalizeFile(path, opt.Method, normOpt)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		img := res.Image
		if first == nil {
			first = img
			covered = make([]bool, img.NVox())
			for v := range covered {
				covered[v] = true
			}
		} else if !SameGrid(img.Dim, img.Aff, first.Dim, first.Aff) {
			return nil, errorf("image %d is on a different grid than the first image", i+1)
		}
		for v, ok := range validVoxels(img.Data) {
			covered[v] = covered[v] && ok
		}
		smoothed[i] = Smooth(img.Data, img.Dim, img.Aff, fwhm)
		factors[i] = res.Factor
		if opt.KeepImages {
			group := "group1"
			if i >= n1 {
				group = "group2"
			}
			vol := &Volume{Dim: img.Dim, NT: 1, Data: smoothed[i], Aff: img.Aff}
			if err := SaveNifti(filepath.Join(imageDir, fmt.Sprintf("%s_%04d.nii.gz", group, i)), vol, false); err != nil {
				return nil, err
			}
		}
		if opt.Progress != nil {
			opt.Progress(i+1, n)
		}
	}

	analysis := opt.AnalysisMask
	if analysis.IsZero() {
		analysis = MaskSpec{Name: "gray-matter"}
	}
	mask, err := GetMask(analysis, first.Dim, first.Aff)
	if err != nil {
		return nil, err
	}
	for v := range mask {
		mask[v] = mask[v] && covered[v]
	}
	if countTrue(mask) < 1000 {
		return nil, errorf("the analysis mask barely overlaps the images; are they in MNI space?")
	}
	result := &GroupComparison{Factors: factors}
	maskPath, factorsPath := prefix+"_mask.nii.gz", prefix+"_factors.tsv"
	result.Files = append(result.Files, NamedFile{"mask", maskPath}, NamedFile{"factors", factorsPath})
	if err := SaveMask(maskPath, mask, first.Dim, first.Aff, ""); err != nil {
		return nil, err
	}
	table := "image\tgroup\tmethod\tfactor\n"
	for i, path := range images {
		group := 1
		if i >= n1 {
			group = 2
		}
		table += fmt.Sprintf("%s\t%d\t%s\t%s\n", path, group, opt.Method, fmtG(factors[i]))
	}
	if err := os.WriteFile(factorsPath, []byte(table), 0o644); err != nil {
		return nil, err
	}

	// Design matrix: cell means [G1, G2, covariates].
	p := 2
	if covColumns != nil {
		p += len(covColumns[0]) - 1
	}
	design := make([][]float64, n)
	for i := range design {
		design[i] = make([]float64, p)
		if i < n1 {
			design[i][0] = 1
		} else {
			design[i][1] = 1
		}
		if covColumns != nil {
			copy(design[i][2:], covColumns[i][1:])
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	test, err := twoSampleTTest(smoothed, design, mask, first.Dim, first.Aff)
	if err != nil {
		return nil, err
	}
	smoothed = nil
	descrip := fmt.Sprintf("GrouVox{T_[%.1f]}{dLh_%.6f}{FWHMx_%.4f FWHMy_%.4f FWHMz_%.4f mm}",
		test.dof, test.dlh, test.fwhm[0], test.fwhm[1], test.fwhm[2])
	if len(descrip) > 80 {
		descrip = descrip[:80]
	}
	tPath := prefix + "_T.nii.gz"
	outputs := []struct {
		role, path string
		vol        *Volume
	}{
		{"t_map", tPath, &Volume{Dim: first.Dim, NT: 1, Data: test.t, Aff: first.Aff, Descrip: descrip}},
		{"beta", prefix + "_beta.nii.gz", &Volume{Dim: first.Dim, NT: p, Data: test.beta, Aff: first.Aff, Descrip: descrip}},
		{"effect_size", prefix + "_cohen_f2.nii.gz", &Volume{Dim: first.Dim, NT: 1, Data: test.f2, Aff: first.Aff, Descrip: descrip}},
	}
	for _, o := range outputs {
		if err := SaveNifti(o.path, o.vol, false); err != nil {
			return nil, err
		}
		result.Files = append(result.Files, NamedFile{o.role, o.path})
	}
	if opt.KeepImages {
		result.Files = append(result.Files, NamedFile{"images", imageDir})
	}
	result.TMap, result.DOF, result.FWHM = tPath, test.dof, test.fwhm

	if opt.Correction == "none" {
		return result, nil
	}
	// The header keeps six decimals of dLh, and that is what GrouVox reads back.
	dlh, _ := strconv.ParseFloat(fmt.Sprintf("%.6f", test.dlh), 64)
	tName := filepath.Base(tPath)
	stem := StripExt(tName)
	tVol := outputs[0].vol
	result.Corrected = true
	var report string
	if opt.Correction == "grf" {
		report = filepath.Join(output, "ClusterReport_"+stem+".csv")
		os.Remove(report)
		grf := grfCorrection(tVol, mask, test.dof, dlh, opt.VoxelP, opt.ClusterP)
		result.ZThreshold, result.MinClusterVoxels = grf.zThr, grf.minSize
		result.Clusters = grf.clusters
		zPath := filepath.Join(output, "Z_ClusterThresholded_"+tName)
		thrPath := filepath.Join(output, "ClusterThresholded_"+tName)
		if err := SaveNifti(zPath, &Volume{Dim: first.Dim, NT: 1, Data: grf.z, Aff: first.Aff, Descrip: descrip}, false); err != nil {
			return nil, err
		}
		if err := SaveNifti(thrPath, &Volume{Dim: first.Dim, NT: 1, Data: grf.t, Aff: first.Aff, Descrip: descrip}, false); err != nil {
			return nil, err
		}
		result.Files = append(result.Files, NamedFile{"thresholded", thrPath})
	} else {
		report = filepath.Join(output, "ClusterReport_FDR_"+stem+".csv")
		os.Remove(report)
		fdr := fdrCorrection(tVol, mask, test.dof, opt.Q)
		result.PThreshold = fdr.pThr
		result.Clusters = fdr.clusters
		thrPath := filepath.Join(output, "FDR_Thresholded_"+tName)
		if err := SaveNifti(thrPath, &Volume{Dim: first.Dim, NT: 1, Data: fdr.t, Aff: first.Aff, Descrip: descrip}, false); err != nil {
			return nil, err
		}
		result.Files = append(result.Files, NamedFile{"thresholded", thrPath})
	}
	result.NClusters = len(result.Clusters)
	annotateClusters(result.Clusters, first.Dim, first.Aff)
	if len(result.Clusters) > 0 {
		if err := writeClusterCSV(result.Clusters, report); err != nil {
			return nil, err
		}
		result.Files = append(result.Files, NamedFile{"cluster_report", report})
	}
	return result, nil
}

type tTest struct {
	t, beta, f2 []float32
	dof         float64
	fwhm        [3]float64
	dlh         float64
}

// twoSampleTTest fits the model at every voxel of the mask and estimates
// the smoothness of the residuals.
func twoSampleTTest(images [][]float32, design [][]float64, mask []bool, dim [3]int, aff Affine) (*tTest, error) {
	n, p := len(design), len(design[0])
	if !fullRank(design) {
		return nil, errorf("the model cannot be estimated: a covariate is confounded with the groups (or with another covariate)")
	}
	dof := float64(n - p)
	if dof < 1 {
		return nil, errorf("too few images (%d) for a model with %d terms", n, p)
	}
	xtxInv, ok := invertMatrix(gram(design))
	if !ok {
		return nil, errorf("the model cannot be estimated: the design matrix is singular")
	}
	pin := make([][]float64, p) // (X'X)^-1 X'
	for j := range pin {
		pin[j] = make([]float64, n)
		for i := 0; i < n; i++ {
			for k := 0; k < p; k++ {
				pin[j][i] += xtxInv[j][k] * design[i][k]
			}
		}
	}
	// Contrast: group 1 minus group 2.
	denomFactor := math.Sqrt(xtxInv[0][0] - xtxInv[0][1] - xtxInv[1][0] + xtxInv[1][1])

	// Reduced model for Cohen's f2: an intercept in place of the two groups.
	reduced := make([][]float64, n)
	for i := range reduced {
		reduced[i] = append([]float64{1}, design[i][2:]...)
	}
	pinR, ok := pinv(reduced)
	if !ok {
		return nil, errorf("the model cannot be estimated: the covariates are redundant")
	}
	pr := len(reduced[0])

	nvox := len(mask)
	index := make([]int32, nvox) // voxel -> row of the residual table, or -1
	var voxels []int32
	for v, m := range mask {
		index[v] = -1
		if m {
			index[v] = int32(len(voxels))
			voxels = append(voxels, int32(v))
		}
	}
	out := &tTest{t: make([]float32, nvox), beta: make([]float32, nvox*p), f2: make([]float32, nvox), dof: dof}
	normed := make([]float32, len(voxels)*n) // residuals / their SD, voxel by voxel
	parallelFor(len(voxels), func(lo, hi int) {
		y := make([]float64, n)
		beta := make([]float64, p)
		betaR := make([]float64, pr)
		for k := lo; k < hi; k++ {
			v := int(voxels[k])
			for i := range y {
				y[i] = float64(images[i][v])
			}
			for j := 0; j < p; j++ {
				var s float64
				for i := 0; i < n; i++ {
					s += pin[j][i] * y[i]
				}
				beta[j] = s
				out.beta[j*nvox+v] = float32(s)
			}
			res := normed[k*n : (k+1)*n]
			var sse, mean float64
			for i := 0; i < n; i++ {
				fit := 0.0
				for j := 0; j < p; j++ {
					fit += design[i][j] * beta[j]
				}
				r := y[i] - fit
				sse += r * r
				res[i] = float32(r)
				mean += float64(res[i])
			}
			for j := 0; j < pr; j++ {
				var s float64
				for i := 0; i < n; i++ {
					s += pinR[j][i] * y[i]
				}
				betaR[j] = s
			}
			var sseR float64
			for i := 0; i < n; i++ {
				fit := 0.0
				for j := 0; j < pr; j++ {
					fit += reduced[i][j] * betaR[j]
				}
				r := y[i] - fit
				sseR += r * r
			}
			if sse > 0 {
				if denom := math.Sqrt(sse/dof) * denomFactor; denom > 0 {
					out.t[v] = float32((beta[0] - beta[1]) / denom)
				}
				out.f2[v] = float32((sseR - sse) / sse)
			} else {
				out.f2[v] = float32(sseR - sse)
			}
			// Standardise the residuals for the smoothness estimate.
			mean /= float64(n)
			var ss float64
			for _, r := range res {
				d := float64(r) - mean
				ss += d * d
			}
			sd := float32(math.Sqrt(ss / float64(n-1)))
			if sd > 0 {
				for i := range res {
					res[i] /= sd
				}
			}
		}
	})

	// Smoothness from the lag-1 autocorrelation along each axis (FSL / DPABI).
	nx, ny, nz := dim[0], dim[1], dim[2]
	steps := [3]int{1, nx, nx * ny}
	var ssMinus, ssTotal [3]float64
	var mu sync.Mutex
	parallelFor(nz, func(lo, hi int) {
		var minus, total [3]float64
		for z := lo; z < hi; z++ {
			for y := 0; y < ny; y++ {
				for x := 0; x < nx; x++ {
					v := x + nx*(y+ny*z)
					kb := index[v]
					if kb < 0 {
						continue
					}
					b := normed[int(kb)*n : (int(kb)+1)*n]
					for axis, ok := range [3]bool{x+1 < nx, y+1 < ny, z+1 < nz} {
						if !ok {
							continue
						}
						ka := index[v+steps[axis]]
						if ka < 0 {
							continue
						}
						a := normed[int(ka)*n : (int(ka)+1)*n]
						var ab, aa, bb float64
						for i := range a {
							fa, fb := float64(a[i]), float64(b[i])
							ab += fa * fb
							aa += fa * fa
							bb += fb * fb
						}
						minus[axis] += ab
						total[axis] += 0.5 * (aa + bb)
					}
				}
			}
		}
		mu.Lock()
		for i := 0; i < 3; i++ {
			ssMinus[i] += minus[i]
			ssTotal[i] += total[i]
		}
		mu.Unlock()
	})
	zoom := VoxelSizes(aff)
	var sigmaSq [3]float64
	for i := 0; i < 3; i++ {
		sigmaSq[i] = 1
		if ssTotal[i] > 0 {
			rho := math.Min(math.Max(ssMinus[i]/ssTotal[i], 1e-15), 1-1e-15)
			sigmaSq[i] = -1.0 / (4.0 * math.Log(math.Abs(rho)))
		}
		out.fwhm[i] = math.Sqrt(8.0*math.Ln2*sigmaSq[i]) * zoom[i]
	}
	out.dlh = dofScale(math.Pow(sigmaSq[0]*sigmaSq[1]*sigmaSq[2], -0.5)/math.Sqrt(8.0), dof)
	return out, nil
}

// dofScale applies the DOF-dependent correction of DPABI.
func dofScale(dlh, dof float64) float64 {
	if dof < 6 {
		return dlh * 1.1
	}
	if dof > 500 {
		return dlh * math.Sqrt(1.0321/dof+1)
	}
	dofs := []float64{6, 7, 8, 9, 10, 12, 15, 20, 30, 50, 100, 200, 500}
	scales := []float64{1.08, 1.07, 1.06, 1.05, 1.04, 1.03, 1.025, 1.02, 1.015, 1.01, 1.005, 1.002, 1.001}
	for i := 1; i < len(dofs); i++ {
		if dof <= dofs[i] {
			f := (dof - dofs[i-1]) / (dofs[i] - dofs[i-1])
			return dlh * (scales[i-1] + f*(scales[i]-scales[i-1]))
		}
	}
	return dlh * scales[len(scales)-1]
}

// clustersOf groups the voxels of a labelled volume by label, each in
// NumPy's C order.
func clustersOf(labels []int32, count int, dim [3]int) [][]int32 {
	groups := make([][]int32, count)
	cOrder(dim, func(idx, _, _, _ int) {
		if l := labels[idx]; l > 0 {
			groups[l-1] = append(groups[l-1], int32(idx))
		}
	})
	return groups
}

func peakOf(voxels []int32, value func(v int32) float64, dim [3]int) (float64, [3]int) {
	best, bestAbs := voxels[0], math.Inf(-1)
	for _, v := range voxels {
		if a := math.Abs(value(v)); a > bestAbs {
			best, bestAbs = v, a
		}
	}
	i := int(best)
	return value(best), [3]int{i % dim[0], (i / dim[0]) % dim[1], i / (dim[0] * dim[1])}
}

type grfResult struct {
	zThr     float64
	minSize  int
	z, t     []float32
	clusters []Cluster
}

// tToZ gives the z-score that has the tail probability of a t-statistic,
// with its sign.  Like GrouVox from 0.2.1 on, it matches the upper tails, and
// does so in log space where the probability is too small for a number, so
// the z-score is finite for every finite t.  (To take the quantile of 1 - p,
// as GrouVox did before, gives infinity for a |z| above about 8.2.)
func tToZ(t, dof float64) float64 {
	var z float64
	if p := tSF(math.Abs(t), dof); p > 1e-290 {
		z = -normPPF(p)
	} else {
		z = normISFLog(tLogSF(math.Abs(t), dof))
	}
	if t < 0 {
		return -z
	}
	return z
}

// grfCorrection is the two-tailed cluster-level correction of GrouVox,
// based on Gaussian random field theory (Friston et al., 1994).
func grfCorrection(tVol *Volume, mask []bool, dof, dlh, voxelP, clusterP float64) *grfResult {
	nvox := tVol.NVox()
	z := make([]float64, nvox)
	parallelFor(nvox, func(lo, hi int) {
		for v := lo; v < hi; v++ {
			t := float64(tVol.Data[v])
			if !mask[v] || t == 0 || math.IsNaN(t) {
				continue
			}
			z[v] = tToZ(t, dof)
		}
	})
	res := &grfResult{zThr: normPPF(1.0 - voxelP/2.0), z: make([]float32, nvox), t: make([]float32, nvox)}
	// Each tail is corrected at clusterP / 2, so both together make clusterP.
	res.minSize = grfClusterThreshold(countTrue(mask), dlh, res.zThr, clusterP/2.0)
	for _, positive := range []bool{true, false} {
		supra := make([]bool, nvox)
		for v := range supra {
			if positive {
				supra[v] = z[v] >= res.zThr
			} else {
				supra[v] = z[v] <= -res.zThr
			}
		}
		labels, count := labelComponents(supra, tVol.Dim, 26)
		for _, voxels := range clustersOf(labels, count, tVol.Dim) {
			if len(voxels) < res.minSize {
				continue
			}
			for _, v := range voxels {
				res.z[v] = float32(z[v])
				res.t[v] = tVol.Data[v]
			}
			peak, where := peakOf(voxels, func(v int32) float64 { return z[v] }, tVol.Dim)
			res.clusters = append(res.clusters, Cluster{Size: len(voxels), Peak: peak, PeakVoxel: where, voxels: voxels})
		}
	}
	sort.SliceStable(res.clusters, func(i, j int) bool { return res.clusters[i].Size > res.clusters[j].Size })
	return res
}

// grfClusterThreshold is the smallest cluster size that is significant.
func grfClusterThreshold(nVoxels int, dlh, zThr, clusterP float64) int {
	const d = 3.0
	nv := float64(nVoxels)
	em := nv * math.Pow(2*math.Pi, -(d+1)/2.0) * dlh * math.Pow(zThr*zThr-1, (d-1)/2.0) * math.Exp(-(zThr*zThr)/2.0)
	en := nv * normSF(zThr)
	if em <= 0 || en <= 0 || math.IsNaN(em) {
		return 1
	}
	beta := math.Pow(math.Gamma(d/2.0+1)*em/en, 2.0/d)
	size := 0
	p := 1.0
	for p >= clusterP && size < nVoxels {
		size++
		p = 1.0 - math.Exp(-em*math.Exp(-beta*math.Pow(float64(size), 2.0/d)))
	}
	return max(size, 1)
}

type fdrResult struct {
	pThr     float64
	t        []float32
	clusters []Cluster
}

// fdrCorrection is the two-tailed Benjamini-Hochberg correction of GrouVox.
func fdrCorrection(tVol *Volume, mask []bool, dof, q float64) *fdrResult {
	nvox := tVol.NVox()
	var voxels []int32
	for v, m := range mask {
		if m {
			voxels = append(voxels, int32(v))
		}
	}
	pvals := make([]float64, len(voxels))
	parallelFor(len(voxels), func(lo, hi int) {
		for k := lo; k < hi; k++ {
			pvals[k] = 2.0 * tSF(math.Abs(float64(tVol.Data[voxels[k]])), dof)
		}
	})
	sorted := append([]float64(nil), pvals...)
	sort.Float64s(sorted)
	res := &fdrResult{t: make([]float32, nvox)}
	m := float64(len(sorted))
	kMax := -1
	for k, p := range sorted {
		if p <= float64(k+1)/m*q {
			kMax = k
		}
	}
	if kMax < 0 {
		return res
	}
	res.pThr = sorted[kMax]
	sig := make([]bool, nvox)
	for k, v := range voxels {
		if pvals[k] <= res.pThr {
			sig[v] = true
			res.t[v] = tVol.Data[v]
		}
	}
	labels, count := labelComponents(sig, tVol.Dim, 26)
	for _, group := range clustersOf(labels, count, tVol.Dim) {
		peak, where := peakOf(group, func(v int32) float64 { return float64(tVol.Data[v]) }, tVol.Dim)
		res.clusters = append(res.clusters, Cluster{Size: len(group), Peak: peak, PeakVoxel: where, voxels: group})
	}
	sort.SliceStable(res.clusters, func(i, j int) bool { return res.clusters[i].Size > res.clusters[j].Size })
	return res
}

type reportAtlas struct {
	labels []int32
	dim    [3]int
	inv    Affine
	names  map[int]string
}

// AtlasNames are the atlases of the cluster report, in column order.
var AtlasNames = [3]string{"AAL", "Harvard-Oxford cortical", "Harvard-Oxford subcortical"}

var reportAtlases = sync.OnceValue(func() [3]*reportAtlas {
	var out [3]*reportAtlas
	for i, name := range []string{"aal", "ho_cort", "ho_sub"} {
		vol := loadEmbedded(name + ".nii.gz")
		a := &reportAtlas{dim: vol.Dim, names: readLabelsJSON(name + "_labels.json"), labels: make([]int32, vol.NVox())}
		for k, v := range vol.Data[:vol.NVox()] {
			a.labels[k] = int32(v)
		}
		a.inv, _ = invAffine(vol.Aff)
		out[i] = a
	}
	return out
})

const emDash = "—"

// round1 rounds like Python's round(x, 1): on the exact decimal value.
func round1(x float64) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 1, 64), 64)
	return v
}

// annotateClusters adds MNI coordinates and atlas labels to the clusters.
func annotateClusters(clusters []Cluster, dim [3]int, aff Affine) {
	if len(clusters) == 0 {
		return
	}
	nx, ny := dim[0], dim[1]
	atlases := reportAtlases()
	for c := range clusters {
		cl := &clusters[c]
		pk := cl.PeakVoxel
		for r := 0; r < 3; r++ {
			cl.MNI[r] = round1(aff[r][0]*float64(pk[0]) + aff[r][1]*float64(pk[1]) + aff[r][2]*float64(pk[2]) + aff[r][3])
		}
		for a, atlas := range atlases {
			toAtlas := mulAffine(atlas.inv, aff)
			lookup := func(x, y, z int) int {
				var idx [3]int
				for r := 0; r < 3; r++ {
					coord := toAtlas[r][0]*float64(x) + toAtlas[r][1]*float64(y) + toAtlas[r][2]*float64(z) + toAtlas[r][3]
					idx[r] = int(math.RoundToEven(coord))
					if idx[r] < 0 || idx[r] >= atlas.dim[r] {
						return 0
					}
				}
				return int(atlas.labels[idx[0]+atlas.dim[0]*(idx[1]+atlas.dim[1]*idx[2])])
			}
			cl.PeakAtlas[a] = emDash
			if id := lookup(pk[0], pk[1], pk[2]); id != 0 {
				if name, ok := atlas.names[id]; ok {
					cl.PeakAtlas[a] = name
				}
			}
			counts := map[int]int{}
			for _, v := range cl.voxels {
				i := int(v)
				counts[lookup(i%nx, (i/nx)%ny, i/(nx*ny))]++
			}
			ids := make([]int, 0, len(counts))
			for id := range counts {
				ids = append(ids, id)
			}
			sort.Ints(ids)
			var regions []Region
			for _, id := range ids {
				name, ok := atlas.names[id]
				if id == 0 || !ok {
					continue
				}
				regions = append(regions, Region{name, counts[id], round1(100.0 * float64(counts[id]) / float64(len(cl.voxels)))})
			}
			sort.SliceStable(regions, func(i, j int) bool { return regions[i].Voxels > regions[j].Voxels })
			cl.Regions[a] = regions
		}
	}
}

// pyFloat writes a number rounded to one decimal the way Python prints it.
func pyFloat(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// RegionsText lists the regions of a cluster as "Name(12.5%); ...".
func RegionsText(regions []Region) string {
	if len(regions) == 0 {
		return emDash
	}
	text := ""
	for i, r := range regions {
		if i > 0 {
			text += "; "
		}
		text += r.Name + "(" + pyFloat(r.Pct) + "%)"
	}
	return text
}

// PeakText formats the peak value of a cluster.
func PeakText(v float64) string {
	switch {
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	return strconv.FormatFloat(v, 'f', 3, 64)
}

// writeClusterCSV writes the cluster report, one row per cluster.
func writeClusterCSV(clusters []Cluster, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	w.UseCRLF = true
	w.Write([]string{"Cluster", "Size", "PeakZ", "MNI_X", "MNI_Y", "MNI_Z",
		"AAL_Peak", "AAL_Regions", "HO_Cort_Peak", "HO_Cort_Regions", "HO_Sub_Peak", "HO_Sub_Regions"})
	for i, c := range clusters {
		row := []string{strconv.Itoa(i + 1), strconv.Itoa(c.Size), PeakText(c.Peak),
			pyFloat(c.MNI[0]), pyFloat(c.MNI[1]), pyFloat(c.MNI[2])}
		for a := 0; a < 3; a++ {
			row = append(row, c.PeakAtlas[a], RegionsText(c.Regions[a]))
		}
		w.Write(row)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
