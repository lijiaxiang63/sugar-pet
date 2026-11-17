import nibabel as nib
import numpy as np
import pytest
from conftest import make_pet

from sugar_pet import MASKS, METHODS, Reference, builtin_mask, labels_mask, normalize, reference_cluster, zmap
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


def test_median_and_target(healthy):
    data = healthy.get_fdata()
    mask = (builtin_mask("pons").get_fdata() > 0) & (data > 0)
    res = normalize(healthy, "pons", statistic="median", target=50)
    assert res.factor == pytest.approx(np.median(data[mask]), rel=1e-5)
    assert np.median(res.image.get_fdata()[mask]) == pytest.approx(50.0, rel=1e-4)
    assert res.info["statistic"] == "median" and res.info["target"] == 50
    with pytest.raises(ValueError, match="statistic"):
        normalize(healthy, "pons", statistic="mode")
    with pytest.raises(ValueError, match="target"):
        normalize(healthy, "pons", target=0)


def test_paper_regions_are_still_available_as_masks(healthy):
    """Brainstem (RRBS) and temporal lobe (RRTL) are masks now, not methods."""
    for name in ("brainstem", "temporal-left", "temporal-right"):
        assert name in MASKS and name not in METHODS
        with pytest.raises(ValueError, match="built-in mask"):
            normalize(healthy, name)
        res = normalize(healthy, "region", mask=name)
        assert res.info["mask"] == name and res.factor > 0


def test_occipital_matches_its_atlas_labels(healthy):
    a = normalize(healthy, "occipital").factor
    b = normalize(healthy, "region", mask=labels_mask([34, 85, 43, 94, 6, 57, 12, 63])).factor
    assert a == pytest.approx(b)
    with pytest.raises(ValueError, match="label numbers"):
        labels_mask([34, 999])


@pytest.fixture(scope="module")
def reference_with_stats(controls):
    return Reference.build(controls, fwhm=0, stats_for=["cerebellum", "spm-global"])


def test_control_stats_match_a_direct_computation(controls, reference_with_stats):
    ref = reference_with_stats
    scaled = np.stack([normalize(c, "cerebellum").image.get_fdata() for c in controls])
    mean, std = ref.control_stats("cerebellum")
    inside = ref.mask
    assert np.allclose(mean[inside], scaled.mean(0)[inside], rtol=1e-4)
    assert np.allclose(std[inside], scaled.std(0, ddof=1)[inside], rtol=1e-2, atol=1e-5)
    assert ref.control_stats("ips")[0] is ref.template
    with pytest.raises(ValueError, match="stats-for pons"):
        ref.control_stats("pons")
    with pytest.raises(ValueError, match="cannot store statistics"):
        Reference.build(controls[:3], fwhm=0, stats_for=["hn"])


def test_stats_survive_a_roundtrip(reference_with_stats, tmp_path):
    loaded = Reference.load(reference_with_stats.save(tmp_path / "ref"))
    assert list(loaded.stats) == ["cerebellum", "spm-global"]
    for a, b in zip(loaded.control_stats("spm-global"), reference_with_stats.control_stats("spm-global")):
        assert np.allclose(a, b)
    assert len(loaded.meta["control_factors"]["cerebellum"]) == 16


@pytest.mark.parametrize("method", ["hn", "ips", "cerebellum"])
def test_zmap_finds_the_lesion(method, patient, healthy, lesion, reference_with_stats):
    ref = reference_with_stats
    core = (lesion > 0) & ref.mask
    rest = ref.mask & ~(lesion > 0)
    z = zmap(patient, method, reference=ref).get_fdata()
    assert np.median(z[core]) < -4  # a 40 % drop is far outside the controls
    assert abs(np.median(z[rest])) < 1
    z0 = zmap(healthy, method, reference=ref).get_fdata()
    assert abs(np.median(z0[ref.mask])) < 0.5 and z0[ref.mask].std() < 2
    assert z[~(z != 0)].size > 0 and not np.isnan(z).any()


def test_zmap_needs_stored_statistics(healthy, reference):
    with pytest.raises(ValueError, match="stats-for pons"):
        zmap(healthy, "pons", reference=reference)
    with pytest.raises(ValueError, match="not available"):
        zmap(healthy, "region", reference=reference)


def test_reference_cluster_avoids_the_lesion(lesion, healthy, reference):
    rng = np.random.default_rng(7)
    group = [make_pet(rng, scale=rng.uniform(3000, 9000), lesion=lesion, drop=0.4) for _ in range(12)]
    cluster = reference_cluster(group, reference, p_threshold=0.01, min_cluster_ml=0.5)
    mask = cluster.get_fdata() > 0
    assert mask.sum() > 1000
    assert not (mask & ~reference.mask).any()
    assert (mask & (lesion > 0.5)).sum() / mask.sum() < 0.01
    # As a reference region it is not dragged down by the lesion the way PS is.
    sick = make_pet(np.random.default_rng(42), scale=TRUE_SCALE, lesion=lesion, drop=0.4)
    bias = {
        m: abs(normalize(sick, m, **kw).factor / normalize(healthy, m, **kw).factor - 1)
        for m, kw in (("ps", {}), ("region", {"mask": cluster}))
    }
    assert bias["region"] < 0.01 < bias["ps"]
    with pytest.raises(ValueError, match="at least 2"):
        reference_cluster(group[:1], reference)
    with pytest.raises(ValueError, match="relatively preserved"):
        reference_cluster(group, reference, p_threshold=1e-12, min_cluster_ml=1e6)


