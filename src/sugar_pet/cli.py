"""Command-line interface: ``sugar-pet <command> ...``."""

from __future__ import annotations

import argparse
import sys
import warnings
from pathlib import Path

import nibabel as nib
from nibabel.filebasedimages import ImageFileError

from . import __version__
from .masks import MASKS, builtin_mask
from .normalize import METHODS, NEEDS_REFERENCE, normalize
from .reference import Reference


def _strip_ext(name: str) -> str:
    for ext in (".nii.gz", ".nii", ".img", ".hdr"):
        if name.endswith(ext):
            return name[: -len(ext)]
    return name


def output_name(path: str | Path, method: str) -> str:
    """``sub-01_pet.nii.gz`` -> ``sub-01_desc-hn_pet.nii.gz`` (BIDS-style)."""
    stem = _strip_ext(Path(path).name)
    label = "desc-" + method.replace("-", "")
    if stem.endswith("_pet"):
        return f"{stem[:-4]}_{label}_pet.nii.gz"
    return f"{stem}_{label}.nii.gz"


def _progress(done: int, total: int) -> None:
    print(f"\r  {done}/{total}", end="" if done < total else "\n", file=sys.stderr, flush=True)


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
    ref = Reference.build(args.controls, mask=args.mask, fwhm=args.fwhm, progress=_progress)
    ref.save(args.output)
    print(f"Reference saved to {args.output}")
    return 0


def cmd_normalize(args) -> int:
    methods = [m for item in args.method for m in item.split(",") if m]
    if methods == ["all"]:
        methods = [m for m in METHODS if m != "region" and (args.reference or m not in NEEDS_REFERENCE)]
    unknown = [m for m in methods if m not in METHODS]
    if unknown:
        print(f"error: unknown method(s): {', '.join(unknown)}. Run 'sugar-pet methods'.", file=sys.stderr)
        return 2
    if args.mask and len(methods) > 1:
        print("error: --mask can only be combined with a single method", file=sys.stderr)
        return 2
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
                        mask=args.mask,
                        fwhm=args.fwhm,
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
            if not args.factors_only:
                res.save(out / output_name(path, method))
            if args.save_masks and res.mask is not None:
                nib.save(res.mask_image(), out / output_name(path, method).replace(".nii.gz", "_mask.nii.gz"))
            excluded = res.info.get("excluded_fraction")
            rows.append(
                (
                    str(path),
                    method,
                    f"{res.factor:.6g}",
                    str(res.info.get("n_voxels", "")),
                    "" if excluded is None else f"{excluded:.4f}",
                )
            )
            print(f"{Path(path).name}\t{method}\t{res.factor:.6g}")

    table = out / "factors.tsv"
    with open(table, "w") as fh:
        fh.write("image\tmethod\tfactor\tn_voxels\texcluded_fraction\n")
        fh.writelines("\t".join(r) + "\n" for r in rows)
    print(f"Factors written to {table}", file=sys.stderr)
    return 1 if failed else 0


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


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="sugar-pet",
        description="Intensity normalisation of brain FDG-PET images in MNI space.",
    )
    parser.add_argument("--version", action="version", version=f"sugar-pet {__version__}")
    sub = parser.add_subparsers(dest="command", required=True)

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
    p.add_argument("--fwhm", type=float, help="smoothing in mm before estimating the factor (0 = none)")
    p.add_argument("--p-threshold", type=float, default=0.01, help="ips/ihn: p-value for excluding voxels [0.01]")
    p.add_argument(
        "--exclude",
        choices=("hypo", "both"),
        default="hypo",
        help="ips/ihn: exclude hypometabolic voxels only, or hyper- as well [hypo]",
    )
    p.add_argument("--max-iter", type=int, default=1, help="ips/ihn: number of re-estimations [1]")
    p.add_argument("--literal-ips", action="store_true", help="ips: plain mean of the remaining voxels")
    p.add_argument("--factors-only", action="store_true", help="only write factors.tsv, no images")
    p.add_argument("--save-masks", action="store_true", help="also save the mask each factor came from")
    p.set_defaults(func=cmd_normalize)

    p = sub.add_parser("build-reference", help="build a healthy-control reference for hn, ips and ihn")
    p.add_argument("controls", nargs="+", help="healthy-control FDG-PET images in MNI space")
    p.add_argument("-o", "--output", required=True, help="folder to write the reference to")
    p.add_argument("--mask", default="gray-matter", help="built-in mask name or NIfTI file [gray-matter]")
    p.add_argument("--fwhm", type=float, default=8.0, help="smoothing in mm; 0 if already smoothed [8]")
    p.set_defaults(func=cmd_build_reference)

    p = sub.add_parser("masks", help="export the built-in masks as NIfTI files")
    p.add_argument("-o", "--output", required=True, help="output folder")
    p.set_defaults(func=cmd_masks)

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
