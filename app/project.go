package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"sugarpet/core"
)

// A project is a folder, laid out as the user guide lays out a study, and a
// file in it that records what the project is and how far its analysis has
// come: its images, the choices of every page, the options of the analyses,
// and what each step last did.  The app works on one project at a time, and
// on nothing while none is open.
//
// The file is written whenever what it records changes, so that a project
// opened again is where it was left.  A path in the folder is written as
// "$PROJECT/…": the folder can be moved, or opened on another computer.

const (
	projectFileName = "sugar-pet-project.json"
	projectFormat   = 1
	projectRoot     = "$PROJECT"
	recentKept      = 8
)

type project struct {
	dir     string // the folder itself, not a link to it
	name    string
	created string // RFC 3339
	// copies: an image added from elsewhere is copied into the folder.
	copies bool
	// native: the images are still in the space of the scanner, and
	// Spatial Normalisation warps them before anything else reads them.
	native             bool
	patients, controls fileList

	// What became of the images last added: how many, and what went wrong.
	st    status
	notes []string

	saved   string    // the state as last written
	saveErr string    // why it could not be written
	tried   time.Time // when it was last tried
	// unsaved: the project is made up, as for the snapshots, and not written.
	unsaved bool
}

// in is a file or folder of the project.
func (p *project) in(name string) string { return filepath.Join(p.dir, name) }

// projectState is what the file of a project holds.
type projectState struct {
	Format     int          `json:"format"`
	Name       string       `json:"name"`
	Created    string       `json:"created"`
	App        string       `json:"app_version"`
	Copies     bool         `json:"copy_images"`
	Native     bool         `json:"native_space"`
	Patients   []string     `json:"patients"`
	Controls   []string     `json:"controls"`
	Covariates covState     `json:"covariates"`
	Reference  string       `json:"reference"`
	Page       string       `json:"page"`
	Normalize  normState    `json:"normalize"`
	Classify   string       `json:"classify_by,omitempty"`
	Compare    compareState `json:"compare"`
	Options    options      `json:"options"`
	Steps      stepRecords  `json:"steps"`
}

type covState struct {
	Table   string          `json:"table"`
	Columns []string        `json:"columns,omitempty"`
	Adjust  map[string]bool `json:"adjust,omitempty"`
}

type regionState struct {
	Kind   int    `json:"kind"`
	Mask   string `json:"mask"`
	File   string `json:"file,omitempty"`
	Labels string `json:"labels,omitempty"`
}

func (r regionChoice) state() regionState {
	return regionState{r.kind, r.mask, r.file, r.labels}
}

func (s regionState) choice() regionChoice {
	r := regionChoice{kind: min(max(s.Kind, 0), 2), mask: s.Mask, file: s.File, labels: s.Labels}
	if !core.IsMask(r.mask) {
		r.mask = "brainstem"
	}
	return r
}

type normState struct {
	Methods []string    `json:"methods"`
	Region  regionState `json:"region"`
}

type compareState struct {
	Method  string `json:"method"`
	Name    string `json:"name"`
	Swapped bool   `json:"swapped,omitempty"`
}

// stepRecords is how far the analysis has come: what each step last did.
type stepRecords struct {
	Spatial   *spatialRun   `json:"spatial,omitempty"`
	Reference *referenceRun `json:"reference,omitempty"`
	Normalize *normRun      `json:"normalize,omitempty"`
	ZMap      *zmapRun      `json:"zmap,omitempty"`
	Classify  *classifyRun  `json:"classify,omitempty"`
	Compare   *compareRun   `json:"compare,omitempty"`
	Cluster   *clusterRun   `json:"cluster,omitempty"`
}

type spatialRun struct {
	Status status            `json:"status"`
	At     string            `json:"at,omitempty"`
	Rows   []spatialRow      `json:"rows,omitempty"`
	Notes  []string          `json:"notes,omitempty"`
	Warped map[string]string `json:"warped,omitempty"`
}

// referenceRun is the last building of the reference of the project, which
// a run of Intensity Normalisation begins with.
type referenceRun struct {
	From *refBuilt `json:"from,omitempty"`
}

