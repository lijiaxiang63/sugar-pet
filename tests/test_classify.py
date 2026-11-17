import json
import struct
import zlib
from pathlib import Path

import nibabel as nib
import numpy as np
import pytest
from conftest import make_pet

from sugar_pet import Reference, classify
from sugar_pet.classify import PATTERNS, REGIONS, _conclude, region_masks
from sugar_pet.cli import main, report_stem
from sugar_pet.masks import builtin_mask
from sugar_pet.report import _texts, montage, number, png

# What each made-up patient has lost: {(region, side): share of the full decrease}.
CASES = {
    "msa-p": {("putamen", "both"): 1, ("pons", "both"): 0.8, ("cerebellum", "both"): 0.5},
    "msa-c": {("cerebellum", "both"): 1, ("vermis", "both"): 1, ("pons", "both"): 0.8},
    "psp": {
        ("midbrain", "both"): 1,
        ("medial-frontal", "both"): 1,
        ("caudate", "both"): 0.7,
        ("thalamus", "both"): 0.7,
    },
    "cbd": {("sensorimotor", "right"): 1, ("parietal", "right"): 1, ("thalamus", "right"): 0.7},
    "dlb": {("occipital", "both"): 1, ("parietal", "both"): 0.7, ("precuneus", "both"): 0.7},
}


def lesion(parts):
    masks = region_masks()
    out = np.zeros(masks["pons", "both"].shape, np.float32)
    for key, share in parts.items():
        out = np.maximum(out, masks[key] * np.float32(share))
    return out


@pytest.fixture(scope="module")
def reference(controls):
    return Reference.build(controls, stats_for=["white-matter"])


def test_regions():
    masks = region_masks()
    for name, paired in REGIONS:
        assert masks[name, "both"].sum() > 100, name
        if paired:
            left, right = masks[name, "left"], masks[name, "right"]
            assert not (left & right).any() and abs(int(left.sum()) - int(right.sum())) < 0.1 * left.sum(), name
            x = np.nonzero(left)[0].mean(), np.nonzero(right)[0].mean()
            assert x[0] < x[1], name  # the x index grows towards the right
    assert not (masks["midbrain", "both"] & masks["pons", "both"]).any()
    assert np.nonzero(masks["midbrain", "both"])[2].min() > np.nonzero(masks["pons", "both"])[2].max()
    assert (masks["posterior-putamen", "both"] & ~masks["putamen", "both"]).sum() == 0
    assert not (masks["medial-frontal", "both"] & masks["lateral-frontal", "both"]).any()


@pytest.mark.parametrize("name", list(CASES))
def test_patterns_are_recognised(name, reference):
    patient = make_pet(np.random.default_rng(7), scale=5000.0, lesion=lesion(CASES[name]), drop=0.25)
    res = classify(patient, reference=reference)
    found = res.conclusion
    assert found["category"] == "pattern" and found["pattern"] == name, res.result["patterns"]
    assert found["side"] == ("right" if name == "cbd" else None)
    scores = {p["id"]: p["score"] for p in res.result["patterns"]}
    assert list(scores) == list(PATTERNS) and scores["pd"] < 0.1
    assert res.result["checks"] == ["ps", "white-matter"] and res.result["score"] == "z"
    assert res.zmap.shape == patient.shape


def test_healthy_and_pd(reference, healthy):
    res = classify(healthy, reference=reference)
    assert res.conclusion == {
        "category": "none",
        "pattern": None,
        "alternative": None,
        "confidence": None,
        "side": None,
    }
    assert all(p["score"] < 0.3 for p in res.result["patterns"])
    assert {n["key"] for n in res.result["notes"]} >= {"few-controls", "unadjusted"}

    # A relative increase where PD shows it; without a decrease anywhere.
    parts = {(r, "both"): 1 for r in ("putamen", "pallidum", "thalamus", "pons", "cerebellum", "sensorimotor")}
    patient = make_pet(np.random.default_rng(7), scale=5000.0, lesion=lesion(parts), drop=-0.2)
    res = classify(patient, reference=reference)
    assert res.conclusion["category"] == "pd" and res.conclusion["pattern"] == "pd"


def test_increase_must_survive_every_normalisation(reference, lesion, patient):
    """A large cortical lesion makes the rest look increased under global scaling only."""
    res = classify(patient, reference=reference, checks=["ps"])
    by_id = {r["id"]: r for r in res.result["regions"]}
    occipital = by_id["occipital"]["both"]
    assert occipital["checks"]["ps"] > occipital["z"]
    assert by_id["sensorimotor"]["left"]["status"] == "hypo" and by_id["sensorimotor"]["left"]["robust"]
    pd = next(p for p in res.result["patterns"] if p["id"] == "pd")
    for feature in pd["features"][:-1]:  # never more than the main normalisation shows
        assert feature["z"] <= by_id[feature["region"]]["both"]["z"] + 1e-4


