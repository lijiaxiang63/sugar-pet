"""Z-score maps: how far a normalised image is from the healthy controls."""

from __future__ import annotations

from pathlib import Path

import nibabel as nib
import numpy as np

from .imageops import smooth, valid_voxels
from .masks import get_mask
from .normalize import normalize
from .reference import Reference


def zmap(
    image: str | Path | nib.Nifti1Image,
    method: str = "hn",
    *,
    reference: Reference | str | Path,
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
    built with ``stats_for`` naming the method; their default region and the
    mean are always used, because that is what the controls were scaled with.

    Voxels outside the brain, or not covered by the image, are set to 0.
    """
    ref = reference if isinstance(reference, Reference) else Reference.load(reference)
    ref.control_stats(method)  # fail before doing any work
    res = normalize(image, method, reference=ref, p_threshold=p_threshold, exclude=exclude, max_iter=max_iter)
    ref = ref.on_grid(res.image)
    mean, std = ref.control_stats(method)

    data = res.image.get_fdata(dtype=np.float32)
    brain = get_mask("gray-matter", res.image) | get_mask("parenchyma", res.image)
    brain &= valid_voxels(data) & (std > 0)
    data = smooth(data, res.image.affine, ref.fwhm)
    z = np.zeros(data.shape, dtype=np.float32)
    z[brain] = (data[brain] - mean[brain]) / std[brain]

    out = nib.Nifti1Image(z, res.image.affine)
    out.header.set_xyzt_units("mm")
    out.header["descrip"] = f"sugar-pet z-score {method} factor={res.factor:.6g} n={ref.n}"[:79]
    return out
