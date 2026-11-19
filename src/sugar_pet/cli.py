"""Command-line interface: ``sugar-pet <command> ...``."""

from __future__ import annotations

import argparse
import sys
import warnings
from pathlib import Path

import nibabel as nib
from nibabel.filebasedimages import ImageFileError

from . import __version__
from .covariates import read_table
from .masks import MASKS, atlas_labels, builtin_mask, labels_mask
from .normalize import METHODS, NEEDS_REFERENCE, normalize
from .reference import STATS_METHODS, Reference
from .study import init_study


def _strip_ext(name: str) -> str:
    for ext in (".nii.gz", ".nii", ".img", ".hdr"):
        if name.endswith(ext):
            return name[: -len(ext)]
    return name


def output_name(path: str | Path, method: str, suffix: str = "pet") -> str:
    """``sub-01_pet.nii.gz`` -> ``sub-01_desc-hn_pet.nii.gz`` (BIDS-style)."""
    stem = _strip_ext(Path(path).name)
    label = "desc-" + method.replace("-", "")
    if stem.endswith("_pet"):
        return f"{stem[:-4]}_{label}_{suffix}.nii.gz"
    return f"{stem}_{label}.nii.gz" if suffix == "pet" else f"{stem}_{label}_{suffix}.nii.gz"


def _split(items: list[str] | None) -> list[str]:
    """``["a,b", "c"]`` -> ``["a", "b", "c"]``."""
    return [part for item in items or [] for part in item.split(",") if part]


def _unknown_methods(methods: list[str]) -> str | None:
    unknown = [m for m in methods if m not in METHODS]
    if not unknown:
        return None
    text = f"unknown method(s): {', '.join(unknown)}. Run 'sugar-pet methods'."
    masks = [m for m in unknown if m in MASKS]
    if masks:
        text += f" '{masks[0]}' is a built-in mask: use -m region --mask {masks[0]}."
    return text


def _progress(done: int, total: int) -> None:
    if sys.stderr.isatty():
        print(f"\r  {done}/{total}", end="" if done < total else "\n", file=sys.stderr, flush=True)


def cmd_init(args) -> int:
    root = Path(args.folder)
    entries = init_study(root)
    print(f"Study folder: {root.resolve()}")
    for title, yours in (("For you to fill:", True), ("Written by the analyses:", False)):
        print(f"\n{title}")
        for entry, created in entries:
            if entry.yours == yours:
                name = entry.name + ("/" if entry.is_folder else "")
                print(f"  {name:18s}{'created' if created else 'already there'}")
                print(f"    {entry.text}")
    made = sum(created for _, created in entries)
    print(f"\n{made} created, {len(entries) - made} already there. Nothing that existed was changed.")
    print("Next: put your images in patients/ and controls/, and fill in participants.tsv.")
    return 0


def cmd_methods(_args) -> int:
    print("Normalisation methods (-m):")
    for name, text in METHODS.items():
        print(f"  {name:15s} {text}")
    print("\nBuilt-in masks (--mask):")
    for name, text in MASKS.items():
        print(f"  {name:15s} {text}")
    return 0


def cmd_build_reference(args) -> int:
    print(f"Building reference from {len(args.controls)} controls ...", file=sys.stderr)
    stats_for = _split(args.stats_for)
    if stats_for == ["all"]:
        stats_for = list(STATS_METHODS)
    covariates = None
    if args.covariates:
        names = _split([args.adjust_for])
        rows = read_table(args.covariates, args.controls, names)
        covariates = {name: [row[name] for row in rows] for name in names}
    ref = Reference.build(
        args.controls, mask=args.mask, fwhm=args.fwhm, stats_for=stats_for, covariates=covariates, progress=_progress
    )
    for term in ref.meta.get("covariates", []):
        detail = f"{term['min']:g} to {term['max']:g}" if term["type"] == "numeric" else " / ".join(term["levels"])
        print(f"Adjusting for {term['name']} ({detail})", file=sys.stderr)
    ref.save(args.output)
    print(f"Reference saved to {args.output}")
    return 0