def test_confidence_goes_by_the_findings_that_hold(reference):
    """A putamen that is low only against the gray matter: the pattern is named, with low confidence."""
    clear = classify(
        make_pet(np.random.default_rng(7), scale=5000.0, lesion=lesion(CASES["msa-p"]), drop=0.25),
        reference=reference,
    )
    top = clear.result["patterns"][0]
    assert top["robust"] == top["score"] and clear.conclusion["confidence"] == "high"
    assert "lowered" not in {n["key"] for n in clear.result["notes"]}

    # The white matter falls with the putamen, so the putamen is normal against the white matter.
    white = (builtin_mask("white-matter").get_fdata() > 0).astype(np.float32)
    both = np.maximum(lesion(CASES["msa-p"]), white)
    res = classify(make_pet(np.random.default_rng(7), scale=5000.0, lesion=both, drop=0.25), reference=reference)
    top = res.result["patterns"][0]
    putamen = next(r for r in res.result["regions"] if r["id"] == "putamen")["both"]
    assert putamen["status"] == "hypo" and not putamen["robust"] and abs(putamen["checks"]["white-matter"]) < 1
    assert top["id"] == "msa-p" and top["score"] >= 0.5 > top["robust"]
    assert res.conclusion["category"] == "pattern" and res.conclusion["pattern"] == "msa-p"
    assert res.conclusion["confidence"] == "low"
    assert res.result["notes"][0] == {"key": "lowered", "values": [round(100 * top["robust"])]}
    assert "置信度已下调" in res.report("zh") and "confidence was lowered" in res.report("en")


def test_region_outside_the_image(reference, healthy):
    data = healthy.get_fdata().copy()
    data[:, :, :24] = 0  # the field of view ends above the cerebellum
    res = classify(nib.Nifti1Image(data.astype(np.float32), healthy.affine), reference=reference, checks=[])
    by_id = {r["id"]: r for r in res.result["regions"]}
    assert by_id["cerebellum"]["left"]["z"] is None and by_id["cerebellum"]["left"]["status"] == "none"
    assert by_id["cerebellum"]["difference"] is None and by_id["putamen"]["both"]["z"] is not None
    note = next(n for n in res.result["notes"] if n["key"] == "not-covered")
    assert ["cerebellum", "left"] in note["regions"] and ["pons", "both"] in note["regions"]
    assert "图像未覆盖" in res.report("zh")


def test_conclusion():
    def read(robust=None, **scores):
        patterns = [{"id": name, "score": scores.get(name.replace("-", "_"), 0.0)} for name in PATTERNS]
        for found in patterns:  # ``robust``: what is left of the first pattern without the unstable findings
            found["robust"] = found["score"] if robust is None or found["score"] == 0.0 else robust
        return _conclude(patterns, "left")

    assert read(psp=0.8, cbd=0.4) == {
        "category": "pattern", "pattern": "psp", "alternative": "cbd", "confidence": "high", "side": "left",
    }  # fmt: skip
    assert read(psp=0.8, cbd=0.6)["confidence"] == "moderate"
    assert read(psp=0.6, dlb=0.5)["category"] == "indeterminate" and read(psp=0.6, dlb=0.5)["alternative"] == "dlb"
    # The two forms of MSA are one disease: the other form is not a rival.
    both = read(msa_p=0.9, msa_c=0.9)
    assert both["category"] == "pattern" and both["pattern"] == "msa-p" and both["alternative"] is None
    assert read(dlb=0.4, pd=0.9)["category"] == "possible" and read(dlb=0.4)["side"] is None
    assert read(dlb=0.2, pd=0.6) == {
        "category": "pd", "pattern": "pd", "alternative": None, "confidence": "moderate", "side": None,
    }  # fmt: skip
    assert read(pd=0.35)["confidence"] == "low" and read(pd=0.2)["category"] == "none"
    # A pattern that rests on findings which change with the normalisation is assigned with less confidence.
    assert read(psp=0.8, robust=0.8)["confidence"] == "high" and read(psp=0.8, robust=0.6)["confidence"] == "moderate"
    assert read(psp=0.8, robust=0.2) == {
        "category": "pattern", "pattern": "psp", "alternative": None, "confidence": "low", "side": None,
    }  # fmt: skip


def test_report(reference):
    patient = make_pet(np.random.default_rng(7), scale=5000.0, lesion=lesion(CASES["cbd"]), drop=0.25)
    res = classify(patient, reference=reference)
    for lang, title, name in (("en", "Impression", "Corticobasal"), ("zh", "印象", "皮质基底节")):
        page = res.report(lang)
        assert page.startswith("<!doctype html>") and title in page and name in page
        assert "{0}" not in page and "{1}" not in page
        assert "data:image/png;base64," in page
    assert "right peri-central (sensorimotor) cortex" in res.report("en")
    with pytest.raises(ValueError, match="en and zh"):
        res.report("fr")

    # The figure: blue where the lesion is, which is on the left of the picture (the patient's right).
    picture, levels = montage(res.image, res.zmap)
    assert picture.shape == (2 * 230 + 6, 5 * 194 + 4 * 6, 3) and levels[0] == -37.5 and levels[-1] == 58.5
    tile = picture[236:, 4 * 200 :].astype(int)  # the highest slice
    blue = (tile[:, :, 2] == 255) & (tile[:, :, 0] == 0)
    assert blue[:, :97].sum() > 5 * max(blue[:, 97:].sum(), 1)

    encoded = png(picture)
    assert encoded[:8] == b"\x89PNG\r\n\x1a\n" and struct.unpack(">II", encoded[16:24]) == (
        picture.shape[1],
        picture.shape[0],
    )
    length = struct.unpack(">I", encoded[33:37])[0]
    assert encoded[37:41] == b"IDAT" and len(zlib.decompress(encoded[41 : 41 + length])) == picture.shape[0] * (
        1 + 3 * picture.shape[1]
    )


