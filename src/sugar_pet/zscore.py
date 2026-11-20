"""Z-score maps: how far a normalised image is from the healthy controls."""

from __future__ import annotations

import warnings
from collections.abc import Mapping
from pathlib import Path

import nibabel as nib
import numpy as np

from .covariates import design_row
from .imageops import smooth, valid_voxels
from .masks import get_mask
from .normalize import normalize
from .reference import Reference


def zmap(
    image: str | Path | nib.Nifti1Image,
    method: str = "hn",
    *,
    reference: Reference | str | Path,
    covariates: Mapping | None = None,
    p_threshold: float = 0.01,
    exclude: str = "hypo",
    max_iter: int = 1,
) -> nib.Nifti1Image:
    """Z-score map of one image against the controls of a reference.

    The image is intensity-normalised with ``method``, smoothed like the
    controls were, and compared voxel by voxel with the controls normalised
    the same way: ``z = (image - control mean) / control SD``.  Negative
    values mean lower uptake than the controls.

    ``ps``, ``ips``, ``hn`` and ``ihn`` work with any reference.  For the
    reference-region methods and ``spm-global`` the reference must have been
    built with ``stats_for`` naming the method.  The factor of the image is
    always estimated the way those of the controls were: from the default
    region of the method, with the statistic and the smoothing the reference
    was built with (the mean and the default smoothing, unless
    ``Reference.build`` was told otherwise).

    With ``covariates`` (for example ``{"age": 63, "sex": "F"}``) the result
    is a W-score map instead: the image is compared with what the controls
    predict for a person of that age and sex,
    ``w = (image - predicted) / residual SD``.  The reference must have been
    built with the same covariates.

    Voxels outside the brain, or not covered by the image, are set to 0.
    """
    z, _brain, res, row, ref = _zmap(image, method, reference, covariates, p_threshold, exclude, max_iter)
    out = nib.Nifti1Image(z, res.image.affine)
    out.header.set_xyzt_units("mm")
    kind = "z-score" if row is None else "w-score"
    out.header["descrip"] = f"sugar-pet {kind} {method} factor={res.factor:.6g} n={ref.n}"[:79]
    return out


def _zmap(image, method, reference, covariates, p_threshold, exclude, max_iter):
    """The work of :func:`zmap`: the scores, the voxels that were scored, and how they came about."""
    ref = reference if isinstance(reference, Reference) else Reference.load(reference)
    ref.control_stats(method)  # fail before doing any work
    row = None
    if covariates is not None:
        ref.control_model(method)
        row, notes = design_row(ref.meta["covariates"], covariates)
        for note in notes:
            warnings.warn(note, stacklevel=3)
    res = normalize(
        image,
        method,
        reference=ref,
        fwhm=ref.factor_fwhm,
        statistic=ref.statistic,
        p_threshold=p_threshold,
        exclude=exclude,
        max_iter=max_iter,
    )
    ref = ref.on_grid(res.image)
    if row is None:
        mean, std = ref.control_stats(method)
    else:
        beta, std = ref.control_model(method)
        mean = np.tensordot(row.astype(np.float32), beta, axes=1)

    data = res.image.get_fdata(dtype=np.float32)
    brain = get_mask("gray-matter", res.image) | get_mask("parenchyma", res.image)
    brain &= valid_voxels(data) & (std > 0)
    data = smooth(data, res.image.affine, ref.fwhm)
    z = np.zeros(data.shape, dtype=np.float32)
    z[brain] = (data[brain] - mean[brain]) / std[brain]
    return z, brain, res, row, ref
