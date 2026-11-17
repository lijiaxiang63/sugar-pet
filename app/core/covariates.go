package core

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Covariate holds one value per image, as text (for example age or sex).
type Covariate struct {
	Name   string
	Values []string
}

// Term describes how a covariate was coded, so that a new subject can be
// coded the same way.  A reference stores its terms.
type Term struct {
	Name   string
	Type   string // "numeric" or "categorical"
	Center float64
	Min    float64
	Max    float64
	Levels []string
}

// MarshalJSON writes a term the way the Python package does.
func (t Term) MarshalJSON() ([]byte, error) {
	if t.Type == "numeric" {
		return json.Marshal(struct {
			Name   string  `json:"name"`
			Type   string  `json:"type"`
			Center float64 `json:"center"`
			Min    float64 `json:"min"`
			Max    float64 `json:"max"`
		}{t.Name, t.Type, t.Center, t.Min, t.Max})
	}
	return json.Marshal(struct {
		Name   string   `json:"name"`
		Type   string   `json:"type"`
		Levels []string `json:"levels"`
	}{t.Name, t.Type, t.Levels})
}

// UnmarshalJSON reads a term.
func (t *Term) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name   string   `json:"name"`
		Type   string   `json:"type"`
		Center float64  `json:"center"`
		Min    float64  `json:"min"`
		Max    float64  `json:"max"`
		Levels []string `json:"levels"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*t = Term{raw.Name, raw.Type, raw.Center, raw.Min, raw.Max, raw.Levels}
	return nil
}

var missingValues = map[string]bool{"": true, "n/a": true, "na": true, "nan": true, "none": true, "null": true}

func parseNumber(text string) (float64, bool) {
	v, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func pyList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "'" + s + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// BuildDesign turns the covariates of a group into a regression design
// matrix (rows x terms, first column all ones).
//
// A covariate whose values are all numbers is used as it is, centred on the
// group mean.  Any other covariate must have exactly two different values
// (e.g. "F" and "M") and becomes a 0/1 column.
func BuildDesign(covariates []Covariate) ([][]float64, []Term, error) {
	if len(covariates) == 0 {
		return nil, nil, errorf("no covariates given")
	}
	n := len(covariates[0].Values)
	columns := [][]float64{make([]float64, n)}
	for i := range columns[0] {
		columns[0][i] = 1
	}
	var terms []Term
	for _, cov := range covariates {
		texts := make([]string, len(cov.Values))
		for i, v := range cov.Values {
			texts[i] = strings.TrimSpace(v)
		}
		if len(texts) != n {
			return nil, nil, errorf("covariate '%s' has %d values, expected one per control (%d)", cov.Name, len(texts), n)
		}
		for _, t := range texts {
			if missingValues[strings.ToLower(t)] {
				return nil, nil, errorf("covariate '%s' is missing for some controls", cov.Name)
			}
		}
		numbers := make([]float64, n)
		numeric := true
		for i, t := range texts {
			v, ok := parseNumber(t)
			if !ok {
				numeric = false
				break
			}
			numbers[i] = v
		}
		col := make([]float64, n)
		if numeric {
			lo, hi, sum := math.Inf(1), math.Inf(-1), 0.0
			for _, v := range numbers {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
				sum += v
			}
			if hi-lo == 0 {
				return nil, nil, errorf("covariate '%s' is the same for all controls, so it cannot be adjusted for", cov.Name)
			}
			centre := sum / float64(n)
			for i, v := range numbers {
				col[i] = v - centre
			}
			terms = append(terms, Term{Name: cov.Name, Type: "numeric", Center: centre, Min: lo, Max: hi})
		} else {
			set := map[string]bool{}
			for _, t := range texts {
				set[t] = true
			}
			levels := make([]string, 0, len(set))
			for t := range set {
				levels = append(levels, t)
			}
			sort.Strings(levels)
			if len(levels) != 2 {
				return nil, nil, errorf("covariate '%s' must be numeric or have exactly two values, found %s",
					cov.Name, pyList(levels[:min(6, len(levels))]))
			}
			for i, t := range texts {
				if t == levels[1] {
					col[i] = 1
				}
			}
			terms = append(terms, Term{Name: cov.Name, Type: "categorical", Levels: levels})
		}
		columns = append(columns, col)
	}
	design := make([][]float64, n)
	for i := range design {
		design[i] = make([]float64, len(columns))
		for j := range columns {
			design[i][j] = columns[j][i]
		}
	}
	if n < len(columns)+3 {
		return nil, nil, errorf("%d controls are too few to adjust for %d covariate(s)", n, len(terms))
	}
	if !fullRank(design) {
		return nil, nil, errorf("the covariates are redundant (one can be predicted exactly from the others)")
	}
	return design, terms, nil
}

// DesignRow codes one subject like the controls were.  It also returns
// warnings, if any.
func DesignRow(terms []Term, values map[string]string) ([]float64, []string, error) {
	row := []float64{1}
	var notes []string
	for _, term := range terms {
		text, ok := values[term.Name]
		text = strings.TrimSpace(text)
		if !ok || missingValues[strings.ToLower(text)] {
			return nil, nil, errorf("covariate '%s' is needed but missing", term.Name)
		}
		if term.Type == "numeric" {
			value, ok := parseNumber(text)
			if !ok {
				return nil, nil, errorf("covariate '%s' must be a number, got '%s'", term.Name, text)
			}
			if value < term.Min || value > term.Max {
				notes = append(notes, term.Name+" = "+fmtG(value)+" is outside the range of the controls ("+
					fmtG(term.Min)+" to "+fmtG(term.Max)+"); the adjustment is an extrapolation")
			}
			row = append(row, value-term.Center)
		} else {
			found := false
			for _, l := range term.Levels {
				found = found || l == text
			}
			if !found {
				return nil, nil, errorf("covariate '%s' must be one of %s, got '%s'", term.Name, pyList(term.Levels), text)
			}
			if len(term.Levels) > 1 && text == term.Levels[1] {
				row = append(row, 1)
			} else {
				row = append(row, 0)
			}
		}
	}
	return row, notes, nil
}

// Table is a TSV or CSV file read into memory.
type Table struct {
	Path   string
	Header []string
	Rows   []map[string]string
}

// LoadTable reads a TSV or CSV table (e.g. a BIDS participants.tsv).
func LoadTable(path string) (*Table, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimPrefix(string(raw), "\xef\xbb\xbf")
	head, _, _ := strings.Cut(text, "\n")
	reader := csv.NewReader(strings.NewReader(text))
	if strings.Contains(head, "\t") {
		reader.Comma = '\t'
	}
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, errorf("%s: %v", filepath.Base(path), err)
	}
	if len(records) < 2 {
		return nil, errorf("%s has no rows", path)
	}
	table := &Table{Path: path, Header: records[0]}
	for _, rec := range records[1:] {
		row := map[string]string{}
		for i, name := range table.Header {
			if i < len(rec) {
				row[name] = rec[i]
			}
		}
		table.Rows = append(table.Rows, row)
	}
	return table, nil
}

// Columns returns the columns that can serve as covariates (all but the key).
func (t *Table) Columns() []string {
	key := t.key()
	var out []string
	for _, h := range t.Header {
		if h != key {
			out = append(out, h)
		}
	}
	return out
}

func (t *Table) key() string {
	for _, want := range []string{"image", "participant_id"} {
		for _, h := range t.Header {
			if h == want {
				return want
			}
		}
	}
	return t.Header[0]
}

// Lookup returns the covariates of each image.
//
// Rows are matched to images through a column called "image" (a file name
// or path) or else "participant_id" (BIDS style: sub-01 matches
// sub-01_..._pet.nii.gz).  Without either, the first column is used.
func (t *Table) Lookup(images []string, names []string) ([]map[string]string, error) {
	var absent []string
	for _, n := range names {
		found := false
		for _, h := range t.Header {
			found = found || h == n
		}
		if !found {
			absent = append(absent, n)
		}
	}
	if len(absent) > 0 {
		return nil, errorf("%s has no column(s) %s; it has %s", t.Path, pyList(absent), pyList(t.Header))
	}
	key := t.key()
	out := make([]map[string]string, 0, len(images))
	for _, image := range images {
		name := filepath.Base(image)
		var exact, prefix []map[string]string
		for _, row := range t.Rows {
			k := strings.TrimSpace(row[key])
			if k == image || k == name {
				exact = append(exact, row)
			}
			if k != "" && (strings.HasPrefix(name, k+"_") || strings.HasPrefix(name, "sub-"+k+"_")) {
				prefix = append(prefix, row)
			}
		}
		found := exact
		if len(found) == 0 {
			found = prefix
		}
		if len(found) != 1 {
			problem := "no row"
			if len(found) > 0 {
				problem = strconv.Itoa(len(found)) + " rows"
			}
			return nil, errorf("%s in %s (column '%s') for %s", problem, filepath.Base(t.Path), key, name)
		}
		values := map[string]string{}
		for _, n := range names {
			values[n] = strings.TrimSpace(found[0][n])
		}
		out = append(out, values)
	}
	return out, nil
}

// ReadTable looks up the covariates of each image in a TSV or CSV table.
func ReadTable(path string, images []string, names []string) ([]map[string]string, error) {
	table, err := LoadTable(path)
	if err != nil {
		return nil, err
	}
	return table.Lookup(images, names)
}

// CovariatesFromRows turns looked-up rows into one Covariate per name.
func CovariatesFromRows(rows []map[string]string, names []string) []Covariate {
	out := make([]Covariate, len(names))
	for j, name := range names {
		out[j].Name = name
		for _, row := range rows {
			out[j].Values = append(out[j].Values, row[name])
		}
	}
	return out
}
