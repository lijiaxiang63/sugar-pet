// Command sugar-pet-cli is the command line of the Go port, with the same
// commands and options as the Python "sugar-pet" program.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sugarpet/core"
)

type args struct {
	pos    []string
	values map[string][]string
	flags  map[string]bool
}

var aliases = map[string]string{"-m": "--method", "-o": "--output", "-r": "--reference"}

// parse splits the command line: bools take no value, lists take every
// following word up to the next option, the rest take one value.
func parse(argv []string, bools, lists []string) (*args, error) {
	a := &args{values: map[string][]string{}, flags: map[string]bool{}}
	has := func(set []string, name string) bool {
		for _, s := range set {
			if s == name {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(argv); i++ {
		word := argv[i]
		if alias, ok := aliases[word]; ok {
			word = alias
		}
		if !strings.HasPrefix(word, "--") {
			a.pos = append(a.pos, word)
			continue
		}
		name, value, inline := strings.Cut(word, "=")
		switch {
		case has(bools, name):
			a.flags[name] = true
		case has(lists, name):
			for i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++
				a.values[name] = append(a.values[name], argv[i])
			}
		case inline:
			a.values[name] = append(a.values[name], value)
		default:
			if i+1 >= len(argv) {
				return nil, fmt.Errorf("option %s needs a value", name)
			}
			i++
			a.values[name] = append(a.values[name], argv[i])
		}
	}
	return a, nil
}

func (a *args) str(name, fallback string) string {
	if v := a.values[name]; len(v) > 0 {
		return v[len(v)-1]
	}
	return fallback
}

func (a *args) num(name string, fallback float64) float64 {
	if v := a.values[name]; len(v) > 0 {
		f, err := strconv.ParseFloat(v[len(v)-1], 64)
		if err != nil {
			fail(2, "option %s: '%s' is not a number", name, v[len(v)-1])
		}
		return f
	}
	return fallback
}

func (a *args) optNum(name string) *float64 {
	if len(a.values[name]) == 0 {
		return nil
	}
	f := a.num(name, 0)
	return &f
}

// split turns ["a,b", "c"] into ["a", "b", "c"].
func split(items []string) []string {
	var out []string
	for _, item := range items {
		for _, part := range strings.Split(item, ",") {
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func fail(code int, format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(code)
}

func check(err error) {
	if err != nil {
		fail(2, "%v", err)
	}
}

func need(a *args, name string) string {
	v := a.str(name, "")
	if v == "" {
		fail(2, "option %s is required", name)
	}
	return v
}

func unknownMethods(methods []string) {
	var unknown []string
	for _, m := range methods {
		if !core.IsMethod(m) {
			unknown = append(unknown, m)
		}
	}
	if len(unknown) == 0 {
		return
	}
	text := "unknown method(s): " + strings.Join(unknown, ", ") + ". Run 'sugar-pet-cli methods'."
	for _, m := range unknown {
		if core.IsMask(m) {
			text += " '" + m + "' is a built-in mask: use -m region --mask " + m + "."
			break
		}
	}
	fail(2, "%s", text)
}

func progress(done, total int) {
	end := "\n"
	if done < total {
		end = ""
	}
	fmt.Fprintf(os.Stderr, "\r  %d/%d%s", done, total, end)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sugar-pet-cli <init|methods|normalize|build-reference|zmap|classify|compare|reference-cluster|masks|labels|spatial-norm> ...")
		os.Exit(2)
	}
	ctx := context.Background()
	command, rest := os.Args[1], os.Args[2:]
	switch command {
	case "--version":
		fmt.Println("sugar-pet", core.Version)
	case "init":
		initStudy(rest)
	case "methods":
		fmt.Println("Normalisation methods (-m):")
		for _, m := range core.Methods {
			fmt.Printf("  %-15s %s\n", m.Name, m.Text)
		}
		fmt.Println("\nBuilt-in masks (--mask):")
		for _, m := range core.Masks {
			fmt.Printf("  %-15s %s\n", m.Name, m.Text)
		}
	case "labels":
		for _, l := range core.AtlasLabels() {
			fmt.Printf("%4d  %s (%s)\n", l.Index, l.Name, l.Hemi)
		}
	case "masks":
		a, err := parse(rest, nil, nil)
		check(err)
		out := need(a, "--output")
		for _, m := range core.Masks {
			vol, _ := core.BuiltinMask(m.Name)
			check(core.SaveNifti(filepath.Join(out, m.Name+".nii.gz"), vol, true))
		}
		fmt.Printf("%d masks written to %s\n", len(core.Masks), out)
	case "build-reference":
		a, err := parse(rest, nil, nil)
		check(err)
		fmt.Fprintf(os.Stderr, "Building reference from %d controls ...\n", len(a.pos))
		opt := core.BuildOptions{FWHM: a.num("--fwhm", 8), Progress: progress,
			Mask: core.MaskSpec{Name: a.str("--mask", "gray-matter")}}
		opt.StatsFor = split(a.values["--stats-for"])
		if len(opt.StatsFor) == 1 && opt.StatsFor[0] == "all" {
			opt.StatsFor = core.StatsMethods()
		}
		if table := a.str("--covariates", ""); table != "" {
			names := split([]string{a.str("--adjust-for", "age,sex")})
			rows, err := core.ReadTable(table, a.pos, names)
			check(err)
			opt.Covariates = core.CovariatesFromRows(rows, names)
		}
		ref, err := core.BuildReference(ctx, a.pos, opt)
		check(err)
		check(ref.Save(need(a, "--output")))
		fmt.Printf("Reference saved to %s\n", a.str("--output", ""))
	case "normalize":
		os.Exit(normalize(rest))
	case "zmap":
		os.Exit(zmap(rest))
	case "classify":
		os.Exit(classify(rest))
	case "compare":
		compare(ctx, rest)
	case "reference-cluster":
		a, err := parse(rest, nil, nil)
		check(err)
		ref, err := core.LoadReference(need(a, "--reference"))
		check(err)
		fmt.Fprintf(os.Stderr, "Comparing %d patients with the controls ...\n", len(a.pos))
		cluster, err := core.ReferenceCluster(ctx, a.pos, ref, core.ClusterOptions{
			PThreshold: a.num("--p-threshold", 0.001), MinClusterML: a.num("--min-cluster", 1), Progress: progress})
		check(err)
		out := need(a, "--output")
		check(core.SaveNifti(out, cluster, true))
		n := 0
		for _, v := range cluster.Data {
			if v > 0 {
				n++
			}
		}
		fmt.Printf("Reference cluster (%.0f mL) written to %s\n", float64(n)*core.VoxelVolumeML(cluster.Aff), out)
	case "spatial-norm":
		a, err := parse(rest, nil, nil)
		check(err)
		out := need(a, "--output")
		targets := make([]string, len(a.pos))
		for i, path := range a.pos {
			targets[i] = filepath.Join(out, core.SpatialOutputName(path))
		}
		results, err := core.SpatialNormalize(ctx, a.pos, targets, core.SpatialOptions{
			SPMPath: need(a, "--spm-path"), Template: a.str("--template", ""), Matlab: a.str("--matlab", ""),
			Jobs: int(a.num("--jobs", 1)), WorkDir: a.str("--work-dir", ""), Progress: progress})
		check(err)
		written := 0
		for _, r := range results {
			if r.Err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", r.Input, r.Err)
				continue
			}
			written++
		}
		fmt.Printf("%d of %d images written to %s\n", written, len(targets), out)
		if written != len(targets) {
			os.Exit(1)
		}
	default:
		fail(2, "unknown command '%s'", command)
	}
}

// initStudy creates the recommended folders of a study and says what each
// is for.
func initStudy(rest []string) {
	for _, word := range rest {
		if word == "-h" || word == "--help" {
			fmt.Println("usage: sugar-pet-cli init [folder]")
			fmt.Println("\nCreate the recommended folders of a study and an empty participants.tsv.")
			fmt.Println("The folder to create or complete defaults to 'study'.")
			return
		}
	}
	// Nothing is made unless the command is understood.
	for _, word := range rest {
		if strings.HasPrefix(word, "-") {
			fail(2, "init has no option '%s': it takes only the folder to create", word)
		}
	}
	if len(rest) > 1 {
		fail(2, "init takes one folder, not %d", len(rest))
	}
	root := "study"
	if len(rest) == 1 {
		root = rest[0]
	}
	created, err := core.InitStudy(root)
	check(err)
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	fmt.Printf("Study folder: %s\n", root)
	made := 0
	for _, group := range []struct {
		title string
		yours bool
	}{{"For you to fill:", true}, {"Written by the analyses:", false}} {
		fmt.Printf("\n%s\n", group.title)
		for i, e := range core.StudyLayout {
			if e.Yours != group.yours {
				continue
			}
			name, state := e.Name, "already there"
			if e.Dir {
				name += "/"
			}
			if created[i] {
				state = "created"
				made++
			}
			fmt.Printf("  %-18s%s\n    %s\n", name, state, e.Text)
		}
	}
	fmt.Printf("\n%d created, %d already there. Nothing that existed was changed.\n", made, len(created)-made)
	fmt.Println("Next: put your images in patients/ and controls/, and fill in participants.tsv.")
}

func normalize(rest []string) int {
	a, err := parse(rest, []string{"--literal-ips", "--factors-only", "--save-masks"}, nil)
	check(err)
	methods := split(a.values["--method"])
	reference := a.str("--reference", "")
	if len(methods) == 1 && methods[0] == "all" {
		methods = nil
		for _, m := range core.Methods {
			if m.Name != "region" && (reference != "" || !core.NeedsReference(m.Name)) {
				methods = append(methods, m.Name)
			}
		}
	}
	unknownMethods(methods)
	mask, labels := a.str("--mask", ""), a.str("--labels", "")
	if mask != "" && labels != "" {
		fail(2, "--mask and --labels cannot be combined")
	} else if (mask != "" || labels != "") && len(methods) > 1 {
		fail(2, "--mask and --labels can only be combined with a single method")
	}
	opt := core.DefaultNormOptions()
	opt.FWHM = a.optNum("--fwhm")
	opt.Statistic = a.str("--statistic", "mean")
	opt.Target = a.num("--target", 1)
	opt.PThreshold = a.num("--p-threshold", 0.01)
	opt.Exclude = a.str("--exclude", "hypo")
	opt.MaxIter = int(a.num("--max-iter", 1))
	opt.LiteralIPS = a.flags["--literal-ips"]
	maskText, labelOf := "", map[string]string{}
	switch {
	case labels != "":
		var numbers []int
		for _, v := range split([]string{labels}) {
			n, err := strconv.Atoi(v)
			check(err)
			numbers = append(numbers, n)
		}
		vol, err := core.LabelsMask(numbers)
		check(err)
		opt.Mask = core.MaskSpec{Vol: vol}
		maskText = "labels:" + strings.Join(split([]string{labels}), ",")
	case mask != "":
		opt.Mask = core.MaskSpec{Name: mask}
		if core.IsMask(mask) {
			labelOf["region"] = mask // name the files after the mask, not "region"
		}
	}
	if reference != "" {
		opt.Reference, err = core.LoadReference(reference)
		check(err)
	}
	out := need(a, "--output")
	check(os.MkdirAll(out, 0o755))
	table := "image\tmethod\tfactor\tn_voxels\texcluded_fraction\tmask\n"
	failed := 0
	for _, path := range a.pos {
		img, loadErr := core.LoadImage(path)
		for _, method := range methods {
			name := filepath.Base(path)
			if loadErr != nil {
				failed++
				fmt.Fprintf(os.Stderr, "error: %s [%s]: %v\n", name, method, loadErr)
				continue
			}
			res, err := core.Normalize(img, method, opt)
			if err != nil { // keep going with the other images
				failed++
				fmt.Fprintf(os.Stderr, "error: %s [%s]: %v\n", name, method, err)
				continue
			}
			for _, w := range res.Warnings {
				fmt.Fprintf(os.Stderr, "warning: %s [%s]: %s\n", name, method, w)
			}
			label := method
			if l, ok := labelOf[method]; ok {
				label = l
			}
			file := core.OutputName(path, label, "pet")
			if !a.flags["--factors-only"] {
				check(core.SaveNifti(filepath.Join(out, file), res.Image, false))
			}
			if a.flags["--save-masks"] && res.Mask != nil {
				check(core.SaveMask(filepath.Join(out, strings.Replace(file, ".nii.gz", "_mask.nii.gz", 1)),
					res.Mask, img.Dim, img.Aff, ""))
			}
			voxels, excluded := "", ""
			if res.Mask != nil {
				voxels = strconv.Itoa(res.NVoxels)
			}
			if res.Iterative {
				excluded = strconv.FormatFloat(res.ExcludedFraction, 'f', 4, 64)
			}
			text := res.MaskName
			if maskText != "" {
				text = maskText
			}
			factor := strconv.FormatFloat(res.Factor, 'g', 6, 64)
			table += strings.Join([]string{path, method, factor, voxels, excluded, text}, "\t") + "\n"
			fmt.Printf("%s\t%s\t%s\n", name, method, factor)
		}
	}
	check(os.WriteFile(filepath.Join(out, "factors.tsv"), []byte(table), 0o644))
	fmt.Fprintf(os.Stderr, "Factors written to %s\n", filepath.Join(out, "factors.tsv"))
	if failed > 0 {
		return 1
	}
	return 0
}

func zmap(rest []string) int {
	a, err := parse(rest, nil, nil)
	check(err)
	methods := split(a.values["--method"])
	unknownMethods(methods)
	ref, err := core.LoadReference(need(a, "--reference"))
	check(err)
	table := a.str("--covariates", "")
	for _, method := range methods { // stop early if the reference cannot serve a method
		_, err := ref.ControlStats(method)
		check(err)
		if table != "" {
			_, err := ref.ControlModel(method)
			check(err)
		}
	}
	out := need(a, "--output")
	check(os.MkdirAll(out, 0o755))
	failed := 0
	for _, path := range a.pos {
		name := filepath.Base(path)
		opt := core.DefaultZMapOptions()
		opt.PThreshold = a.num("--p-threshold", 0.01)
		opt.Exclude = a.str("--exclude", "hypo")
		opt.MaxIter = int(a.num("--max-iter", 1))
		if table != "" {
			rows, err := core.ReadTable(table, []string{path}, ref.Covariates())
			if err != nil {
				failed++
				fmt.Fprintf(os.Stderr, "error: %s: %v\n", name, err)
				continue
			}
			opt.Covariates = rows[0]
		}
		img, loadErr := core.LoadImage(path)
		for _, method := range methods {
			if loadErr != nil {
				failed++
				fmt.Fprintf(os.Stderr, "error: %s [%s]: %v\n", name, method, loadErr)
				continue
			}
			z, warnings, err := core.ZMap(img, method, ref, opt)
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "warning: %s [%s]: %s\n", name, method, w)
			}
			if err != nil {
				failed++
				fmt.Fprintf(os.Stderr, "error: %s [%s]: %v\n", name, method, err)
				continue
			}
			suffix := "zmap"
			if opt.Covariates != nil {
				suffix = "wmap"
			}
			file := core.OutputName(path, method, suffix)
			check(core.SaveNifti(filepath.Join(out, file), z, false))
			fmt.Println(file)
		}
	}
	if failed > 0 {
		return 1
	}
	return 0
}

func classify(rest []string) int {
	a, err := parse(rest, nil, nil)
	check(err)
	method := a.str("--method", "hn")
	ref, err := core.LoadReference(need(a, "--reference"))
	check(err)
	checks := core.DefaultChecks(ref, method)
	if given, ok := a.values["--check"]; ok {
		checks = split(given)
		if checks == nil || (len(checks) == 1 && checks[0] == "none") {
			checks = []string{}
		}
	}
	unknownMethods(append([]string{method}, checks...))
	if first, second, stem, found := core.SharedReportStem(a.pos); found {
		fail(2, "%s and %s would both be written as %s_report.html; rename one or run them separately", first, second, stem)
	}
	table := a.str("--covariates", "")
	for _, m := range append([]string{method}, checks...) { // stop early if the reference cannot serve a method
		_, err := ref.ControlStats(m)
		check(err)
		if table != "" {
			_, err := ref.ControlModel(m)
			check(err)
		}
	}
	lang := a.str("--lang", "en")
	if lang != "en" && lang != "zh" {
		fail(2, "option --lang: choose en or zh, not '%s'", lang)
	}
	out := need(a, "--output")
	check(os.MkdirAll(out, 0o755))
	var rows []string
	failed := 0
	for _, path := range a.pos {
		name := filepath.Base(path)
		opt := core.DefaultClassifyOptions()
		opt.Checks = checks
		opt.PThreshold = a.num("--p-threshold", 0.01)
		opt.Exclude = a.str("--exclude", "hypo")
		opt.MaxIter = int(a.num("--max-iter", 1))
		var res *core.Classification
		var err error
		if table != "" {
			var found []map[string]string
			if found, err = core.ReadTable(table, []string{path}, ref.Covariates()); err == nil {
				opt.Covariates = found[0]
			}
		}
		var img *core.Volume
		if err == nil {
			img, err = core.LoadImage(path)
		}
		if err == nil {
			res, err = core.Classify(img, name, method, ref, opt)
		}
		if err != nil { // keep going with the other images
			failed++
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", name, err)
			continue
		}
		for _, w := range res.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s: %s\n", name, w)
		}
		files, err := res.Save(out, core.ReportStem(path), lang)
		check(err)
		found := res.Conclusion
		cells := []string{path, found.Category, found.Pattern, found.Alternative, found.Confidence, found.Side}
		for _, id := range core.Patterns {
			for _, p := range res.Patterns {
				if p.ID == id {
					cells = append(cells, strconv.Itoa(core.Percent(p.Score)))
				}
			}
		}
		rows = append(rows, strings.Join(append(cells, filepath.Base(files.Report)), "\t"))
		dash := func(s string) string {
			if s == "" {
				return "-"
			}
			return s
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", name, found.Category, dash(found.Pattern), dash(found.Confidence))
	}
	head := append([]string{"image", "category", "pattern", "alternative", "confidence", "side"}, core.Patterns...)
	text := strings.Join(append(head, "report"), "\t") + "\n"
	for _, row := range rows {
		text += row + "\n"
	}
	check(os.WriteFile(filepath.Join(out, "classification.tsv"), []byte(text), 0o644))
	fmt.Fprintf(os.Stderr, "Reports and classification.tsv written to %s\n", out)
	if failed > 0 {
		return 1
	}
	return 0
}

func compare(ctx context.Context, rest []string) {
	a, err := parse(rest, []string{"--keep-images"}, []string{"--group1", "--group2", "--covariates"})
	check(err)
	group1, group2 := a.values["--group1"], a.values["--group2"]
	opt := core.DefaultCompareOptions()
	opt.Method = a.str("--method", "hn")
	unknownMethods([]string{opt.Method})
	images := append(append([]string(nil), group1...), group2...)
	if tables := a.values["--covariates"]; len(tables) > 0 {
		names := split([]string{a.str("--adjust-for", "age,sex")})
		rows := make([]map[string]string, len(images))
		for i, image := range images {
			var problems []string
			for _, table := range tables {
				found, err := core.ReadTable(table, []string{image}, names)
				if err == nil {
					rows[i] = found[0]
					break
				}
				problems = append(problems, err.Error())
			}
			if rows[i] == nil {
				fail(2, "%s", strings.Join(problems, "; "))
			}
		}
		opt.Covariates = core.CovariatesFromRows(rows, names)
	}
	if mask := a.str("--mask", ""); mask != "" {
		opt.Mask = core.MaskSpec{Name: mask}
	}
	if reference := a.str("--reference", ""); reference != "" {
		opt.Reference, err = core.LoadReference(reference)
		check(err)
	}
	opt.FWHM = a.optNum("--fwhm")
	opt.AnalysisMask = core.MaskSpec{Name: a.str("--analysis-mask", "gray-matter")}
	opt.Correction = a.str("--correction", "grf")
	opt.VoxelP, opt.ClusterP, opt.Q = a.num("--voxel-p", 0.001), a.num("--cluster-p", 0.05), a.num("--q", 0.05)
	opt.Name = a.str("--name", "group1_vs_group2")
	opt.KeepImages = a.flags["--keep-images"]
	opt.Progress = progress
	fmt.Fprintf(os.Stderr, "Comparing %d images (group 1) with %d (group 2), method %s ...\n", len(group1), len(group2), opt.Method)
	res, err := core.CompareGroups(ctx, group1, group2, need(a, "--output"), opt)
	check(err)
	fmt.Printf("T-map (group 1 minus group 2): %s\n", res.TMap)
	fmt.Printf("Degrees of freedom %g, smoothness %.1f x %.1f x %.1f mm\n", res.DOF, res.FWHM[0], res.FWHM[1], res.FWHM[2])
	if res.Corrected {
		how := fmt.Sprintf("FDR q < %g", opt.Q)
		if opt.Correction == "grf" {
			how = fmt.Sprintf("voxel p < %g, cluster p < %g (clusters of at least %d voxels)", opt.VoxelP, opt.ClusterP, res.MinClusterVoxels)
		}
		fmt.Printf("%d cluster(s) survive %s\n", res.NClusters, how)
		for _, role := range []string{"thresholded", "cluster_report"} {
			if path := res.File(role); path != "" {
				fmt.Printf("  %s\n", path)
			}
		}
	}
}