type normRun struct {
	Status status    `json:"status"`
	At     string    `json:"at,omitempty"`
	Folder string    `json:"folder,omitempty"`
	Rows   []normRow `json:"rows,omitempty"`
	Notes  []string  `json:"notes,omitempty"`
	Done   *normDone `json:"done,omitempty"`
}

type zmapRun struct {
	Status status    `json:"status"`
	At     string    `json:"at,omitempty"`
	Folder string    `json:"folder,omitempty"`
	Rows   []zmapRow `json:"rows,omitempty"`
	Notes  []string  `json:"notes,omitempty"`
	Done   *zmapDone `json:"done,omitempty"`
	// Current: the maps were made of what step 1 last did.
	Current bool `json:"of_last_normalisation,omitempty"`
}

type classifyRun struct {
	Status status        `json:"status"`
	At     string        `json:"at,omitempty"`
	Folder string        `json:"folder,omitempty"`
	Rows   []classifyRow `json:"rows,omitempty"`
	Notes  []string      `json:"notes,omitempty"`
	Method string        `json:"method,omitempty"`
	// Checked: the findings were cross-checked with other normalisations.
	Checked bool `json:"cross_checked,omitempty"`
	// Current: the reports were made of what step 2 last did.
	Current bool `json:"of_last_maps,omitempty"`
}

type compareRun struct {
	Status status                `json:"status"`
	At     string                `json:"at,omitempty"`
	How    string                `json:"how,omitempty"`
	Result *core.GroupComparison `json:"result,omitempty"`
}

type clusterRun struct {
	Status status  `json:"status"`
	At     string  `json:"at,omitempty"`
	Saved  string  `json:"saved,omitempty"`
	Volume float64 `json:"volume_ml,omitempty"`
}

// at is when the last run of a page ended, as the file of a project keeps it.
func (a *app) at(page string) string {
	if t, ok := a.ranAt[page]; ok {
		return t.Format(time.RFC3339)
	}
	return ""
}

func (a *app) setAt(page, at string) {
	t, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return
	}
	if a.ranAt == nil {
		a.ranAt = map[string]time.Time{}
	}
	a.ranAt[page] = t
}

// projectState is the project as it is now, to be written.
func (a *app) projectState() *projectState {
	p := a.proj
	s := &projectState{Format: projectFormat, Name: p.name, Created: p.created, App: core.Version,
		Copies: p.copies, Native: p.native, Patients: p.patients.files, Controls: p.controls.files,
		Reference: a.norm.reference, Page: a.page, Classify: a.classify.main, Options: a.opt}
	if s.Patients == nil {
		s.Patients = []string{}
	}
	if s.Controls == nil {
		s.Controls = []string{}
	}
	s.Covariates = covState{Table: a.cov.path, Columns: a.cov.columns, Adjust: a.cov.adjust}
	s.Normalize = normState{Methods: a.norm.chosen(), Region: a.norm.region.state()}
	s.Compare = compareState{Method: a.cmp.method, Name: a.cmp.name, Swapped: a.cmp.swapped}

	ran := func(st status, more bool) bool { return st.ok != "" || st.err != "" || more }
	if sp := &a.spatial; ran(sp.st, len(sp.rows) > 0) {
		s.Steps.Spatial = &spatialRun{Status: sp.st, At: a.at("spatial"), Rows: sp.rows, Notes: sp.notes, Warped: sp.warped}
	}
	if a.ref.built != nil {
		s.Steps.Reference = &referenceRun{From: a.ref.built}
	}
	if n := &a.norm; ran(n.st, len(n.rows) > 0) {
		s.Steps.Normalize = &normRun{Status: n.st, At: a.at("normalize"), Folder: n.outDir, Rows: n.rows, Notes: n.notes, Done: n.done}
	}
	if z := &a.zmap; ran(z.st, len(z.rows) > 0) {
		s.Steps.ZMap = &zmapRun{Status: z.st, At: a.at("zmap"), Folder: z.outDir, Rows: z.rows, Notes: z.notes, Done: z.done,
			Current: z.done != nil && z.done.from == a.norm.done}
	}
	if cl := &a.classify; ran(cl.st, len(cl.rows) > 0) {
		s.Steps.Classify = &classifyRun{Status: cl.st, At: a.at("classify"), Folder: cl.outDir, Rows: cl.rows, Notes: cl.notes,
			Method: cl.method, Checked: cl.checked, Current: cl.from != nil && cl.from == a.zmap.done}
	}
	if cmp := &a.cmp; ran(cmp.st, cmp.result != nil) {
		s.Steps.Compare = &compareRun{Status: cmp.st, At: a.at("compare"), How: cmp.how, Result: cmp.result}
	}
	if cl := &a.cluster; ran(cl.st, cl.saved != "") {
		s.Steps.Cluster = &clusterRun{Status: cl.st, At: a.at("cluster"), Saved: cl.saved, Volume: cl.volume}
	}
	return s
}