def cmd_normalize(args) -> int:
    methods = _split(args.method)
    if methods == ["all"]:
        methods = [m for m in METHODS if m != "region" and (args.reference or m not in NEEDS_REFERENCE)]
    problem = _unknown_methods(methods)
    if args.mask and args.labels:
        problem = "--mask and --labels cannot be combined"
    elif (args.mask or args.labels) and len(methods) > 1:
        problem = "--mask and --labels can only be combined with a single method"
    if problem:
        print(f"error: {problem}", file=sys.stderr)
        return 2
    mask, mask_text, label_of = args.mask, None, {}
    if args.labels:
        mask = labels_mask([int(v) for v in _split([args.labels])])
        mask_text = "labels:" + ",".join(_split([args.labels]))
    elif args.mask in MASKS:
        label_of["region"] = args.mask  # name the files after the mask, not "region"
    reference = Reference.load(args.reference) if args.reference else None
    out = Path(args.output)
    out.mkdir(parents=True, exist_ok=True)

    rows, failed = [], 0
    for path in args.images:
        for method in methods:
            try:
                with warnings.catch_warnings(record=True) as caught:
                    warnings.simplefilter("always")
                    res = normalize(
                        path,
                        method,
                        reference=reference,
                        mask=mask,
                        fwhm=args.fwhm,
                        statistic=args.statistic,
                        target=args.target,
                        p_threshold=args.p_threshold,
                        exclude=args.exclude,
                        max_iter=args.max_iter,
                        literal_ips=args.literal_ips,
                    )
                for w in caught:
                    print(f"warning: {Path(path).name} [{method}]: {w.message}", file=sys.stderr)
            except (ValueError, OSError, ImageFileError) as err:  # keep going with the other images
                failed += 1
                print(f"error: {Path(path).name} [{method}]: {err}", file=sys.stderr)
                continue
            name = output_name(path, label_of.get(method, method))
            if not args.factors_only:
                res.save(out / name)
            if args.save_masks and res.mask is not None:
                nib.save(res.mask_image(), out / name.replace(".nii.gz", "_mask.nii.gz"))
            excluded = res.info.get("excluded_fraction")
            rows.append(
                (
                    str(path),
                    method,
                    f"{res.factor:.6g}",
                    str(res.info.get("n_voxels", "")),
                    "" if excluded is None else f"{excluded:.4f}",
                    mask_text or res.info.get("mask", ""),
                )
            )
            print(f"{Path(path).name}\t{method}\t{res.factor:.6g}")

    table = out / "factors.tsv"
    with open(table, "w") as fh:
        fh.write("image\tmethod\tfactor\tn_voxels\texcluded_fraction\tmask\n")
        fh.writelines("\t".join(r) + "\n" for r in rows)
    print(f"Factors written to {table}", file=sys.stderr)
    return 1 if failed else 0


def cmd_zmap(args) -> int:
    from .zscore import zmap

    methods = _split(args.method)
    problem = _unknown_methods(methods)
    if problem:
        print(f"error: {problem}", file=sys.stderr)
        return 2
    reference = Reference.load(args.reference)
    for method in methods:  # stop early if the reference cannot serve a method
        reference.control_stats(method)
        if args.covariates:
            reference.control_model(method)
    out = Path(args.output)
    out.mkdir(parents=True, exist_ok=True)
    failed = 0
    for path in args.images:
        covariates = None
        if args.covariates:
            try:
                covariates = read_table(args.covariates, [path], reference.covariates)[0]
            except ValueError as err:
                failed += 1
                print(f"error: {Path(path).name}: {err}", file=sys.stderr)
                continue
        for method in methods:
            try:
                with warnings.catch_warnings(record=True) as caught:
                    warnings.simplefilter("always")
                    z = zmap(
                        path,
                        method,
                        reference=reference,
                        covariates=covariates,
                        p_threshold=args.p_threshold,
                        exclude=args.exclude,
                        max_iter=args.max_iter,
                    )
                for w in caught:
                    print(f"warning: {Path(path).name} [{method}]: {w.message}", file=sys.stderr)
            except (ValueError, OSError, ImageFileError) as err:  # keep going with the other images
                failed += 1
                print(f"error: {Path(path).name} [{method}]: {err}", file=sys.stderr)
                continue
            name = output_name(path, method, "wmap" if covariates else "zmap")
            nib.save(z, out / name)
            print(name)
    return 1 if failed else 0


