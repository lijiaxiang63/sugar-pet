package core

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Atypical parkinsonism: which published FDG pattern does one patient's
// scan look like?  One image is compared with the healthy controls of a
// reference (a z-score or W-score map), the map is summarised over the
// regions that tell the parkinsonian syndromes apart, and the regional
// profile is matched against the patterns described for MSA, PSP, CBD, DLB
// and PD.  The result is decision support for the reader of the scan, not
// a diagnosis.

// RegionDef names a region of the classification; Paired: one on each side.
type RegionDef struct {
	Name   string
	Paired bool
}

// ClassRegions lists the regions in the order of the report.
var ClassRegions = []RegionDef{
	{"caudate", true},
	{"putamen", true},
	{"posterior-putamen", true},
	{"pallidum", true},
	{"thalamus", true},
	{"midbrain", false},
	{"pons", false},
	{"cerebellum", true},
	{"vermis", false},
	{"medial-frontal", true},
	{"lateral-frontal", true},
	{"sensorimotor", true},
	{"parietal", true},
	{"precuneus", true},
	{"posterior-cingulate", true},
	{"lateral-temporal", true},
	{"occipital", true},
}

// CerebrA labels of the right hemisphere; the left ones are 51 higher.
var classRight = []struct {
	name string
	ids  []int
}{
	{"caudate", []int{49}},
	{"putamen", []int{21}},
	{"pallidum", []int{27}},
	{"thalamus", []int{40}},
	{"cerebellum", []int{46, 39}},          // gray and white matter
	{"sensorimotor", []int{35, 13, 16}},    // pre-, post- and paracentral
	{"parietal", []int{9, 10, 51}},         // superior and inferior parietal, supramarginal
	{"precuneus", []int{31}},               //
	{"posterior-cingulate", []int{47, 33}}, // posterior and isthmus cingulate
	{"lateral-temporal", []int{3, 28, 45}}, // inferior, middle and superior temporal
	{"occipital", []int{34, 43, 6, 12}},    // lateral occipital, cuneus, pericalcarine, lingual
}

var (
	lblAnteriorCingulate = []int{8, 30}         // rostral and caudal
	lblLateralFrontal    = []int{1, 42, 32, 22} // rostral and caudal middle frontal, pars opercularis and triangularis
)

const (
	lblSuperiorFrontal = 38
	medialX            = 12.0  // the superior frontal gyrus within this distance (mm) of the midline counts as medial
	midbrainZ          = -26.0 // the brainstem above this MNI z (mm), where the built-in pons ends
)

// Patterns lists the patterns a scan is matched against.
var Patterns = []string{"msa-p", "msa-c", "psp", "cbd", "dlb", "pd"}

// The two forms of MSA are one disease; PD is read on its own.
var patternGroup = []Named{{"msa-p", "msa"}, {"msa-c", "msa"}, {"psp", "psp"}, {"cbd", "cbd"}, {"dlb", "dlb"}}

const (
	// A region counts as abnormal from |score| 2, as mildly abnormal from 1.5.
	scoreAbnormal, scoreMild = 2.0, 1.5
	// A finding is robust when every other normalisation shows it with at least this |score|.
	scoreRobust = 1.0
	// A region needs this share of its voxels inside the image to be scored.
	minCoverage = 0.5
	// A voxel centre this close (in voxels) to the edge of the grid still counts as inside it.
	gridEdge = 1e-6
	// Evidence of a regional decrease (increase) grows from 0 at |score| 1 to 1 at 2.5.
	evidenceFrom, evidenceSpan = 1.0, 1.5
	// The hemispheres must differ by more than 0.5 for an asymmetric pattern, and by 1.5 for full evidence.
	asymmetryFrom, asymmetrySpan = 0.5, 1.0
	// The posterior cingulate must be this much better preserved than precuneus and occipital cortex.
	islandFrom, islandSpan = 0.5, 1.5
	// A pattern is the sum of its core (60 %) and supportive (40 %) features.
	weightCore, weightSupport = 0.6, 0.4
	// Reading of the scores: see conclude.
	scoreMatch, scoreWeak, scoreMargin, scoreHigh, scoreHighMargin = 0.5, 0.3, 0.15, 0.7, 0.3
	// Above this share of the brain below -2, the decrease is called widespread.
	shareWidespread = 0.25
)

type regionKey struct{ name, side string }

