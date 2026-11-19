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

// studyEntryPresent reports whether an entry of a study is there already.
// One of the wrong kind is an error, and so is a link that leads nowhere:
// writing through it would create a file outside the study.
func studyEntryPresent(target string, dir bool) (bool, error) {
	if _, err := os.Lstat(target); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	info, err := os.Stat(target)
	switch {
	case os.IsNotExist(err):
		return false, errorf("'%s' is a link that leads nowhere", target)
	case err != nil:
		return false, err
	case dir && !info.IsDir():
		return false, errorf("'%s' exists and is not a folder", target)
	case !dir && !info.Mode().IsRegular():
		// A pipe or a device cannot be read as a table.
		return false, errorf("'%s' exists and is not a file", target)
	}
	return true, nil
}

// InitStudy creates the folders of a study and an empty participants.tsv in
// path.  It reports, for each entry of StudyLayout, whether it was created
// (false: it was there already).  Nothing that exists is changed.
func InitStudy(path string) ([]bool, error) {
	if path == "" {
		path = "."
	}
	if len(path) == 2 && path[1] == ':' && filepath.VolumeName(path) == path {
		path += "." // "C:" is the current folder of the drive, "C:\raw" is not in it
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return nil, errorf("'%s' is a file, not a folder", path)
	}
	// The entries are named as the system reads the path.  filepath.Join
	// would shorten "link/../study" to "study", which is another folder
	// when link leads elsewhere.
	in := func(name string) string { return path + string(filepath.Separator) + name }
	// Everything is looked at before anything is made.
	created := make([]bool, len(StudyLayout))
	for i, e := range StudyLayout {
		present, err := studyEntryPresent(in(e.Name), e.Dir)
		if err != nil {
			return nil, err
		}
		created[i] = !present
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	for i, e := range StudyLayout {
		if !created[i] {
			continue
		}
		target := in(e.Name)
		var err error
		if e.Dir {
			err = os.Mkdir(target, 0o755)
		} else {
			// O_EXCL: never written over, also if it appeared just now.
			var table *os.File
			if table, err = os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644); err == nil {
				_, err = table.WriteString(ParticipantsHeader)
				if closed := table.Close(); err == nil {
					err = closed
				}
				if err != nil {
					// No table is better than one without its header,
					// which would be kept from then on.
					os.Remove(target)
				}
			}
		}
		if os.IsExist(err) {
			// It appeared meanwhile: it is kept, if it is what it should be.
			present, err := studyEntryPresent(target, e.Dir)
			if err != nil {
				return nil, err
			}
			if !present {
				return nil, errorf("'%s' changed while the folders were created: run it again", target)
			}
			created[i] = false
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return created, nil
}
