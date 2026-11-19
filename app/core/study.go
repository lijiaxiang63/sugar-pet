package core

import (
	"os"
	"path/filepath"
)

// StudyEntry is a folder of a study, or its table of ages and sexes, laid
// out as the user guide recommends.
type StudyEntry struct {
	Name  string
	Dir   bool
	Yours bool // the user fills it; the others are written by the analyses
	Text  string
}

// StudyLayout is what a study folder holds, in the order of the guide.
var StudyLayout = []StudyEntry{
	{"raw", true, true, "Images still in native scanner space. Only needed if yours are not in MNI space yet: 'spatial-norm' warps them into patients/ and controls/."},
	{"patients", true, true, "Patient images in MNI space, one scan per subject, e.g. sub-pt01_space-MNI_pet.nii.gz."},
	{"controls", true, true, "Healthy-control images in MNI space, from the same scanner and preprocessing as the patients."},
	{"participants.tsv", false, true, "Age and sex of every subject, patients and controls, one row each. Used to adjust for them."},
	{"hc_reference", true, false, "The reference that 'build-reference' makes from controls/. Most other steps read it."},
	{"normalised", true, false, "Intensity-normalised images and factors.tsv, from 'normalize'."},
	{"zmaps", true, false, "Z-score or W-score maps of the patients, from 'zmap'."},
	{"reports", true, false, "Classification reports of the patients, from 'classify'."},
	{"compare", true, false, "T-maps and cluster tables of patients against controls, from 'compare'."},
}

// ParticipantsHeader is the first line of a new participants.tsv.
const ParticipantsHeader = "participant_id\tage\tsex\n"

// InitStudy creates the folders of a study and an empty participants.tsv in
// path.  It reports, for each entry of StudyLayout, whether it was created
// (false: it was there already).  Nothing that exists is changed.
func InitStudy(path string) ([]bool, error) {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return nil, errorf("'%s' is a file, not a folder", path)
	}
	created := make([]bool, len(StudyLayout))
	for i, e := range StudyLayout {
		target := filepath.Join(path, e.Name)
		info, err := os.Stat(target)
		switch {
		case err != nil:
			created[i] = true
		case info.IsDir() != e.Dir && e.Dir:
			return nil, errorf("'%s' exists and is not a folder", target)
		case info.IsDir() != e.Dir:
			return nil, errorf("'%s' exists and is not a file", target)
		}
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	for i, e := range StudyLayout {
		if !created[i] {
			continue
		}
		target := filepath.Join(path, e.Name)
		var err error
		if e.Dir {
			err = os.Mkdir(target, 0o755)
		} else {
			err = os.WriteFile(target, []byte(ParticipantsHeader), 0o644)
		}
		if err != nil {
			return nil, err
		}
	}
	return created, nil
}