// applyState makes the pages what the file of the project says they were.
func (a *app) applyState(s *projectState) {
	p := a.proj
	p.name, p.created, p.copies, p.native = s.Name, s.Created, s.Copies, s.Native
	if p.name == "" {
		p.name = filepath.Base(p.dir)
	}
	p.patients.files, p.controls.files = s.Patients, s.Controls
	a.opt = s.Options
	a.opt.sound()
	for _, pg := range pages {
		if pg.id == s.Page {
			a.page = s.Page
		}
	}
	a.cov = covChoice{path: s.Covariates.Table, adjust: s.Covariates.Adjust, columns: s.Covariates.Columns}
	if a.cov.adjust == nil {
		a.cov.adjust = map[string]bool{}
	}
	a.cov.read()
	a.norm.methods = map[string]bool{}
	for _, m := range s.Normalize.Methods {
		a.norm.methods[m] = true
	}
	a.norm.region = s.Normalize.Region.choice()
	a.classify.main = s.Classify
	a.cmp.swapped = s.Compare.Swapped
	if slices.ContainsFunc(core.Methods, func(m core.Named) bool { return m.Name == s.Compare.Method }) {
		a.cmp.method = s.Compare.Method
	}
	if s.Compare.Name != "" {
		a.cmp.name = s.Compare.Name
	}
	if s.Reference != "" && core.IsReferenceFolder(s.Reference) {
		a.setReference(s.Reference)
	}

	gone := T("The files this step wrote are no longer in the project: run it again.", "这一步写出的文件已不在项目中：请重新运行。")
	there := func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.Mode().IsRegular()
	}
	if r := s.Steps.Spatial; r != nil {
		sp := &a.spatial
		sp.st, sp.rows, sp.notes, sp.warped, sp.outDir = r.Status, r.Rows, r.Notes, r.Warped, p.dir
		for native, warped := range sp.warped {
			if !there(warped) { // warped anew when the step is run again
				delete(sp.warped, native)
			}
		}
		for _, row := range sp.rows {
			if !row.failed && there(row.path) {
				sp.written = append(sp.written, row.path)
			}
		}
		a.setAt("spatial", r.At)
	}
	if r := s.Steps.Reference; r != nil && core.IsReferenceFolder(a.ref.output) {
		a.ref.built = r.From
	}
	if r := s.Steps.Normalize; r != nil {
		n := &a.norm
		n.st, n.rows, n.notes, n.outDir, n.done = r.Status, r.Rows, r.Notes, r.Folder, r.Done
		if d := n.done; d != nil {
			if !d.written(there) {
				n.done, n.st = nil, status{err: gone}
			} else {
				d.look()
			}
		}
		a.setAt("normalize", r.At)
	}
	if r := s.Steps.ZMap; r != nil {
		z := &a.zmap
		z.st, z.rows, z.notes, z.outDir, z.done = r.Status, r.Rows, r.Notes, r.Folder, r.Done
		if d := z.done; d != nil {
			d.from = &normDone{} // of a run of step 1 that is no more
			if r.Current && a.norm.done != nil {
				d.from = a.norm.done
			}
			for _, maps := range d.maps {
				for _, path := range maps {
					if !there(path) {
						z.done, z.st = nil, status{err: gone}
					}
				}
			}
		}
		a.setAt("zmap", r.At)
	}
	if r := s.Steps.Classify; r != nil {
		cl := &a.classify
		cl.st, cl.rows, cl.notes, cl.outDir, cl.method, cl.checked = r.Status, r.Rows, r.Notes, r.Folder, r.Method, r.Checked
		if r.Current && a.zmap.done != nil {
			cl.from = a.zmap.done
		}
		a.setAt("classify", r.At)
	}
	if r := s.Steps.Compare; r != nil {
		a.cmp.st, a.cmp.how, a.cmp.result = r.Status, r.How, r.Result
		if res := a.cmp.result; res != nil && !there(res.TMap) {
			a.cmp.result, a.cmp.how, a.cmp.st = nil, "", status{err: gone}
		}
		a.setAt("compare", r.At)
	}
	if r := s.Steps.Cluster; r != nil {
		a.cluster.st, a.cluster.saved, a.cluster.volume = r.Status, r.Saved, r.Volume
		if a.cluster.saved != "" && !there(a.cluster.saved) {
			a.cluster.saved, a.cluster.volume, a.cluster.st = "", 0, status{err: gone}
		}
		a.setAt("cluster", r.At)
	}
}

