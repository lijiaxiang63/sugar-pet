"""The healthy-control reference needed by HN, iPS, iHN and by z-score maps."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

import nibabel as nib
import numpy as np

from . import __version__
from .covariates import build_design
from .imageops import histogram_mode, load_image, region_stat, smooth, spm_global, valid_voxels
from .masks import REFERENCE_REGIONS, get_mask, resample_to, same_grid

_VOLUMES = ("template", "ps_std", "hn_mean", "hn_std", "mask")

# Methods whose control statistics can be stored on request (for z-score
# maps).  PS/iPS and HN/iHN statistics are always part of a reference.
STATS_METHODS = (*REFERENCE_REGIONS, "spm-global")


def _fit(volumes, design: np.ndarray) -> tuple[np.ndarray, np.ndarray]:
    """Voxel-wise least squares of the control images on the design matrix.

    Returns the coefficients (terms x volume) and the residual standard
    deviation.  The images are visited once, so they need not fit in memory
    a second time.
    """
    n, p = design.shape
    pinv = np.linalg.pinv(design)
    beta = xty = squares = None
    for i, y in enumerate(volumes):
        y = y.astype(np.float64)
        if beta is None:
            beta = np.zeros((p,) + y.shape)
            xty = np.zeros((p,) + y.shape)
            squares = np.zeros(y.shape)
        for j in range(p):
            beta[j] += pinv[j, i] * y
            xty[j] += design[i, j] * y
        squares += y * y
    residual = np.maximum(squares - (beta * xty).sum(axis=0), 0.0)
    return beta.astype(np.float32), np.sqrt(residual / (n - p)).astype(np.float32)


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
        Voxels used to estimate factors (the gray-matter mask).
    n
        Number of controls.
    fwhm
        Smoothing (mm) applied to the controls before computing statistics.
    stats
        ``{method: (mean, std)}`` of the controls after normalisation with
        further methods, when asked for with ``stats_for``.
    models
        ``{method: (coefficients, residual SD)}`` of the regression of the
        normalised controls on their covariates (age, sex, ...), when the
        reference was built with ``covariates``.  Used for W-scores.
    statistic, factor_fwhm
        How the factors of the controls were estimated (see :meth:`build`).
        An image is to be compared with the controls only when its own
        factor is estimated the same way, which :func:`~sugar_pet.zmap`
        sees to.
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
    stats: dict[str, tuple[np.ndarray, np.ndarray]] = field(default_factory=dict)
    models: dict[str, tuple[np.ndarray, np.ndarray]] = field(default_factory=dict)

    @classmethod
    def build(
        cls,
        controls: list[str | Path | nib.Nifti1Image],
        mask: str | Path | nib.Nifti1Image = "gray-matter",
        fwhm: float = 8.0,
        stats_for: tuple[str, ...] | list[str] = (),
        covariates: dict[str, list] | None = None,
        progress=None,
        *,
        statistic: str = "mean",
        factor_fwhm: float | None = None,
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
        stats_for
            Reference-region methods (or ``"spm-global"``) whose control mean
            and standard deviation should be stored as well, so that z-score
            maps can be made with them.  PS, iPS, HN and iHN never need this.
        covariates
            ``{name: one value per control}``, for example
            ``{"age": [63, 58, ...], "sex": ["F", "M", ...]}``.  When given,
            the controls are also regressed on these at every voxel, so
            that z-score maps can be adjusted for them (W-scores).
        statistic, factor_fwhm
            How the factors of the controls are estimated, as the arguments
            ``statistic`` and ``fwhm`` of :func:`~sugar_pet.normalize`: the
            mean or the median of a region (for PS and the reference
            regions), and the smoothing in mm before a factor is estimated
            (``None``: the default of each method).  The reference remembers
            them, and z-score maps estimate the factor of an image the same
            way.  Leave them alone unless you normalise your images so.
        """
        if statistic not in ("mean", "median"):
            raise ValueError("statistic must be 'mean' or 'median'")
        if len(controls) < 3:
            raise ValueError("at least 3 control images are needed to build a reference")
        stats_for = list(dict.fromkeys(stats_for))
        unknown = [m for m in stats_for if m not in STATS_METHODS]
        if unknown:
            raise ValueError(f"cannot store statistics for {unknown}; choose from: {', '.join(STATS_METHODS)}")
        design = terms = None
        if covariates:
            design, terms = build_design(covariates)
            if len(design) != len(controls):
                raise ValueError(f"{len(design)} covariate values for {len(controls)} controls")
        models = {}
        first = load_image(controls[0])
        mask0 = get_mask(mask, first)
        regions = {m: get_mask(REFERENCE_REGIONS[m], first) for m in stats_for if m != "spm-global"}
        other: dict[str, list[float]] = {m: [] for m in stats_for}
        scaled = np.empty((len(controls),) + first.shape, dtype=np.float32)
        # The voxels of the mask as the factors are estimated from them: as
        # given, or smoothed when a smoothing is set for the factors.
        estimate = np.empty((len(controls), int(mask0.sum())), dtype=np.float32)
        in_fov = np.zeros(first.shape, dtype=np.int32)
        for i, item in enumerate(controls):
            img = first if i == 0 else load_image(item)
            if not same_grid(img.shape, img.affine, first.shape, first.affine):
                raise ValueError(f"control {i + 1} is on a different grid than the first control")
            data = img.get_fdata(dtype=np.float32)
            valid = valid_voxels(data)
            in_fov += valid
            est = data if factor_fwhm is None else smooth(data, img.affine, factor_fwhm)
            estimate[i] = est[mask0]
            for m in stats_for:
                if m == "spm-global":
                    other[m].append(spm_global(est))
                else:
                    other[m].append(region_stat(est, regions[m] & valid, statistic))
            scaled[i] = smooth(data, img.affine, fwhm)
            if progress:
                progress(i + 1, len(controls))

        # Keep only voxels that every control actually covers.
        final = mask0 & (in_fov == len(controls))
        if final.sum() < 1000:
            raise ValueError("the mask barely overlaps the controls; are they in MNI space?")
        # Proportional scaling is defined on the images as given (before
        # smoothing, unless one is set for the factors), so that all
        # data-driven methods share one scale.
        estimate = estimate[:, final[mask0]]
        if statistic == "mean":
            ps = estimate.mean(axis=1, dtype=np.float64)
        else:
            ps = np.median(estimate, axis=1).astype(np.float64)
        if factor_fwhm is None:
            estimate = None  # the HN factor is of the controls as smoothed for the statistics
        mask0 = final
        for i in range(len(controls)):
            scaled[i] /= ps[i]
        template = scaled.mean(axis=0, dtype=np.float64).astype(np.float32)
        ps_std = scaled.std(axis=0, ddof=1, dtype=np.float64).astype(np.float32)
        if design is not None:
            models["ps"] = _fit(scaled, design)

        # HN factor of each control against the template, then HN statistics.
        hn = []
        for i in range(len(controls)):
            if estimate is None:
                mode = histogram_mode(scaled[i][mask0] / template[mask0])
            else:
                # With a smoothing set for the factors the control is taken
                # as normalize() takes an image: smoothed so, against the
                # template.
                keep = template[mask0] > 0
                mode = histogram_mode(estimate[i][keep] / np.float32(ps[i]) / template[mask0][keep])
            hn.append(ps[i] * mode)
            scaled[i] /= mode
        hn_mean = scaled.mean(axis=0, dtype=np.float64).astype(np.float32)
        hn_std = scaled.std(axis=0, ddof=1, dtype=np.float64).astype(np.float32)
        if design is not None:
            models["hn"] = _fit(scaled, design)

        # Control statistics under the other methods: ``scaled`` now holds
        # smoothed / HN factor, so rescale it by HN factor / method factor.
        stats = {}
        for m in stats_for:
            ratio = (np.asarray(hn) / np.asarray(other[m])).astype(np.float32)
            total = np.zeros(first.shape, dtype=np.float64)
            squares = np.zeros(first.shape, dtype=np.float64)
            for i in range(len(controls)):
                x = scaled[i] * ratio[i]
                total += x
                squares += np.square(x, dtype=np.float64)
            n = len(controls)
            mean = total / n
            var = np.maximum(squares - n * mean**2, 0.0) / (n - 1)
            stats[m] = (mean.astype(np.float32), np.sqrt(var).astype(np.float32))
            if design is not None:
                models[m] = _fit((scaled[i] * ratio[i] for i in range(len(controls))), design)

        factors = {"ps": ps, "hn": hn, **other}
        meta = {
            "sugar_pet_version": __version__,
            "mask": mask if isinstance(mask, str) else "custom",
            "control_factors": {k: [float(v) for v in vals] for k, vals in factors.items()},
        }
        # Only what is not the usual way is written down, so that the file
        # of a usual reference stays what it always was.
        if statistic != "mean":
            meta["statistic"] = statistic
        if factor_fwhm is not None:
            meta["factor_fwhm_mm"] = float(factor_fwhm)
        if terms:
            meta["covariates"] = terms
        n = len(controls)
        return cls(template, ps_std, hn_mean, hn_std, mask0, first.affine, n, float(fwhm), meta, stats, models)

    @property
    def statistic(self) -> str:
        """``"mean"`` or ``"median"``: how the factor of a region was taken of the controls."""
        return self.meta.get("statistic", "mean")

    @property
    def factor_fwhm(self) -> float | None:
        """Smoothing (mm) before the factors of the controls were estimated; ``None`` for the default of each method."""
        return self.meta.get("factor_fwhm_mm")

    @property
    def covariates(self) -> list[str]:
        """Names of the covariates W-scores are adjusted for (empty if none)."""
        return [term["name"] for term in self.meta.get("covariates", [])]

    def control_model(self, method: str) -> tuple[np.ndarray, np.ndarray]:
        """Regression coefficients and residual SD of the controls normalised with ``method``."""
        if not self.models:
            raise ValueError("this reference was built without covariates; rebuild it with covariates (--covariates)")
        key = {"ips": "ps", "ihn": "hn"}.get(method, method)
        if key in self.models:
            return self.models[key]
        self.control_stats(method)  # explains what is missing
        raise ValueError(f"this reference holds no covariate model for '{method}'")

    def control_stats(self, method: str) -> tuple[np.ndarray, np.ndarray]:
        """Mean and standard deviation of the controls normalised with ``method``."""
        if method in ("ps", "ips"):
            return self.template, self.ps_std
        if method in ("hn", "ihn"):
            return self.hn_mean, self.hn_std
        if method in self.stats:
            return self.stats[method]
        if method in STATS_METHODS:
            raise ValueError(
                f"this reference holds no control statistics for '{method}'; "
                f"rebuild it with stats_for=['{method}'] (--stats-for {method})"
            )
        raise ValueError(f"control statistics are not available for method '{method}'")

    def save(self, folder: str | Path) -> Path:
        """Write the reference as a folder of NIfTI files plus ``reference.json``."""
        folder = Path(folder)
        folder.mkdir(parents=True, exist_ok=True)
        for name in _VOLUMES:
            data = getattr(self, name)
            data = data.astype(np.uint8) if name == "mask" else data.astype(np.float32)
            nib.save(nib.Nifti1Image(data, self.affine), folder / f"{name}.nii.gz")
        for method, (mean, std) in self.stats.items():
            nib.save(nib.Nifti1Image(mean.astype(np.float32), self.affine), folder / f"stats-{method}_mean.nii.gz")
            nib.save(nib.Nifti1Image(std.astype(np.float32), self.affine), folder / f"stats-{method}_std.nii.gz")
        for method, (beta, sd) in self.models.items():
            nib.save(nib.Nifti1Image(np.moveaxis(beta, 0, -1), self.affine), folder / f"model-{method}_beta.nii.gz")
            nib.save(nib.Nifti1Image(sd, self.affine), folder / f"model-{method}_sd.nii.gz")
        info = {"n_controls": self.n, "fwhm_mm": self.fwhm, **self.meta}
        info.update(stats_for=list(self.stats), models=list(self.models))
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
        stats = {}
        for method in info.pop("stats_for", []):
            pair = (nib.load(folder / f"stats-{method}_{part}.nii.gz") for part in ("mean", "std"))
            stats[method] = tuple(np.asarray(img.get_fdata(dtype=np.float32)) for img in pair)
        models = {}
        for method in info.pop("models", []):
            beta = nib.load(folder / f"model-{method}_beta.nii.gz").get_fdata(dtype=np.float32)
            sd = nib.load(folder / f"model-{method}_sd.nii.gz").get_fdata(dtype=np.float32)
            models[method] = (np.moveaxis(np.asarray(beta), -1, 0), np.asarray(sd))
        return cls(**vols, affine=affine, n=n, fwhm=fwhm, meta=info, stats=stats, models=models)

    def on_grid(self, target: nib.Nifti1Image) -> Reference:
        """Return this reference resampled onto the grid of ``target``."""
        if same_grid(self.template.shape, self.affine, target.shape, target.affine):
            return self
        vols = {}
        for name in _VOLUMES:
            src = nib.Nifti1Image(getattr(self, name).astype(np.float32), self.affine)
            vols[name] = resample_to(src, target, order=1)
        vols["mask"] = vols["mask"] > 0.5
        stats = {}
        for method, pair in self.stats.items():
            stats[method] = tuple(resample_to(nib.Nifti1Image(v, self.affine), target, order=1) for v in pair)
        move = lambda v: resample_to(nib.Nifti1Image(v, self.affine), target, order=1)
        models = {m: (np.stack([move(b) for b in beta]), move(sd)) for m, (beta, sd) in self.models.items()}
        return Reference(
            **vols, affine=target.affine, n=self.n, fwhm=self.fwhm, meta=self.meta, stats=stats, models=models
        )
