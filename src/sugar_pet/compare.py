"""Voxel-wise comparison of two groups after intensity normalisation.

The statistics themselves (GLM, cluster-level correction, cluster tables)
are done by GrouVox; this module prepares its input so that both groups
are normalised and smoothed the same way and the covariates end up in the
right order.
"""

from __future__ import annotations

import tempfile
from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from pathlib import Path

import nibabel as nib
import numpy as np

from .covariates import build_design
from .imageops import smooth, valid_voxels
from .masks import get_mask, same_grid
from .normalize import normalize
from .reference import Reference

_INSTALL_HINT = (
    "group statistics need the GrouVox package: "
    'pip install "sugar-pet[stats] @ git+https://github.com/lijiaxiang63/sugar-pet"'
)


@dataclass
class GroupComparison:
    """Outcome of :func:`compare_groups`.

    Attributes
    ----------
    t_map
        Path of the T-map.  Positive values: group 1 higher than group 2.
    dof, fwhm
        Degrees of freedom and estimated smoothness (mm) of the residuals.
    n_clusters, cluster_table
        Clusters that survive the correction, with atlas labels.
    thresholds
        What the correction used, e.g. the smallest cluster size kept.
    files
        Every file that was written, by role.
    factors
        Normalisation factor of each image, group 1 first.
    """

    t_map: Path
    dof: float
    fwhm: tuple[float, float, float]
    n_clusters: int | None
    cluster_table: list[dict] = field(default_factory=list)
    thresholds: dict = field(default_factory=dict)
    files: dict[str, Path] = field(default_factory=dict)
    factors: list[float] = field(default_factory=list)


