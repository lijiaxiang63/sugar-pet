"""Small image helpers: loading, smoothing and the basic factor estimators."""

from __future__ import annotations

from pathlib import Path

import nibabel as nib
import numpy as np
from scipy import ndimage

_FWHM_TO_SIGMA = 1.0 / (2.0 * np.sqrt(2.0 * np.log(2.0)))


def load_image(image: str | Path | nib.Nifti1Image) -> nib.Nifti1Image:
    """Load a 3D PET image as float32 (a single-frame 4D image is squeezed)."""
    img = nib.load(image) if isinstance(image, (str, Path)) else image
    data = np.asarray(img.get_fdata(dtype=np.float32))
    if data.ndim == 4 and data.shape[3] == 1:
        data = data[..., 0]
    if data.ndim != 3:
        name = image if isinstance(image, (str, Path)) else "image"
        raise ValueError(f"{name}: expected a 3D image, got shape {data.shape}")
    return nib.Nifti1Image(data, img.affine)


def valid_voxels(data: np.ndarray) -> np.ndarray:
    """Voxels that hold real signal (finite and above zero)."""
    return np.isfinite(data) & (data > 0)


def smooth(data: np.ndarray, affine: np.ndarray, fwhm: float) -> np.ndarray:
    """Gaussian smoothing with a kernel given as FWHM in millimetres."""
    if not fwhm:
        return data
    sigma = fwhm * _FWHM_TO_SIGMA / nib.affines.voxel_sizes(affine)
    clean = np.nan_to_num(data, nan=0.0, posinf=0.0, neginf=0.0)
    return ndimage.gaussian_filter(clean, sigma, mode="constant").astype(np.float32)


def region_mean(data: np.ndarray, mask: np.ndarray) -> float:
    """Mean uptake inside a mask."""
    if not mask.any():
        raise ValueError("the mask does not overlap the image; is the image in MNI space?")
    return float(data[mask].mean(dtype=np.float64))


def region_stat(data: np.ndarray, mask: np.ndarray, statistic: str = "mean") -> float:
    """Mean or median uptake inside a mask."""
    if statistic == "mean":
        return region_mean(data, mask)
    if statistic != "median":
        raise ValueError("statistic must be 'mean' or 'median'")
    if not mask.any():
        raise ValueError("the mask does not overlap the image; is the image in MNI space?")
    return float(np.median(data[mask]))


def spm_global(data: np.ndarray) -> float:
    """SPM's global mean: the mean of all voxels brighter than 1/8 of the image mean."""
    values = data[np.isfinite(data)]
    return float(values[values > values.mean(dtype=np.float64) / 8.0].mean(dtype=np.float64))


def histogram_mode(values: np.ndarray, bandwidth: float | None = None, n_bins: int = 2048) -> float:
    """Most frequent value of a sample (the peak of its smoothed histogram).

    A plain histogram maximum jumps around with the bin width, so the
    histogram is first smoothed with a Gaussian kernel (a kernel density
    estimate) and the peak position is refined by a parabola fit.

    Parameters
    ----------
    bandwidth
        Kernel standard deviation in the units of ``values``.  Defaults to a
        quarter of the robust spread (MAD) of the sample, which is narrow
        enough to keep the peak of a skewed distribution in place.
    """
    values = values[np.isfinite(values)]
    if values.size < 100:
        raise ValueError("too few voxels to estimate a histogram mode")
    lo, hi = np.percentile(values, [0.5, 99.5])
    if hi <= lo:
        return float(lo)
    median = np.median(values)
    spread = 1.4826 * np.median(np.abs(values - median))
    if bandwidth is None:
        bandwidth = 0.25 * spread if spread > 0 else (hi - lo) / 50.0
    counts, edges = np.histogram(values, bins=n_bins, range=(lo, hi))
    bin_width = edges[1] - edges[0]
    density = ndimage.gaussian_filter1d(counts.astype(np.float64), bandwidth / bin_width, mode="constant")
    k = int(np.argmax(density))
    centres = 0.5 * (edges[:-1] + edges[1:])
    if 0 < k < n_bins - 1:
        y0, y1, y2 = density[k - 1 : k + 2]
        denom = y0 - 2.0 * y1 + y2
        shift = 0.5 * (y0 - y2) / denom if denom != 0 else 0.0
        return float(centres[k] + shift * bin_width)
    return float(centres[k])
