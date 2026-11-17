"""Built-in MNI-space masks, derived from the bundled CerebrA atlas.

All masks live on a 2 mm grid in MNI space (ICBM 2009c symmetric) and are
resampled onto the grid of whatever image they are applied to.
"""

from __future__ import annotations

from functools import cache
from importlib import resources
from pathlib import Path

import nibabel as nib
import numpy as np
from nibabel.processing import resample_from_to
from scipy import ndimage

_PREFIX = "tpl-MNI152NLin2009cSym_res-2_"

# CerebrA label numbers (right hemisphere, left hemisphere).
_BRAINSTEM = (11, 62)
_CEREBELLUM_GM = (46, 97)
_CEREBELLUM_WM = (39, 90)
_VERMIS = (50, 101, 2, 53, 20, 71)
_TEMPORAL_R = (3, 28, 45, 14, 24, 18, 36)  # inf/mid/sup/transverse, fusiform, parahipp., entorhinal
_TEMPORAL_L = tuple(i + 51 for i in _TEMPORAL_R)
_SENSORIMOTOR = (35, 86, 13, 64, 16, 67)  # precentral, postcentral, paracentral
_OCCIPITAL = (34, 85, 43, 94, 6, 57, 12, 63)  # lateral occipital, cuneus, pericalcarine, lingual

# Pons = the part of the brainstem between these MNI z coordinates (mm).
_PONS_Z = (-44.0, -26.0)

MASKS: dict[str, str] = {
    "gray-matter": "Gray matter (probability > 0.1); the paper's mask for PS and HN",
    "parenchyma": "Whole brain parenchyma (gray + white matter probability > 0.5)",
    "cerebellum": "Whole cerebellum, gray and white matter (RRC in the paper)",
    "cerebellar-gm": "Cerebellar gray matter, hemispheres and vermis",
    "pons": "Central pons (brainstem between z = -44 and -26 mm, eroded)",
    "white-matter": "Deep cerebral white matter (eroded, centrum semiovale)",
    "occipital": "Occipital cortex (lateral occipital, cuneus, pericalcarine, lingual)",
    "sensorimotor": "Primary sensorimotor cortex (pre-, post- and paracentral)",
    "brainstem": "Whole brainstem (RRBS in the paper)",
    "temporal-left": "Left temporal cortex (RRTL for a right-sided lesion)",
    "temporal-right": "Right temporal cortex (RRTL for a left-sided lesion)",
}

# Built-in masks that are also offered as reference-region methods of the
# same name.  The others can still be used with ``method="region"``.
REFERENCE_REGIONS = ("cerebellum", "cerebellar-gm", "pons", "white-matter", "occipital", "sensorimotor")


def _load(name: str) -> nib.Nifti1Image:
    path = resources.files("sugar_pet") / "data" / f"{_PREFIX}{name}.nii.gz"
    with resources.as_file(path) as p:
        img = nib.load(p)
        return nib.Nifti1Image(np.asanyarray(img.get_fdata(dtype=np.float32)), img.affine)


@cache
def _atlas() -> tuple[np.ndarray, np.ndarray]:
    img = _load("atlas-CerebrA_dseg")
    return np.round(img.get_fdata()).astype(np.uint8), img.affine


@cache
def _prob(tissue: str) -> np.ndarray:
    return _load(f"label-{tissue}_probseg").get_fdata()


def atlas_labels() -> dict[int, str]:
    """Return ``{label number: 'Name (L/R)'}`` for the bundled CerebrA atlas."""
    text = (resources.files("sugar_pet") / "data" / "atlas-CerebrA_dseg.tsv").read_text()
    out = {}
    for line in text.strip().splitlines()[1:]:
        index, name, hemi = line.split("\t")
        out[int(index)] = f"{name} ({hemi})"
    return out


