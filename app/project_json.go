package main

import (
	"encoding/json"
	"path/filepath"
)

// How what the pages hold of their runs is written into the file of a
// project, and read back.  Every field of a type is named here once for
// writing and once for reading: TestProjectRecords fails for one left out.

type statusJSON struct {
	OK    string `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s status) MarshalJSON() ([]byte, error) { return json.Marshal(statusJSON{s.ok, s.err}) }

func (s *status) UnmarshalJSON(raw []byte) error {
	var j statusJSON
	err := json.Unmarshal(raw, &j)
	s.ok, s.err = j.OK, j.Error
	return err
}

type stampJSON struct {
	Size     int64 `json:"size"`
	Modified int64 `json:"modified_ns"`
}

func (s fileStamp) MarshalJSON() ([]byte, error) {
	if !s.found {
		return []byte("null"), nil
	}
	return json.Marshal(stampJSON{s.size, s.modified})
}

func (s *fileStamp) UnmarshalJSON(raw []byte) error {
	*s = fileStamp{}
	if string(raw) == "null" {
		return nil
	}
	var j stampJSON
	err := json.Unmarshal(raw, &j)
	s.found, s.size, s.modified = err == nil, j.Size, j.Modified
	return err
}

type normRowJSON struct {
	Image    string  `json:"image"`
	Group    string  `json:"group,omitempty"`
	Method   string  `json:"method"`
	Factor   string  `json:"factor"`
	Voxels   string  `json:"voxels,omitempty"`
	Excluded string  `json:"excluded,omitempty"`
	Mask     string  `json:"mask,omitempty"`
	Value    float64 `json:"value"`
}

func (r normRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(normRowJSON{r.image, r.group, r.method, r.factor, r.voxels, r.excluded, r.mask, r.value})
}

func (r *normRow) UnmarshalJSON(raw []byte) error {
	var j normRowJSON
	err := json.Unmarshal(raw, &j)
	*r = normRow{image: j.Image, group: j.Group, method: j.Method, factor: j.Factor, voxels: j.Voxels, excluded: j.Excluded, mask: j.Mask, value: j.Value}
	return err
}

type zmapRowJSON struct {
	Image  string `json:"image"`
	Method string `json:"method"`
	File   string `json:"file"`
	Lowest string `json:"lowest"`
	Share  string `json:"share"`
	Source string `json:"source"`
	Path   string `json:"path"`
	WScore bool   `json:"w_score,omitempty"`
}

func (r zmapRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(zmapRowJSON{r.image, r.method, r.file, r.lowest, r.share, r.source, r.path, r.wscore})
}

func (r *zmapRow) UnmarshalJSON(raw []byte) error {
	var j zmapRowJSON
	err := json.Unmarshal(raw, &j)
	*r = zmapRow{image: j.Image, method: j.Method, file: j.File, lowest: j.Lowest, share: j.Share, source: j.Source, path: j.Path, wscore: j.WScore}
	return err
}

type scoreJSON struct {
	ID      string `json:"pattern"`
	Percent int    `json:"percent"`
}

type classifyRowJSON struct {
	Image      string      `json:"image"`
	Reading    string      `json:"reading"`
	Detail     string      `json:"detail"`
	Report     string      `json:"report"`
	Source     string      `json:"source"`
	ZMap       string      `json:"map,omitempty"`
	WScore     bool        `json:"w_score,omitempty"`
	Scores     []scoreJSON `json:"scores,omitempty"`
	Leading    string      `json:"leading,omitempty"`
	Category   string      `json:"category"`
	Confidence string      `json:"confidence,omitempty"`
	Badge      string      `json:"badge,omitempty"`
}

func (r classifyRow) MarshalJSON() ([]byte, error) {
	j := classifyRowJSON{r.image, r.reading, r.detail, r.report, r.source, r.zmap, r.wscore, nil, r.leading, r.category, r.confidence, r.badge}
	for _, s := range r.scores {
		j.Scores = append(j.Scores, scoreJSON{s.id, s.percent})
	}
	return json.Marshal(j)
}

func (r *classifyRow) UnmarshalJSON(raw []byte) error {
	var j classifyRowJSON
	err := json.Unmarshal(raw, &j)
	*r = classifyRow{image: j.Image, reading: j.Reading, detail: j.Detail, report: j.Report, source: j.Source, zmap: j.ZMap, wscore: j.WScore,
		leading: j.Leading, category: j.Category, confidence: j.Confidence, badge: j.Badge}
	for _, s := range j.Scores {
		r.scores = append(r.scores, patternScore{s.ID, s.Percent})
	}
	return err
}

type spatialRowJSON struct {
	Image  string `json:"image"`
	Output string `json:"output"`
	Path   string `json:"path"`
	Failed bool   `json:"failed,omitempty"`
}

func (r spatialRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(spatialRowJSON{r.image, r.output, r.path, r.failed})
}

func (r *spatialRow) UnmarshalJSON(raw []byte) error {
	var j spatialRowJSON
	err := json.Unmarshal(raw, &j)
	*r = spatialRow{image: j.Image, output: j.Output, path: j.Path, failed: j.Failed}
	return err
}

type normDoneJSON struct {
	Asked     []string                      `json:"asked"`
	Files     []string                      `json:"files"`
	Controls  []string                      `json:"controls,omitempty"`
	Methods   []string                      `json:"methods"`
	Reference string                        `json:"reference,omitempty"`
	RefSig    string                        `json:"reference_built_with,omitempty"`
	Settings  string                        `json:"settings"`
	Images    map[string]map[string]string  `json:"images"`
	Factors   map[string]map[string]float64 `json:"factors"`
	Target    float64                       `json:"target"`
	Median    bool                          `json:"median,omitempty"`
	FWHM      *float64                      `json:"smoothing_mm,omitempty"`
	Rebuilt   bool                          `json:"reference_rebuilt,omitempty"`
	Stamps    map[string]fileStamp          `json:"image_stamps,omitempty"`
	RefStamp  fileStamp                     `json:"reference_stamp"`
}

func (d normDone) MarshalJSON() ([]byte, error) {
	return json.Marshal(normDoneJSON{d.asked, d.files, d.controls, d.methods, d.reference, d.refSig, d.settings, d.images, d.factors,
		d.target, d.median, d.fwhm, d.rebuilt, d.stamps, d.refStamp})
}

func (d *normDone) UnmarshalJSON(raw []byte) error {
	var j normDoneJSON
	err := json.Unmarshal(raw, &j)
	*d = normDone{asked: j.Asked, files: j.Files, controls: j.Controls, methods: j.Methods, reference: j.Reference, refSig: j.RefSig,
		settings: j.Settings, images: j.Images,
		factors: j.Factors, target: j.Target, median: j.Median, fwhm: j.FWHM, rebuilt: j.Rebuilt, stamps: j.Stamps, refStamp: j.RefStamp}
	return err
}

type zmapDoneJSON struct {
	Files   []string                     `json:"files"`
	Methods []string                     `json:"methods"`
	Table   string                       `json:"covariate_table,omitempty"`
	RefSig  string                       `json:"reference_built_with,omitempty"`
	Maps    map[string]map[string]string `json:"maps"`
}

// The run of step 1 a run of step 2 took (from) is not written with it: the
// file says whether it is the one step 1 holds (zmapRun.Current).
func (d zmapDone) MarshalJSON() ([]byte, error) {
	return json.Marshal(zmapDoneJSON{d.files, d.methods, d.table, d.refSig, d.maps})
}

func (d *zmapDone) UnmarshalJSON(raw []byte) error {
	var j zmapDoneJSON
	err := json.Unmarshal(raw, &j)
	*d = zmapDone{files: j.Files, methods: j.Methods, table: j.Table, refSig: j.RefSig, maps: j.Maps}
	return err
}

// stamp notes the images a run of step 1 reads as they are on disk now, and
// stampReference the reference it uses, once that is built: a project opened
// later tells by them whether they are still what was normalised.
func (d *normDone) stamp() {
	d.stamps = map[string]fileStamp{}
	for _, path := range d.asked {
		d.stamps[path] = stampOf(path)
	}
}

func (d *normDone) stampReference() {
	if d.reference != "" {
		d.refStamp = stampOf(filepath.Join(d.reference, "reference.json"))
	}
}

type refBuiltJSON struct {
	Signature string               `json:"built_with"`
	StatsFor  []string             `json:"stats_for,omitempty"`
	Controls  map[string]fileStamp `json:"controls"`
	Stamp     fileStamp            `json:"stamp"`
}

func (b refBuilt) MarshalJSON() ([]byte, error) {
	return json.Marshal(refBuiltJSON{b.sig, b.statsFor, b.controls, b.stamp})
}

func (b *refBuilt) UnmarshalJSON(raw []byte) error {
	var j refBuiltJSON
	err := json.Unmarshal(raw, &j)
	*b = refBuilt{sig: j.Signature, statsFor: j.StatsFor, controls: j.Controls, stamp: j.Stamp}
	return err
}

// look compares the images and the reference with what stamp noted: an
// image changed since is not the one that was normalised, and a reference
// built anew is not the one that was used.
func (d *normDone) look() {
	for path, was := range d.stamps {
		if stampOf(path) != was {
			d.changed = true
		}
	}
	if d.reference != "" && d.refStamp.found && stampOf(filepath.Join(d.reference, "reference.json")) != d.refStamp {
		d.rebuilt = true
	}
}

// written reports whether every image the run wrote is there.
func (d *normDone) written(there func(string) bool) bool {
	for _, images := range d.images {
		for _, path := range images {
			if !there(path) {
				return false
			}
		}
	}
	return true
}
