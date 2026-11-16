import nibabel as nib
import numpy as np
import pytest

from sugar_pet import METHODS, Reference, builtin_mask, normalize
from sugar_pet.imageops import histogram_mode, spm_global
from sugar_pet.normalize import REGION_METHODS, single_subject_t

TRUE_SCALE = 5000.0


@pytest.fixture(scope="module")
def reference(controls):
    return Reference.build(controls, fwhm=0)


def test_histogram_mode_ignores_a_heavy_tail():
    rng = np.random.default_rng(1)
    values = np.concatenate([rng.normal(1.0, 0.05, 80_000), rng.normal(0.6, 0.1, 30_000)])
    assert abs(histogram_mode(values) - 1.0) < 0.01
    assert values.mean() < 0.9  # ... while the mean is dragged down


def test_single_subject_t_is_the_two_sample_t_test():
    from scipy import stats

    rng = np.random.default_rng(3)
    group = rng.normal(1.0, 0.1, (25, 50))
    subject = rng.normal(0.9, 0.1, 50)
    expected = stats.ttest_ind(subject[None], group, equal_var=True).statistic
    got = single_subject_t(subject, group.mean(0), group.std(0, ddof=1), 25)
    assert np.allclose(got, expected, atol=1e-5)


def test_spm_global():
    data = np.zeros((10, 10, 10), np.float32)
    data[:5] = 8.0  # image mean 4 -> threshold 0.5 -> mean of the bright half
    assert spm_global(data) == pytest.approx(8.0)


@pytest.mark.parametrize("method", list(REGION_METHODS))
def test_reference_region_is_the_region_mean(healthy, method):
    res = normalize(healthy, method)
    mask = builtin_mask(REGION_METHODS[method]).get_fdata() > 0
    data = healthy.get_fdata()
    assert res.factor == pytest.approx(data[mask & (data > 0)].mean(), rel=1e-5)
    assert res.image.get_fdata()[res.mask].mean() == pytest.approx(1.0, rel=1e-4)


def test_output_is_input_divided_by_factor(healthy):
    res = normalize(healthy, "ps", fwhm=8)  # smoothing must not leak into the output
    assert np.allclose(res.image.get_fdata() * res.factor, healthy.get_fdata(), rtol=1e-4)
    assert res.info["fwhm_mm"] == 8


def test_scale_invariance(healthy, reference):
    doubled = nib.Nifti1Image(healthy.get_fdata(dtype=np.float32) * 2, healthy.affine)
    for method in ("cerebellum", "ps", "spm-global", "hn", "ihn", "ips"):
        a = normalize(healthy, method, reference=reference).factor
        b = normalize(doubled, method, reference=reference).factor
        assert b == pytest.approx(2 * a, rel=1e-3), method


def test_custom_region(healthy):
    with pytest.raises(ValueError, match="needs a mask"):
        normalize(healthy, "region")
    a = normalize(healthy, "region", mask=builtin_mask("pons"))
    assert a.factor == pytest.approx(normalize(healthy, "pons").factor)


def test_reference_methods_need_a_reference(healthy):
    with pytest.raises(ValueError, match="needs a reference"):
        normalize(healthy, "hn")
    with pytest.raises(ValueError, match="unknown method"):
        normalize(healthy, "magic")
    assert set(REGION_METHODS) < set(METHODS)


def test_reference_roundtrip(reference, tmp_path, healthy):
    loaded = Reference.load(reference.save(tmp_path / "ref"))
    assert loaded.n == reference.n == 16
    assert np.allclose(loaded.template, reference.template)
    assert np.array_equal(loaded.mask, reference.mask)
    assert reference.template[reference.mask].mean() == pytest.approx(1.0, abs=1e-4)
    a = normalize(healthy, "hn", reference=reference).factor
    b = normalize(healthy, "hn", reference=tmp_path / "ref").factor
    assert a == pytest.approx(b, rel=1e-5)


def test_healthy_subject_gets_the_same_factor_from_all_data_driven_methods(healthy, reference):
    ps = normalize(healthy, "ps").factor
    for method in ("hn", "ips", "ihn"):
        factor = normalize(healthy, method, reference=reference).factor
        assert factor == pytest.approx(ps, rel=0.02), method


def test_lesion_bias_ranking(patient, healthy, reference):
    """The paper's central result: PS is dragged down by a large lesion,
    HN much less, and the iterative versions improve on both."""
    truth = {m: normalize(healthy, m, reference=reference).factor for m in ("ps", "hn")}
    bias = {}
    for method in ("ps", "ips", "hn", "ihn"):
        factor = normalize(patient, method, reference=reference).factor
        bias[method] = abs(factor / truth["ps" if method.endswith("ps") else "hn"] - 1.0)
    assert bias["ps"] > 0.02  # the lesion really does bias plain PS
    assert bias["ips"] < bias["ps"]
    assert bias["hn"] < bias["ps"]
    assert bias["ihn"] <= bias["hn"] + 0.002
    assert bias["hn"] < 0.01 and bias["ihn"] < 0.01


def test_iterative_mask_excludes_the_lesion(patient, lesion, reference):
    res = normalize(patient, "ihn", reference=reference)
    removed = reference.mask & ~res.mask
    core = (lesion > 0) & reference.mask
    assert 0 < res.info["excluded_fraction"] < 0.5
    assert removed[core].mean() > 0.5  # most of the lesion is gone
    assert removed[reference.mask & ~core].mean() < 0.1  # little else is
    assert res.info["iterations"] == 1


def test_reference_on_another_grid(patient, reference):
    """Images on a different grid than the reference still work."""
    affine = np.array([[-2.0, 0, 0, 90], [0, 2, 0, -126], [0, 0, 2, -72], [0, 0, 0, 1]])
    from nibabel.processing import resample_from_to

    moved = resample_from_to(patient, ((91, 109, 91), affine), order=1)
    a = normalize(patient, "hn", reference=reference).factor
    b = normalize(moved, "hn", reference=reference).factor
    assert b == pytest.approx(a, rel=0.01)


def test_image_outside_mni_space_is_rejected():
    far = nib.Nifti1Image(
        np.ones((20, 20, 20), np.float32), np.diag([2.0, 2, 2, 1]) + np.eye(4) * 0 + [[0, 0, 0, 900]] * 4
    )
    with pytest.raises(ValueError, match="MNI space"):
        normalize(far, "cerebellum")
