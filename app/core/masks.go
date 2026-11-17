package core

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed data
var dataFS embed.FS

const dataPrefix = "tpl-MNI152NLin2009cSym_res-2_"

func errorf(format string, args ...any) error { return fmt.Errorf(format, args...) }

// CerebrA label numbers (right hemisphere, left hemisphere).
var (
	lblBrainstem    = []int{11, 62}
	lblCerebellumGM = []int{46, 97}
	lblCerebellumWM = []int{39, 90}
	lblVermis       = []int{50, 101, 2, 53, 20, 71}
	lblTemporalR    = []int{3, 28, 45, 14, 24, 18, 36}
	lblSensorimotor = []int{35, 86, 13, 64, 16, 67}
	lblOccipital    = []int{34, 85, 43, 94, 6, 57, 12, 63}
)

// Pons = the part of the brainstem between these MNI z coordinates (mm).
var ponsZ = [2]float64{-44, -26}

// Named describes a method or a mask.
type Named struct{ Name, Text string }

// Masks lists the built-in masks.
var Masks = []Named{
	{"gray-matter", "Gray matter (probability > 0.1); the paper's mask for PS and HN"},
	{"parenchyma", "Whole brain parenchyma (gray + white matter probability > 0.5)"},
	{"cerebellum", "Whole cerebellum, gray and white matter (RRC in the paper)"},
	{"cerebellar-gm", "Cerebellar gray matter, hemispheres and vermis"},
	{"pons", "Central pons (brainstem between z = -44 and -26 mm, eroded)"},
	{"white-matter", "Deep cerebral white matter (eroded, centrum semiovale)"},
	{"occipital", "Occipital cortex (lateral occipital, cuneus, pericalcarine, lingual)"},
	{"sensorimotor", "Primary sensorimotor cortex (pre-, post- and paracentral)"},
	{"brainstem", "Whole brainstem (RRBS in the paper)"},
	{"temporal-left", "Left temporal cortex (RRTL for a right-sided lesion)"},
	{"temporal-right", "Right temporal cortex (RRTL for a left-sided lesion)"},
}

// ReferenceRegions maps the reference-region methods to their built-in mask, in order.
var ReferenceRegions = []Named{
	{"whole-brain", "parenchyma"},
	{"cerebellum", "cerebellum"},
	{"cerebellar-gm", "cerebellar-gm"},
	{"pons", "pons"},
	{"white-matter", "white-matter"},
	{"occipital", "occipital"},
	{"sensorimotor", "sensorimotor"},
}

// IsMask reports whether name is a built-in mask.
func IsMask(name string) bool { return maskText(name) != "" }

func maskText(name string) string {
	for _, m := range Masks {
		if m.Name == name {
			return m.Text
		}
	}
	return ""
}

func maskNames() string {
	names := make([]string, len(Masks))
	for i, m := range Masks {
		names[i] = m.Name
	}
	return strings.Join(names, ", ")
}

func regionMask(method string) string {
	for _, r := range ReferenceRegions {
		if r.Name == method {
			return r.Text
		}
	}
	return ""
}

