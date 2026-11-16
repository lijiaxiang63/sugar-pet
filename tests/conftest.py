"""Synthetic FDG-like images built from the bundled tissue maps."""

import nibabel as nib
import numpy as np
import pytest
from scipy import ndimage

from sugar_pet import builtin_mask
from sugar_pet.masks import _atlas, _prob


def make_pet(rng, scale=1.0, lesion=None, drop=0.0, noise=0.05):
    """A fake MNI-space FDG image: bright gray matter, darker white matter."""
    base = _prob("GM") * 1.0 + _prob("WM") * 0.35
    field = ndimage.gaussian_filter(rng.standard_normal(base.shape), 3.0)
    field *= noise / field.std()
    data = base * (1.0 + field)
    if lesion is not None:
        data = data * (1.0 - drop * lesion)
    data = ndimage.gaussian_filter(data, 1.5) * scale
    return nib.Nifti1Image(data.astype(np.float32), _atlas()[1])


@pytest.fixture(scope="session")
def controls():
    rng = np.random.default_rng(0)
    return [make_pet(rng, scale=rng.uniform(3000, 9000)) for _ in range(16)]


@pytest.fixture(scope="session")
def lesion():
    """A large left-hemisphere lesion (temporal + sensorimotor cortex)."""
    mask = builtin_mask("temporal-left").get_fdata() + builtin_mask("sensorimotor").get_fdata()
    left = np.zeros(mask.shape, bool)
    left[: mask.shape[0] // 2] = True  # x index grows towards the right in the atlas
    return ((mask > 0) & left).astype(np.float32)


@pytest.fixture(scope="session")
def patient(lesion):
    """Patient with true global scale 5000 and 40 % hypometabolism in the lesion."""
    return make_pet(np.random.default_rng(42), scale=5000.0, lesion=lesion, drop=0.4)


@pytest.fixture(scope="session")
def healthy():
    """Same anatomy and noise as ``patient`` but without the lesion."""
    return make_pet(np.random.default_rng(42), scale=5000.0)