// A JSON string, as it stands in a file: what a path is written in.
var jsonString = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// mapStrings changes every string of a JSON text, names and values alike,
// and leaves the rest of the text as it is.
func mapStrings(raw []byte, change func(string) string) []byte {
	return jsonString.ReplaceAllFunc(raw, func(token []byte) []byte {
		var text string
		if json.Unmarshal(token, &text) != nil {
			return token
		}
		changed := change(text)
		if changed == text {
			return token
		}
		var out bytes.Buffer
		enc := json.NewEncoder(&out)
		enc.SetEscapeHTML(false)
		if enc.Encode(changed) != nil {
			return token
		}
		return bytes.TrimRight(out.Bytes(), "\n")
	})
}

// portable writes the paths in a project folder as "$PROJECT/…", and local
// reads them back as paths of this computer.
func portable(raw []byte, dir string) []byte {
	prefix := dir + string(filepath.Separator)
	return mapStrings(raw, func(s string) string {
		switch {
		case s == dir:
			return projectRoot
		case strings.HasPrefix(s, prefix):
			return projectRoot + "/" + filepath.ToSlash(s[len(prefix):])
		}
		return s
	})
}

func local(raw []byte, dir string) []byte {
	return mapStrings(raw, func(s string) string {
		switch {
		case s == projectRoot:
			return dir
		case strings.HasPrefix(s, projectRoot+"/"):
			return filepath.Join(dir, filepath.FromSlash(s[len(projectRoot)+1:]))
		}
		return s
	})
}

// isProject reports whether a folder holds a project.
func isProject(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, projectFileName))
	return err == nil && info.Mode().IsRegular()
}

// saveProject writes the file of the project if what it records changed.
// It is called after every frame: a project is never to be saved by hand.
func (a *app) saveProject() {
	p := a.proj
	if p == nil || p.unsaved {
		return
	}
	raw, err := json.MarshalIndent(a.projectState(), "", "  ")
	if err != nil {
		p.saveErr = err.Error()
		return
	}
	text := string(raw)
	if text == p.saved {
		return
	}
	// A folder that cannot be written, as on a drive taken out, is not
	// tried at every frame.
	if p.saveErr != "" && time.Since(p.tried) < 5*time.Second {
		return
	}
	p.tried = time.Now()
	target := filepath.Join(p.dir, projectFileName)
	// Written beside the file and moved over it: a write that is cut short
	// leaves the project as it was.
	tmp := target + ".tmp"
	if err = os.WriteFile(tmp, append(portable(raw, p.dir), '\n'), 0o644); err == nil {
		err = os.Rename(tmp, target)
	}
	if err != nil {
		os.Remove(tmp)
		p.saveErr = err.Error()
		return
	}
	p.saved, p.saveErr = text, ""
}

// resetPages puts the pages back as the app first has them, for another
// project.  What is of the computer and not of a project stays: the language,
// SPM and MATLAB.
func (a *app) resetPages() {
	spm, bad, matlab := a.spatial.spm, a.spatial.spmBad, a.spatial.matlab
	a.spatial = spatialPage{spm: spm, spmBad: bad, matlab: matlab}
	a.norm, a.ref, a.zmap, a.classify = normPage{}, refPage{}, zmapPage{}, classifyPage{}
	a.cmp, a.cluster, a.masks = comparePage{}, clusterPage{}, masksPage{}
	a.norm.init()
	a.ref.init()
	a.zmap.init()
	a.classify.init()
	a.cmp.init()
	a.cluster.init()
	a.cov = covChoice{adjust: map[string]bool{}}
	a.opt = defaultOptions()
	a.page, a.reveal, a.scrolls, a.seen, a.normTold, a.ranAt = "data", "", nil, nil, nil, nil
}