func loadEmbedded(file string) *Volume {
	raw, err := dataFS.ReadFile("data/" + file)
	if err != nil {
		panic(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		panic(err)
	}
	vol, err := decodeNifti(plain, file, "")
	if err != nil {
		panic(err)
	}
	return vol
}

type atlasData struct {
	labels []uint8
	dim    [3]int
	aff    Affine
	gm, wm []float32
}

var atlasOnce = sync.OnceValue(func() *atlasData {
	lab := loadEmbedded(dataPrefix + "atlas-CerebrA_dseg.nii.gz")
	a := &atlasData{dim: lab.Dim, aff: lab.Aff, labels: make([]uint8, len(lab.Data))}
	for i, v := range lab.Data {
		a.labels[i] = uint8(math.RoundToEven(float64(v)))
	}
	a.gm = loadEmbedded(dataPrefix + "label-GM_probseg.nii.gz").Data
	a.wm = loadEmbedded(dataPrefix + "label-WM_probseg.nii.gz").Data
	return a
})

// AtlasGrid returns the grid of the built-in masks (2 mm MNI space).
func AtlasGrid() ([3]int, Affine) {
	a := atlasOnce()
	return a.dim, a.aff
}

// AtlasLabel is one region of the bundled CerebrA atlas.
type AtlasLabel struct {
	Index      int
	Name, Hemi string
}

// AtlasLabels lists the regions of the bundled CerebrA atlas by number.
var AtlasLabels = sync.OnceValue(func() []AtlasLabel {
	raw, err := dataFS.ReadFile("data/atlas-CerebrA_dseg.tsv")
	if err != nil {
		panic(err)
	}
	var out []AtlasLabel
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, line := range lines[1:] {
		parts := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(parts) < 3 {
			continue
		}
		index, _ := strconv.Atoi(parts[0])
		out = append(out, AtlasLabel{index, parts[1], parts[2]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
})

func isIn(labels []uint8, sets ...[]int) []bool {
	var want [256]bool
	for _, set := range sets {
		for _, v := range set {
			if v >= 0 && v < 256 {
				want[v] = true
			}
		}
	}
	out := make([]bool, len(labels))
	for i, v := range labels {
		out[i] = want[v]
	}
	return out
}

var (
	builtinCache = map[string][]bool{}
	builtinMu    sync.Mutex
)

// builtin returns a built-in mask on the atlas grid (shared: do not modify).
func builtin(name string) []bool {
	builtinMu.Lock()
	defer builtinMu.Unlock()
	if m, ok := builtinCache[name]; ok {
		return m
	}
	a := atlasOnce()
	var m []bool
	switch name {
	case "gray-matter":
		m = make([]bool, len(a.gm))
		for i, v := range a.gm {
			m[i] = float64(v) > 0.1
		}
	case "parenchyma":
		m = make([]bool, len(a.gm))
		for i := range a.gm {
			m[i] = float64(a.gm[i])+float64(a.wm[i]) > 0.5
		}
	case "cerebellum":
		m = isIn(a.labels, lblCerebellumGM, lblCerebellumWM, lblVermis)
	case "cerebellar-gm":
		m = isIn(a.labels, lblCerebellumGM, lblVermis)
	case "brainstem":
		m = isIn(a.labels, lblBrainstem)
	case "pons":
		m = isIn(a.labels, lblBrainstem)
		nx, ny := a.dim[0], a.dim[1]
		for i := range m {
			z := a.aff[2][2]*float64(i/(nx*ny)) + a.aff[2][3]
			if z < ponsZ[0] || z > ponsZ[1] {
				m[i] = false
			}
		}
		// Erode in-plane only, so the slab keeps its full height.
		m = binaryMorph(m, a.dim, inPlane33, 1, true)
	case "white-matter":
		infra := isIn(a.labels, lblBrainstem, lblCerebellumGM, lblCerebellumWM, lblVermis)
		infra = binaryMorph(infra, a.dim, cross6, 2, false)
		m = make([]bool, len(a.wm))
		for i, v := range a.wm {
			m[i] = float64(v) > 0.9 && !infra[i]
		}
		m = binaryMorph(m, a.dim, cross6, 2, true)
	case "sensorimotor":
		m = isIn(a.labels, lblSensorimotor)
	case "occipital":
		m = isIn(a.labels, lblOccipital)
	case "temporal-left":
		left := make([]int, len(lblTemporalR))
		for i, v := range lblTemporalR {
			left[i] = v + 51
		}
		m = isIn(a.labels, left)
	case "temporal-right":
		m = isIn(a.labels, lblTemporalR)
	default:
		panic("unknown mask " + name)
	}
	builtinCache[name] = m
	return m
}

// MaskSpec names a mask: a built-in one, a NIfTI file, or one held in memory
// (any grid, as long as it is in the same space as the image it is used on).
type MaskSpec struct {
	Name string // built-in mask name or file path
	Vol  *Volume
	Text string // how to describe an in-memory mask
}

// IsZero reports whether no mask was given.
func (m MaskSpec) IsZero() bool { return m.Name == "" && m.Vol == nil }

func boolVolume(mask []bool, dim [3]int, aff Affine) *Volume {
	vol := NewVolume(dim, aff)
	for i, m := range mask {
		if m {
			vol.Data[i] = 1
		}
	}
	return vol
}

// BuiltinMask returns a built-in mask as an image on the 2 mm MNI atlas grid.
func BuiltinMask(name string) (*Volume, error) {
	if !IsMask(name) {
		return nil, errorf("unknown mask '%s'; choose from: %s", name, maskNames())
	}
	a := atlasOnce()
	return boolVolume(builtin(name), a.dim, a.aff), nil
}

// LabelsMask returns a mask made of the given CerebrA label numbers.
func LabelsMask(labels []int) (*Volume, error) {
	known := map[int]bool{}
	for _, l := range AtlasLabels() {
		known[l.Index] = true
	}
	var unknown []int
	seen := map[int]bool{}
	for _, l := range labels {
		if !known[l] && !seen[l] {
			unknown = append(unknown, l)
			seen[l] = true
		}
	}
	if len(unknown) > 0 {
		sort.Ints(unknown)
		return nil, errorf("not CerebrA label numbers: %v; see the list of atlas labels", unknown)
	}
	if len(labels) == 0 {
		return nil, errorf("no labels given")
	}
	a := atlasOnce()
	return boolVolume(isIn(a.labels, labels), a.dim, a.aff), nil
}

// GetMask returns a boolean mask on the given grid.
func GetMask(spec MaskSpec, dim [3]int, aff Affine) ([]bool, error) {
	var vol *Volume
	switch {
	case spec.Vol != nil:
		vol = NewVolume(spec.Vol.Dim, spec.Vol.Aff)
		for i, v := range spec.Vol.Data[:vol.NVox()] {
			if f := float64(v); f > 0 && !math.IsNaN(f) {
				vol.Data[i] = 1
			}
		}
	case IsMask(spec.Name):
		a := atlasOnce()
		vol = boolVolume(builtin(spec.Name), a.dim, a.aff)
	default:
		if _, err := os.Stat(spec.Name); err != nil {
			return nil, errorf("'%s' is neither a built-in mask (%s) nor an existing file", spec.Name, maskNames())
		}
		loaded, err := LoadNifti(spec.Name)
		if err != nil {
			return nil, err
		}
		if loaded.NT != 1 {
			return nil, errorf("%s: a mask must be a 3D image", filepath.Base(spec.Name))
		}
		vol = NewVolume(loaded.Dim, loaded.Aff)
		for i, v := range loaded.Data {
			if f := float64(v); f > 0 && !math.IsNaN(f) {
				vol.Data[i] = 1
			}
		}
	}
	out := make([]bool, dim[0]*dim[1]*dim[2])
	if SameGrid(vol.Dim, vol.Aff, dim, aff) {
		for i, v := range vol.Data {
			out[i] = v > 0.5
		}
		return out, nil
	}
	res := resampleLinear(vol.Data, vol.Dim, vol.Aff, dim, aff, true)
	for i, v := range res {
		out[i] = v > 0.5
	}
	return out, nil
}

// resampleTo resamples a float volume onto another grid (trilinear).
func resampleTo(data []float32, sDim [3]int, sAff Affine, tDim [3]int, tAff Affine) []float32 {
	if SameGrid(sDim, sAff, tDim, tAff) {
		return data
	}
	return resampleLinear(data, sDim, sAff, tDim, tAff, false)
}

// readLabelsJSON reads an atlas label table ({"1": "Name", ...}).
func readLabelsJSON(file string) map[int]string {
	raw, err := dataFS.ReadFile("data/" + file)
	if err != nil {
		panic(err)
	}
	var table map[string]string
	if err := json.Unmarshal(raw, &table); err != nil {
		panic(err)
	}
	out := make(map[int]string, len(table))
	for k, v := range table {
		if id, err := strconv.Atoi(k); err == nil {
			out[id] = v
		}
	}
	return out
}