// regionMasks holds the regions on the 2 mm atlas grid (shared: do not modify).
var regionMasks = sync.OnceValue(func() map[regionKey][]bool {
	a := atlasOnce()
	nx, ny := a.dim[0], a.dim[1]
	n := len(a.labels)
	shifted := func(ids []int, shift int) []int {
		out := make([]int, len(ids))
		for i, v := range ids {
			out[i] = v + shift
		}
		return out
	}
	out := map[regionKey][]bool{}
	for _, s := range []struct {
		side  string
		shift int
	}{{"right", 0}, {"left", 51}} {
		for _, r := range classRight {
			out[regionKey{r.name, s.side}] = isIn(a.labels, shifted(r.ids, s.shift))
		}
		putamen := out[regionKey{"putamen", s.side}]
		posterior := make([]bool, n)
		medial := isIn(a.labels, shifted(lblAnteriorCingulate, s.shift))
		lateral := isIn(a.labels, shifted(lblLateralFrontal, s.shift))
		for i := 0; i < n; i++ {
			x := a.aff[0][0]*float64(i%nx) + a.aff[0][3]
			y := a.aff[1][1]*float64((i/nx)%ny) + a.aff[1][3]
			posterior[i] = putamen[i] && y < 0
			if int(a.labels[i]) == lblSuperiorFrontal+s.shift {
				if math.Abs(x) <= medialX {
					medial[i] = true
				} else {
					lateral[i] = true
				}
			}
		}
		out[regionKey{"posterior-putamen", s.side}] = posterior
		out[regionKey{"medial-frontal", s.side}] = medial
		out[regionKey{"lateral-frontal", s.side}] = lateral
	}
	midbrain := isIn(a.labels, lblBrainstem)
	for i := range midbrain {
		if !(a.aff[2][2]*float64(i/(nx*ny))+a.aff[2][3] > midbrainZ) {
			midbrain[i] = false
		}
	}
	out[regionKey{"midbrain", "both"}] = midbrain
	out[regionKey{"pons", "both"}] = builtin("pons")
	out[regionKey{"vermis", "both"}] = isIn(a.labels, lblVermis)
	for _, r := range ClassRegions {
		if r.Paired {
			left, right := out[regionKey{r.Name, "left"}], out[regionKey{r.Name, "right"}]
			both := make([]bool, n)
			for i := range both {
				both[i] = left[i] || right[i]
			}
			out[regionKey{r.Name, "both"}] = both
		}
	}
	return out
})

// RegionMask returns one region of the classification as an image on the
// atlas grid; side is "left", "right" or "both".
func RegionMask(name, side string) (*Volume, error) {
	mask, ok := regionMasks()[regionKey{name, side}]
	if !ok {
		return nil, errorf("no region '%s' (%s)", name, side)
	}
	a := atlasOnce()
	return boolVolume(mask, a.dim, a.aff), nil
}

// DefaultChecks are the normalisations a finding is checked against: global
// scaling, and white matter if the reference has it.
func DefaultChecks(ref *Reference, method string) []string {
	out := []string{}
	for _, m := range []string{"ps", "white-matter"} {
		_, stored := ref.Stats[m]
		if (m == "ps" || stored) && m != method {
			out = append(out, m)
		}
	}
	return out
}

// RegionScore is what the map shows in one region.
type RegionScore struct {
	Z        *float64            `json:"z"`        // mean score; nil if the region is not inside the image
	Coverage float64             `json:"coverage"` // share of the region inside the image
	Below    *float64            `json:"below"`    // share of its voxels at -2 or lower
	Above    *float64            `json:"above"`    // share at 2 or higher
	Checks   map[string]*float64 `json:"checks"`   // mean score under the other normalisations
	Status   string              `json:"status"`   // hypo, mild-hypo, normal, mild-hyper, hyper or none
	Robust   bool                `json:"robust"`   // every other normalisation shows the finding too
}

// RegionEntry is one row of the table of regions.
type RegionEntry struct {
	ID          string
	Paired      bool
	Left, Right *RegionScore // nil for a region on the midline
	Both        *RegionScore
	Difference  *float64 // left minus right
}

// Side returns the scores of one side ("left", "right" or "both").
func (e *RegionEntry) Side(side string) *RegionScore {
	switch side {
	case "left":
		return e.Left
	case "right":
		return e.Right
	}
	return e.Both
}