// projectOutputs makes every page write into its folder of the project.
func (a *app) projectOutputs() {
	p := a.proj
	a.ref.output, a.norm.output, a.zmap.output = p.in("hc_reference"), p.in("normalised"), p.in("zmaps")
	a.classify.output, a.cmp.output = p.in("reports"), p.in("compare")
	a.cluster.output = p.in("reference_cluster.nii.gz")
}

// realDir is a folder itself, not a link to it: what a project is known by.
func realDir(dir string) string {
	dir = filepath.Clean(dir)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	return dir
}

// openProject opens the project a folder holds.
func (a *app) openProject(dir string) error {
	if a.job != nil {
		return errorString(T("A task is running: wait for it, or cancel it.", "有任务正在运行：请等它完成，或先取消。"))
	}
	dir = realDir(dir)
	raw, err := os.ReadFile(filepath.Join(dir, projectFileName))
	if err != nil {
		return errorString(Tf("%s holds no project (%s is not in it).", "%s 中没有项目（找不到 %s）。", shortDir(dir), projectFileName))
	}
	s := projectState{Options: defaultOptions()}
	if err := json.Unmarshal(local(raw, dir), &s); err != nil {
		return errorString(Tf("The project file cannot be read: %s", "无法读取项目文件：%s", err.Error()))
	}
	if s.Format > projectFormat {
		return errorString(T("This project was written by a newer version of SUGAR-PET: update the app to open it.",
			"该项目由更新版本的 SUGAR-PET 写入：请先更新应用再打开。"))
	}
	a.saveProject() // the one that is left
	a.resetPages()
	a.proj = &project{dir: dir}
	a.projectOutputs()
	a.applyState(&s)
	a.feed()
	// What the file says is what is open: nothing is written until it changes.
	if now, err := json.MarshalIndent(a.projectState(), "", "  "); err == nil && bytes.Equal(portable(now, dir), bytes.TrimRight(raw, "\n")) {
		a.proj.saved = string(now)
	}
	a.remember(dir)
	return nil
}

// createProject makes a folder a project and opens it.  The folders of a
// study are laid out in it; what the folder holds already is kept, and its
// images and its reference are taken into the project.
func (a *app) createProject(dir, name string, copies bool) error {
	if a.job != nil {
		return errorString(T("A task is running: wait for it, or cancel it.", "有任务正在运行：请等它完成，或先取消。"))
	}
	if isProject(dir) {
		return errorString(Tf("%s is a project already: open it.", "%s 已经是一个项目：请直接打开。", shortDir(dir)))
	}
	if _, err := core.InitStudy(dir); err != nil {
		return err
	}
	dir = realDir(dir)
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(dir)
	}
	a.saveProject()
	a.resetPages()
	a.proj = &project{dir: dir, name: strings.TrimSpace(name), created: time.Now().Format(time.RFC3339), copies: copies}
	p := a.proj
	a.projectOutputs()
	p.patients.add([]string{p.in("patients")})
	p.controls.add([]string{p.in("controls")})
	if own := p.in("hc_reference"); core.IsReferenceFolder(own) {
		a.setReference(own)
	}
	// A table of the subjects that was filled in already is used.
	if table := p.in("participants.tsv"); filled(table) {
		a.cov.path = table
		a.cov.load()
	}
	a.feed()
	a.saveProject()
	if p.saveErr != "" {
		err := errorString(Tf("The project cannot be written into %s: %s", "无法把项目写入 %s：%s", shortDir(dir), p.saveErr))
		a.proj = nil
		a.resetPages()
		return err
	}
	a.remember(dir)
	return nil
}

// filled reports whether a table has a row besides its header.
func filled(table string) bool {
	raw, err := os.ReadFile(table)
	return err == nil && len(strings.Fields(string(raw))) > len(strings.Fields(core.ParticipantsHeader))
}

