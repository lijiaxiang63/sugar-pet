"""Reference-cluster normalisation: a reference region found from the data of a patient group."""

from __future__ import annotations

from pathlib import Path

import nibabel as nib
import numpy as np
from scipy import ndimage, stats

from .imageops import load_image, region_mean, smooth, valid_voxels
from .masks import same_grid
from .reference import Reference


def reference_cluster(
    patients: list[str | Path | nib.Nifti1Image],
    reference: Reference | str | Path,
    p_threshold: float = 0.001,
    min_cluster_ml: float = 1.0,
    progress=None,
) -> nib.Nifti1Image:
    """Find the voxels that are best preserved in a group of patients.

    Following Yakushev et al. (2009), all images are first scaled by their
    global mean (PS).  Regions spared by the disease then look *hyper*-
    metabolic, because the lesions pulled the global mean down.  A voxel-wise
    two-sample t-test (patients > controls) picks those regions out, and the
    resulting mask can be used as the reference region of every patient::

        cluster = reference_cluster(patient_files, "hc_reference")
        normalize(image, "region", mask=cluster)

    The mask depends on the group it came from.  Using it to compare that
    same group with controls is circular; derive it from other patients, or
    cross-validate.

    Parameters
    ----------
    patients
        MNI-space images of the patient group, all on the same grid.
    reference
        A :class:`Reference` (or its folder) built from healthy controls.
    p_threshold
        One-sided, uncorrected p-value of the t-test.
    min_cluster_ml
        Connected clusters smaller than this volume (mL) are dropped.

    Returns
    -------
    nibabel.Nifti1Image
        Binary mask on the grid of the patients.
    """
    if len(patients) < 2:
        raise ValueError("at least 2 patient images are needed")
    ref = reference if isinstance(reference, Reference) else Reference.load(reference)
    first = load_image(patients[0])
    ref = ref.on_grid(first)

    total = np.zeros(first.shape, dtype=np.float64)
    squares = np.zeros(first.shape, dtype=np.float64)
    covered = np.zeros(first.shape, dtype=np.int32)
    for i, item in enumerate(patients):
        img = first if i == 0 else load_image(item)
        if not same_grid(img.shape, img.affine, first.shape, first.affine):
            raise ValueError(f"patient {i + 1} is on a different grid than the first patient")
        raw = img.get_fdata(dtype=np.float32)
        valid = valid_voxels(raw)
        covered += valid
        # Same proportional scaling as the controls of the reference.
        scaled = smooth(raw, img.affine, ref.fwhm) / region_mean(raw, ref.mask & valid)
        total += scaled
        squares += np.square(scaled, dtype=np.float64)
        if progress:
            progress(i + 1, len(patients))

    n1, n2 = len(patients), ref.n
    mean1 = total / n1
    var1 = np.maximum(squares - n1 * mean1**2, 0.0) / (n1 - 1)
    pooled = ((n1 - 1) * var1 + (n2 - 1) * ref.ps_std.astype(np.float64) ** 2) / (n1 + n2 - 2)
    inside = ref.mask & (covered == n1) & (pooled > 0)
    t = np.zeros(first.shape, dtype=np.float64)
    t[inside] = (mean1[inside] - ref.template[inside]) / np.sqrt(pooled[inside] * (1.0 / n1 + 1.0 / n2))
    cluster = inside & (t > stats.t.ppf(1.0 - p_threshold, n1 + n2 - 2))

    # Drop small clusters (18-connectivity, as in SPM).
    labels, n_labels = ndimage.label(cluster, structure=ndimage.generate_binary_structure(3, 2))
    if n_labels:
        voxel_ml = abs(np.linalg.det(first.affine[:3, :3])) / 1000.0
        sizes = ndimage.sum_labels(cluster, labels, index=np.arange(1, n_labels + 1)) * voxel_ml
        cluster = np.isin(labels, np.flatnonzero(sizes >= min_cluster_ml) + 1)
    if not cluster.any():
        raise ValueError(
            "no voxels are relatively preserved at this threshold; try a larger p_threshold or a smaller min_cluster_ml"
        )

    out = nib.Nifti1Image(cluster.astype(np.uint8), first.affine)
    out.header.set_xyzt_units("mm")
    out.header["descrip"] = f"sugar-pet reference cluster p<{p_threshold:g} n={n1} vs {n2}"[:79]
    return out