@pytest.fixture(scope="module")
def ageing():
    """Controls whose occipital uptake falls by 0.8 % per year and is 10 % higher in sensorimotor cortex of men."""
    rng = np.random.default_rng(11)
    occipital = builtin_mask("occipital").get_fdata()
    motor = builtin_mask("sensorimotor").get_fdata()

    def person(age, sex, seed=None):
        effect = 0.008 * (age - 55) * occipital - (0.10 if sex == "M" else 0.0) * motor
        return make_pet(rng if seed is None else np.random.default_rng(seed), scale=4000.0, lesion=effect, drop=1.0)

    ages = [float(a) for a in np.linspace(30, 80, 24)]
    sexes = ["F", "M"] * 12
    images = [person(a, s) for a, s in zip(ages, sexes)]
    ref = Reference.build(images, fwhm=0, covariates={"age": ages, "sex": sexes}, stats_for=["cerebellum"])
    return ref, images, ages, sexes, person, occipital > 0, motor > 0


def test_covariate_model_is_ordinary_least_squares(ageing):
    ref, images, ages, sexes, *_ = ageing
    assert ref.covariates == ["age", "sex"]
    y = np.stack([normalize(img, "cerebellum").image.get_fdata()[ref.mask] for img in images])
    design = np.column_stack([np.ones(24), np.array(ages) - np.mean(ages), [s == "M" for s in sexes]])
    expected, *_ = np.linalg.lstsq(design, y, rcond=None)
    beta, sd = ref.control_model("cerebellum")
    assert np.allclose(beta[:, ref.mask], expected, atol=2e-4)
    resid = y - design @ expected
    assert np.allclose(sd[ref.mask], np.sqrt((resid**2).sum(0) / (24 - 3)), rtol=2e-2, atol=1e-5)
    assert ref.control_model("ihn")[0] is ref.models["hn"][0]


@pytest.mark.parametrize("method", ["hn", "cerebellum"])
def test_w_scores_remove_age_and_sex_effects(ageing, method):
    ref, _, _, _, person, occipital, motor = ageing
    old_man = person(78, "M", seed=5)  # healthy for his age and sex
    z = zmap(old_man, method, reference=ref).get_fdata()
    w = zmap(old_man, method, reference=ref, covariates={"age": 78, "sex": "M"}).get_fdata()
    occipital, motor = occipital & ref.mask, motor & ref.mask
    assert np.median(z[occipital]) < -1.0  # looks hypometabolic next to mostly younger controls
    assert abs(np.median(w[occipital])) < 0.6  # ... but is normal for 78
    assert abs(np.median(w[motor])) < abs(np.median(z[motor]))
    rest = ref.mask & ~occipital & ~motor
    assert abs(np.median(w[rest])) < 0.5 and w[rest].std() < 2


def test_w_scores_survive_a_roundtrip_and_another_grid(ageing, tmp_path):
    from nibabel.processing import resample_from_to

    ref, _, _, _, person, *_ = ageing
    loaded = Reference.load(ref.save(tmp_path / "ref"))
    assert set(loaded.models) == {"ps", "hn", "cerebellum"} and loaded.covariates == ["age", "sex"]
    img = person(70, "F", seed=9)
    who = {"age": 70, "sex": "F"}
    a = zmap(img, "hn", reference=ref, covariates=who).get_fdata()
    b = zmap(img, "hn", reference=loaded, covariates=who).get_fdata()
    assert np.allclose(a, b, atol=1e-3)
    affine = np.array([[-2.0, 0, 0, 90], [0, 2, 0, -126], [0, 0, 2, -72], [0, 0, 0, 1]])
    moved = resample_from_to(img, ((91, 109, 91), affine), order=1)
    c = zmap(moved, "hn", reference=loaded, covariates=who)
    back = resample_from_to(c, img, order=1).get_fdata()
    inside = ref.mask & (back != 0)
    assert np.corrcoef(a[inside], back[inside])[0, 1] > 0.8


def test_covariate_errors(ageing, controls, healthy, reference):
    ref, images, ages, *_ = ageing
    with pytest.raises(ValueError, match="built without covariates"):
        zmap(healthy, "hn", reference=reference, covariates={"age": 60, "sex": "F"})
    with pytest.raises(ValueError, match="needed but missing"):
        zmap(healthy, "hn", reference=ref, covariates={"age": 60})
    with pytest.raises(ValueError, match="one of"):
        zmap(healthy, "hn", reference=ref, covariates={"age": 60, "sex": "female"})
    with pytest.raises(ValueError, match="stats-for pons"):
        zmap(healthy, "pons", reference=ref, covariates={"age": 60, "sex": "F"})
    with pytest.warns(UserWarning, match="extrapolation"):
        zmap(healthy, "hn", reference=ref, covariates={"age": 95, "sex": "F"})
    for bad, message in (
        ({"age": ages[:5]}, "5 covariate values"),
        ({"sex": ["F"] * 24}, "exactly two values"),
        ({"age": [60.0] * 24}, "same for all controls"),
        ({"age": ages, "site": ["a", "b", "c"] * 8}, "exactly two values"),
        ({"age": ages[:-1] + ["n/a"]}, "missing"),
        ({"age": ages, "years": ages}, "redundant"),
    ):
        with pytest.raises(ValueError, match=message):
            Reference.build(images, fwhm=0, covariates=bad)


def test_whole_brain_is_the_parenchyma_mean(healthy):
    data = healthy.get_fdata()
    mask = (builtin_mask("parenchyma").get_fdata() > 0) & (data > 0)
    assert normalize(healthy, "whole-brain").factor == pytest.approx(data[mask].mean(), rel=1e-5)
    # ... and proportional scaling is the same thing on the gray-matter mask
    assert normalize(healthy, "ps").factor == normalize(healthy, "region", mask="gray-matter").factor
    assert normalize(healthy, "ps", mask="parenchyma").factor == normalize(healthy, "whole-brain").factor