// closeProject leaves the project: the app shows where to open another.
func (a *app) closeProject() {
	if a.proj == nil || a.job != nil {
		return
	}
	a.saveProject()
	a.proj = nil
	a.resetPages()
	if a.settingsWin != nil {
		a.settingsWin.Invalidate()
	}
}

// remember puts a project first among those opened lately.
func (a *app) remember(dir string) {
	a.recent = slices.DeleteFunc(a.recent, func(d string) bool { return d == dir })
	a.recent = append([]string{dir}, a.recent...)
	if len(a.recent) > recentKept {
		a.recent = a.recent[:recentKept]
	}
	if parent := filepath.Dir(dir); parent != dir {
		a.welcome.parent = parent
	}
}

func (a *app) forget(dir string) {
	a.recent = slices.DeleteFunc(a.recent, func(d string) bool { return d == dir })
}

// openFolder opens the project of a folder; of a folder that holds none, as
// a study of an earlier version, it offers to make one.
func (a *app) openFolder(dir string) {
	fail := func(err error) {
		if err != nil {
			a.welcome.err = err.Error()
			if a.proj != nil { // said over the project that stays open
				a.alert(T("The project was not opened", "未能打开项目"), err.Error())
			}
		}
	}
	a.welcome.err = ""
	if isProject(dir) {
		fail(a.openProject(dir))
		return
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		fail(errorString(Tf("%s cannot be reached.", "无法访问 %s。", shortDir(dir))))
		return
	}
	a.confirm(T("Make this folder a project?", "把这个文件夹建成项目？"),
		Tf("%s holds no project yet. Making it one keeps everything in it, adds the folders that are missing, and takes the images in patients and controls into the project.",
			"%s 中还没有项目。建成项目后，其中已有的内容全部保留，补上缺少的文件夹，并把 patients 和 controls 里的图像纳入项目。", shortDir(dir)),
		T("Make Project", "建成项目"), func() { fail(a.createProject(dir, "", false)) })
}

// confirm asks before something is done.  Without a window, as in tests, it
// is done.
func (a *app) confirm(title, detail, yes string, then func()) {
	if a.win == nil {
		then()
		return
	}
	parent := a.dialogs
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: parent, Type: mygo.MessageQuestion, Message: title, Detail: detail,
			Buttons: []string{yes, T("Cancel", "取消")}, CancelButton: 1})
		if err == nil && res.Button == 0 {
			a.update(then)
		}
	}()
}

// images are the images of the project in MNI space: its own or, of images
// in native space, what Spatial Normalisation made of them.
func (a *app) images() (patients, controls []string) {
	p := a.proj
	if !p.native {
		return p.patients.files, p.controls.files
	}
	warped := func(native []string) (out []string) {
		for _, f := range native {
			if w := a.spatial.warped[f]; w != "" {
				out = append(out, w)
			}
		}
		return out
	}
	return warped(p.patients.files), warped(p.controls.files)
}

// feed gives the pages what they take of the project: its images and its
// covariate table.  No page asks for them itself.
func (a *app) feed() {
	p := a.proj
	if p == nil {
		return
	}
	patients, controls := a.images()
	a.ref.files.files, a.cluster.files.files = controls, patients
	// Intensity Normalisation takes both groups, the patients first.
	a.norm.files.files, a.norm.controls = append(append([]string(nil), patients...), controls...), controls
	if a.cmp.swapped {
		patients, controls = controls, patients
	}
	a.cmp.g1.files, a.cmp.g2.files = patients, controls
	a.spatial.files.files = nil
	if p.native {
		a.spatial.files.files = append(append([]string(nil), p.patients.files...), p.controls.files...)
	}
	a.zmap.covTable = a.cov.path
}

// groupFolder is the folder of the project that the images of a group are
// copied into: raw while they are still in native space.
func (p *project) groupFolder(group string) string {
	if p.native {
		return "raw"
	}
	return group
}