def report_stem(path: str | Path) -> str:
    """``sub-01_pet.nii.gz`` -> ``sub-01``: what the files about one image are named after."""
    stem = _strip_ext(Path(path).name)
    return stem[:-4] if stem.endswith("_pet") and len(stem) > 4 else stem


def shared_report_stem(images) -> str | None:
    """What is wrong if two of the images would get the same report: the second would replace the first.

    ``a/pet.nii.gz`` and ``b/pet.nii.gz`` share a name, and so do
    ``sub-01.nii.gz`` and ``sub-01_pet.nii.gz``.  Names that differ only in
    case count as one, as they are one file on most Macs and on Windows.
    """
    first: dict[str, str] = {}
    for path in images:
        stem = report_stem(path)
        if stem.casefold() in first:
            return (
                f"{first[stem.casefold()]} and {path} would both be written as {stem}_report.html; "
                "rename one or run them separately"
            )
        first[stem.casefold()] = str(path)
    return None


def cmd_classify(args) -> int:
    from .classify import PATTERNS, classify, default_checks
    from .report import percent

    reference = Reference.load(args.reference)
    checks = default_checks(reference, args.method) if args.check is None else _split(args.check)
    if checks == ["none"]:
        checks = []
    problem = _unknown_methods([args.method, *checks]) or shared_report_stem(args.images)
    if problem:
        print(f"error: {problem}", file=sys.stderr)
        return 2
    for method in (args.method, *checks):  # stop early if the reference cannot serve a method
        reference.control_stats(method)
        if args.covariates:
            reference.control_model(method)
    out = Path(args.output)
    out.mkdir(parents=True, exist_ok=True)
    rows, failed = [], 0
    for path in args.images:
        name = Path(path).name
        try:
            covariates = read_table(args.covariates, [path], reference.covariates)[0] if args.covariates else None
            res = classify(
                path,
                reference=reference,
                method=args.method,
                checks=checks,
                covariates=covariates,
                p_threshold=args.p_threshold,
                exclude=args.exclude,
                max_iter=args.max_iter,
            )
        except (ValueError, OSError, ImageFileError) as err:  # keep going with the other images
            failed += 1
            print(f"error: {name}: {err}", file=sys.stderr)
            continue
        for note in res.result["warnings"]:
            print(f"warning: {name}: {note}", file=sys.stderr)
        files = res.save(out, report_stem(path), args.lang)
        found = res.conclusion
        scores = {p["id"]: percent(p["score"]) for p in res.result["patterns"]}
        cells = [found[key] or "" for key in ("category", "pattern", "alternative", "confidence", "side")]
        rows.append([str(path), *cells, *(str(scores[name]) for name in PATTERNS), files["report"].name])
        print(f"{name}\t{found['category']}\t{found['pattern'] or '-'}\t{found['confidence'] or '-'}")

    table = out / "classification.tsv"
    with open(table, "w") as fh:
        fh.write("\t".join(["image", "category", "pattern", "alternative", "confidence", "side", *PATTERNS, "report"]))
        fh.write("\n")
        fh.writelines("\t".join(r) + "\n" for r in rows)
    print(f"Reports and classification.tsv written to {out}", file=sys.stderr)
    return 1 if failed else 0


def _lookup(tables: list[str], image: str, names: list[str]) -> dict[str, str]:
    """Covariates of one image, from whichever table lists it."""
    problems = []
    for table in tables:
        try:
            return read_table(table, [image], names)[0]
        except ValueError as err:
            problems.append(str(err))
    raise ValueError("; ".join(problems))


