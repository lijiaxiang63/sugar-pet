"""The healthy-control reference needed by the HN, iPS and iHN methods."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

import nibabel as nib
import numpy as np

from . import __version__
from .imageops import histogram_mode, load_image, smooth, valid_voxels
from .masks import get_mask, resample_to, same_grid

_VOLUMES = ("template", "ps_std", "hn_mean", "hn_std", "mask")


@dataclass
class Reference:
    """Voxel-wise summary of a group of healthy controls.

    Attributes
    ----------
    template
        Mean of the smoothed controls after proportional scaling.  This is
        the "normality template" of the HN method and also the control mean
        used by iPS.
    ps_std
        Standard deviation of the controls after proportional scaling.
    hn_mean, hn_std
        Mean and standard deviation of the controls after HN normalisation.
    mask
        Voxels used to estimate factors (the parenchyma mask).
    n
        Number of controls.
    fwhm
        Smoothing (mm) applied to the controls before computing statistics.
    """

    template: np.ndarray
    ps_std: np.ndarray
    hn_mean: np.ndarray
    hn_std: np.ndarray
    mask: np.ndarray
    affine: np.ndarray
    n: int
    fwhm: float
    meta: dict = field(default_factory=dict)

    @classmethod
    def build(
        cls,
        controls: list[str | Path | nib.Nifti1Image],
        mask: str | Path | nib.Nifti1Image = "gray-matter",
        fwhm: float = 8.0,
        progress=None,
    ) -> Reference:
        """Build a reference from spatially normalised healthy-control images.

        Parameters
        ----------
        controls
            MNI-space FDG-PET images, all on the same grid.  More is better;
            below about 10 controls the voxel statistics get unreliable.
        mask
            Voxels used to estimate factors: a built-in mask name or a file.
        fwhm
            Smoothing in mm applied before computing statistics.  Use 0 if
            the images are already smoothed.
        """
        if len(controls) < 3:
            raise ValueError("at least 3 control images are needed to build a reference")
        first = load_image(controls[0])
        mask0 = get_mask(mask, first)
        scaled = np.empty((len(controls),) + first.shape, dtype=np.float32)
        unsmoothed = np.empty((len(controls), int(mask0.sum())), dtype=np.float32)
        in_fov = np.zeros(first.shape, dtype=np.int32)
        for i, item in enumerate(controls):
            img = first if i == 0 else load_image(item)
            if not same_grid(img.shape, img.affine, first.shape, first.affine):
                raise ValueError(f"control {i + 1} is on a different grid than the first control")
            data = img.get_fdata(dtype=np.float32)
            in_fov += valid_voxels(data)
            unsmoothed[i] = data[mask0]
            scaled[i] = smooth(data, img.affine, fwhm)
            if progress:
                progress(i + 1, len(controls))

        # Keep only voxels that every control actually covers.
        final = mask0 & (in_fov == len(controls))
        if final.sum() < 1000:
            raise ValueError("the mask barely overlaps the controls; are they in MNI space?")
        # Proportional scaling is defined on the images as given (before
        # smoothing), so that all data-driven methods share one scale.
        ps = unsmoothed[:, final[mask0]].mean(axis=1, dtype=np.float64)
        del unsmoothed
        mask0 = final
        for i in range(len(controls)):
            scaled[i] /= ps[i]
        template = scaled.mean(axis=0, dtype=np.float64).astype(np.float32)
        ps_std = scaled.std(axis=0, ddof=1, dtype=np.float64).astype(np.float32)

        # HN factor of each control against the template, then HN statistics.
        hn = []
        for i in range(len(controls)):
            mode = histogram_mode(scaled[i][mask0] / template[mask0])
            hn.append(ps[i] * mode)
            scaled[i] /= mode
        hn_mean = scaled.mean(axis=0, dtype=np.float64).astype(np.float32)
        hn_std = scaled.std(axis=0, ddof=1, dtype=np.float64).astype(np.float32)

        meta = {
            "sugar_pet_version": __version__,
            "mask": mask if isinstance(mask, str) else "custom",
            "control_factors": {"ps": [float(v) for v in ps], "hn": [float(v) for v in hn]},
        }
        return cls(template, ps_std, hn_mean, hn_std, mask0, first.affine, len(controls), float(fwhm), meta)

    def save(self, folder: str | Path) -> Path:
        """Write the reference as a folder of NIfTI files plus ``reference.json``."""
        folder = Path(folder)
        folder.mkdir(parents=True, exist_ok=True)
        for name in _VOLUMES:
            data = getattr(self, name)
            data = data.astype(np.uint8) if name == "mask" else data.astype(np.float32)
            nib.save(nib.Nifti1Image(data, self.affine), folder / f"{name}.nii.gz")
        info = {"n_controls": self.n, "fwhm_mm": self.fwhm, **self.meta}
        (folder / "reference.json").write_text(json.dumps(info, indent=2))
        return folder

    @classmethod
    def load(cls, folder: str | Path) -> Reference:
        folder = Path(folder)
        if not (folder / "reference.json").exists():
            raise FileNotFoundError(f"{folder} is not a sugar-pet reference (no reference.json)")
        info = json.loads((folder / "reference.json").read_text())
        vols = {}
        affine = None
        for name in _VOLUMES:
            img = nib.load(folder / f"{name}.nii.gz")
            vols[name] = np.asarray(img.get_fdata(dtype=np.float32))
            affine = img.affine
        vols["mask"] = vols["mask"] > 0.5
        n, fwhm = info.pop("n_controls"), info.pop("fwhm_mm")
        return cls(**vols, affine=affine, n=n, fwhm=fwhm, meta=info)

    def on_grid(self, target: nib.Nifti1Image) -> Reference:
        """Return this reference resampled onto the grid of ``target``."""
        if same_grid(self.template.shape, self.affine, target.shape, target.affine):
            return self
        vols = {}
        for name in _VOLUMES:
            src = nib.Nifti1Image(getattr(self, name).astype(np.float32), self.affine)
            vols[name] = resample_to(src, target, order=1)
        vols["mask"] = vols["mask"] > 0.5
        return Reference(**vols, affine=target.affine, n=self.n, fwhm=self.fwhm, meta=self.meta)
