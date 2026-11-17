import nibabel as nib
import numpy as np
import pytest

from sugar_pet import MASKS, atlas_labels, builtin_mask, labels_mask
from sugar_pet.masks import get_mask


@pytest.mark.parametrize("name", list(MASKS))
def test_builtin_masks_are_not_empty(name):
    assert builtin_mask(name).get_fdata().sum() > 500


def test_region_volumes_are_plausible():
    cm3 = lambda name: builtin_mask(name).get_fdata().sum() * 0.008
    assert 120 < cm3("cerebellum") < 220
    assert 20 < cm3("brainstem") < 45
    assert 5 < cm3("pons") < 15
    assert 60 < cm3("occipital") < 130


def test_nested_regions():
    get = lambda name: builtin_mask(name).get_fdata() > 0
    assert not (get("pons") & ~get("brainstem")).any()
    assert not (get("cerebellar-gm") & ~get("cerebellum")).any()
    assert not (get("temporal-left") & get("temporal-right")).any()


def test_left_is_left():
    img = builtin_mask("temporal-left")
    x = nib.affines.apply_affine(img.affine, np.argwhere(img.get_fdata() > 0))[:, 0]
    assert x.mean() < -30  # negative x is left in MNI space


def test_mask_follows_image_grid():
    """A mask must land on the same anatomy whatever the image grid is."""
    src = builtin_mask("cerebellum")
    # SPM-style grid: 91x109x91, x axis flipped
    affine = np.array([[-2.0, 0, 0, 90], [0, 2, 0, -126], [0, 0, 2, -72], [0, 0, 0, 1]])
    target = nib.Nifti1Image(np.zeros((91, 109, 91), np.float32), affine)
    mask = get_mask("cerebellum", target)
    centre_src = nib.affines.apply_affine(src.affine, np.argwhere(src.get_fdata() > 0).mean(0))
    centre_new = nib.affines.apply_affine(affine, np.argwhere(mask).mean(0))
    assert np.allclose(centre_src, centre_new, atol=1.0)
    assert abs(mask.sum() - src.get_fdata().sum()) / src.get_fdata().sum() < 0.03


def test_custom_mask_file_and_labels(tmp_path):
    labels = atlas_labels()
    assert labels[11] == "Brainstem (R)" and labels[62] == "Brainstem (L)"
    custom = labels_mask([11, 62])
    path = tmp_path / "bs.nii.gz"
    nib.save(custom, path)
    assert np.array_equal(get_mask(path, custom), builtin_mask("brainstem").get_fdata() > 0)


def test_unknown_mask_name():
    with pytest.raises(ValueError, match="neither a built-in mask"):
        get_mask("no-such-region", builtin_mask("pons"))