@cache
def _builtin(name: str) -> np.ndarray:
    labels, affine = _atlas()
    if name == "gray-matter":
        return _prob("GM") > 0.1
    if name == "parenchyma":
        return (_prob("GM") + _prob("WM")) > 0.5
    if name == "cerebellum":
        return np.isin(labels, _CEREBELLUM_GM + _CEREBELLUM_WM + _VERMIS)
    if name == "cerebellar-gm":
        return np.isin(labels, _CEREBELLUM_GM + _VERMIS)
    if name == "brainstem":
        return np.isin(labels, _BRAINSTEM)
    if name == "pons":
        z = affine[2, 2] * np.arange(labels.shape[2]) + affine[2, 3]
        slab = (z >= _PONS_Z[0]) & (z <= _PONS_Z[1])
        pons = np.isin(labels, _BRAINSTEM) & slab[None, None, :]
        # Erode in-plane only, so the slab keeps its full height.
        return ndimage.binary_erosion(pons, structure=np.ones((3, 3, 1), bool))
    if name == "white-matter":
        infratentorial = np.isin(labels, _BRAINSTEM + _CEREBELLUM_GM + _CEREBELLUM_WM + _VERMIS)
        wm = (_prob("WM") > 0.9) & ~ndimage.binary_dilation(infratentorial, iterations=2)
        return ndimage.binary_erosion(wm, iterations=2)
    if name == "sensorimotor":
        return np.isin(labels, _SENSORIMOTOR)
    if name == "occipital":
        return np.isin(labels, _OCCIPITAL)
    if name == "temporal-left":
        return np.isin(labels, _TEMPORAL_L)
    if name == "temporal-right":
        return np.isin(labels, _TEMPORAL_R)
    raise KeyError(name)


def builtin_mask(name: str) -> nib.Nifti1Image:
    """Return a built-in mask as a NIfTI image on the 2 mm MNI atlas grid."""
    if name not in MASKS:
        raise ValueError(f"unknown mask '{name}'; choose from: {', '.join(MASKS)}")
    return nib.Nifti1Image(_builtin(name).astype(np.uint8), _atlas()[1])


def labels_mask(labels: list[int]) -> nib.Nifti1Image:
    """Return a mask made of the given CerebrA label numbers."""
    atlas, affine = _atlas()
    unknown = sorted(set(labels) - set(atlas_labels()))
    if unknown:
        raise ValueError(f"not CerebrA label numbers: {unknown}; see atlas_labels()")
    if not labels:
        raise ValueError("no labels given")
    return nib.Nifti1Image(np.isin(atlas, labels).astype(np.uint8), affine)


def same_grid(a_shape, a_affine, b_shape, b_affine) -> bool:
    return tuple(a_shape[:3]) == tuple(b_shape[:3]) and np.allclose(a_affine, b_affine, atol=1e-3)


def resample_to(img: nib.Nifti1Image, target: nib.Nifti1Image, order: int = 1) -> np.ndarray:
    """Resample ``img`` onto the grid of ``target`` and return the array."""
    if same_grid(img.shape, img.affine, target.shape, target.affine):
        return np.asarray(img.get_fdata(dtype=np.float32))
    out = resample_from_to(img, (target.shape[:3], target.affine), order=order, cval=0.0)
    return np.asarray(out.get_fdata(dtype=np.float32))


def get_mask(mask: str | Path | nib.Nifti1Image | np.ndarray, target: nib.Nifti1Image) -> np.ndarray:
    """Return a boolean mask on the grid of ``target``.

    ``mask`` may be the name of a built-in mask, a NIfTI file or image (any
    grid, as long as it is in the same space as ``target``), or an array that
    already matches ``target``.
    """
    if isinstance(mask, np.ndarray):
        if mask.shape != target.shape[:3]:
            raise ValueError("mask array does not match the image grid")
        return mask.astype(bool)
    if isinstance(mask, str) and mask in MASKS:
        mask_img = builtin_mask(mask)
    elif isinstance(mask, (str, Path)):
        if not Path(mask).exists():
            raise ValueError(f"'{mask}' is neither a built-in mask ({', '.join(MASKS)}) nor an existing file")
        loaded = nib.load(mask)
        mask_img = nib.Nifti1Image((np.nan_to_num(loaded.get_fdata()) > 0).astype(np.uint8), loaded.affine)
    else:
        mask_img = nib.Nifti1Image((np.nan_to_num(mask.get_fdata()) > 0).astype(np.uint8), mask.affine)
    return resample_to(mask_img, target, order=1) > 0.5