func (e RegionEntry) MarshalJSON() ([]byte, error) {
	if !e.Paired {
		return json.Marshal(struct {
			ID   string       `json:"id"`
			Both *RegionScore `json:"both"`
		}{e.ID, e.Both})
	}
	return json.Marshal(struct {
		ID         string       `json:"id"`
		Left       *RegionScore `json:"left"`
		Right      *RegionScore `json:"right"`
		Both       *RegionScore `json:"both"`
		Difference *float64     `json:"difference"`
	}{e.ID, e.Left, e.Right, e.Both, e.Difference})
}

// Feature is one thing a pattern asks for, and how far the map shows it.
type Feature struct {
	Role      string   `json:"role"`      // core or support
	Region    string   `json:"region"`    //
	Side      string   `json:"side"`      //
	Direction string   `json:"direction"` // hypo, hyper, asymmetry, island or spared
	Z         *float64 `json:"z"`         //
	Evidence  float64  `json:"evidence"`  // 0 to 1
}

// Pattern is how well the regional profile matches one published pattern
// (0 to 1).  Robust is the match that is left without the findings that
// change with the normalisation.
type Pattern struct {
	ID       string    `json:"id"`
	Score    float64   `json:"score"`
	Core     float64   `json:"core"`
	Support  float64   `json:"support"`
	Features []Feature `json:"features"`
	Robust   float64   `json:"robust"`
}

// Conclusion is the reading of the pattern scores.  Category is "pattern"
// (one pattern), "indeterminate" (two that cannot be told apart),
// "possible" (partial features), "pd" or "none"; the rest is empty when it
// does not apply.
type Conclusion struct {
	Category    string
	Pattern     string
	Alternative string
	Confidence  string
	Side        string
}

func (c Conclusion) MarshalJSON() ([]byte, error) {
	null := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	return json.Marshal(struct {
		Category    string  `json:"category"`
		Pattern     *string `json:"pattern"`
		Alternative *string `json:"alternative"`
		Confidence  *string `json:"confidence"`
		Side        *string `json:"side"`
	}{c.Category, null(c.Pattern), null(c.Alternative), null(c.Confidence), null(c.Side)})
}

// Note is a remark of the report: a sentence with numbers, or with regions.
type Note struct {
	Key     string
	Values  []int
	Regions [][2]string // region and side; nil for a note with numbers
}

func (n Note) MarshalJSON() ([]byte, error) {
	if n.Regions != nil {
		return json.Marshal(struct {
			Key     string      `json:"key"`
			Regions [][2]string `json:"regions"`
		}{n.Key, n.Regions})
	}
	values := n.Values
	if values == nil {
		values = []int{}
	}
	return json.Marshal(struct {
		Key    string `json:"key"`
		Values []int  `json:"values"`
	}{n.Key, values})
}

// ClassReference says which controls a classification was made against.
type ClassReference struct {
	NControls  int      `json:"n_controls"`
	FWHM       float64  `json:"fwhm_mm"`
	Covariates []string `json:"covariates"`
}

// BrainShare is the share of the brain that is abnormal.
type BrainShare struct {
	Below float64 `json:"below"`
	Above float64 `json:"above"`
}

// Classification is the outcome of Classify: everything that was measured
// and concluded, the map, and the image the map was made from.
type Classification struct {
	Version    string             `json:"sugar_pet_version"`
	Image      string             `json:"image"`
	Score      string             `json:"score"` // "z" or "w"
	Covariates map[string]string  `json:"covariates"`
	Reference  ClassReference     `json:"reference"`
	Method     string             `json:"method"`
	Checks     []string           `json:"checks"`
	Factors    map[string]float64 `json:"factors"`
	Brain      BrainShare         `json:"brain"`
	Regions    []RegionEntry      `json:"regions"`
	Patterns   []Pattern          `json:"patterns"`
	Conclusion Conclusion         `json:"conclusion"`
	Notes      []Note             `json:"notes"`
	Warnings   []string           `json:"warnings"`

	ZMap       *Volume `json:"-"` // the z-score or W-score map under the main normalisation
	Normalised *Volume `json:"-"` // the intensity-normalised image
}

// ClassifyOptions are the settings of Classify.
type ClassifyOptions struct {
	Checks     []string          // other normalisations; nil for the default ones, empty for none
	Covariates map[string]string // nil for z-scores; else W-scores adjusted for them
	PThreshold float64
	Exclude    string
	MaxIter    int
}

