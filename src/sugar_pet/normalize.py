"""Intensity normalisation methods for brain FDG-PET.

Every method boils down to one number per image, the *normalisation factor*;
the normalised image is simply ``image / factor``.  The methods differ in how
that factor is estimated.
"""

from __future__ import annotations

import warnings
from dataclasses import dataclass, field
from pathlib import Path

import nibabel as nib
import numpy as np
from scipy import stats

from .imageops import histogram_mode, load_image, region_mean, smooth, spm_global, valid_voxels
from .masks import MASKS, get_mask
from .reference import Reference

# Reference-region methods: method name -> built-in mask.
REGION_METHODS: dict[str, str] = {
    "cerebellum": "cerebellum",
    "cerebellar-gm": "cerebellar-gm",
    "pons": "pons",
    "brainstem": "brainstem",
    "white-matter": "white-matter",
    "sensorimotor": "sensorimotor",
    "temporal-left": "temporal-left",
    "temporal-right": "temporal-right",
}

METHODS: dict[str, str] = {
    **{name: f"Reference region: {MASKS[mask]}" for name, mask in REGION_METHODS.items()},
    "region": "Reference region: mean uptake in your own mask (needs mask=...)",
    "ps": "Proportional scaling: mean uptake of the brain parenchyma mask",
    "spm-global": "SPM-style global mean (voxels above 1/8 of the image mean)",
    "hn": "Histogram normalisation: peak of the subject/template ratio histogram [needs reference]",
    "ips": "Iterative PS: PS, then re-estimated without hypometabolic voxels [needs reference]",
    "ihn": "Iterative HN: HN, then re-estimated without hypometabolic voxels [needs reference]",
}

NEEDS_REFERENCE = ("hn", "ips", "ihn")


@dataclass
class NormResult:
    """Outcome of normalising one image.

    Attributes
    ----------
    image
        The normalised image (``input / factor``).
    factor
        The normalisation factor, in the units of the input image.
    mask
        The voxels the factor was finally estimated from.
    info
        Extra details: number of voxels, smoothing, and for the iterative
        methods the starting factor and the fraction of voxels excluded.
    """

    method: str
    factor: float
    image: nib.Nifti1Image
    mask: np.ndarray | None = None
    info: dict = field(default_factory=dict)

    def save(self, path: str | Path) -> Path:
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        nib.save(self.image, path)
        return path

    def mask_image(self) -> nib.Nifti1Image:
        if self.mask is None:
            raise ValueError(f"method '{self.method}' does not use a mask")
        return nib.Nifti1Image(self.mask.astype(np.uint8), self.image.affine)


def single_subject_t(data: np.ndarray, mean: np.ndarray, std: np.ndarray, n: int) -> np.ndarray:
    """t statistic of one subject against ``n`` controls, with ``n - 1`` degrees of freedom.

    This is the two-sample t-test with equal variances when one of the
    groups has a single member (Crawford & Howell, 1998).  Negative values
    mean lower uptake than the controls.
    """
    t = np.zeros(data.shape, dtype=np.float32)
    ok = std > 0
    t[ok] = (data[ok] - mean[ok]) / (std[ok] * np.sqrt(1.0 + 1.0 / n))
    return t


def _exclusion(t: np.ndarray, df: int, p_threshold: float, exclude: str) -> np.ndarray:
    """Voxels whose uptake differs significantly from the controls."""
    if exclude == "hypo":
        return t < stats.t.ppf(p_threshold, df)
    if exclude == "both":
        return np.abs(t) > stats.t.ppf(1.0 - p_threshold / 2.0, df)
    raise ValueError("exclude must be 'hypo' or 'both'")


def _iterate(base, data, ref, mask0, factor0, p_threshold, exclude, max_iter, min_fraction, literal):
    """Refine a PS or HN factor by leaving out voxels that look abnormal.

    The subject, scaled by the current factor, is compared with the
    controls voxel by voxel; significant voxels are dropped from the mask
    and the factor is estimated again from what is left.
    """
    mean, std = (ref.template, ref.ps_std) if base == "ps" else (ref.hn_mean, ref.hn_std)
    usable = mask0 & (std > 0)
    factor, mask, n_iter = factor0, mask0, 0
    for _ in range(max_iter):
        t = single_subject_t(data / factor, mean, std, ref.n)
        new_mask = usable & ~_exclusion(t, ref.n - 1, p_threshold, exclude)
        if new_mask.sum() < min_fraction * mask0.sum():
            warnings.warn(
                f"iterative step would keep under {min_fraction:.0%} of the mask; stopping with the previous factor",
                stacklevel=3,
            )
            break
        if n_iter and np.array_equal(new_mask, mask):
            break
        mask = new_mask
        n_iter += 1
        if base == "hn":
            factor = histogram_mode(data[mask] / ref.template[mask])
        elif literal:
            factor = region_mean(data, mask)
        else:
            # Dividing by the control mean of the *same* voxels keeps the
            # scale comparable to the controls whatever the mask has lost.
            factor = region_mean(data, mask) / region_mean(ref.template, mask)
    return factor, mask, n_iter