def cmd_compare(args) -> int:
    from .compare import compare_groups

    problem = _unknown_methods([args.method])
    if problem:
        print(f"error: {problem}", file=sys.stderr)
        return 2
    images = [*args.group1, *args.group2]
    covariates = None
    if args.covariates:
        names = _split([args.adjust_for])
        rows = [_lookup(args.covariates, image, names) for image in images]
        covariates = {name: [row[name] for row in rows] for name in names}
    options = {"mask": args.mask} if args.mask else {}
    print(
        f"Comparing {len(args.group1)} images (group 1) with {len(args.group2)} (group 2), method {args.method} ...",
        file=sys.stderr,
    )
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")  # per-image notes; the factors are in the table
        res = compare_groups(
            args.group1,
            args.group2,
            args.output,
            args.method,
            reference=args.reference,
            covariates=covariates,
            fwhm=args.fwhm,
            analysis_mask=args.analysis_mask,
            correction=args.correction,
            voxel_p=args.voxel_p,
            cluster_p=args.cluster_p,
            q=args.q,
            name=args.name,
            keep_images=args.keep_images,
            progress=_progress,
            **options,
        )
    print(f"T-map (group 1 minus group 2): {res.t_map}")
    print(f"Degrees of freedom {res.dof:g}, smoothness {' x '.join(f'{v:.1f}' for v in res.fwhm)} mm")
    if res.n_clusters is not None:
        if "min_cluster_voxels" in res.thresholds:
            limit = res.thresholds
            how = f"voxel p < {limit['voxel_p']:g}, cluster p < {limit['cluster_p']:g}"
            how += f" (clusters of at least {limit['min_cluster_voxels']} voxels)"
        else:
            how = f"FDR q < {res.thresholds['q']:g}"
        print(f"{res.n_clusters} cluster(s) survive {how}")
        for key in ("thresholded", "cluster_report"):
            if key in res.files:
                print(f"  {res.files[key]}")
    return 0


def cmd_reference_cluster(args) -> int:
    from .cluster import reference_cluster

    print(f"Comparing {len(args.patients)} patients with the controls ...", file=sys.stderr)
    cluster = reference_cluster(
        args.patients,
        args.reference,
        p_threshold=args.p_threshold,
        min_cluster_ml=args.min_cluster,
        progress=_progress,
    )
    out = Path(args.output)
    out.parent.mkdir(parents=True, exist_ok=True)
    nib.save(cluster, out)
    voxel_ml = abs(float(nib.affines.voxel_sizes(cluster.affine).prod())) / 1000.0
    print(f"Reference cluster ({cluster.get_fdata().sum() * voxel_ml:.0f} mL) written to {out}")
    print(f"Use it with: sugar-pet normalize ... -m region --mask {out}")
    return 0


def cmd_masks(args) -> int:
    out = Path(args.output)
    out.mkdir(parents=True, exist_ok=True)
    for name in MASKS:
        nib.save(builtin_mask(name), out / f"{name}.nii.gz")
    print(f"{len(MASKS)} masks written to {out}")
    return 0


def cmd_spatial_norm(args) -> int:
    from .spatial import spatial_normalize

    out = Path(args.output)
    targets = []
    for path in args.images:
        stem = _strip_ext(Path(path).name)
        name = f"{stem[:-4]}_space-MNI_pet" if stem.endswith("_pet") else f"{stem}_space-MNI"
        targets.append(out / f"{name}.nii.gz")
    written = spatial_normalize(
        args.images, targets, spm_path=args.spm_path, template=args.template, matlab=args.matlab, n_jobs=args.jobs
    )
    print(f"{len(written)} of {len(targets)} images written to {out}")
    return 0 if len(written) == len(targets) else 1


def cmd_labels(_args) -> int:
    for index, name in sorted(atlas_labels().items()):
        print(f"{index:4d}  {name}")
    return 0