// DefaultClassifyOptions returns the defaults of the Python package.
func DefaultClassifyOptions() ClassifyOptions {
	return ClassifyOptions{PThreshold: 0.01, Exclude: "hypo", MaxIter: 1}
}

func clamp01(v float64) float64 { return math.Min(math.Max(v, 0), 1) }

// evidenceLow is the evidence (0 to 1) that a region's uptake is decreased.
func evidenceLow(z *float64) float64 {
	if z == nil {
		return 0
	}
	return clamp01((-*z - evidenceFrom) / evidenceSpan)
}

// evidenceHigh is the evidence (0 to 1) that a region's uptake is increased.
func evidenceHigh(z *float64) float64 {
	if z == nil {
		return 0
	}
	return clamp01((*z - evidenceFrom) / evidenceSpan)
}

func meanOf(values ...float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func regionStatus(z *float64) string {
	switch {
	case z == nil:
		return "none"
	case *z <= -scoreAbnormal:
		return "hypo"
	case *z <= -scoreMild:
		return "mild-hypo"
	case *z >= scoreAbnormal:
		return "hyper"
	case *z >= scoreMild:
		return "mild-hyper"
	}
	return "normal"
}

// summarise gives the mean score of a region, and how much of it is abnormal.
// regionView is the share of each region that lies inside a grid.  On a
// grid that ends inside a region only a part of the region is left: what
// is scored there must be counted against the whole region.
func regionView(dim [3]int, aff Affine) map[regionKey]float64 {
	a := atlasOnce()
	out := map[regionKey]float64{}
	inv, ok := invAffine(aff)
	if !ok {
		return out
	}
	var m [3][4]float64 // from a voxel of the atlas to a voxel of the grid
	for r := 0; r < 3; r++ {
		for c := 0; c < 4; c++ {
			for k := 0; k < 4; k++ {
				m[r][c] += inv[r][k] * a.aff[k][c]
			}
		}
	}
	nx, ny := a.dim[0], a.dim[1]
	for key, mask := range regionMasks() {
		total, inside := 0, 0
		for v, in := range mask {
			if !in {
				continue
			}
			total++
			x, y, z := float64(v%nx), float64(v/nx%ny), float64(v/(nx*ny))
			seen := true
			for r := 0; r < 3; r++ {
				at := m[r][0]*x + m[r][1]*y + m[r][2]*z + m[r][3]
				if !(at > -0.5-gridEdge && at < float64(dim[r])-0.5+gridEdge) {
					seen = false
					break
				}
			}
			if seen {
				inside++
			}
		}
		if total > 0 {
			out[key] = float64(inside) / float64(total)
		}
	}
	return out
}

// summarise gives the mean score of a region and how much of it is
// abnormal.  mask is the region on the grid of the image, inView the share
// of the whole region that this grid holds.
func summarise(z []float32, brain, mask []bool, inView float64) *RegionScore {
	total, n, below, above := 0, 0, 0, 0
	sum := 0.0
	for v, in := range mask {
		if !in {
			continue
		}
		total++
		if brain[v] {
			n++
			value := float64(z[v])
			sum += value
			if value <= -scoreAbnormal {
				below++
			}
			if value >= scoreAbnormal {
				above++
			}
		}
	}
	out := &RegionScore{}
	if total > 0 {
		out.Coverage = float64(n) / float64(total) * inView
	}
	if out.Coverage < minCoverage {
		return out
	}
	mean, lo, hi := sum/float64(n), float64(below)/float64(n), float64(above)/float64(n)
	out.Z, out.Below, out.Above = &mean, &lo, &hi
	return out
}

type regionScores map[string]map[string]*float64

// matchPatterns scores the regional profile against each published
// pattern.  score holds the regional scores under the main normalisation;
// robustHigh the lowest one under any normalisation, which is what an
// increase has to show.
func matchPatterns(score regionScores, robustHigh map[string]*float64) ([]Pattern, string) {
	low := func(role, region, side string) Feature {
		z := score[region][side]
		return Feature{role, region, side, "hypo", z, evidenceLow(z)}
	}
	weigh := func(core, support float64) float64 { return weightCore*core + weightSupport*support }
	var out []Pattern

	// MSA-P: putamen (the posterior part first), then pons and cerebellum.
	f := []Feature{low("core", "putamen", "both"), low("core", "posterior-putamen", "both"),
		low("support", "pons", "both"), low("support", "cerebellum", "both")}
	core, support := math.Max(f[0].Evidence, f[1].Evidence), meanOf(f[2].Evidence, f[3].Evidence)
	out = append(out, Pattern{ID: "msa-p", Score: weigh(core, support), Core: core, Support: support, Features: f})

	// MSA-C: cerebellum, then pons and putamen.
	f = []Feature{low("core", "cerebellum", "both"), low("support", "pons", "both"), low("support", "putamen", "both")}
	core, support = f[0].Evidence, meanOf(f[1].Evidence, f[2].Evidence)
	out = append(out, Pattern{ID: "msa-c", Score: weigh(core, support), Core: core, Support: support, Features: f})

	// PSP: midbrain and medial frontal cortex, then caudate, thalamus and lateral frontal cortex.
	f = []Feature{low("core", "midbrain", "both"), low("core", "medial-frontal", "both"),
		low("support", "caudate", "both"), low("support", "thalamus", "both"), low("support", "lateral-frontal", "both")}
	core, support = meanOf(f[0].Evidence, f[1].Evidence), meanOf(f[2].Evidence, f[3].Evidence, f[4].Evidence)
	out = append(out, Pattern{ID: "psp", Score: weigh(core, support), Core: core, Support: support, Features: f})

	// CBD: the peri-central and parietal cortex of one hemisphere, with the thalamus and striatum of that side.
	cortex := map[string]*float64{}
	for _, side := range []string{"left", "right"} {
		var values []float64
		for _, region := range []string{"sensorimotor", "parietal"} {
			if z := score[region][side]; z != nil {
				values = append(values, *z)
			}
		}
		if len(values) > 0 {
			mean := meanOf(values...)
			cortex[side] = &mean
		}
	}
	worse, asymmetry := "", 0.0
	var difference *float64
	if cortex["left"] != nil && cortex["right"] != nil {
		worse = "right"
		if *cortex["left"] < *cortex["right"] {
			worse = "left"
		}
		d := math.Abs(*cortex["left"] - *cortex["right"])
		difference = &d
		asymmetry = clamp01((d - asymmetryFrom) / asymmetrySpan)
	}
	side := worse
	if side == "" {
		side = "left"
	}
	f = []Feature{{"core", "", side, "asymmetry", difference, asymmetry},
		low("core", "sensorimotor", side), low("core", "parietal", side),
		low("support", "thalamus", side), low("support", "putamen", side), low("support", "lateral-frontal", side)}
	core = asymmetry * meanOf(f[1].Evidence, f[2].Evidence)
	support = asymmetry * meanOf(f[3].Evidence, f[4].Evidence, f[5].Evidence)
	out = append(out, Pattern{ID: "cbd", Score: weigh(core, support), Core: core, Support: support, Features: f})

	// DLB: occipital cortex, then parietal cortex and precuneus with a preserved posterior cingulate.
	occipital := low("core", "occipital", "both")
	var islandZ *float64
	island := 0.0
	cingulate, precuneus := score["posterior-cingulate"]["both"], score["precuneus"]["both"]
	if occipital.Z != nil && cingulate != nil && precuneus != nil {
		d := *cingulate - (*precuneus+*occipital.Z)/2.0
		islandZ = &d
		if occipital.Evidence > 0 {
			island = clamp01((d - islandFrom) / islandSpan)
		}
	}
	f = []Feature{occipital, low("support", "parietal", "both"), low("support", "precuneus", "both"),
		{"support", "posterior-cingulate", "both", "island", islandZ, island}}
	core, support = occipital.Evidence, meanOf(f[1].Evidence, f[2].Evidence, f[3].Evidence)
	out = append(out, Pattern{ID: "dlb", Score: weigh(core, support), Core: core, Support: support, Features: f})

	// PD: no decrease in striatum, brainstem or cerebellum; a relative increase there under every normalisation.
	f = nil
	sum := 0.0
	for _, region := range []string{"putamen", "pallidum", "thalamus", "pons", "cerebellum", "sensorimotor"} {
		z := robustHigh[region]
		f = append(f, Feature{"core", region, "both", "hyper", z, evidenceHigh(z)})
		sum += evidenceHigh(z)
	}
	decrease := 0.0
	for _, region := range []string{"putamen", "caudate", "thalamus", "midbrain", "pons", "cerebellum"} {
		decrease = math.Max(decrease, evidenceLow(score[region]["both"]))
	}
	core, support = sum/float64(len(f)), 1.0-decrease
	f = append(f, Feature{"support", "", "both", "spared", nil, support})
	out = append(out, Pattern{ID: "pd", Score: core * support, Core: core, Support: support, Features: f})
	return out, worse
}

// conclude reads the pattern scores: one pattern, two that cannot be told
// apart, or none.  The confidence in a pattern goes by its robust score:
// low if that alone would not assign the pattern.
func conclude(patterns []Pattern, worse string) Conclusion {
	byID := map[string]Pattern{}
	for _, p := range patterns {
		byID[p.ID] = p
	}
	top, group := Pattern{Score: math.Inf(-1)}, ""
	for _, g := range patternGroup {
		if p := byID[g.Name]; p.Score > top.Score {
			top, group = p, g.Text
		}
	}
	rival := Pattern{Score: math.Inf(-1)}
	for _, g := range patternGroup {
		if p := byID[g.Name]; g.Text != group && p.Score > rival.Score {
			rival = p
		}
	}
	margin := top.Score - rival.Score
	out := Conclusion{Category: "none"}
	switch {
	case top.Score >= scoreMatch:
		out.Pattern = top.ID
		if margin >= scoreMargin {
			out.Category, out.Confidence = "pattern", "moderate"
			if top.Robust < scoreMatch {
				out.Confidence = "low"
			} else if top.Robust >= scoreHigh && margin >= scoreHighMargin {
				out.Confidence = "high"
			}
		} else {
			out.Category, out.Confidence = "indeterminate", "low"
		}
		if rival.Score >= scoreWeak {
			out.Alternative = rival.ID
		}
	case top.Score >= scoreWeak:
		out.Category, out.Pattern, out.Confidence = "possible", top.ID, "low"
		if rival.Score >= scoreWeak {
			out.Alternative = rival.ID
		}
	case byID["pd"].Score >= scoreWeak:
		out.Category, out.Pattern, out.Confidence = "pd", "pd", "low"
		if byID["pd"].Score >= scoreMatch {
			out.Confidence = "moderate"
		}
	}
	if out.Pattern == "cbd" || out.Alternative == "cbd" {
		out.Side = worse
	}
	return out
}

func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

func round4p(v *float64) {
	if v != nil {
		*v = round4(*v)
	}
}

// Classify matches one patient's FDG-PET image (in MNI space) against the
// patterns of the parkinsonian syndromes.
//
// The image is turned into a z-score map against the controls of ref, with
// the normalisation method; with covariates into a W-score map adjusted for
// them.  The map is averaged over the striatum, thalamus, brainstem,
// cerebellum and the cortical regions that separate the syndromes, each
// side on its own, and the regional profile is scored against the patterns
// of MSA-P, MSA-C, PSP, CBD, DLB and PD.
//
// The checks are further normalisations the map is made with.  A finding
// that only one normalisation shows is reported as such, and an increase
// counts for PD only if all of them show it.
func Classify(img *Volume, name, method string, ref *Reference, opt ClassifyOptions) (*Classification, error) {
	checks := []string{}
	if opt.Checks == nil {
		checks = DefaultChecks(ref, method)
	} else {
		for _, m := range opt.Checks {
			if m != method && !contains(checks, m) {
				checks = append(checks, m)
			}
		}
	}
	methods := append([]string{method}, checks...)
	for _, m := range methods { // fail before doing any work
		if _, err := ref.ControlStats(m); err != nil {
			return nil, err
		}
		if opt.Covariates != nil {
			if _, err := ref.ControlModel(m); err != nil {
				return nil, err
			}
		}
	}
	zopt := ZMapOptions{Covariates: opt.Covariates, PThreshold: opt.PThreshold, Exclude: opt.Exclude, MaxIter: opt.MaxIter}
	maps := map[string]*zmapParts{}
	warnings := []string{}
	for _, m := range methods {
		parts, err := zmapScores(img, m, ref, zopt)
		if err != nil {
			return nil, err
		}
		maps[m] = parts
		for _, w := range parts.warnings {
			if !contains(warnings, w) {
				warnings = append(warnings, w)
			}
		}
	}
	main := maps[method]

	a := atlasOnce()
	onGrid := map[regionKey][]bool{}
	for key, mask := range regionMasks() {
		resampled, err := GetMask(MaskSpec{Vol: boolVolume(mask, a.dim, a.aff)}, img.Dim, img.Aff)
		if err != nil {
			return nil, err
		}
		onGrid[key] = resampled
	}
	view := regionView(img.Dim, img.Aff)
	var regions []RegionEntry
	score, steady := regionScores{}, regionScores{}
	robustHigh := map[string]*float64{}
	for _, r := range ClassRegions {
		entry := RegionEntry{ID: r.Name, Paired: r.Paired}
		score[r.Name], steady[r.Name] = map[string]*float64{}, map[string]*float64{}
		sides := []string{"both"}
		if r.Paired {
			sides = []string{"left", "right", "both"}
		}
		for _, side := range sides {
			mask := onGrid[regionKey{r.Name, side}]
			inView := view[regionKey{r.Name, side}]
			found := summarise(main.scores, main.brain, mask, inView)
			found.Checks = map[string]*float64{}
			found.Status = regionStatus(found.Z)
			found.Robust = true
			for _, m := range checks {
				other := summarise(maps[m].scores, maps[m].brain, mask, inView).Z
				found.Checks[m] = other
				if found.Status != "none" && found.Status != "normal" &&
					!(other != nil && *other**found.Z > 0 && math.Abs(*other) >= scoreRobust) {
					found.Robust = false
				}
			}
			switch side {
			case "left":
				entry.Left = found
			case "right":
				entry.Right = found
			default:
				entry.Both = found
			}
			score[r.Name][side], steady[r.Name][side] = found.Z, found.Z
			if !found.Robust {
				steady[r.Name][side] = new(float64)
			}
		}
		// What an increase must show: the lowest score under any normalisation.
		if entry.Both.Z != nil {
			lowest := *entry.Both.Z
			for _, m := range checks {
				if v := entry.Both.Checks[m]; v != nil {
					lowest = math.Min(lowest, *v)
				}
			}
			robustHigh[r.Name] = &lowest
		}
		if r.Paired && entry.Left.Z != nil && entry.Right.Z != nil {
			d := *entry.Left.Z - *entry.Right.Z
			entry.Difference = &d
		}
		regions = append(regions, entry)
	}

	patterns, worse := matchPatterns(score, robustHigh)
	// The match that is left without the findings that change with the normalisation.
	kept, _ := matchPatterns(steady, robustHigh)
	unchecked := make([]Pattern, len(patterns))
	for i := range patterns {
		patterns[i].Robust = math.Min(patterns[i].Score, kept[i].Score)
		unchecked[i] = patterns[i]
		unchecked[i].Robust = patterns[i].Score
	}
	conclusion := conclude(patterns, worse)
	inside, below, above := 0, 0, 0
	for v, in := range main.brain {
		if in {
			inside++
			if value := float64(main.scores[v]); value <= -scoreAbnormal {
				below++
			} else if value >= scoreAbnormal {
				above++
			}
		}
	}
	var share BrainShare
	if inside > 0 {
		share = BrainShare{float64(below) / float64(inside), float64(above) / float64(inside)}
	}

	notes := []Note{}
	if conclusion.Confidence != conclude(unchecked, worse).Confidence {
		for _, p := range patterns {
			if p.ID == conclusion.Pattern {
				notes = append(notes, Note{Key: "lowered", Values: []int{int(math.Floor(100*p.Robust + 0.5))}})
			}
		}
	}
	if ref.N < 30 {
		notes = append(notes, Note{Key: "few-controls", Values: []int{ref.N}})
	}
	if opt.Covariates == nil {
		notes = append(notes, Note{Key: "unadjusted"})
	}
	if share.Below > shareWidespread {
		notes = append(notes, Note{Key: "widespread", Values: []int{int(math.Floor(100*share.Below + 0.5))}})
	}
	var notCovered, dependent, globalOnly [][2]string
	for i := range regions {
		entry := &regions[i]
		sides := []string{"both"}
		if entry.Paired {
			sides = []string{"left", "right"}
		}
		for _, side := range sides {
			found := entry.Side(side)
			if found.Z == nil {
				notCovered = append(notCovered, [2]string{entry.ID, side})
			} else if !found.Robust {
				dependent = append(dependent, [2]string{entry.ID, side})
			}
			if scaled := found.Checks["ps"]; found.Z != nil && scaled != nil && *scaled >= scoreAbnormal && *found.Z < scoreMild {
				globalOnly = append(globalOnly, [2]string{entry.ID, side})
			}
		}
	}
	for _, listed := range []Note{{Key: "not-covered", Regions: notCovered}, {Key: "dependent", Regions: dependent},
		{Key: "global-only", Regions: globalOnly}} {
		if len(listed.Regions) > 0 {
			notes = append(notes, listed)
		}
	}

	// Round the numbers, so that the file reads well.
	share.Below, share.Above = round4(share.Below), round4(share.Above)
	for i := range regions {
		entry := &regions[i]
		for _, found := range []*RegionScore{entry.Left, entry.Right, entry.Both} {
			if found == nil {
				continue
			}
			found.Coverage = round4(found.Coverage)
			round4p(found.Z)
			round4p(found.Below)
			round4p(found.Above)
			for _, v := range found.Checks {
				round4p(v)
			}
		}
		round4p(entry.Difference)
	}
	for i := range patterns {
		p := &patterns[i]
		p.Score, p.Core, p.Support, p.Robust = round4(p.Score), round4(p.Core), round4(p.Support), round4(p.Robust)
		for j := range p.Features {
			// A feature may share its score with the table of regions, which is rounded already.
			if z := p.Features[j].Z; z != nil {
				rounded := round4(*z)
				p.Features[j].Z = &rounded
			}
			p.Features[j].Evidence = round4(p.Features[j].Evidence)
		}
	}

	out := &Classification{
		Version:    Version,
		Image:      name,
		Score:      "z",
		Reference:  ClassReference{ref.N, ref.FWHM, ref.Covariates()},
		Method:     method,
		Checks:     checks,
		Factors:    map[string]float64{},
		Brain:      share,
		Regions:    regions,
		Patterns:   patterns,
		Conclusion: conclusion,
		Notes:      notes,
		Warnings:   warnings,
		ZMap:       main.volume(img, method, ref),
		Normalised: main.res.Image,
	}
	if main.adjusted {
		out.Score = "w"
		out.Covariates = map[string]string{}
		for _, covariate := range ref.Covariates() {
			out.Covariates[covariate] = strings.TrimSpace(opt.Covariates[covariate])
		}
	}
	for _, m := range methods {
		out.Factors[m], _ = strconv.ParseFloat(fmtG(maps[m].res.Factor), 64)
	}
	return out, nil
}

// JSON returns everything that was measured and concluded, as a file.
func (c *Classification) JSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReportStem is what the files about one image are named after:
// sub-01_pet.nii.gz -> sub-01.
func ReportStem(path string) string {
	stem := StripExt(baseName(path))
	if strings.HasSuffix(stem, "_pet") && len(stem) > 4 {
		return stem[:len(stem)-4]
	}
	return stem
}

// SharedReportStem finds two images whose reports would get the same name,
// so that the second would replace the first: a/pet.nii.gz and
// b/pet.nii.gz, or sub-01.nii.gz and sub-01_pet.nii.gz.  Names that differ
// only in case count as one, as they are one file on most Macs and on
// Windows.
func SharedReportStem(paths []string) (first, second, stem string, found bool) {
	seen := map[string]string{}
	for _, path := range paths {
		stem := ReportStem(path)
		if other, ok := seen[strings.ToLower(stem)]; ok {
			return other, path, stem, true
		}
		seen[strings.ToLower(stem)] = path
	}
	return "", "", "", false
}

// ClassFiles are the files Save writes.
type ClassFiles struct{ Map, JSON, Report string }

// Save writes the map, the report (in English, "en", or Chinese, "zh") and
// the numbers into a folder.
func (c *Classification) Save(folder, stem, lang string) (ClassFiles, error) {
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return ClassFiles{}, err
	}
	suffix := "zmap"
	if c.Score == "w" {
		suffix = "wmap"
	}
	files := ClassFiles{
		Map:    filepath.Join(folder, stem+"_desc-"+strings.ReplaceAll(c.Method, "-", "")+"_"+suffix+".nii.gz"),
		JSON:   filepath.Join(folder, stem+"_classification.json"),
		Report: filepath.Join(folder, stem+"_report.html"),
	}
	page, err := c.Report(lang)
	if err != nil {
		return files, err
	}
	data, err := c.JSON()
	if err != nil {
		return files, err
	}
	if err := SaveNifti(files.Map, c.ZMap, false); err != nil {
		return files, err
	}
	if err := os.WriteFile(files.JSON, data, 0o644); err != nil {
		return files, err
	}
	return files, os.WriteFile(files.Report, []byte(page), 0o644)
}