def compare_groups(
    group1: Sequence[str | Path],
    group2: Sequence[str | Path],
    output: str | Path,
    method: str = "hn",
    *,
    reference: Reference | str | Path | None = None,
    covariates: Mapping[str, Sequence] | None = None,
    fwhm: float | None = None,
    analysis_mask: str | Path | nib.Nifti1Image = "gray-matter",
    correction: str = "grf",
    voxel_p: float = 0.001,
    cluster_p: float = 0.05,
    q: float = 0.05,
    name: str = "group1_vs_group2",
    keep_images: bool = False,
    progress=None,
    **normalize_options,
) -> GroupComparison:
    """Compare two groups voxel by voxel (two-sample t-test, group 1 minus group 2).

    Every image is intensity-normalised with ``method``, smoothed, and passed
    to GrouVox together with the covariates.

    Parameters
    ----------
    group1, group2
        MNI-space images, all on the same grid.  With patients as group 1
        and controls as group 2, negative T means hypometabolism.
    output
        Folder for the results.
    method, reference, normalize_options
        As for :func:`sugar_pet.normalize`; the same for both groups.
    covariates
        ``{name: values}`` with one value per image, group 1 first, then
        group 2 (e.g. age and sex).  Numeric, or with exactly two levels.
    fwhm
        Smoothing in mm applied to the normalised images.  Defaults to the
        smoothing of the reference, or 8 mm without one.  Use 0 if the
        images are already smoothed.
    analysis_mask
        Voxels to test: a built-in mask name or a file.  Voxels that any
        image does not cover are left out.
    correction
        ``"grf"`` (cluster-level, Gaussian random field theory, using
        ``voxel_p`` and ``cluster_p``), ``"fdr"`` (voxel-level, using ``q``)
        or ``"none"``.  Both are two-tailed.
    keep_images
        Keep the normalised, smoothed images that went into the test.
    """
    try:
        import grouvox
        import pandas as pd
    except ImportError as err:
        raise RuntimeError(_INSTALL_HINT) from err

    if correction not in ("grf", "fdr", "none"):
        raise ValueError("correction must be 'grf', 'fdr' or 'none'")
    group1, group2 = list(group1), list(group2)
    if min(len(group1), len(group2)) < 2:
        raise ValueError("each group needs at least 2 images")
    images = group1 + group2
    table = None
    if covariates:
        design, terms = build_design(covariates)
        if len(design) != len(images):
            raise ValueError(f"{len(design)} covariate values for {len(images)} images (group 1 first, then group 2)")
        table = pd.DataFrame(design[:, 1:], columns=[t["name"] for t in terms])
    ref = None
    if reference is not None:
        ref = reference if isinstance(reference, Reference) else Reference.load(reference)
    if fwhm is None:
        fwhm = ref.fwhm if ref is not None else 8.0

    out = Path(output)
    out.mkdir(parents=True, exist_ok=True)
    prefix = out / name
    holder = None
    if keep_images:
        image_dir = out / f"{name}_images"
        image_dir.mkdir(exist_ok=True)
    else:
        holder = tempfile.TemporaryDirectory(dir=out)
        image_dir = Path(holder.name)

    try:
        paths, factors, first, covered = [], [], None, None
        for i, item in enumerate(images):
            res = normalize(item, method, reference=ref, **normalize_options)
            img = res.image
            if first is None:
                first, covered = img, np.ones(img.shape, bool)
            elif not same_grid(img.shape, img.affine, first.shape, first.affine):
                raise ValueError(f"image {i + 1} is on a different grid than the first image")
            data = img.get_fdata(dtype=np.float32)
            covered &= valid_voxels(data)
            group = "group1" if i < len(group1) else "group2"
            paths.append(image_dir / f"{group}_{i:04d}.nii.gz")
            nib.save(nib.Nifti1Image(smooth(data, img.affine, fwhm), img.affine), paths[-1])
            factors.append(res.factor)
            if progress:
                progress(i + 1, len(images))

        mask = get_mask(analysis_mask, first) & covered
        if mask.sum() < 1000:
            raise ValueError("the analysis mask barely overlaps the images; are they in MNI space?")
        files = {"mask": Path(f"{prefix}_mask.nii.gz"), "factors": Path(f"{prefix}_factors.tsv")}
        nib.save(nib.Nifti1Image(mask.astype(np.uint8), first.affine), files["mask"])
        with open(files["factors"], "w") as fh:
            fh.write("image\tgroup\tmethod\tfactor\n")
            for i, (item, factor) in enumerate(zip(images, factors)):
                label = item if isinstance(item, (str, Path)) else f"image {i + 1}"
                fh.write(f"{label}\t{1 if i < len(group1) else 2}\t{method}\t{factor:.6g}\n")

        n1 = len(group1)
        test = grouvox.two_sample_ttest(paths[:n1], paths[n1:], prefix, mask=files["mask"], covariates=table)
    finally:
        if holder is not None:
            holder.cleanup()

    t_map = Path(f"{prefix}_T.nii.gz")
    files.update(t_map=t_map, beta=Path(f"{prefix}_beta.nii.gz"), effect_size=Path(f"{prefix}_cohen_f2.nii.gz"))
    if keep_images:
        files["images"] = image_dir
    n_clusters, clusters, thresholds = None, [], {}
    if correction != "none":
        # GrouVox writes the report only when clusters survive; remove one left by an earlier run.
        report = out / f"ClusterReport_{'FDR_' if correction == 'fdr' else ''}{name}_T.csv"
        report.unlink(missing_ok=True)
        if correction == "grf":
            result = grouvox.grf_correction(t_map, files["mask"], voxel_p=voxel_p, cluster_p=cluster_p)
            files["thresholded"] = out / f"ClusterThresholded_{t_map.name}"
            thresholds = {
                "voxel_p": voxel_p,
                "cluster_p": cluster_p,
                "z": float(result.z_threshold),
                "min_cluster_voxels": int(result.cluster_size_threshold),
            }
        else:
            result = grouvox.fdr_correction(t_map, files["mask"], q=q)
            files["thresholded"] = out / f"FDR_Thresholded_{t_map.name}"
            thresholds = {"q": q, "p": float(result.p_threshold)}
        n_clusters, clusters = result.n_clusters, result.cluster_table
        if report.exists():
            files["cluster_report"] = report
    return GroupComparison(
        t_map=t_map,
        dof=float(test.dof),
        fwhm=tuple(float(v) for v in test.fwhm),
        n_clusters=n_clusters,
        cluster_table=clusters,
        thresholds=thresholds,
        files=files,
        factors=[float(f) for f in factors],
    )