def test_texts_and_numbers():
    for key, entry in _texts().items():
        assert set(entry) == {"en", "zh"} and all(entry.values()), key
    for name, _ in REGIONS:
        assert "region-" + name in _texts()
    for name in PATTERNS:
        assert {"name-" + name, "expect-" + name, "caveat-" + name} <= set(_texts())
    assert [number(v) for v in (-2.25, -0.04, 0.04, None, 3)] == ["−2.2", "0.0", "0.0", "–", "3.0"]
    assert number(-37.5, 0) == "−38" and number(2.5, 0) == "2"
    # The desktop app carries a copy of the texts and the style sheet.
    here = Path(__file__).parent.parent
    for name in ("classify_text.json", "report.css"):
        copy = here / "app" / "core" / "data" / name
        if copy.exists():
            assert copy.read_bytes() == (here / "src" / "sugar_pet" / "data" / name).read_bytes(), name


def test_cli(tmp_path, controls, capsys):
    rows = ["participant_id\tage\tsex"]
    paths = []
    for i, img in enumerate(controls[:10]):
        paths.append(str(tmp_path / f"sub-hc{i}_pet.nii.gz"))
        nib.save(img, paths[-1])
        rows.append(f"sub-hc{i}\t{55 + 2 * i}\t{'FM'[i % 2]}")
    patient = make_pet(np.random.default_rng(7), scale=5000.0, lesion=lesion(CASES["msa-p"]), drop=0.25)
    patient_path = str(tmp_path / "sub-01_pet.nii.gz")
    nib.save(patient, patient_path)
    rows.append("sub-01\t90\tF")
    table = tmp_path / "participants.tsv"
    table.write_text("\n".join(rows) + "\n")
    assert (
        report_stem(patient_path) == "sub-01"
        and report_stem("wFDG.nii") == "wFDG"
        and report_stem("_pet.nii") == "_pet"
    )

    ref = str(tmp_path / "ref")
    assert main(["build-reference", *paths, "-o", ref, "--covariates", str(table)]) == 0
    out = tmp_path / "out"
    capsys.readouterr()
    assert main(["classify", patient_path, "-r", ref, "-o", str(out), "--covariates", str(table), "--lang", "zh"]) == 0
    printed = capsys.readouterr()
    assert printed.out.strip() == "sub-01_pet.nii.gz\tpattern\tmsa-p\thigh"
    assert "outside the range of the controls" in printed.err
    lines = (out / "classification.tsv").read_text().strip().splitlines()
    assert lines[0].split("\t") == [
        "image", "category", "pattern", "alternative", "confidence", "side", *PATTERNS, "report",
    ]  # fmt: skip
    cells = lines[1].split("\t")
    assert (
        cells[1:6] == ["pattern", "msa-p", "", "high", ""] and int(cells[6]) > 70 and cells[-1] == "sub-01_report.html"
    )
    result = json.loads((out / "sub-01_classification.json").read_text())
    assert result["score"] == "w" and result["covariates"] == {"age": "90", "sex": "F"} and result["checks"] == ["ps"]
    assert len(result["warnings"]) == 1  # said once, not once per normalisation
    assert (out / "sub-01_desc-hn_wmap.nii.gz").exists()
    page = (out / "sub-01_report.html").read_text(encoding="utf-8")
    assert "多系统萎缩帕金森型" in page and "age 90" in page and "W 值" in page

    # Plain z-scores, no other normalisation; a check the reference cannot serve is refused.
    assert main(["classify", patient_path, "-r", ref, "-o", str(out), "--check", "none"]) == 0
    result = json.loads((out / "sub-01_classification.json").read_text())
    assert result["checks"] == [] and result["score"] == "z" and (out / "sub-01_desc-hn_zmap.nii.gz").exists()
    assert "no other normalisation" in (out / "sub-01_report.html").read_text(encoding="utf-8")
    assert main(["classify", patient_path, "-r", ref, "-o", str(out), "--check", "pons"]) == 2
    assert "--stats-for pons" in capsys.readouterr().err
    assert main(["classify", patient_path, str(tmp_path / "missing.nii.gz"), "-r", ref, "-o", str(out)]) == 1