def _add_iterative_options(p: argparse.ArgumentParser) -> None:
    p.add_argument("--p-threshold", type=float, default=0.01, help="ips/ihn: p-value for excluding voxels [0.01]")
    p.add_argument(
        "--exclude",
        choices=("hypo", "both"),
        default="hypo",
        help="ips/ihn: exclude hypometabolic voxels only, or hyper- as well [hypo]",
    )
    p.add_argument("--max-iter", type=int, default=1, help="ips/ihn: number of re-estimations [1]")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="sugar-pet",
        description="Intensity normalisation of brain FDG-PET images in MNI space.",
    )
    parser.add_argument("--version", action="version", version=f"sugar-pet {__version__}")
    sub = parser.add_subparsers(dest="command", required=True)

    p = sub.add_parser("init", help="create the recommended folders of a study and an empty participants.tsv")
    p.add_argument("folder", nargs="?", default="study", help="the study folder to create or complete [study]")
    p.set_defaults(func=cmd_init)

    p = sub.add_parser("methods", help="list the normalisation methods and built-in masks")
    p.set_defaults(func=cmd_methods)

    p = sub.add_parser("normalize", help="normalise one or more images")
    p.add_argument("images", nargs="+", help="FDG-PET images in MNI space")
    p.add_argument(
        "-m",
        "--method",
        action="append",
        required=True,
        help="method name; repeat or separate by commas for several, or 'all'",
    )
    p.add_argument("-o", "--output", required=True, help="output folder")
    p.add_argument("-r", "--reference", help="reference folder (needed for hn, ips, ihn)")
    p.add_argument("--mask", help="built-in mask name or NIfTI file replacing the method's default region")
    p.add_argument("--labels", help="atlas label numbers to use as the region, e.g. 34,85 (see 'sugar-pet labels')")
    p.add_argument(
        "--statistic",
        choices=("mean", "median"),
        default="mean",
        help="reference regions and ps: mean or median of the region [mean]",
    )
    p.add_argument("--target", type=float, default=1.0, help="value of the reference in the output images [1]")
    p.add_argument("--fwhm", type=float, help="smoothing in mm before estimating the factor (0 = none)")
    _add_iterative_options(p)
    p.add_argument("--literal-ips", action="store_true", help="ips: plain mean of the remaining voxels")
    p.add_argument("--factors-only", action="store_true", help="only write factors.tsv, no images")
    p.add_argument("--save-masks", action="store_true", help="also save the mask each factor came from")
    p.set_defaults(func=cmd_normalize)

    p = sub.add_parser("build-reference", help="build a healthy-control reference for hn, ips and ihn")
    p.add_argument("controls", nargs="+", help="healthy-control FDG-PET images in MNI space")
    p.add_argument("-o", "--output", required=True, help="folder to write the reference to")
    p.add_argument("--mask", default="gray-matter", help="built-in mask name or NIfTI file [gray-matter]")
    p.add_argument("--fwhm", type=float, default=8.0, help="smoothing in mm; 0 if already smoothed [8]")
    p.add_argument(
        "--stats-for",
        action="append",
        help=f"also store control statistics for z-maps with these methods ({', '.join(STATS_METHODS)}, or 'all')",
    )
    p.add_argument(
        "--covariates",
        help="TSV/CSV table with the covariates of the controls (e.g. BIDS participants.tsv), for W-scores",
    )
    p.add_argument("--adjust-for", default="age,sex", help="columns of that table to adjust for [age,sex]")
    p.set_defaults(func=cmd_build_reference)

    p = sub.add_parser("zmap", help="z-score (or covariate-adjusted W-score) maps against the controls")
    p.add_argument("images", nargs="+", help="FDG-PET images in MNI space")
    p.add_argument("-m", "--method", action="append", required=True, help="normalisation method(s) to use")
    p.add_argument("-r", "--reference", required=True, help="reference folder")
    p.add_argument("-o", "--output", required=True, help="output folder")
    p.add_argument(
        "--covariates",
        help="TSV/CSV table with the covariates of these images: writes W-score maps adjusted for them",
    )
    _add_iterative_options(p)
    p.set_defaults(func=cmd_zmap)

    p = sub.add_parser(
        "classify", help="match a patient against the patterns of parkinsonian syndromes; writes a report"
    )
    p.add_argument("images", nargs="+", help="FDG-PET images of patients, in MNI space")
    p.add_argument("-r", "--reference", required=True, help="reference folder built from healthy controls")
    p.add_argument("-o", "--output", required=True, help="output folder")
    p.add_argument("-m", "--method", default="hn", help="normalisation the classification rests on [hn]")
    p.add_argument(
        "--check",
        action="append",
        help="other normalisations the findings are checked with, or 'none' [ps, and white-matter if the reference has it]",
    )
    p.add_argument(
        "--covariates", help="TSV/CSV table with the covariates of these images: W-scores instead of z-scores"
    )
    p.add_argument("--lang", choices=("en", "zh"), default="en", help="language of the report [en]")
    _add_iterative_options(p)
    p.set_defaults(func=cmd_classify)

    p = sub.add_parser("compare", help="voxel-wise comparison of two groups (needs the 'stats' extra)")
    p.add_argument("--group1", nargs="+", required=True, help="images of group 1, e.g. the patients")
    p.add_argument("--group2", nargs="+", required=True, help="images of group 2, e.g. the controls")
    p.add_argument("-m", "--method", default="hn", help="normalisation method for both groups [hn]")
    p.add_argument("-r", "--reference", help="reference folder (needed for hn, ips, ihn)")
    p.add_argument("-o", "--output", required=True, help="output folder")
    p.add_argument("--name", default="group1_vs_group2", help="prefix of the result files [group1_vs_group2]")
    p.add_argument("--covariates", nargs="+", help="TSV/CSV table(s) listing the covariates of all images")
    p.add_argument("--adjust-for", default="age,sex", help="columns to put in the model [age,sex]")
    p.add_argument("--mask", help="built-in mask name or NIfTI file replacing the method's default region")
    p.add_argument("--fwhm", type=float, help="smoothing in mm before the test [that of the reference, else 8]")
    p.add_argument("--analysis-mask", default="gray-matter", help="voxels to test: mask name or file [gray-matter]")
    p.add_argument("--correction", choices=("grf", "fdr", "none"), default="grf", help="multiple-comparison correction")
    p.add_argument("--voxel-p", type=float, default=0.001, help="grf: voxel-level p, two-tailed [0.001]")
    p.add_argument("--cluster-p", type=float, default=0.05, help="grf: cluster-level p [0.05]")
    p.add_argument("--q", type=float, default=0.05, help="fdr: q value [0.05]")
    p.add_argument("--keep-images", action="store_true", help="keep the normalised, smoothed images")
    p.set_defaults(func=cmd_compare)

    p = sub.add_parser("reference-cluster", help="find a reference region from a patient group (Yakushev 2009)")
    p.add_argument("patients", nargs="+", help="FDG-PET images of the patient group, in MNI space")
    p.add_argument("-r", "--reference", required=True, help="reference folder built from healthy controls")
    p.add_argument("-o", "--output", required=True, help="mask file to write, e.g. cluster.nii.gz")
    p.add_argument("--p-threshold", type=float, default=0.001, help="one-sided uncorrected p-value [0.001]")
    p.add_argument("--min-cluster", type=float, default=1.0, help="smallest cluster to keep, in mL [1]")
    p.set_defaults(func=cmd_reference_cluster)

    p = sub.add_parser("masks", help="export the built-in masks as NIfTI files")
    p.add_argument("-o", "--output", required=True, help="output folder")
    p.set_defaults(func=cmd_masks)

    p = sub.add_parser("labels", help="list the label numbers of the built-in atlas (for --labels)")
    p.set_defaults(func=cmd_labels)

    p = sub.add_parser("spatial-norm", help="(optional) warp native PET to MNI space with SPM")
    p.add_argument("images", nargs="+", help="native-space PET images")
    p.add_argument("-o", "--output", required=True, help="output folder")
    p.add_argument("--spm-path", required=True, help="SPM12/SPM25 folder")
    p.add_argument("--template", help="PET template in MNI space [SPM's PET.nii]")
    p.add_argument("--matlab", help="MATLAB executable [searched automatically]")
    p.add_argument("--jobs", type=int, default=1, help="parallel MATLAB processes [1]")
    p.set_defaults(func=cmd_spatial_norm)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        return args.func(args)
    except (ValueError, FileNotFoundError, RuntimeError) as err:
        print(f"error: {err}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