def normalize(
    image: str | Path | nib.Nifti1Image,
    method: str = "hn",
    *,
    reference: Reference | str | Path | None = None,
    mask: str | Path | nib.Nifti1Image | np.ndarray | None = None,
    fwhm: float | None = None,
    p_threshold: float = 0.01,
    exclude: str = "hypo",
    max_iter: int = 1,
    min_fraction: float = 0.2,
    literal_ips: bool = False,
) -> NormResult:
    """Intensity-normalise one spatially normalised FDG-PET image.

    Parameters
    ----------
    image
        FDG-PET image in MNI space (file name or nibabel image).
    method
        One of :data:`METHODS`.  ``"hn"``, ``"ips"`` and ``"ihn"`` need a
        ``reference`` built from healthy controls.
    reference
        A :class:`Reference` or the folder it was saved to.
    mask
        Replace the default region of a method by your own mask (a built-in
        mask name or a NIfTI file in MNI space).  Required for
        ``method="region"``; ignored by the reference-based methods, which
        use the mask stored in the reference (so does ``"ps"`` when a
        reference is given and no mask is passed).
    fwhm
        Smoothing (mm) applied before the factor is estimated.  The output
        image itself is never smoothed.  Defaults to the smoothing of the
        reference for ``hn``/``ips``/``ihn`` and to no smoothing otherwise.
        Pass 0 if your images are already smoothed.
    p_threshold, exclude
        For ``ips``/``ihn``: voxels are dropped when they differ from the
        controls at this uncorrected p-value.  ``exclude="hypo"`` drops only
        hypometabolic voxels (as in the paper), ``"both"`` also drops
        hypermetabolic ones.
    max_iter
        For ``ips``/``ihn``: number of re-estimations.  The paper uses 1.
    min_fraction
        For ``ips``/``ihn``: never let the mask shrink below this fraction.
    literal_ips
        For ``ips``: use the plain mean of the remaining voxels, as written
        in the paper, instead of the mean relative to the control template.

    Returns
    -------
    NormResult
    """
    if method not in METHODS:
        raise ValueError(f"unknown method '{method}'; choose from: {', '.join(METHODS)}")
    img = load_image(image)
    raw = img.get_fdata(dtype=np.float32)
    info: dict = {}

    if method in NEEDS_REFERENCE and reference is None:
        raise ValueError(f"method '{method}' needs a reference built from healthy controls")
    ref = None
    if reference is not None:
        ref = reference if isinstance(reference, Reference) else Reference.load(reference)
        ref = ref.on_grid(img)
        if fwhm is None and method in NEEDS_REFERENCE:
            fwhm = ref.fwhm
    fwhm = float(fwhm or 0.0)
    data = smooth(raw, img.affine, fwhm)
    valid = valid_voxels(raw)
    info["fwhm_mm"] = fwhm

    final_mask = None
    if method == "spm-global":
        factor = spm_global(data)
    elif method in REGION_METHODS or method in ("region", "ps"):
        if method == "region" and mask is None:
            raise ValueError("method 'region' needs a mask")
        if mask is None and method == "ps" and ref is not None:
            # Same voxels as the reference-based methods, so that PS, iPS,
            # HN and iHN are directly comparable.
            region, info["mask"] = ref.mask, "reference"
        else:
            region = mask if mask is not None else REGION_METHODS.get(method, "gray-matter")
            info["mask"] = region if isinstance(region, str) else "custom"
        final_mask = get_mask(region, img) & valid
        factor = region_mean(data, final_mask)
    else:
        mask0 = ref.mask & valid & (ref.template > 0)
        if mask0.sum() < 0.5 * ref.mask.sum():
            warnings.warn(
                "less than half of the reference mask is covered by the image; check that it is in MNI space",
                stacklevel=2,
            )
        base = "ps" if method == "ips" else "hn"
        if base == "ps":
            factor = region_mean(data, mask0) / region_mean(ref.template, mask0)
        else:
            factor = histogram_mode(data[mask0] / ref.template[mask0])
        final_mask = mask0
        if method in ("ips", "ihn"):
            info["initial_factor"] = float(factor)
            factor, final_mask, n_iter = _iterate(
                base, data, ref, mask0, factor, p_threshold, exclude, max_iter, min_fraction, literal_ips
            )
            info["iterations"] = n_iter
            info["excluded_fraction"] = float(1.0 - final_mask.sum() / mask0.sum())

    if not np.isfinite(factor) or factor <= 0:
        raise ValueError(f"method '{method}' gave an unusable factor ({factor})")
    if final_mask is not None:
        info["n_voxels"] = int(final_mask.sum())

    out = nib.Nifti1Image((raw / factor).astype(np.float32), img.affine)
    out.header.set_xyzt_units("mm")
    out.header["descrip"] = f"sugar-pet {method} factor={factor:.6g}"[:79]
    return NormResult(method=method, factor=float(factor), image=out, mask=final_mask, info=info)