// addImages takes images into a group of the project.  Where the project
// copies its images, those from elsewhere are copied into its folder first,
// in the background.
func (a *app) addImages(list *fileList, group string, paths []string) {
	p := a.proj
	files := expandPaths(paths)
	p.st, p.notes = status{}, nil
	if !p.copies {
		list.add(files)
		return
	}
	var outside []string
	for _, f := range files {
		if insideDir(f, p.dir) {
			list.add([]string{f})
		} else {
			outside = append(outside, f)
		}
	}
	if len(outside) == 0 {
		return
	}
	if a.job != nil {
		p.st.err = T("Another task is running: add the images once it is done.", "另一项任务正在运行：请等它完成后再添加图像。")
		return
	}
	folder := p.groupFolder(group)
	dest := p.in(folder)
	a.start("data", Tf("Copying images into %s", "正在把图像复制到 %s", folder), func(ctx context.Context, step func(int, int)) (func(), error) {
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return nil, err
		}
		var copied, notes []string
		step(0, len(outside))
		for i, src := range outside {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			target := filepath.Join(dest, filepath.Base(src))
			if err := copyImage(src, target); err != nil {
				notes = append(notes, filepath.Base(src)+": "+err.Error())
			} else {
				copied = append(copied, target)
			}
			step(i+1, len(outside))
		}
		return func() {
			list.add(copied)
			p.notes = notes
			if len(notes) > 0 {
				p.st.err = Tf("%d of %d image(s) were not copied; see the notes below the images.", "%d / %d 幅图像未能复制，详见图像下方的说明。", len(notes), len(outside))
			} else {
				p.st.ok = Tf("%d image(s) copied into %s.", "已把 %d 幅图像复制到 %s。", len(copied), folder)
			}
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}

// copyImage copies an image, and the other file of a header and image pair,
// into the project.  A file that is there under its name is kept if it is
// the same image, and is in the way if it is another.
func copyImage(src, dst string) error {
	pair := []string{src}
	lower := strings.ToLower(src)
	for ext, mate := range map[string]string{".hdr": ".img", ".img": ".hdr"} {
		if strings.HasSuffix(lower, ext) {
			other := src[:len(src)-len(ext)] + mate
			if _, err := os.Stat(other); err == nil {
				pair = append(pair, other)
			}
		}
	}
	for _, from := range pair {
		to := filepath.Join(filepath.Dir(dst), filepath.Base(from))
		if err := copyFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		if same, err := sameContent(from, to); err != nil {
			return err
		} else if !same {
			return errorString(Tf("another file of this name is in %s already", "%s 中已有同名的另一个文件", filepath.Base(filepath.Dir(to))))
		}
		return nil
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if closed := out.Close(); err == nil {
		err = closed
	}
	if err != nil {
		os.Remove(to) // half an image would be read as one
	}
	return err
}

// sameContent reports whether two files hold the same bytes.
func sameContent(a, b string) (bool, error) {
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if os.SameFile(ia, ib) {
		return true, nil
	}
	if ia.Size() != ib.Size() {
		return false, nil
	}
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()
	bufA, bufB := make([]byte, 1<<16), make([]byte, 1<<16)
	for {
		na, errA := io.ReadFull(fa, bufA)
		nb, _ := io.ReadFull(fb, bufB)
		if na != nb || !bytes.Equal(bufA[:na], bufB[:nb]) {
			return false, nil
		}
		if errA != nil { // the end of both
			return true, nil
		}
	}
}

// sameDir reports whether two paths name the same folder, however they are
// spelt: through a link, or in other letter case where that is the same.
func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// insideDir reports whether a file or folder lies in a folder (or is it).
func insideDir(path, dir string) bool {
	if path == "" || dir == "" {
		return false
	}
	below := func(path, dir string) bool {
		rel, err := filepath.Rel(dir, path)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if below(path, dir) {
		return true
	}
	// A link elsewhere may lead into the folder.
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		real = path
	} else if realDir, err := filepath.EvalSymlinks(dir); err == nil && below(real, realDir) {
		return true
	}
	// The folder may be spelt otherwise: in other letter case, where that
	// is the same folder.  It is then known by what it is, not by its name.
	above := func(p string) bool {
		for {
			if sameDir(p, dir) {
				return true
			}
			parent := filepath.Dir(p)
			if parent == p {
				return false
			}
			p = parent
		}
	}
	return above(path) || real != path && above(real)
}
